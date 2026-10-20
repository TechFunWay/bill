package user

import (
	"errors"
	"net/http"
	"strconv"

	"smallgo/server/audit"
	"smallgo/server/database"
	"smallgo/server/response"
	"smallgo/server/sysconfig"
	"smallgo/server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseUserID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.ErrorBadRequest(c, "无效的用户 ID")
		return 0, false
	}
	return uint(id), true
}

func handleSetupRequired(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var count int64
		if err := db.Model(&database.User{}).Count(&count).Error; err != nil {
			response.ErrorInternal(c, "检查初始化状态失败")
			return
		}
		response.Success(c, map[string]interface{}{
			"setup_required": count == 0,
		})
	}
}

func handleRegister(db *gorm.DB, gatewayPrefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入用户名和密码")
			return
		}

		secret := getJWTSecret(db)
		result, err := Register(db, req.Username, req.Password, secret)
		if err != nil {
			switch {
			case errors.Is(err, ErrRegisterDisabled):
				response.Error(c, http.StatusForbidden, response.CodeRegisterDisabled, err.Error())
			case errors.Is(err, ErrUserExists):
				response.Error(c, http.StatusConflict, response.CodeUserExists, err.Error())
			case errors.Is(err, ErrPasswordTooShort), errors.Is(err, ErrPasswordTooLong):
				response.ErrorBadRequest(c, err.Error())
			default:
				response.ErrorInternal(c, "注册失败")
			}
			return
		}

		// 网关域上 Authorization 头会被飞牛接入层拦截，种 HttpOnly 会话
		// Cookie 维持登录态（直连端口仍走 JWT，不种）。
		// 注册流程没有「记住我」选项，JWT 用的是 rememberedLoginTTL，
		// 所以这里固定传 true 让 Cookie 寿命与令牌一致。
		issueSessionCookie(c, db, gatewayPrefix, result, true)
		response.Success(c, result)
	}
}

func handleLogin(db *gorm.DB, gatewayPrefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Username    string `json:"username" binding:"required"`
			Password    string `json:"password" binding:"required"`
			PasswordMd5 string `json:"password_md5"`
			Remember    *bool  `json:"remember"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入用户名和密码")
			return
		}

		remember := true
		if req.Remember != nil {
			remember = *req.Remember
		}

		secret := getJWTSecret(db)
		result, err := Login(db, req.Username, req.Password, req.PasswordMd5, remember, secret)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, response.CodeInvalidCredentials, err.Error())
			return
		}

		// 网关域上 Authorization 头会被飞牛接入层拦截，种 HttpOnly 会话
		// Cookie 维持登录态（直连端口仍走 JWT，不种）。
		issueSessionCookie(c, db, gatewayPrefix, result, remember)

		// Populate context so the audit entry is attributed to the logged-in user.
		if u, ok := result["user"].(map[string]interface{}); ok {
			if uid, ok := u["id"].(uint); ok {
				c.Set("userID", uid)
				c.Set("username", req.Username)
			}
		}
		audit.Log(db, c, "login", "user", c.GetUint("userID"), "用户登录")

		response.Success(c, result)
	}
}

func handleCheckAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		requireLogin, _ := sysconfig.GetConfig(db, "require_login", 0)

		userID, _ := c.Get("userID")
		uid, _ := userID.(uint)

		result := CheckAuth(db, uid, requireLogin)
		response.Success(c, result)
	}
}

// handleLogout 退出登录：飞牛登录只是身份来源，应用会话必须能独立结束
// （父仓库记忆 fnos-login-session：用户在应用里点退出就该退得掉，不需要退出飞牛）。
// 会话 Cookie 不分端口（同主机名共享），所以无论请求来自网关 socket 还是直连
// 端口都要清，防止网关域上的会话残留把人认回去。
//
// 审计与 smallgo 基线对齐：登出也留痕（此前只有清 Cookie，审计里看不到登出）。
func handleLogout(db *gorm.DB, gatewayPrefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		clearSessionCookie(c, gatewayPrefix)
		if uid := c.GetUint("userID"); uid > 0 {
			audit.Log(db, c, "logout", "user", uid, "退出登录")
		}
		response.Success(c, map[string]interface{}{"ok": true})
	}
}

func handleGetCurrentUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		uid, _ := userID.(uint)

		result, err := GetCurrentUser(db, uid)
		if err != nil {
			response.ErrorInternal(c, "获取用户信息失败")
			return
		}

		response.Success(c, result)
	}
}

func handleChangePassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			OldPassword    string `json:"old_password" binding:"required"`
			OldPasswordMd5 string `json:"old_password_md5"`
			NewPassword    string `json:"new_password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入旧密码和新密码")
			return
		}

		userID, _ := c.Get("userID")
		uid, _ := userID.(uint)

		if err := ChangePassword(db, uid, req.OldPassword, req.OldPasswordMd5, req.NewPassword); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}

		response.Success(c, nil)
	}
}

