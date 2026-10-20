// 网关域的会话 Cookie 通道（2026-10-16 自 smallgo 基线回灌）。
//
// 飞牛统一网关的接入层会把请求里的 Authorization 当成**它自己的**会话 token，
// 应用自己的 `Bearer <jwt>` 它认不出来，于是直接返回 200 纯文本
// "invalid token"，请求根本到不了应用。所以 SPA 留在网关域名下时，应用自己的
// JWT 没有传输通道，登录态必须另找一条路：登录成功时把令牌同时种成 HttpOnly
// 会话 Cookie（middleware.authenticate 里 Cookie 与 Authorization 同级）。
//
// 硬规矩（父仓库记忆 fnos-login-explicit-only）：账密登录/注册绝不读取网关注入
// 身份，这里只是给应用自己的会话补传输通道，不做任何隐式认人。
package user

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"gorm.io/gorm"
)

const sessionCookieName = "token"

// IsFnOSGatewayRequest 报告请求是否来自飞牛网关的 Unix Socket（而非公网 TCP
// 监听）。同样的 Header 名 TCP 客户端也能发，所以只能靠连接来源判真伪——
// 这也是信任 X-Trim-* 身份头的唯一依据。
func IsFnOSGatewayRequest(r *http.Request) bool {
	return r.Context().Value(fnOSGatewayContextKey{}) == true
}

// sessionCookieMaxAge 跟随本次登录签发的令牌 TTL。
//
// 必须把 remember 传进来：早先这里恒按「记住我」取 TTL（默认 30 天），用户
// 勾了「不记住」拿到的是 24 小时 JWT + 30 天 Cookie，两者自相矛盾——关掉
// 浏览器再打开仍处于登录态，与 setToken(token, false) 的 sessionStorage 策略
// 相反（2026-10-18 审查发现）。db 为 nil 时按常量兜底。
func sessionCookieMaxAge(db *gorm.DB, remember bool) int {
	if db == nil {
		if remember {
			return defaultRememberLoginDays * 24 * 3600
		}
		return int(sessionTokenTTL.Seconds())
	}
	return int(loginTokenTTL(db, remember).Seconds())
}

func setSessionCookie(c *gin.Context, prefix, token string, maxAge int) {
	if prefix == "" || token == "" {
		return
	}
	// Path=网关前缀：宿主机内与兄弟应用互不干扰（Cookie 不分端口，同主机名共享）。
	// Secure=false：网关在局域网走 http。HttpOnly=true：前端脚本读不到。
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, token, maxAge, prefix, "", false, true)
}

func clearSessionCookie(c *gin.Context, prefix string) {
	if prefix == "" {
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, prefix, "", false, true)
}

// issueSessionCookie 在网关请求上为刚签发的令牌种会话 Cookie；
// 直连端口请求维持纯 JWT（Authorization 头），不种。
//
// remember 决定 Cookie 寿命，与签发 JWT 时用的是同一个 remember，避免
// 「JWT 24 小时、Cookie 30 天」这种两套寿命并存。
func issueSessionCookie(c *gin.Context, db *gorm.DB, prefix string, result map[string]interface{}, remember bool) {
	if !IsFnOSGatewayRequest(c.Request) {
		return
	}
	token, _ := result["token"].(string)
	if token == "" {
		return
	}
	setSessionCookie(c, prefix, token, sessionCookieMaxAge(db, remember))
}
