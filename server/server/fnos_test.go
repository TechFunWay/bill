package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"smallgo/server/config"
	"smallgo/server/database"
	"smallgo/server/sysconfig"
	"smallgo/server/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func setupFnOSRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := database.InitDB(filepath.Join(t.TempDir(), "fnos.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := sysconfig.InitDefaultConfigs(db); err != nil {
		t.Fatalf("init configs: %v", err)
	}
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil {
		t.Fatalf("jwt secret: %v", err)
	}
	cfg := config.Config{CORSOrigin: "*", FnOSApp: true, GatewayPrefix: "/app/techfunway-bill"}
	return NewRouter(cfg, db, secret), db
}

func doFnOSRequest(r http.Handler, route string, uid string, username string, body interface{}, trusted bool) *httptest.ResponseRecorder {
	var payload bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&payload).Encode(body)
	}
	req := httptest.NewRequest(http.MethodPost, route, &payload)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Trim-Userid", uid)
	req.Header.Set("X-Trim-Username", username)
	if trusted {
		req = req.WithContext(user.MarkFnOSGateway(req.Context()))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestFnOSLoginBindingAndTrustedSocket(t *testing.T) {
	r, db := setupFnOSRouter(t)
	base := "/app/techfunway-bill/api/auth/fnos"

	identityReq := httptest.NewRequest(http.MethodGet, base+"/identity", nil)
	identityReq.Header.Set("X-Trim-Userid", "1000")
	identityReq.Header.Set("X-Trim-Username", "nas-admin")
	identityReq = identityReq.WithContext(user.MarkFnOSGateway(identityReq.Context()))
	identityRes := httptest.NewRecorder()
	r.ServeHTTP(identityRes, identityReq)
	if identityRes.Code != http.StatusOK || decode(t, identityRes)["data"].(map[string]interface{})["fnos_username"] != "nas-admin" {
		t.Fatalf("identity: status %d body %s", identityRes.Code, identityRes.Body.String())
	}

	if got := doFnOSRequest(r, base+"/login", "1000", "nas-admin", nil, false).Code; got != http.StatusUnauthorized {
		t.Fatalf("untrusted headers status = %d, want 401", got)
	}

	w := doFnOSRequest(r, base+"/login", "1000", "nas-admin", nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("unbound login: status %d body %s", w.Code, w.Body.String())
	}
	unbound := decode(t, w)["data"].(map[string]interface{})
	if unbound["binding_required"] != true || unbound["has_accounts"] != false || unbound["suggested_mode"] != "register" || unbound["suggested_username"] != "" {
		t.Fatalf("unexpected first binding guidance: %#v", unbound)
	}

	w = doFnOSRequest(r, base+"/bind", "1000", "nas-admin", map[string]string{
		"mode": "register", "username": "admin", "password": "secret123",
	}, true)
	if w.Code != http.StatusOK {
		t.Fatalf("bind: status %d body %s", w.Code, w.Body.String())
	}

	w = doFnOSRequest(r, base+"/login", "1000", "renamed-admin", map[string]interface{}{"remember": false}, true)
	if w.Code != http.StatusOK {
		t.Fatalf("bound login: status %d body %s", w.Code, w.Body.String())
	}
	data := decode(t, w)["data"].(map[string]interface{})
	if data["token"] == "" {
		t.Fatal("bound login returned no token")
	}
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil {
		t.Fatalf("jwt secret: %v", err)
	}
	if got := tokenDuration(t, data["token"].(string), secret); got != 24*time.Hour {
		t.Fatalf("fnOS session token TTL = %v, want 24h", got)
	}

	// Once accounts exist, an unbound NAS user must be guided to verify and
	// bind an existing account instead of accidentally creating a data silo.
	w = doFnOSRequest(r, "/app/techfunway-bill/api/auth/register", "", "", map[string]string{
		"username": "same-name", "password": "secret123",
	}, false)
	if w.Code != http.StatusOK {
		t.Fatalf("create matching application account: status %d body %s", w.Code, w.Body.String())
	}
	w = doFnOSRequest(r, base+"/login", "2000", "same-name", nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("unbound existing-account login: status %d body %s", w.Code, w.Body.String())
	}
	guidance := decode(t, w)["data"].(map[string]interface{})
	if guidance["binding_required"] != true || guidance["has_accounts"] != true || guidance["suggested_mode"] != "bind" || guidance["suggested_username"] != "same-name" {
		t.Fatalf("unexpected existing-account guidance: %#v", guidance)
	}
	w = doFnOSRequest(r, base+"/bind", "2000", "same-name", map[string]string{
		"mode": "bind", "username": "same-name", "password": "secret123",
	}, true)
	if w.Code != http.StatusOK || decode(t, w)["data"].(map[string]interface{})["token"] == "" {
		t.Fatalf("existing account bind: status %d body %s", w.Code, w.Body.String())
	}
}

func TestFnOSRoutesDisabledForNormalDeployments(t *testing.T) {
	r, _, _ := setupTestRouter(t)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/fnos/identity", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("normal deployment exposed fnOS identity route: %d", w.Code)
	}
	w = doFnOSRequest(r, "/api/auth/fnos/login", "1000", "nas-admin", nil, true)
	if w.Code != http.StatusNotFound {
		t.Fatalf("normal deployment exposed fnOS route: %d", w.Code)
	}
}

