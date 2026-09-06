package billing

import (
	"errors"
	"sort"
	"strings"
	"time"

	"smallgo/server/database"

	"gorm.io/gorm"
)

var (
	errNotFound  = errors.New("记录不存在")
	errForbidden = errors.New("没有权限执行此操作")
	errInvalid   = errors.New("请求参数无效")
	errNotMember = errors.New("你不是该共享账本的成员")
)

type userBrief struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type memberView struct {
	ID        uint      `json:"id"`
	LedgerID  uint      `json:"ledger_id"`
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type shareInput struct {
	UserID      uint  `json:"user_id"`
	AmountCents int64 `json:"amount_cents"`
}

type balanceView struct {
	UserID       uint   `json:"user_id"`
	Username     string `json:"username"`
	BalanceCents int64  `json:"balance_cents"`
	PaidCents    int64  `json:"paid_cents"`
	ShareCents   int64  `json:"share_cents"`
}

type transferView struct {
	FromUserID   uint   `json:"from_user_id"`
	FromUsername string `json:"from_username"`
	ToUserID     uint   `json:"to_user_id"`
	ToUsername   string `json:"to_username"`
	AmountCents  int64  `json:"amount_cents"`
}

func validKind(kind string) bool {
	return kind == KindExpense || kind == KindIncome
}

func cleanText(value string, max int) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > max {
		return string([]rune(value)[:max])
	}
	return value
}

func parseOccurredAt(raw string) (time.Time, error) {
	if raw == "" {
		return time.Now(), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errInvalid
}

func getLedger(db *gorm.DB, ledgerID uint) (SharedLedger, error) {
	var ledger SharedLedger
	if err := db.First(&ledger, ledgerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ledger, errNotFound
		}
		return ledger, err
	}
	return ledger, nil
}

func writableLedger(db *gorm.DB, ledgerID uint) error {
	ledger, err := getLedger(db, ledgerID)
	if err != nil {
		return err
	}
	if ledger.Archived {
		return errors.New("该共享账本已归档，不能继续修改")
	}
	return nil
}

func activeMember(db *gorm.DB, ledgerID, userID uint) (SharedMember, error) {
	var member SharedMember
	err := db.Where("ledger_id = ? AND user_id = ? AND status = ?", ledgerID, userID, MemberActive).First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return member, errNotMember
	}
	return member, err
}

func ownerMember(db *gorm.DB, ledgerID, userID uint) error {
	member, err := activeMember(db, ledgerID, userID)
	if err != nil {
		return err
	}
	if member.Role != MemberOwner {
		return errForbidden
	}
	return nil
}

func loadMembers(db *gorm.DB, ledgerID uint, includePending bool) ([]memberView, error) {
	query := db.Table("shared_members").
		Select("shared_members.id, shared_members.ledger_id, shared_members.user_id, users.username, shared_members.role, shared_members.status, shared_members.created_at").
		Joins("JOIN users ON users.id = shared_members.user_id").
		Where("shared_members.ledger_id = ?", ledgerID)
	if !includePending {
		query = query.Where("shared_members.status = ?", MemberActive)
	}
	var members []memberView
	err := query.Order("shared_members.role DESC, users.username ASC").Scan(&members).Error
	return members, err
}

func normalizeShares(amount int64, shares []shareInput, memberIDs []uint) ([]SharedShare, error) {
	if amount <= 0 || len(memberIDs) == 0 {
		return nil, errInvalid
	}
	allowed := make(map[uint]bool, len(memberIDs))
	for _, id := range memberIDs {
		allowed[id] = true
	}
	if len(shares) == 0 {
		base := amount / int64(len(memberIDs))
		remainder := amount % int64(len(memberIDs))
		result := make([]SharedShare, 0, len(memberIDs))
		for i, id := range memberIDs {
			value := base
			if int64(i) < remainder {
				value++
			}
			result = append(result, SharedShare{UserID: id, AmountCents: value})
		}
		return result, nil
	}
	var total int64
	seen := map[uint]bool{}
	result := make([]SharedShare, 0, len(shares))
	for _, input := range shares {
		if !allowed[input.UserID] || seen[input.UserID] || input.AmountCents < 0 {
			return nil, errInvalid
		}
		seen[input.UserID] = true
		total += input.AmountCents
		result = append(result, SharedShare{UserID: input.UserID, AmountCents: input.AmountCents})
	}
	if total != amount {
		return nil, errors.New("分摊金额之和必须等于账单金额")
	}
	return result, nil
}

