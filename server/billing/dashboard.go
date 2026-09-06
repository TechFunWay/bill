package billing

import (
	"strings"
	"time"

	"smallgo/server/database"
	"smallgo/server/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func handleDashboard(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("userID")
		if _, err := ensurePersonalSetup(db, userID); err != nil {
			response.ErrorInternal(c, "获取首页数据失败")
			return
		}
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Format("2006-01-02")
		end := now.Format("2006-01-02")
		summary, categories, trend, err := personalStats(db, userID, start, end)
		if err != nil {
			response.ErrorInternal(c, "获取首页数据失败")
			return
		}
		var recent []PersonalTransaction
		if err := db.Where("user_id = ?", userID).Order("occurred_at DESC, id DESC").Limit(5).Find(&recent).Error; err != nil {
			response.ErrorInternal(c, "获取首页数据失败")
			return
		}
		if err := loadPersonalRelations(db, recent); err != nil {
			response.ErrorInternal(c, "获取首页数据失败")
			return
		}

		ledgers, err := listLedgersForUser(db, userID)
		if err != nil {
			response.ErrorInternal(c, "获取首页数据失败")
			return
		}
		var personalLedgers []PersonalLedger
		if err := db.Where("user_id = ? AND archived = ?", userID, false).Order("sort_order ASC, is_default DESC, created_at ASC").Find(&personalLedgers).Error; err != nil {
			response.ErrorInternal(c, "获取首页数据失败")
			return
		}
		var pending int64
		if err := db.Model(&SharedMember{}).Where("user_id = ? AND status = ?", userID, MemberPending).Count(&pending).Error; err != nil {
			response.ErrorInternal(c, "获取首页数据失败")
			return
		}
		response.Success(c, gin.H{
			"month":  gin.H{"summary": summary, "categories": categories, "trend": trend},
			"recent": recent, "ledgers": ledgers, "personal_ledgers": personalLedgers, "pending_invitations": pending,
		})
	}
}

func handleSearchUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db.Model(&database.User{}).Where("status = ? AND id <> ?", 1, c.GetUint("userID"))
		if search := strings.TrimSpace(c.Query("q")); search != "" {
			query = query.Where("username LIKE ?", "%"+search+"%")
		}
		var users []userBrief
		if err := query.Select("id, username").Order("username ASC").Limit(20).Scan(&users).Error; err != nil {
			response.ErrorInternal(c, "搜索用户失败")
			return
		}
		response.Success(c, users)
	}
}
