package billing

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"smallgo/server/audit"
	"smallgo/server/database"
	"smallgo/server/response"
	"smallgo/server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ledgerListItem struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	Currency      string `json:"currency"`
	OwnerUserID   uint   `json:"owner_user_id"`
	OwnerUsername string `json:"owner_username"`
	Archived      bool   `json:"archived"`
	MemberStatus  string `json:"member_status"`
	MemberRole    string `json:"member_role"`
	MemberCount   int64  `json:"member_count"`
	BalanceCents  int64  `json:"balance_cents"`
	ExpenseCents  int64  `json:"expense_cents"`
	UpdatedAt     string `json:"updated_at"`
}

type ledgerRequest struct {
	Name         string   `json:"name"`
	Currency     string   `json:"currency"`
	Usernames    []string `json:"usernames"`
	ShowAccount  *bool    `json:"show_account"`
	ShowCategory *bool    `json:"show_category"`
	ShowNote     *bool    `json:"show_note"`
}

type sharedTransactionRequest struct {
	Kind        string                 `json:"kind"`
	ActorUserID uint                   `json:"actor_user_id"`
	AccountID   uint                   `json:"account_id"`
	AmountCents int64                  `json:"amount_cents"`
	Category    string                 `json:"category"`
	Note        string                 `json:"note"`
	OccurredAt  string                 `json:"occurred_at"`
	Shares      []shareInput           `json:"shares"`
	Attachments *[]billAttachmentInput `json:"attachments"`
}

type settlementRequest struct {
	FromUserID  uint   `json:"from_user_id"`
	ToUserID    uint   `json:"to_user_id"`
	AmountCents int64  `json:"amount_cents"`
	Note        string `json:"note"`
	OccurredAt  string `json:"occurred_at"`
}

type transactionView struct {
	SharedTransaction
	ActorUsername   string        `json:"actor_username"`
	CreatorUsername string        `json:"creator_username"`
	Shares          []shareDetail `json:"shares"`
}

type shareDetail struct {
	UserID      uint   `json:"user_id"`
	Username    string `json:"username"`
	AmountCents int64  `json:"amount_cents"`
}

type settlementView struct {
	Settlement
	FromUsername string `json:"from_username"`
	ToUsername   string `json:"to_username"`
}

type invalidInvitationError struct {
	username string
}

func (e invalidInvitationError) Error() string {
	return "邀请用户不存在或已停用：" + e.username
}

func routeID(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil || value == 0 {
		response.ErrorBadRequest(c, "无效的 ID")
		return 0, false
	}
	return uint(value), true
}

func listLedgersForUser(db *gorm.DB, userID uint) ([]ledgerListItem, error) {
	var rows []ledgerListItem
	err := db.Table("shared_ledgers").
		Select(`shared_ledgers.id, shared_ledgers.name, shared_ledgers.currency,
			shared_ledgers.owner_user_id, owners.username AS owner_username,
			shared_ledgers.archived, shared_members.status AS member_status,
			shared_members.role AS member_role, shared_ledgers.updated_at`).
		Joins("JOIN shared_members ON shared_members.ledger_id = shared_ledgers.id").
		Joins("JOIN users owners ON owners.id = shared_ledgers.owner_user_id").
		Where("shared_members.user_id = ?", userID).
		Order("shared_members.status ASC, shared_ledgers.archived ASC, shared_ledgers.updated_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if err := db.Model(&SharedMember{}).Where("ledger_id = ? AND status = ?", rows[i].ID, MemberActive).Count(&rows[i].MemberCount).Error; err != nil {
			return nil, err
		}
		var expense struct{ Total int64 }
		if err := db.Model(&SharedTransaction{}).Where("ledger_id = ? AND kind = ?", rows[i].ID, KindExpense).
			Select("COALESCE(SUM(amount_cents), 0) AS total").Scan(&expense).Error; err != nil {
			return nil, err
		}
		rows[i].ExpenseCents = expense.Total
		if rows[i].MemberStatus == MemberActive {
			balances, err := calculateBalances(db, rows[i].ID)
			if err != nil {
				return nil, err
			}
			for _, balance := range balances {
				if balance.UserID == userID {
					rows[i].BalanceCents = balance.BalanceCents
					break
				}
			}
		}
	}
	return rows, nil
}

