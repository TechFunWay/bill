package billing

import "time"

const (
	KindExpense = "expense"
	KindIncome  = "income"

	MemberOwner   = "owner"
	MemberRegular = "member"

	MemberPending  = "pending"
	MemberActive   = "active"
	MemberRejected = "rejected"

	AccountTypeBankCard = "bank_card"
	AccountTypeWechat   = "wechat"
	AccountTypeAlipay   = "alipay"

	CardKindDebit  = "debit"
	CardKindCredit = "credit"
)

// PersonalTransaction is an income or expense owned by exactly one user.
// AmountCents is always positive; Kind determines the direction.
type PersonalTransaction struct {
	ID          uint             `gorm:"primarykey" json:"id"`
	UserID      uint             `gorm:"not null;index:idx_personal_user_time" json:"user_id"`
	LedgerID    uint             `gorm:"not null;default:0;index:idx_personal_ledger_time" json:"ledger_id"`
	AccountID   uint             `gorm:"not null;default:0;index" json:"account_id"`
	Kind        string           `gorm:"size:16;not null;index" json:"kind"`
	AmountCents int64            `gorm:"not null" json:"amount_cents"`
	Category    string           `gorm:"size:64;not null;index" json:"category"`
	Note        string           `gorm:"size:500" json:"note"`
	OccurredAt  time.Time        `gorm:"not null;index:idx_personal_user_time" json:"occurred_at"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	LedgerName  string           `gorm:"-" json:"ledger_name"`
	AccountName string           `gorm:"-" json:"account_name"`
	AccountType string           `gorm:"-" json:"account_type"`
	Tags        []PersonalTag    `gorm:"many2many:personal_transaction_tags;joinForeignKey:TransactionID;joinReferences:TagID" json:"tags"`
	Attachments []BillAttachment `gorm:"-" json:"attachments"`
}

// PersonalAccount is a bookkeeping account reference. It intentionally stores
// only display metadata: when supplied, complete card numbers are reduced to a
// keyed fingerprint plus last four digits, and no payment credentials are retained.
type PersonalAccount struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	UserID           uint      `gorm:"not null;index;uniqueIndex:idx_personal_account_user_name;uniqueIndex:idx_personal_account_user_card" json:"user_id"`
	Type             string    `gorm:"size:20;not null;index" json:"type"`
	Name             string    `gorm:"size:80;not null;uniqueIndex:idx_personal_account_user_name" json:"name"`
	Institution      string    `gorm:"size:80;not null" json:"institution"`
	Identifier       string    `gorm:"size:64" json:"identifier"`
	CardKind         string    `gorm:"size:16;not null;default:''" json:"card_kind"`
	CardFingerprint  *string   `gorm:"size:64;uniqueIndex:idx_personal_account_user_card" json:"-"`
	Archived         bool      `gorm:"not null;default:false;index" json:"archived"`
	TransactionCount int64     `gorm:"-" json:"transaction_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// PersonalLedger is private to one user. It is deliberately separate from
// SharedLedger so a one-person bookkeeping workflow never acquires membership
// and settlement semantics.
type PersonalLedger struct {
	ID                  uint      `gorm:"primarykey" json:"id"`
	UserID              uint      `gorm:"not null;index;uniqueIndex:idx_personal_ledger_user_name" json:"user_id"`
	Name                string    `gorm:"size:100;not null;uniqueIndex:idx_personal_ledger_user_name" json:"name"`
	Currency            string    `gorm:"size:8;not null;default:CNY" json:"currency"`
	Cover               string    `gorm:"size:512;not null;default:coral" json:"cover"`
	SortOrder           int       `gorm:"not null;default:0;index" json:"sort_order"`
	IsDefault           bool      `gorm:"not null;default:false;index" json:"is_default"`
	CategoriesSeeded    bool      `gorm:"not null;default:false" json:"-"`
	SubcategoriesSeeded bool      `gorm:"not null;default:false" json:"-"`
	Archived            bool      `gorm:"not null;default:false;index" json:"archived"`
	TransactionCount    int64     `gorm:"-" json:"transaction_count"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type PersonalCategory struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	UserID           uint      `gorm:"not null;index;uniqueIndex:idx_personal_category_user_kind_name" json:"user_id"`
	Kind             string    `gorm:"size:16;not null;uniqueIndex:idx_personal_category_user_kind_name" json:"kind"`
	Name             string    `gorm:"size:64;not null;uniqueIndex:idx_personal_category_user_kind_name" json:"name"`
	ParentID         uint      `gorm:"not null;default:0;index" json:"parent_id"`
	ParentName       string    `gorm:"-" json:"parent_name,omitempty"`
	IsDefault        bool      `gorm:"not null;default:false" json:"is_default"`
	TransactionCount int64     `gorm:"-" json:"transaction_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PersonalTag struct {
	ID               uint      `gorm:"primarykey" json:"id"`
	UserID           uint      `gorm:"not null;index;uniqueIndex:idx_personal_tag_user_name" json:"user_id"`
	Name             string    `gorm:"size:50;not null;uniqueIndex:idx_personal_tag_user_name" json:"name"`
	TransactionCount int64     `gorm:"-" json:"transaction_count"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PersonalTransactionTag struct {
	TransactionID uint `gorm:"primaryKey" json:"transaction_id"`
	TagID         uint `gorm:"primaryKey;index" json:"tag_id"`
}

type SharedLedger struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	Name         string    `gorm:"size:100;not null" json:"name"`
	Currency     string    `gorm:"size:8;not null;default:CNY" json:"currency"`
	OwnerUserID  uint      `gorm:"not null;index" json:"owner_user_id"`
	Archived     bool      `gorm:"not null;default:false;index" json:"archived"`
	ShowAccount  bool      `gorm:"not null;default:true" json:"show_account"`
	ShowCategory bool      `gorm:"not null;default:true" json:"show_category"`
	ShowNote     bool      `gorm:"not null;default:true" json:"show_note"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SharedMember struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	LedgerID  uint      `gorm:"not null;uniqueIndex:idx_ledger_user" json:"ledger_id"`
	UserID    uint      `gorm:"not null;uniqueIndex:idx_ledger_user;index" json:"user_id"`
	Role      string    `gorm:"size:16;not null;default:member" json:"role"`
	Status    string    `gorm:"size:16;not null;default:pending;index" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SharedTransaction represents either money paid for a group expense or
// money received for the group. ActorUserID is the payer for expenses and
// the receiver for income. Shares contain each member's economic share.
type SharedTransaction struct {
	ID          uint             `gorm:"primarykey" json:"id"`
	LedgerID    uint             `gorm:"not null;index:idx_shared_ledger_time" json:"ledger_id"`
	CreatedBy   uint             `gorm:"not null;index" json:"created_by"`
	Kind        string           `gorm:"size:16;not null;index" json:"kind"`
	ActorUserID uint             `gorm:"not null;index" json:"actor_user_id"`
	AccountID   uint             `gorm:"not null;default:0;index" json:"account_id"`
	AccountName string           `gorm:"-" json:"account_name"`
	AccountType string           `gorm:"-" json:"account_type"`
	AmountCents int64            `gorm:"not null" json:"amount_cents"`
	Category    string           `gorm:"size:64;not null;index" json:"category"`
	Note        string           `gorm:"size:500" json:"note"`
	OccurredAt  time.Time        `gorm:"not null;index:idx_shared_ledger_time" json:"occurred_at"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	Attachments []BillAttachment `gorm:"-" json:"attachments"`
}

// BillAttachment associates an uploaded image with either a personal or
// shared transaction. The physical file is managed by the existing upload
// service; this table only keeps the safe app-relative path and display name.
type BillAttachment struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	TransactionType string    `gorm:"size:16;not null;uniqueIndex:idx_bill_attachment_transaction_path" json:"-"`
	TransactionID   uint      `gorm:"not null;index;uniqueIndex:idx_bill_attachment_transaction_path" json:"-"`
	Path            string    `gorm:"size:512;not null;uniqueIndex:idx_bill_attachment_transaction_path" json:"path"`
	Name            string    `gorm:"size:160;not null" json:"name"`
	SortOrder       int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt       time.Time `json:"created_at"`
}

type SharedShare struct {
	ID            uint  `gorm:"primarykey" json:"id"`
	TransactionID uint  `gorm:"not null;uniqueIndex:idx_transaction_user" json:"transaction_id"`
	UserID        uint  `gorm:"not null;uniqueIndex:idx_transaction_user;index" json:"user_id"`
	AmountCents   int64 `gorm:"not null" json:"amount_cents"`
}

// Settlement records money actually transferred between members.
type Settlement struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	LedgerID    uint      `gorm:"not null;index:idx_settlement_ledger_time" json:"ledger_id"`
	FromUserID  uint      `gorm:"not null;index" json:"from_user_id"`
	ToUserID    uint      `gorm:"not null;index" json:"to_user_id"`
	AmountCents int64     `gorm:"not null" json:"amount_cents"`
	Note        string    `gorm:"size:500" json:"note"`
	OccurredAt  time.Time `gorm:"not null;index:idx_settlement_ledger_time" json:"occurred_at"`
	CreatedBy   uint      `gorm:"not null;index" json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}
