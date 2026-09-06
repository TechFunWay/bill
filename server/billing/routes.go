package billing

import (
	"smallgo/server/apps"
	"smallgo/server/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func init() {
	database.RegisterModels(
		&PersonalAccount{},
		&PersonalLedger{},
		&PersonalCategory{},
		&PersonalTag{},
		&PersonalTransactionTag{},
		&PersonalTransaction{},
		&BillAttachment{},
		&SharedLedger{},
		&SharedMember{},
		&SharedTransaction{},
		&SharedShare{},
		&Settlement{},
	)
	database.RegisterUserDeleteGuard(hasBillingData)
	database.RegisterUserDeleteCleanup(cleanupBillingUserData)
	apps.Register(apps.App{
		Name:        "billing",
		DisplayName: "账单",
		Icon:        "wallet",
		RoutePrefix: "/billing",
		NavPosition: 10,
		SetupAuth:   setupRoutes,
	})
}

func cleanupBillingUserData(db *gorm.DB, userID uint) error {
	if err := db.Where("user_id = ? AND is_default = ?", userID, true).Delete(&PersonalCategory{}).Error; err != nil {
		return err
	}
	return db.Where("user_id = ? AND is_default = ?", userID, true).Delete(&PersonalLedger{}).Error
}

func hasBillingData(db *gorm.DB, userID uint) (bool, error) {
	checks := []struct {
		model interface{}
		query string
		args  []interface{}
	}{
		{&PersonalTransaction{}, "user_id = ?", []interface{}{userID}},
		{&PersonalLedger{}, "user_id = ? AND is_default = ?", []interface{}{userID, false}},
		{&PersonalCategory{}, "user_id = ? AND is_default = ?", []interface{}{userID, false}},
		{&PersonalTag{}, "user_id = ?", []interface{}{userID}},
		{&PersonalAccount{}, "user_id = ?", []interface{}{userID}},
		{&SharedLedger{}, "owner_user_id = ?", []interface{}{userID}},
		{&SharedMember{}, "user_id = ?", []interface{}{userID}},
		{&SharedTransaction{}, "created_by = ? OR actor_user_id = ?", []interface{}{userID, userID}},
		{&SharedShare{}, "user_id = ?", []interface{}{userID}},
		{&Settlement{}, "from_user_id = ? OR to_user_id = ? OR created_by = ?", []interface{}{userID, userID, userID}},
	}
	for _, check := range checks {
		var count int64
		if err := db.Model(check.model).Where(check.query, check.args...).Limit(1).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func setupRoutes(api *gin.RouterGroup, db *gorm.DB) {
	group := api.Group("/billing")

	group.GET("/dashboard", handleDashboard(db))
	group.GET("/users", handleSearchUsers(db))

	group.GET("/personal/transactions", handleListPersonal(db))
	group.POST("/personal/transactions", handleCreatePersonal(db))
	group.PUT("/personal/transactions/:id", handleUpdatePersonal(db))
	group.DELETE("/personal/transactions/:id", handleDeletePersonal(db))
	group.GET("/personal/stats", handlePersonalStats(db))
	group.GET("/personal/export", handleExportPersonal(db))
	group.GET("/personal/accounts", handleListPersonalAccounts(db))
	group.GET("/personal/account-options", handlePersonalAccountOptions())
	group.POST("/personal/accounts", handleCreatePersonalAccount(db))
	group.PUT("/personal/accounts/:id", handleUpdatePersonalAccount(db))
	group.DELETE("/personal/accounts/:id", handleArchivePersonalAccount(db))
	group.POST("/personal/accounts/:id/restore", handleRestorePersonalAccount(db))
	group.GET("/personal/ledgers", handleListPersonalLedgers(db))
	group.POST("/personal/ledgers", handleCreatePersonalLedger(db))
	group.PUT("/personal/ledgers/order", handleOrderPersonalLedgers(db))
	group.PUT("/personal/ledgers/:id", handleUpdatePersonalLedger(db))
	group.DELETE("/personal/ledgers/:id", handleArchivePersonalLedger(db))
	group.POST("/personal/ledgers/:id/restore", handleRestorePersonalLedger(db))
	group.GET("/personal/categories", handleListPersonalCategories(db))
	group.POST("/personal/categories", handleCreatePersonalCategory(db))
	group.PUT("/personal/categories/:id", handleUpdatePersonalCategory(db))
	group.DELETE("/personal/categories/:id", handleDeletePersonalCategory(db))
	group.GET("/personal/tags", handleListPersonalTags(db))
	group.POST("/personal/tags", handleCreatePersonalTag(db))
	group.PUT("/personal/tags/:id", handleUpdatePersonalTag(db))
	group.DELETE("/personal/tags/:id", handleDeletePersonalTag(db))

	group.GET("/ledgers", handleListLedgers(db))
	group.POST("/ledgers", handleCreateLedger(db))
	group.GET("/ledgers/:id", handleGetLedger(db))
	group.PUT("/ledgers/:id", handleUpdateLedger(db))
	group.DELETE("/ledgers/:id", handleArchiveLedger(db))
	group.POST("/ledgers/:id/restore", handleRestoreLedger(db))
	group.POST("/ledgers/:id/members", handleInviteMember(db))
	group.DELETE("/ledgers/:id/members/:userId", handleRemoveMember(db))
	group.POST("/ledgers/:id/invitations/respond", handleRespondInvitation(db))

	group.POST("/ledgers/:id/transactions", handleCreateSharedTransaction(db))
	group.PUT("/ledgers/:id/transactions/:transactionId", handleUpdateSharedTransaction(db))
	group.DELETE("/ledgers/:id/transactions/:transactionId", handleDeleteSharedTransaction(db))
	group.POST("/ledgers/:id/settlements", handleCreateSettlement(db))
	group.DELETE("/ledgers/:id/settlements/:settlementId", handleDeleteSettlement(db))
}