func handleListLedgers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := listLedgersForUser(db, c.GetUint("userID"))
		if err != nil {
			response.ErrorInternal(c, "获取共享账本失败")
			return
		}
		response.Success(c, items)
	}
}

func handleCreateLedger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ledgerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入账本名称")
			return
		}
		req.Name = cleanText(req.Name, 100)
		if req.Name == "" {
			response.ErrorBadRequest(c, "请输入账本名称")
			return
		}
		req.Currency = strings.ToUpper(cleanText(req.Currency, 8))
		if req.Currency == "" {
			req.Currency = "CNY"
		}
		userID := c.GetUint("userID")
		ledger := SharedLedger{Name: req.Name, Currency: req.Currency, OwnerUserID: userID}
		err := db.Transaction(func(tx *gorm.DB) error {
			invited, err := resolveInvitedUsers(tx, userID, req.Usernames)
			if err != nil {
				return err
			}
			if err := tx.Create(&ledger).Error; err != nil {
				return err
			}
			if err := tx.Create(&SharedMember{
				LedgerID: ledger.ID, UserID: userID, Role: MemberOwner, Status: MemberActive,
			}).Error; err != nil {
				return err
			}
			for _, user := range invited {
				if err := tx.Create(&SharedMember{
					LedgerID: ledger.ID, UserID: user.ID, Role: MemberRegular, Status: MemberPending,
				}).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			var invalidInvite invalidInvitationError
			if errors.As(err, &invalidInvite) {
				response.ErrorBadRequest(c, err.Error())
				return
			}
			response.ErrorInternal(c, "创建共享账本失败")
			return
		}
		audit.Log(db, c, "shared_ledger_create", "shared_ledger", ledger.ID, "创建共享账本")
		response.Success(c, ledger)
	}
}

// resolveInvitedUsers validates every requested username before a ledger is
// created. It also removes duplicate entries and the owner from the invitation
// list, so a malformed request cannot result in a partially-created ledger.
func resolveInvitedUsers(db *gorm.DB, ownerUserID uint, usernames []string) ([]database.User, error) {
	seenNames := make(map[string]bool, len(usernames))
	seenUsers := map[uint]bool{ownerUserID: true}
	users := make([]database.User, 0, len(usernames))
	for _, raw := range usernames {
		username := strings.TrimSpace(raw)
		if seenNames[username] {
			continue
		}
		seenNames[username] = true
		var user database.User
		err := db.Where("username = ?", username).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || err == nil && user.Status != 1 {
			return nil, invalidInvitationError{username: username}
		}
		if err != nil {
			return nil, err
		}
		if seenUsers[user.ID] {
			continue
		}
		seenUsers[user.ID] = true
		users = append(users, user)
	}
	return users, nil
}

