package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"smallgo/server/audit"
	"smallgo/server/auth"
	"smallgo/server/database"
	"smallgo/server/response"
	"smallgo/server/sysconfig"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	ErrFnOSNotBound     = errors.New("此飞牛 NAS 账号尚未绑定应用账号")
	ErrFnOSAlreadyBound = errors.New("此飞牛 NAS 账号已绑定其他应用账号")
)

// FnOSBinding keeps NAS identity outside the core user model so the gateway
// integration stays optional and independently maintainable.
type FnOSBinding struct {
	ID           uint   `gorm:"primarykey"`
	UserID       uint   `gorm:"not null;uniqueIndex"`
	FnOSUserID   uint   `gorm:"column:fnos_user_id;not null;uniqueIndex"`
	FnOSUsername string `gorm:"column:fnos_username;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type FnOSIdentity struct {
	UserID   uint
	Username string
	IsAdmin  bool
}

type fnOSGatewayContextKey struct{}

// fnOSLoginTicket carries a gateway-verified NAS identity to the plain TCP
// listener, where the gateway headers are absent. Tickets are short lived and
// single use so a leaked value cannot be replayed later.
const fnOSTicketTTL = 2 * time.Minute

type fnOSLoginTicket struct {
	Identity  FnOSIdentity
	ExpiresAt time.Time
}

var (
	fnOSTicketsMu sync.Mutex
	fnOSTickets   = make(map[string]fnOSLoginTicket)
)

func newFnOSTicketValue() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func storeFnOSTicket(identity FnOSIdentity) (string, error) {
	ticket, err := newFnOSTicketValue()
	if err != nil {
		return "", err
	}
	now := time.Now()
	fnOSTicketsMu.Lock()
	for key, item := range fnOSTickets {
		if !item.ExpiresAt.After(now) {
			delete(fnOSTickets, key)
		}
	}
	fnOSTickets[ticket] = fnOSLoginTicket{Identity: identity, ExpiresAt: now.Add(fnOSTicketTTL)}
	fnOSTicketsMu.Unlock()
	return ticket, nil
}

func consumeFnOSTicket(ticket string) {
	if ticket == "" {
		return
	}
	fnOSTicketsMu.Lock()
	delete(fnOSTickets, ticket)
	fnOSTicketsMu.Unlock()
}

func init() {
	database.RegisterModels(&FnOSBinding{})
	database.RegisterUserDeleteCleanup(func(db *gorm.DB, userID uint) error {
		return db.Where("user_id = ?", userID).Delete(&FnOSBinding{}).Error
	})
}

// MarkFnOSGateway marks connections accepted from the package Unix socket.
// Header names alone are never trusted because TCP clients can forge them.
func MarkFnOSGateway(ctx context.Context) context.Context {
	return context.WithValue(ctx, fnOSGatewayContextKey{}, true)
}

func fnOSIdentity(c *gin.Context) (FnOSIdentity, bool) {
	if c.Request.Context().Value(fnOSGatewayContextKey{}) != true {
		// Plain TCP access: the gateway headers cannot be trusted here, so the
		// identity must arrive via a one-time ticket minted on the gateway.
		ticket := c.GetHeader("X-FnOS-Ticket")
		if ticket == "" {
			response.ErrorUnauthorized(c, "请从飞牛桌面中的应用入口使用 NAS 登录")
			return FnOSIdentity{}, false
		}
		fnOSTicketsMu.Lock()
		item, ok := fnOSTickets[ticket]
		if ok && !item.ExpiresAt.After(time.Now()) {
			delete(fnOSTickets, ticket)
			ok = false
		}
		fnOSTicketsMu.Unlock()
		if !ok {
			response.ErrorUnauthorized(c, "飞牛登录凭证无效或已过期，请重新点击飞牛授权登录")
			return FnOSIdentity{}, false
		}
		return item.Identity, true
	}
	uid, err := strconv.ParseUint(c.GetHeader("X-Trim-Userid"), 10, 32)
	username := c.GetHeader("X-Trim-Username")
	if err != nil || uid == 0 || username == "" {
		response.ErrorUnauthorized(c, "未获取到飞牛 NAS 登录信息")
		return FnOSIdentity{}, false
	}
	return FnOSIdentity{UserID: uint(uid), Username: username, IsAdmin: c.GetHeader("X-Trim-Isadmin") == "true"}, true
}

func fnOSLoginResult(db *gorm.DB, user database.User, remember bool, jwtSecret string) (map[string]interface{}, error) {
	token, err := auth.GenerateTokenWithTTL(user.ID, user.Username, user.Role, user.AuthVersion, jwtSecret, loginTokenTTL(db, remember))
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"token": token,
		"user":  map[string]interface{}{"id": user.ID, "username": user.Username, "role": user.Role},
	}, nil
}

func loginWithFnOS(db *gorm.DB, identity FnOSIdentity, remember bool, jwtSecret string) (map[string]interface{}, error) {
	var binding FnOSBinding
	if err := db.Where("fnos_user_id = ?", identity.UserID).First(&binding).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFnOSNotBound
		}
		return nil, err
	}
	var user database.User
	if err := db.First(&user, binding.UserID).Error; err != nil {
		return nil, err
	}
	if user.Status != 1 {
		return nil, fmt.Errorf("账号已停用")
	}
	if user.AuthVersion == 0 {
		user.AuthVersion = 1
		if err := db.Model(&user).Update("auth_version", user.AuthVersion).Error; err != nil {
			return nil, err
		}
	}
	if binding.FnOSUsername != identity.Username {
		if err := db.Model(&binding).Update("fnos_username", identity.Username).Error; err != nil {
			return nil, err
		}
	}
	return fnOSLoginResult(db, user, remember, jwtSecret)
}

func bindFnOSAccount(db *gorm.DB, identity FnOSIdentity, username, password, mode string, remember bool, jwtSecret string) (map[string]interface{}, error) {
	if identity.UserID == 0 || identity.Username == "" {
		return nil, fmt.Errorf("飞牛登录信息无效")
	}
	var result map[string]interface{}
	err := db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&FnOSBinding{}).Where("fnos_user_id = ?", identity.UserID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrFnOSAlreadyBound
		}

		var user database.User
		switch mode {
		case "register":
			var userCount int64
			if err := tx.Model(&database.User{}).Count(&userCount).Error; err != nil {
				return err
			}
			if userCount > 0 {
				allowRegister, err := sysconfig.GetConfig(tx, "allow_register", 0)
				if err != nil {
					return err
				}
				if allowRegister != "true" {
					return ErrRegisterDisabled
				}
			}
			if err := validatePassword(password); err != nil {
				return err
			}
			if err := tx.Where("username = ?", username).First(&user).Error; err == nil {
				return ErrUserExists
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			hashed, err := auth.HashPassword(password)
			if err != nil {
				return err
			}
			role := "user"
			if userCount == 0 {
				role = "admin"
			}
			user = database.User{Username: username, Password: hashed, Role: role, Status: 1, APIKey: auth.GenerateAPIKey(), AuthVersion: 1}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
		case "bind":
			if err := tx.Where("username = ?", username).First(&user).Error; err != nil || user.Status != 1 {
				return fmt.Errorf("用户名或密码错误")
			}
			verified, needsRehash := verifyStoredPassword(user.Password, password, "")
			if !verified {
				return fmt.Errorf("用户名或密码错误")
			}
			if needsRehash {
				hashed, err := auth.HashPassword(password)
				if err != nil {
					return err
				}
				if err := tx.Model(&user).Update("password", hashed).Error; err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("无效的绑定方式")
		}

		var existing FnOSBinding
		if err := tx.Where("user_id = ?", user.ID).First(&existing).Error; err == nil {
			return fmt.Errorf("该应用账号已绑定其他飞牛 NAS 账号")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		binding := FnOSBinding{UserID: user.ID, FnOSUserID: identity.UserID, FnOSUsername: identity.Username}
		if err := tx.Create(&binding).Error; err != nil {
			return err
		}
		var err error
		result, err = fnOSLoginResult(tx, user, remember, jwtSecret)
		return err
	})
	return result, err
}

func setFnOSAuditContext(c *gin.Context, result map[string]interface{}) {
	if account, ok := result["user"].(map[string]interface{}); ok {
		if uid, ok := account["id"].(uint); ok {
			c.Set("userID", uid)
		}
		if username, ok := account["username"].(string); ok {
			c.Set("username", username)
		}
	}
}

func RegisterFnOSRoutes(public *gin.RouterGroup, db *gorm.DB) {
	// Minted only for requests that arrived through the gateway unix socket:
	// the browser session was already verified by fnOS and the identity headers
	// cannot be forged there. Tickets let the SPA on the plain TCP port log in
	// without ever seeing or sending the gateway headers.
	public.POST("/auth/fnos/ticket", func(c *gin.Context) {
		if c.Request.Context().Value(fnOSGatewayContextKey{}) != true {
			response.ErrorUnauthorized(c, "请从飞牛桌面中的应用入口使用 NAS 登录")
			return
		}
		identity, ok := fnOSIdentity(c)
		if !ok {
			return
		}
		ticket, err := storeFnOSTicket(identity)
		if err != nil {
			response.ErrorInternal(c, "生成飞牛登录凭证失败")
			return
		}
		response.Success(c, gin.H{"ticket": ticket, "fnos_username": identity.Username})
	})

	public.GET("/auth/fnos/identity", func(c *gin.Context) {
		identity, ok := fnOSIdentity(c)
		if !ok {
			return
		}
		response.Success(c, gin.H{"fnos_username": identity.Username})
	})

	public.POST("/auth/fnos/login", func(c *gin.Context) {
		identity, ok := fnOSIdentity(c)
		if !ok {
			return
		}
		var req struct {
			Remember *bool `json:"remember"`
		}
		if c.Request.ContentLength != 0 {
			if err := c.ShouldBindJSON(&req); err != nil {
				response.ErrorBadRequest(c, "登录参数无效")
				return
			}
		}
		remember := true
		if req.Remember != nil {
			remember = *req.Remember
		}
		result, err := loginWithFnOS(db, identity, remember, getJWTSecret(db))
		if errors.Is(err, ErrFnOSNotBound) {
			var accountCount int64
			if err := db.Model(&database.User{}).Count(&accountCount).Error; err != nil {
				response.ErrorInternal(c, "检查应用账号状态失败")
				return
			}
			var matchingAccountCount int64
			if err := db.Model(&database.User{}).Where("username = ?", identity.Username).Count(&matchingAccountCount).Error; err != nil {
				response.ErrorInternal(c, "检查飞牛账号绑定状态失败")
				return
			}
			suggestedMode := "register"
			if accountCount > 0 {
				suggestedMode = "bind"
			}
			suggestedUsername := ""
			if matchingAccountCount > 0 {
				suggestedUsername = identity.Username
			}
			response.Success(c, gin.H{
				"binding_required":   true,
				"fnos_username":      identity.Username,
				"has_accounts":       accountCount > 0,
				"suggested_mode":     suggestedMode,
				"suggested_username": suggestedUsername,
			})
			return
		}
		if err != nil {
			response.Error(c, http.StatusUnauthorized, response.CodeInvalidCredentials, "飞牛 NAS 登录失败")
			return
		}
		setFnOSAuditContext(c, result)
		audit.Log(db, c, "fnos_login", "user", c.GetUint("userID"), "飞牛 NAS 登录")
		consumeFnOSTicket(c.GetHeader("X-FnOS-Ticket"))
		response.Success(c, result)
	})

	public.POST("/auth/fnos/bind", func(c *gin.Context) {
		identity, ok := fnOSIdentity(c)
		if !ok {
			return
		}
		var req struct {
			Mode     string `json:"mode" binding:"required"`
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
			Remember *bool  `json:"remember"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入应用账号和密码")
			return
		}
		remember := true
		if req.Remember != nil {
			remember = *req.Remember
		}
		result, err := bindFnOSAccount(db, identity, req.Username, req.Password, req.Mode, remember, getJWTSecret(db))
		if err != nil {
			switch {
			case errors.Is(err, ErrRegisterDisabled):
				response.Error(c, http.StatusForbidden, response.CodeRegisterDisabled, err.Error())
			case errors.Is(err, ErrUserExists), errors.Is(err, ErrFnOSAlreadyBound):
				response.Error(c, http.StatusConflict, response.CodeUserExists, err.Error())
			default:
				response.ErrorBadRequest(c, err.Error())
			}
			return
		}
		setFnOSAuditContext(c, result)
		audit.Log(db, c, "fnos_bind", "user", c.GetUint("userID"), "绑定飞牛 NAS 账号")
		consumeFnOSTicket(c.GetHeader("X-FnOS-Ticket"))
		response.Success(c, result)
	})
}