func calculateBalances(db *gorm.DB, ledgerID uint) ([]balanceView, error) {
	members, err := loadMembers(db, ledgerID, false)
	if err != nil {
		return nil, err
	}
	index := make(map[uint]*balanceView, len(members))
	result := make([]balanceView, len(members))
	for i, member := range members {
		result[i] = balanceView{UserID: member.UserID, Username: member.Username}
		index[member.UserID] = &result[i]
	}

	var transactions []SharedTransaction
	if err := db.Where("ledger_id = ?", ledgerID).Find(&transactions).Error; err != nil {
		return nil, err
	}
	for _, item := range transactions {
		actor := index[item.ActorUserID]
		if actor != nil {
			if item.Kind == KindExpense {
				actor.BalanceCents += item.AmountCents
				actor.PaidCents += item.AmountCents
			} else {
				actor.BalanceCents -= item.AmountCents
				actor.PaidCents -= item.AmountCents
			}
		}
		var shares []SharedShare
		if err := db.Where("transaction_id = ?", item.ID).Find(&shares).Error; err != nil {
			return nil, err
		}
		for _, share := range shares {
			member := index[share.UserID]
			if member == nil {
				continue
			}
			if item.Kind == KindExpense {
				member.BalanceCents -= share.AmountCents
				member.ShareCents += share.AmountCents
			} else {
				member.BalanceCents += share.AmountCents
				member.ShareCents -= share.AmountCents
			}
		}
	}

	var settlements []Settlement
	if err := db.Where("ledger_id = ?", ledgerID).Find(&settlements).Error; err != nil {
		return nil, err
	}
	for _, item := range settlements {
		if from := index[item.FromUserID]; from != nil {
			from.BalanceCents += item.AmountCents
		}
		if to := index[item.ToUserID]; to != nil {
			to.BalanceCents -= item.AmountCents
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].BalanceCents > result[j].BalanceCents
	})
	return result, nil
}

func suggestTransfers(balances []balanceView) []transferView {
	type account struct {
		id       uint
		username string
		amount   int64
	}
	var creditors, debtors []account
	for _, b := range balances {
		if b.BalanceCents > 0 {
			creditors = append(creditors, account{b.UserID, b.Username, b.BalanceCents})
		} else if b.BalanceCents < 0 {
			debtors = append(debtors, account{b.UserID, b.Username, -b.BalanceCents})
		}
	}
	sort.Slice(creditors, func(i, j int) bool { return creditors[i].amount > creditors[j].amount })
	sort.Slice(debtors, func(i, j int) bool { return debtors[i].amount > debtors[j].amount })

	var result []transferView
	for i, j := 0, 0; i < len(debtors) && j < len(creditors); {
		amount := debtors[i].amount
		if creditors[j].amount < amount {
			amount = creditors[j].amount
		}
		if amount > 0 {
			result = append(result, transferView{
				FromUserID: debtors[i].id, FromUsername: debtors[i].username,
				ToUserID: creditors[j].id, ToUsername: creditors[j].username,
				AmountCents: amount,
			})
		}
		debtors[i].amount -= amount
		creditors[j].amount -= amount
		if debtors[i].amount == 0 {
			i++
		}
		if creditors[j].amount == 0 {
			j++
		}
	}
	return result
}

func userMap(db *gorm.DB, ids []uint) (map[uint]string, error) {
	var users []database.User
	if err := db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	result := make(map[uint]string, len(users))
	for _, user := range users {
		result[user.ID] = user.Username
	}
	return result, nil
}