func handleGetLedger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		if _, err := activeMember(db, ledgerID, c.GetUint("userID")); err != nil {
			if errors.Is(err, errNotMember) {
				response.ErrorForbidden(c, err.Error())
			} else {
				response.ErrorInternal(c, "获取共享账本失败")
			}
			return
		}
		ledger, err := getLedger(db, ledgerID)
		if err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, err.Error())
			return
		}
		members, err := loadMembers(db, ledgerID, true)
		if err != nil {
			response.ErrorInternal(c, "获取成员失败")
			return
		}
		page, pageSize := transactionPagination(c)
		transactions, totalTransactions, err := loadTransactions(db, ledgerID, page, pageSize)
		if err != nil {
			response.ErrorInternal(c, "获取共享账单失败")
			return
		}
		settlements, err := loadSettlements(db, ledgerID)
		if err != nil {
			response.ErrorInternal(c, "获取结算记录失败")
			return
		}
		balances, err := calculateBalances(db, ledgerID)
		if err != nil {
			response.ErrorInternal(c, "计算成员余额失败")
			return
		}
		var totals struct {
			ExpenseCents int64 `json:"expense_cents"`
			IncomeCents  int64 `json:"income_cents"`
			ExpenseCount int64 `json:"expense_count"`
			IncomeCount  int64 `json:"income_count"`
		}
		if err := db.Model(&SharedTransaction{}).Where("ledger_id = ?", ledgerID).
			Select("COALESCE(SUM(CASE WHEN kind = ? THEN amount_cents ELSE 0 END), 0) AS expense_cents, COALESCE(SUM(CASE WHEN kind = ? THEN amount_cents ELSE 0 END), 0) AS income_cents, COALESCE(SUM(CASE WHEN kind = ? THEN 1 ELSE 0 END), 0) AS expense_count, COALESCE(SUM(CASE WHEN kind = ? THEN 1 ELSE 0 END), 0) AS income_count", KindExpense, KindIncome, KindExpense, KindIncome).
			Scan(&totals).Error; err != nil {
			response.ErrorInternal(c, "获取共享账单失败")
			return
		}
		response.Success(c, gin.H{
			"ledger": ledger, "members": members, "transactions": transactions,
			"transactions_total": totalTransactions, "transactions_page": page, "transactions_page_size": pageSize,
			"settlements": settlements, "balances": balances,
			"suggested_transfers": suggestTransfers(balances), "totals": totals,
		})
	}
}

func transactionPagination(c *gin.Context) (int, int) {
	page, pageSize := 1, 50
	if value, err := strconv.Atoi(c.Query("transaction_page")); err == nil && value > 0 {
		page = value
	}
	if value, err := strconv.Atoi(c.Query("transaction_page_size")); err == nil && value > 0 {
		pageSize = value
	}
	if pageSize > utils.MaxPageSize {
		pageSize = utils.MaxPageSize
	}
	return page, pageSize
}

func handleUpdateLedger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		if err := ownerMember(db, ledgerID, c.GetUint("userID")); err != nil {
			response.ErrorForbidden(c, err.Error())
			return
		}
		if err := writableLedger(db, ledgerID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		var req ledgerRequest
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
			response.ErrorBadRequest(c, "请输入账本名称")
			return
		}
		updates := map[string]interface{}{"name": cleanText(req.Name, 100)}
		if currency := strings.ToUpper(cleanText(req.Currency, 8)); currency != "" {
			updates["currency"] = currency
		}
		if req.ShowAccount != nil {
			updates["show_account"] = *req.ShowAccount
		}
		if req.ShowCategory != nil {
			updates["show_category"] = *req.ShowCategory
		}
		if req.ShowNote != nil {
			updates["show_note"] = *req.ShowNote
		}
		if err := db.Model(&SharedLedger{}).Where("id = ?", ledgerID).Updates(updates).Error; err != nil {
			response.ErrorInternal(c, "更新共享账本失败")
			return
		}
		response.Success(c, nil)
	}
}

func handleArchiveLedger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		if err := ownerMember(db, ledgerID, c.GetUint("userID")); err != nil {
			response.ErrorForbidden(c, err.Error())
			return
		}
		if err := writableLedger(db, ledgerID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		if err := db.Model(&SharedLedger{}).Where("id = ?", ledgerID).Update("archived", true).Error; err != nil {
			response.ErrorInternal(c, "归档账本失败")
			return
		}
		audit.Log(db, c, "shared_ledger_archive", "shared_ledger", ledgerID, "归档共享账本")
		response.Success(c, nil)
	}
}

func handleRestoreLedger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		if err := ownerMember(db, ledgerID, c.GetUint("userID")); err != nil {
			response.ErrorForbidden(c, err.Error())
			return
		}
		if err := db.Model(&SharedLedger{}).Where("id = ?", ledgerID).Update("archived", false).Error; err != nil {
			response.ErrorInternal(c, "恢复账本失败")
			return
		}
		audit.Log(db, c, "shared_ledger_restore", "shared_ledger", ledgerID, "恢复共享账本")
		response.Success(c, nil)
	}
}

