package apps

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestSetupProtectedAllUsesTheCorrectRouteGroups(t *testing.T) {
	previous := registered
	registered = nil
	t.Cleanup(func() { registered = previous })

	Register(App{
		Name: "protected-example",
		SetupAuth: func(api *gin.RouterGroup, _ *gorm.DB) {
			api.GET("/authenticated", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		},
		SetupAdmin: func(api *gin.RouterGroup, _ *gorm.DB) {
			api.GET("/administrator", func(c *gin.Context) { c.Status(http.StatusNoContent) })
		},
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	SetupProtectedAll(r.Group("/auth"), r.Group("/admin"), nil)
	for _, path := range []string{"/auth/authenticated", "/admin/administrator"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNoContent {
			t.Fatalf("%s: status %d, want %d", path, w.Code, http.StatusNoContent)
		}
	}
}
