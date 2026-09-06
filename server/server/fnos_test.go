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