func handleInviteMember(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		if err := ownerMember(db, ledgerID, c.GetUint("userID")); err != nil {
			response.ErrorForbidden(c, err.Error())
			return
		}
		if err := writableLedger(db, ledgerID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		var req struct {
			Username string `json:"username"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Username) == "" {
			response.ErrorBadRequest(c, "请输入用户名")
			return
		}
		var user database.User
		if err := db.Where("username = ? AND status = ?", strings.TrimSpace(req.Username), 1).First(&user).Error; err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "用户不存在")
			return
		}
		if user.ID == c.GetUint("userID") {
			response.ErrorBadRequest(c, "你已经是账本成员")
			return
		}
		var member SharedMember
		err := db.Where("ledger_id = ? AND user_id = ?", ledgerID, user.ID).First(&member).Error
		if err == nil {
			if member.Status == MemberActive || member.Status == MemberPending {
				response.ErrorBadRequest(c, "该用户已加入或已收到邀请")
				return
			}
			err = db.Model(&member).Updates(map[string]interface{}{"status": MemberPending, "role": MemberRegular}).Error
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			err = db.Create(&SharedMember{LedgerID: ledgerID, UserID: user.ID, Role: MemberRegular, Status: MemberPending}).Error
		}
		if err != nil {
			response.ErrorInternal(c, "邀请成员失败")
			return
		}
		response.Success(c, nil)
	}
}

func handleRespondInvitation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		if err := writableLedger(db, ledgerID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		var req struct {
			Accept bool `json:"accept"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请求无效")
			return
		}
		status := MemberRejected
		if req.Accept {
			status = MemberActive
		}
		result := db.Model(&SharedMember{}).
			Where("ledger_id = ? AND user_id = ? AND status = ?", ledgerID, c.GetUint("userID"), MemberPending).
			Update("status", status)
		if result.Error != nil {
			response.ErrorInternal(c, "处理邀请失败")
			return
		}
		if result.RowsAffected == 0 {
			response.ErrorBadRequest(c, "邀请不存在或已处理")
			return
		}
		response.Success(c, nil)
	}
}

func handleRemoveMember(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		memberUserID, ok := routeID(c, "userId")
		if !ok {
			return
		}
		if err := ownerMember(db, ledgerID, c.GetUint("userID")); err != nil {
			response.ErrorForbidden(c, err.Error())
			return
		}
		if err := writableLedger(db, ledgerID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		if memberUserID == c.GetUint("userID") {
			response.ErrorBadRequest(c, "账本所有者不能移除自己")
			return
		}
		var referenced int64
		if err := db.Model(&SharedTransaction{}).Where("ledger_id = ? AND actor_user_id = ?", ledgerID, memberUserID).Count(&referenced).Error; err != nil {
			response.ErrorInternal(c, "检查成员账单记录失败")
			return
		}
		if referenced == 0 {
			if err := db.Model(&SharedShare{}).
				Joins("JOIN shared_transactions ON shared_transactions.id = shared_shares.transaction_id").
				Where("shared_transactions.ledger_id = ? AND shared_shares.user_id = ?", ledgerID, memberUserID).
				Count(&referenced).Error; err != nil {
				response.ErrorInternal(c, "检查成员账单记录失败")
				return
			}
		}
		if referenced == 0 {
			if err := db.Model(&Settlement{}).
				Where("ledger_id = ? AND (from_user_id = ? OR to_user_id = ?)", ledgerID, memberUserID, memberUserID).
				Count(&referenced).Error; err != nil {
				response.ErrorInternal(c, "检查成员结算记录失败")
				return
			}
		}
		if referenced > 0 {
			response.ErrorBadRequest(c, "该成员已有账单记录，不能移除")
			return
		}
		if err := db.Where("ledger_id = ? AND user_id = ?", ledgerID, memberUserID).Delete(&SharedMember{}).Error; err != nil {
			response.ErrorInternal(c, "移除成员失败")
			return
		}
		response.Success(c, nil)
	}
}

