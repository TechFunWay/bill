package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"smallgo/server/apps"
	"smallgo/server/audit"
	"smallgo/server/config"
	"smallgo/server/database"
	"smallgo/server/logger"
	"smallgo/server/middleware"
	"smallgo/server/response"
	"smallgo/server/scheduler"
	"smallgo/server/security"
	"smallgo/server/sysconfig"
	"smallgo/server/upload"
	"smallgo/server/user"
	"smallgo/server/version"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Start() {
	cfg := config.C

	if cfg.ShowHelp {
		config.PrintHelp()
		os.Exit(0)
	}

	if cfg.ShowVersion {
		version.PrintVersion()
		os.Exit(0)
	}

	if cfg.ResetAdminPassword {
		db, err := database.InitDB(cfg.DBPath)
		if err != nil {
			fmt.Printf("Failed to connect database: %v\n", err)
			os.Exit(1)
		}
		defer database.CloseDB(db)

		if err := user.ResetAdminPassword(db); err != nil {
			fmt.Printf("Failed to reset admin password: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if cfg.LogMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	if err := logger.Init(cfg.LogDir, cfg.LogRetentionDays); err != nil {
		fmt.Printf("Failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		fmt.Printf("Failed to connect database: %v\n", err)
		os.Exit(1)
	}
	defer database.CloseDB(db)

	if err := database.AutoMigrate(db); err != nil {
		fmt.Printf("Failed to migrate database: %v\n", err)
		os.Exit(1)
	}

	if err := database.RunUpgrades(db, version.Version, database.Upgrades); err != nil {
		fmt.Printf("Failed to run upgrades: %v\n", err)
		os.Exit(1)
	}

	if err := apps.MigrateAll(db); err != nil {
		fmt.Printf("Failed to run app migrations: %v\n", err)
		os.Exit(1)
	}

	if err := sysconfig.InitDefaultConfigs(db); err != nil {
		fmt.Printf("Failed to init default configs: %v\n", err)
		os.Exit(1)
	}

	logger.SetRetentionFunc(func() int {
		val, err := sysconfig.GetConfig(db, "log_retention_days", 0)
		if err != nil || val == "" {
			return cfg.LogRetentionDays
		}
		if days, err := strconv.Atoi(val); err == nil {
			return days
		}
		return cfg.LogRetentionDays
	})

	jwtSecret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil || jwtSecret == "" {
		fmt.Println("Failed to get JWT secret")
		os.Exit(1)
	}

	r := NewRouter(cfg, db, jwtSecret)

	// Start background jobs registered by the framework or apps.
	sched := scheduler.Start()
	defer sched.Stop()

	logger.Info("========================================")
	logger.Info("  %s %s", version.AppName, version.Version)
	logger.Info("========================================")
	logger.Info("  Data Dir:    %s", cfg.DataDir)
	logger.Info("  Database:    %s", cfg.DBPath)
	logger.Info("  Web Dir:     %s", cfg.WebDir)
	logger.Info("  Upload Dir:  %s", cfg.UploadDir)
	logger.Info("  Access URL:  http://localhost:%d", cfg.Port)
	if cfg.FnOSApp {
		logger.Info("  Gateway:     %s", cfg.GatewayPrefix)
		logger.Info("  Socket:      %s", cfg.GatewaySocket)
	}
	if cfg.RateLimit > 0 {
		logger.Info("  Rate Limit:  %d req/min per IP", cfg.RateLimit)
	}
	logger.Info("========================================")

	listeners, err := listen(cfg)
	if err != nil {
		logger.Error("Failed to listen: %v", err)
		return
	}
	for _, binding := range listeners {
		defer binding.Listener.Close()
	}
	if cfg.FnOSApp {
		defer os.Remove(cfg.GatewaySocket)
	}

	servers := make([]*http.Server, 0, len(listeners))
	for _, binding := range listeners {
		handler := http.Handler(r)
		if !binding.IsFnOSGateway {
			handler = newDirectHandler(r, cfg)
		}
		srv := newHTTPServer(handler, binding.IsFnOSGateway)
		servers = append(servers, srv)
		go func(server *http.Server, listener net.Listener) {
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				logger.Error("Server error: %v", err)
			}
		}(srv, binding.Listener)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, srv := range servers {
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Server forced to shutdown: %v", err)
		}
	}

	apps.ShutdownAll()

	logger.Info("Server exited")
}

type listenerBinding struct {
	Listener      net.Listener
	IsFnOSGateway bool
}

func newHTTPServer(handler http.Handler, fnOSGateway bool) *http.Server {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if fnOSGateway {
		srv.ConnContext = func(ctx context.Context, _ net.Conn) context.Context {
			return user.MarkFnOSGateway(ctx)
		}
	}
	return srv
}

func listen(cfg config.Config) ([]listenerBinding, error) {
	tcpListener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return nil, err
	}
	listeners := []listenerBinding{{Listener: tcpListener}}
	if !cfg.FnOSApp {
		return listeners, nil
	}
	if cfg.GatewaySocket == "" {
		tcpListener.Close()
		return nil, fmt.Errorf("-fnos-app requires -gateway-socket")
	}
	if !strings.HasPrefix(cfg.GatewayPrefix, "/app/") {
		tcpListener.Close()
		return nil, fmt.Errorf("-gateway-prefix must begin with /app/")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.GatewaySocket), 0755); err != nil {
		tcpListener.Close()
		return nil, fmt.Errorf("create gateway socket directory: %w", err)
	}
	if err := os.Remove(cfg.GatewaySocket); err != nil && !os.IsNotExist(err) {
		tcpListener.Close()
		return nil, fmt.Errorf("remove stale gateway socket: %w", err)
	}
	socketListener, err := net.Listen("unix", cfg.GatewaySocket)
	if err != nil {
		tcpListener.Close()
		return nil, fmt.Errorf("listen on gateway socket: %w", err)
	}
	return append(listeners, listenerBinding{Listener: socketListener, IsFnOSGateway: true}), nil
}