func handleRegenerateAPIKey(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		uid, _ := userID.(uint)

		newKey, err := RegenerateAPIKey(db, uid)
		if err != nil {
			response.ErrorInternal(c, "重新生成 API Key 失败")
			return
		}

		response.Success(c, map[string]string{"api_key": newKey})
	}
}

func handleGetAllUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := utils.Atoi(c.Query("page"), 1)
		pageSize := utils.Atoi(c.Query("pageSize"), utils.DefaultPageSize)
		search := c.Query("search")

		users, total, err := GetAllUsers(db, page, pageSize, search)
		if err != nil {
			response.ErrorInternal(c, "获取用户列表失败")
			return
		}

		page, pageSize = utils.NormalizePage(page, pageSize)
		response.SuccessPage(c, users, total, page, pageSize)
	}
}

func handleUpdateUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Role   *string `json:"role"`
			Status *int    `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请求无效")
			return
		}

		userID, ok := parseUserID(c)
		if !ok {
			return
		}

		if err := UpdateUser(db, userID, req.Role, req.Status); err != nil {
			if errors.Is(err, ErrUserNotFound) {
				response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			} else {
				response.ErrorBadRequest(c, err.Error())
			}
			return
		}

		audit.Log(db, c, "user_update", "user", userID, "管理员修改用户信息")
		response.Success(c, nil)
	}
}

func handleDeleteUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := parseUserID(c)
		if !ok {
			return
		}
		if userID == c.GetUint("userID") {
			response.ErrorBadRequest(c, "不能删除自己的账号")
			return
		}
		if err := DeleteUser(db, userID); err != nil {
			switch {
			case errors.Is(err, ErrUserNotFound):
				response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			case errors.Is(err, ErrLastAdmin), errors.Is(err, ErrUserHasAppData):
				response.ErrorBadRequest(c, err.Error())
			default:
				response.ErrorInternal(c, "删除用户失败")
			}
			return
		}

		audit.Log(db, c, "user_delete", "user", userID, "管理员删除用户")
		response.Success(c, nil)
	}
}

func handleToggleStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := parseUserID(c)
		if !ok {
			return
		}

		if err := ToggleUserStatus(db, userID); err != nil {
			switch {
			case errors.Is(err, ErrUserNotFound):
				response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			case errors.Is(err, ErrLastAdmin):
				response.ErrorBadRequest(c, err.Error())
			default:
				response.ErrorInternal(c, "切换用户状态失败")
			}
			return
		}

		audit.Log(db, c, "user_status", "user", userID, "管理员切换用户状态")
		response.Success(c, nil)
	}
}

func handleResetPassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			NewPassword string `json:"new_password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入新密码")
			return
		}

		userID, ok := parseUserID(c)
		if !ok {
			return
		}
		if err := ResetPassword(db, userID, req.NewPassword); err != nil {
			if errors.Is(err, ErrPasswordTooShort) || errors.Is(err, ErrPasswordTooLong) {
				response.ErrorBadRequest(c, err.Error())
			} else if errors.Is(err, ErrUserNotFound) {
				response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			} else {
				response.ErrorInternal(c, "重置密码失败")
			}
			return
		}

		audit.Log(db, c, "password_reset", "user", userID, "管理员重置用户密码")
		response.Success(c, nil)
	}
}

// RegisterRoutes wires user and authentication routes.
func RegisterRoutes(publicGroup *gin.RouterGroup, optionalAuthGroup *gin.RouterGroup, authGroup *gin.RouterGroup, adminGroup *gin.RouterGroup, db *gorm.DB, gatewayPrefix string) {
	publicGroup.GET("/auth/setup-required", handleSetupRequired(db))
	publicGroup.POST("/auth/register", handleRegister(db, gatewayPrefix))
	publicGroup.POST("/auth/login", handleLogin(db, gatewayPrefix))

	optionalAuthGroup.GET("/auth/check", handleCheckAuth(db))

	authGroup.GET("/auth/me", handleGetCurrentUser(db))
	authGroup.PUT("/auth/password", handleChangePassword(db))
	authGroup.POST("/auth/apikey", handleRegenerateAPIKey(db))
	authGroup.POST("/auth/logout", handleLogout(db, gatewayPrefix))

	adminGroup.GET("/users", handleGetAllUsers(db))
	adminGroup.PUT("/users/:id", handleUpdateUser(db))
	adminGroup.DELETE("/users/:id", handleDeleteUser(db))
	adminGroup.PUT("/users/:id/status", handleToggleStatus(db))
	adminGroup.PUT("/users/:id/password", handleResetPassword(db))
}