func loadTransactions(db *gorm.DB, ledgerID uint, page, pageSize int) ([]transactionView, int64, error) {
	var total int64
	if err := db.Model(&SharedTransaction{}).Where("ledger_id = ?", ledgerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var raw []SharedTransaction
	if err := db.Where("ledger_id = ?", ledgerID).Order("occurred_at DESC, id DESC").Offset(utils.Offset(page, pageSize)).Limit(pageSize).Find(&raw).Error; err != nil {
		return nil, 0, err
	}
	ids := make([]uint, 0, len(raw)*2)
	for _, item := range raw {
		ids = append(ids, item.ActorUserID, item.CreatedBy)
	}
	users, err := userMap(db, ids)
	if err != nil {
		return nil, 0, err
	}
	transactionIDs := make([]uint, 0, len(raw))
	for _, item := range raw {
		transactionIDs = append(transactionIDs, item.ID)
	}
	attachments, err := loadBillAttachments(db, attachmentTypeShared, transactionIDs)
	if err != nil {
		return nil, 0, err
	}
	result := make([]transactionView, 0, len(raw))
	for _, item := range raw {
		item.Attachments = attachments[item.ID]
		actorName, actorOK := users[item.ActorUserID]
		creatorName, creatorOK := users[item.CreatedBy]
		if !actorOK || !creatorOK {
			return nil, 0, errNotFound
		}
		var shares []shareDetail
		if err := db.Table("shared_shares").
			Select("shared_shares.user_id, users.username, shared_shares.amount_cents").
			Joins("JOIN users ON users.id = shared_shares.user_id").
			Where("shared_shares.transaction_id = ?", item.ID).
			Order("shared_shares.id ASC").Scan(&shares).Error; err != nil {
			return nil, 0, err
		}
		result = append(result, transactionView{
			SharedTransaction: item, ActorUsername: actorName,
			CreatorUsername: creatorName, Shares: shares,
		})
		if item.AccountID > 0 {
			var account PersonalAccount
			if err := db.Select("id, name, type").First(&account, item.AccountID).Error; err == nil {
				result[len(result)-1].AccountName = account.Name
				result[len(result)-1].AccountType = account.Type
			}
		}
	}
	return result, total, nil
}

func activeMemberIDs(db *gorm.DB, ledgerID uint) ([]uint, error) {
	var ids []uint
	err := db.Model(&SharedMember{}).Where("ledger_id = ? AND status = ?", ledgerID, MemberActive).Pluck("user_id", &ids).Error
	return ids, err
}

func validateSharedRequest(db *gorm.DB, ledgerID uint, req sharedTransactionRequest) (SharedTransaction, []SharedShare, error) {
	if !validKind(req.Kind) || req.AmountCents <= 0 || req.ActorUserID == 0 {
		return SharedTransaction{}, nil, errInvalid
	}
	ledger, err := getLedger(db, ledgerID)
	if err != nil {
		return SharedTransaction{}, nil, err
	}
	category := cleanText(req.Category, 64)
	if ledger.ShowCategory && category == "" {
		return SharedTransaction{}, nil, errors.New("请选择分类")
	}
	if !ledger.ShowCategory {
		category = "未分类"
	}
	occurredAt, err := parseOccurredAt(req.OccurredAt)
	if err != nil {
		return SharedTransaction{}, nil, errors.New("发生时间格式不正确")
	}
	ids, err := activeMemberIDs(db, ledgerID)
	if err != nil {
		return SharedTransaction{}, nil, err
	}
	actorOK := false
	for _, id := range ids {
		if id == req.ActorUserID {
			actorOK = true
			break
		}
	}
	if !actorOK {
		return SharedTransaction{}, nil, errors.New("付款人或收款人必须是账本成员")
	}
	shares, err := normalizeShares(req.AmountCents, req.Shares, ids)
	if err != nil {
		return SharedTransaction{}, nil, err
	}
	item := SharedTransaction{
		LedgerID: ledgerID, Kind: req.Kind, ActorUserID: req.ActorUserID,
		AccountID: req.AccountID, AmountCents: req.AmountCents, Category: category, Note: cleanText(req.Note, 500),
		OccurredAt: occurredAt,
	}
	if !ledger.ShowAccount {
		item.AccountID = 0
	}
	if item.AccountID > 0 {
		if _, err := ownedPersonalAccount(db, item.ActorUserID, item.AccountID, true); err != nil {
			return SharedTransaction{}, nil, errors.New("收支账户不存在、已归档或不属于付款/收款人")
		}
	}
	if !ledger.ShowNote {
		item.Note = ""
	}
	return item, shares, nil
}

func handleCreateSharedTransaction(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		if _, err := activeMember(db, ledgerID, c.GetUint("userID")); err != nil {
			response.ErrorForbidden(c, err.Error())
			return
		}
		if err := writableLedger(db, ledgerID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		var req sharedTransactionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入完整账单信息")
			return
		}
		item, shares, err := validateSharedRequest(db, ledgerID, req)
		if err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		item.CreatedBy = c.GetUint("userID")
		attachments := []BillAttachment{}
		if req.Attachments != nil {
			attachments, err = validateBillAttachments(*req.Attachments)
			if err != nil {
				response.ErrorBadRequest(c, err.Error())
				return
			}
		}
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
			for i := range shares {
				shares[i].TransactionID = item.ID
			}
			if err := tx.Create(&shares).Error; err != nil {
				return err
			}
			return replaceBillAttachments(tx, attachmentTypeShared, item.ID, attachments)
		})
		if err != nil {
			response.ErrorInternal(c, "创建共享账单失败")
			return
		}
		item.Attachments = attachments
		audit.Log(db, c, "shared_transaction_create", "shared_transaction", item.ID, "新增共享账单")
		response.Success(c, item)
	}
}