func newDirectHandler(handler http.Handler, cfg config.Config) http.Handler {
	if !cfg.FnOSApp {
		return handler
	}
	prefix := strings.TrimSuffix(cfg.GatewayPrefix, "/") + "/"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, prefix, http.StatusTemporaryRedirect)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

// NewRouter builds the full API router (all middleware, routes and registered
// apps) without starting an HTTP listener. Start uses it in production; tests
// use it to exercise the API against an in-memory database.
func NewRouter(cfg config.Config, db *gorm.DB, jwtSecret string) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(cfg.CORSOrigin))
	r.Use(middleware.LimitJSONBody(1 << 20))

	appGroup := r.Group("")
	if cfg.FnOSApp {
		appGroup = r.Group(strings.TrimSuffix(cfg.GatewayPrefix, "/"))
	}
	api := appGroup.Group("/api")

	api.GET("/version", func(c *gin.Context) {
		response.Success(c, version.GetVersion())
	})

	// Liveness/readiness probe for Docker, NAS health checks, uptime monitors.
	api.GET("/health", func(c *gin.Context) {
		response.Success(c, gin.H{
			"status":  "ok",
			"version": version.Version,
			"time":    time.Now().Format(time.RFC3339),
		})
	})

	// Rate limiter for public endpoints
	var publicLimiter gin.HandlerFunc
	if cfg.RateLimit > 0 {
		rl := middleware.NewRateLimiter(cfg.RateLimit, 1*time.Minute)
		publicLimiter = middleware.LimitRequests(rl)
	} else {
		publicLimiter = middleware.LimitRequests(nil)
	}

	// Auth route groups shared by all features.
	publicGroup := api.Group("", publicLimiter)
	optionalAuthGroup := api.Group("")
	optionalAuthGroup.Use(middleware.OptionalAuth(jwtSecret, db))
	authGroup := api.Group("")
	authGroup.Use(middleware.RequireAuth(jwtSecret, db))
	adminGroup := api.Group("")
	adminGroup.Use(middleware.RequireAuth(jwtSecret, db))
	adminGroup.Use(middleware.RequireAdmin())

	// Config: public read is rate-limited; user config read & write require
	// auth; system config read & metadata require admin.
	sysconfig.RegisterRoutes(publicGroup, authGroup, adminGroup, db)

	// Security questions: forgot-password flow is public (rate limited); managing
	// one's own questions requires auth.
	security.RegisterRoutes(publicGroup, authGroup, db)

	// File upload requires authentication; uploaded content is still readable by
	// URL so it can be embedded in public pages.
	authGroup.POST("/upload", upload.HandleUpload(cfg.UploadDir))
	appGroup.GET("/uploads/*filepath", upload.ServeUpload(cfg.UploadDir))

	user.RegisterRoutes(publicGroup, optionalAuthGroup, authGroup, adminGroup, db)
	if cfg.FnOSApp {
		user.RegisterFnOSRoutes(publicGroup, db)
	}

	// Admin operation log (browse + CSV export).
	audit.RegisterRoutes(adminGroup, db)

	// Register apps from the registry (includes the qrcode example app).
	apps.SetupAll(publicGroup, db)
	apps.SetupProtectedAll(authGroup, adminGroup, db)

	if cfg.WebDir != "" {
		appGroup.Static("/assets", cfg.WebDir+"/assets")
		appGroup.StaticFile("/favicon.ico", cfg.WebDir+"/favicon.ico")
		appGroup.StaticFile("/favicon.svg", cfg.WebDir+"/favicon.svg")
		appGroup.StaticFile("/favicon-16.png", cfg.WebDir+"/favicon-16.png")
		appGroup.StaticFile("/favicon-32.png", cfg.WebDir+"/favicon-32.png")
		appGroup.StaticFile("/favicon-192.png", cfg.WebDir+"/favicon-192.png")
		appGroup.StaticFile("/favicon-512.png", cfg.WebDir+"/favicon-512.png")
		appGroup.StaticFile("/apple-touch-icon.png", cfg.WebDir+"/apple-touch-icon.png")
		appGroup.GET("/manifest.webmanifest", func(c *gin.Context) {
			c.Header("Content-Type", "application/manifest+json")
			c.File(cfg.WebDir + "/manifest.webmanifest")
		})
		// Gateway trampoline: served on the gateway prefix with the real
		// service port injected, it exchanges the fnOS gateway session for a
		// one-time ticket and sends the browser to the plain TCP listener.
		appGroup.GET("/fnos-entry.html", func(c *gin.Context) {
			data, err := os.ReadFile(filepath.Join(cfg.WebDir, "fnos-entry.html"))
			if err != nil {
				c.String(http.StatusNotFound, "未找到飞牛登录跳板页")
				return
			}
			page := strings.ReplaceAll(string(data), "__APP_PORT__", strconv.Itoa(cfg.Port))
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
		})
		r.NoRoute(func(c *gin.Context) {
			if !cfg.FnOSApp || c.Request.URL.Path == strings.TrimSuffix(cfg.GatewayPrefix, "/") || strings.HasPrefix(c.Request.URL.Path, strings.TrimSuffix(cfg.GatewayPrefix, "/")+"/") {
				c.File(cfg.WebDir + "/index.html")
				return
			}
			c.Status(http.StatusNotFound)
		})
	}

	return r
}