// TestGatewayLoginSetsSessionCookie：网关域上飞牛登录必须种 HttpOnly 会话
// Cookie——飞牛网关会拦掉应用自己的 Authorization 头，SPA 留在网关域名下时
// 只能靠 Cookie 维持登录态（否则刷新即掉登录态）。
// 注意 bill 的飞牛登录未绑定时返回 binding_required（不自动开通账号），
// 所以这里走 bind 流程拿到令牌。
func TestGatewayLoginSetsSessionCookie(t *testing.T) {
	r, _ := setupFnOSRouter(t)
	base := "/app/techfunway-bill/api/auth/fnos"

	w := doFnOSRequest(r, base+"/bind", "3000", "cookie-user", map[string]string{
		"mode": "register", "username": "cookie-user", "password": "secret123",
	}, true)
	if w.Code != http.StatusOK {
		t.Fatalf("飞牛绑定失败: %d %s", w.Code, w.Body.String())
	}
	token := decode(t, w)["data"].(map[string]interface{})["token"].(string)
	if token == "" {
		t.Fatal("绑定响应里没有令牌")
	}

	cookies := w.Result().Cookies()
	var found *http.Cookie
	for _, ck := range cookies {
		if ck.Name == "token" {
			found = ck
		}
	}
	if found == nil {
		t.Fatalf("网关域登录未种会话 Cookie，实际 Set-Cookie：%v", cookies)
	}
	if found.Value != token {
		t.Fatal("会话 Cookie 的值与登录令牌不一致")
	}
	// Path 必须是网关前缀：宿主机内与兄弟应用互不干扰（Cookie 不分端口）。
	if found.Path != "/app/techfunway-bill" {
		t.Fatalf("会话 Cookie Path = %q，期望 /app/techfunway-bill", found.Path)
	}
	if !found.HttpOnly {
		t.Fatal("会话 Cookie 必须 HttpOnly")
	}
}

// TestGatewayLogoutClearsSessionCookie：退出登录必须在两个 listener 上都清
// Cookie——Cookie 不分端口（同主机名共享），只清前端的话网关上会被认回去。
func TestGatewayLogoutClearsSessionCookie(t *testing.T) {
	r, _ := setupFnOSRouter(t)

	// 先登录拿令牌（账密登录同样要在网关域种 Cookie）。
	loginBody := map[string]string{"username": "logout-user", "password": "secret123"}
	w := doFnOSRequest(r, "/app/techfunway-bill/api/auth/register", "", "", loginBody, true)
	if w.Code != http.StatusOK {
		t.Fatalf("注册失败: %d %s", w.Code, w.Body.String())
	}
	token := decode(t, w)["data"].(map[string]interface{})["token"].(string)

	// 带 Cookie 调登出：应返回清空动作（MaxAge<0 或空值）。
	req := httptest.NewRequest(http.MethodPost, "/app/techfunway-bill/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: token})
	req = req.WithContext(user.MarkFnOSGateway(req.Context()))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("登出失败: %d %s", res.Code, res.Body.String())
	}
	var cleared bool
	for _, ck := range res.Result().Cookies() {
		if ck.Name == "token" && (ck.MaxAge < 0 || ck.Value == "") {
			cleared = true
		}
	}
	if !cleared {
		t.Fatalf("登出未清会话 Cookie，实际 Set-Cookie：%v", res.Result().Cookies())
	}
}