func canEditShared(db *gorm.DB, ledgerID, transactionID, userID uint) (SharedTransaction, error) {
	var item SharedTransaction
	if err := db.Where("id = ? AND ledger_id = ?", transactionID, ledgerID).First(&item).Error; err != nil {
		return item, errNotFound
	}
	if item.CreatedBy == userID {
		return item, nil
	}
	if err := ownerMember(db, ledgerID, userID); err != nil {
		return item, errForbidden
	}
	return item, nil
}

func handleUpdateSharedTransaction(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		transactionID, ok := routeID(c, "transactionId")
		if !ok {
			return
		}
		existing, err := canEditShared(db, ledgerID, transactionID, c.GetUint("userID"))
		if err != nil {
			response.ErrorForbidden(c, err.Error())
			return
		}
		if err := writableLedger(db, ledgerID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		var req sharedTransactionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入完整账单信息")
			return
		}
		item, shares, err := validateSharedRequest(db, ledgerID, req)
		if err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		var attachments []BillAttachment
		if req.Attachments != nil {
			attachments, err = validateBillAttachments(*req.Attachments)
			if err != nil {
				response.ErrorBadRequest(c, err.Error())
				return
			}
		}
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&existing).Updates(map[string]interface{}{
				"kind": item.Kind, "actor_user_id": item.ActorUserID, "amount_cents": item.AmountCents,
				"account_id": item.AccountID, "category": item.Category, "note": item.Note, "occurred_at": item.OccurredAt,
			}).Error; err != nil {
				return err
			}
			if err := tx.Where("transaction_id = ?", existing.ID).Delete(&SharedShare{}).Error; err != nil {
				return err
			}
			for i := range shares {
				shares[i].TransactionID = existing.ID
			}
			if err := tx.Create(&shares).Error; err != nil {
				return err
			}
			if req.Attachments != nil {
				return replaceBillAttachments(tx, attachmentTypeShared, existing.ID, attachments)
			}
			return nil
		})
		if err != nil {
			response.ErrorInternal(c, "更新共享账单失败")
			return
		}
		response.Success(c, nil)
	}
}

