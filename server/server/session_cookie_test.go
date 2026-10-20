package server

import (
	"net/http"
	"testing"
	"time"
)

// findSessionCookie 从响应里取出会话 Cookie。
func findSessionCookie(t *testing.T, w *http.Response) *http.Cookie {
	t.Helper()
	for _, ck := range w.Cookies() {
		if ck.Name == "token" {
			return ck
		}
	}
	return nil
}

// TestSessionCookieTTLFollowsRemember 锁住 2026-10-18 审查发现的问题：
// 会话 Cookie 的寿命必须跟随本次登录的 remember，不能恒按「记住我」取。
//
// 早先 sessionCookieMaxAge 固定用 loginTokenTTL(db, true)：用户勾「不记住」
// 拿到 24 小时 JWT + 30 天 Cookie，两者自相矛盾，关掉浏览器再开仍处于登录态。
func TestSessionCookieTTLFollowsRemember(t *testing.T) {
	r, _ := setupFnOSRouter(t)
	base := "/app/techfunway-bill/api/auth"

	// 先注册一个账号（注册接口本身固定按 remembered TTL 种 Cookie）。
	w := doFnOSRequest(r, base+"/register", "", "", map[string]string{
		"username": "ttl-user", "password": "secret123",
	}, true)
	if w.Code != http.StatusOK {
		t.Fatalf("注册失败: %d %s", w.Code, w.Body.String())
	}

	// 不记住：Cookie 寿命应为 24 小时（sessionTokenTTL）。
	w = doFnOSRequest(r, base+"/login", "", "", map[string]interface{}{
		"username": "ttl-user", "password": "secret123", "remember": false,
	}, true)
	if w.Code != http.StatusOK {
		t.Fatalf("登录失败: %d %s", w.Code, w.Body.String())
	}
	ck := findSessionCookie(t, w.Result())
	if ck == nil {
		t.Fatal("remember=false 时未种会话 Cookie")
	}
	wantSession := int((24 * time.Hour).Seconds())
	if ck.MaxAge != wantSession {
		t.Fatalf("remember=false 的 Cookie MaxAge = %d，期望 %d（24 小时）", ck.MaxAge, wantSession)
	}

	// 记住我：Cookie 寿命应跟随 login_expiry_days（默认 30 天）。
	w = doFnOSRequest(r, base+"/login", "", "", map[string]interface{}{
		"username": "ttl-user", "password": "secret123", "remember": true,
	}, true)
	if w.Code != http.StatusOK {
		t.Fatalf("登录失败: %d %s", w.Code, w.Body.String())
	}
	ck = findSessionCookie(t, w.Result())
	if ck == nil {
		t.Fatal("remember=true 时未种会话 Cookie")
	}
	wantRemembered := int((30 * 24 * time.Hour).Seconds())
	if ck.MaxAge != wantRemembered {
		t.Fatalf("remember=true 的 Cookie MaxAge = %d，期望 %d（30 天）", ck.MaxAge, wantRemembered)
	}
	if ck.MaxAge == wantSession {
		t.Fatal("记住我与不记住的 Cookie 寿命不应相同")
	}
}