func handleDeleteSharedTransaction(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		transactionID, ok := routeID(c, "transactionId")
		if !ok {
			return
		}
		if _, err := canEditShared(db, ledgerID, transactionID, c.GetUint("userID")); err != nil {
			response.ErrorForbidden(c, err.Error())
			return
		}
		if err := writableLedger(db, ledgerID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("transaction_id = ?", transactionID).Delete(&SharedShare{}).Error; err != nil {
				return err
			}
			if err := deleteBillAttachments(tx, attachmentTypeShared, transactionID); err != nil {
				return err
			}
			return tx.Where("id = ? AND ledger_id = ?", transactionID, ledgerID).Delete(&SharedTransaction{}).Error
		})
		if err != nil {
			response.ErrorInternal(c, "删除共享账单失败")
			return
		}
		response.Success(c, nil)
	}
}

func loadSettlements(db *gorm.DB, ledgerID uint) ([]settlementView, error) {
	var items []Settlement
	if err := db.Where("ledger_id = ?", ledgerID).Order("occurred_at DESC, id DESC").Limit(500).Find(&items).Error; err != nil {
		return nil, err
	}
	result := make([]settlementView, 0, len(items))
	for _, item := range items {
		names, err := userMap(db, []uint{item.FromUserID, item.ToUserID})
		if err != nil {
			return nil, err
		}
		if _, ok := names[item.FromUserID]; !ok {
			return nil, errNotFound
		}
		if _, ok := names[item.ToUserID]; !ok {
			return nil, errNotFound
		}
		result = append(result, settlementView{Settlement: item, FromUsername: names[item.FromUserID], ToUsername: names[item.ToUserID]})
	}
	return result, nil
}

func handleCreateSettlement(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		member, err := activeMember(db, ledgerID, c.GetUint("userID"))
		if err != nil {
			response.ErrorForbidden(c, err.Error())
			return
		}
		if err := writableLedger(db, ledgerID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		var req settlementRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.AmountCents <= 0 || req.FromUserID == req.ToUserID {
			response.ErrorBadRequest(c, "请输入有效的结算信息")
			return
		}
		if member.Role != MemberOwner && req.FromUserID != member.UserID && req.ToUserID != member.UserID {
			response.ErrorForbidden(c, "只能记录与你有关的结算")
			return
		}
		if _, err := activeMember(db, ledgerID, req.FromUserID); err != nil {
			response.ErrorBadRequest(c, "付款成员无效")
			return
		}
		if _, err := activeMember(db, ledgerID, req.ToUserID); err != nil {
			response.ErrorBadRequest(c, "收款成员无效")
			return
		}
		occurredAt, err := parseOccurredAt(req.OccurredAt)
		if err != nil {
			response.ErrorBadRequest(c, "结算时间格式不正确")
			return
		}
		item := Settlement{
			LedgerID: ledgerID, FromUserID: req.FromUserID, ToUserID: req.ToUserID,
			AmountCents: req.AmountCents, Note: cleanText(req.Note, 500),
			OccurredAt: occurredAt, CreatedBy: c.GetUint("userID"),
		}
		if err := db.Create(&item).Error; err != nil {
			response.ErrorInternal(c, "记录结算失败")
			return
		}
		audit.Log(db, c, "settlement_create", "settlement", item.ID, "记录共享账本结算")
		response.Success(c, item)
	}
}

func handleDeleteSettlement(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		ledgerID, ok := routeID(c, "id")
		if !ok {
			return
		}
		settlementID, ok := routeID(c, "settlementId")
		if !ok {
			return
		}
		var item Settlement
		if err := db.Where("id = ? AND ledger_id = ?", settlementID, ledgerID).First(&item).Error; err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "结算记录不存在")
			return
		}
		if err := writableLedger(db, ledgerID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		if item.CreatedBy != c.GetUint("userID") {
			if err := ownerMember(db, ledgerID, c.GetUint("userID")); err != nil {
				response.ErrorForbidden(c, "只有记录人或账本所有者可以删除")
				return
			}
		}
		if err := db.Delete(&item).Error; err != nil {
			response.ErrorInternal(c, "删除结算记录失败")
			return
		}
		response.Success(c, nil)
	}
}
