package billing

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"smallgo/server/database"
	userservice "smallgo/server/user"
)

func TestPersonalAccountConflictMessage(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{errors.New("UNIQUE constraint failed: personal_accounts.user_id, personal_accounts.card_fingerprint"), "这张银行卡已经添加过了"},
		{errors.New("UNIQUE constraint failed: personal_accounts.user_id, personal_accounts.name"), "账户名称已存在"},
		{errors.New("database is locked"), ""},
	}
	for _, test := range tests {
		if got := personalAccountConflictMessage(test.err); got != test.want {
			t.Fatalf("personalAccountConflictMessage(%q) = %q, want %q", test.err, got, test.want)
		}
	}
}

func TestNormalizeOpeningBalance(t *testing.T) {
	opening := int64(15000)
	if got, err := normalizeOpeningBalance(&opening); err != nil || got != 15000 {
		t.Fatalf("normalizeOpeningBalance(15000) = %d, %v", got, err)
	}
	if got, err := normalizeOpeningBalance(nil); err != nil || got != 0 {
		t.Fatalf("normalizeOpeningBalance(nil) = %d, %v", got, err)
	}
	outOfRange := int64(maxMoneyCents) + 1
	if _, err := normalizeOpeningBalance(&outOfRange); err == nil {
		t.Fatal("expected out-of-range opening balance to fail")
	}
	negative := int64(-500)
	if got, err := normalizeOpeningBalance(&negative); err != nil || got != -500 {
		t.Fatalf("negative opening balance should be allowed, got %d, %v", got, err)
	}
}

func TestLoadPersonalAccountBalances(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-account-balance.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	user := database.User{Username: "balance-owner", Password: "test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	accounts := []PersonalAccount{
		{UserID: user.ID, Type: AccountTypeWechat, Name: "日常微信", Institution: "微信", OpeningBalanceCents: 10000},
		{UserID: user.ID, Type: AccountTypeAlipay, Name: "生活支付宝", Institution: "支付宝", OpeningBalanceCents: -2000},
		{UserID: user.ID, Type: AccountTypeWechat, Name: "无流水账户", Institution: "微信"},
	}
	for i := range accounts {
		if err := db.Create(&accounts[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	transactions := []PersonalTransaction{
		{UserID: user.ID, AccountID: accounts[0].ID, Kind: KindIncome, AmountCents: 5000, Category: "工资", OccurredAt: time.Now()},
		{UserID: user.ID, AccountID: accounts[0].ID, Kind: KindExpense, AmountCents: 3000, Category: "餐饮", OccurredAt: time.Now()},
		{UserID: user.ID, AccountID: accounts[1].ID, Kind: KindExpense, AmountCents: 800, Category: "交通", OccurredAt: time.Now()},
	}
	for i := range transactions {
		if err := db.Create(&transactions[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := loadPersonalAccountBalances(db, user.ID, accounts); err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"日常微信": 12000, "生活支付宝": -2800, "无流水账户": 0}
	for _, account := range accounts {
		if account.BalanceCents != want[account.Name] {
			t.Fatalf("account %q balance = %d, want %d", account.Name, account.BalanceCents, want[account.Name])
		}
	}
}

func TestNormalizeSharesEqualKeepsExactTotal(t *testing.T) {
	shares, err := normalizeShares(100, nil, []uint{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != 3 {
		t.Fatalf("got %d shares, want 3", len(shares))
	}
	var total int64
	for _, share := range shares {
		total += share.AmountCents
	}
	if total != 100 {
		t.Fatalf("got total %d, want 100", total)
	}
	if shares[0].AmountCents != 34 || shares[1].AmountCents != 33 || shares[2].AmountCents != 33 {
		t.Fatalf("unexpected allocation: %#v", shares)
	}
}

func TestNormalizeSharesRejectsInvalidTotal(t *testing.T) {
	_, err := normalizeShares(100, []shareInput{
		{UserID: 1, AmountCents: 50},
		{UserID: 2, AmountCents: 49},
	}, []uint{1, 2})
	if err == nil {
		t.Fatal("expected invalid total to fail")
	}
}

func TestValidateBillAttachments(t *testing.T) {
	items, err := validateBillAttachments([]billAttachmentInput{
		{Path: "/uploads/2026/08/22/8ea090f7-93a0-4a42-a835-4e483998165e.jpg", Name: " 商场小票.jpg "},
		{Path: "/uploads/2026/08/22/12345678-1234-1234-1234-123456789abc.webp", Name: "现场/照片.webp"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Name != "商场小票.jpg" || items[1].Name != "现场照片.webp" {
		t.Fatalf("unexpected attachments: %#v", items)
	}
	if _, err := validateBillAttachments([]billAttachmentInput{{Path: "https://example.com/receipt.jpg", Name: "外部图片"}}); err == nil {
		t.Fatal("external attachment path must be rejected")
	}
	if _, err := validateBillAttachments([]billAttachmentInput{
		{Path: "/uploads/2026/08/22/8ea090f7-93a0-4a42-a835-4e483998165e.jpg", Name: "a"},
		{Path: "/uploads/2026/08/22/8ea090f7-93a0-4a42-a835-4e483998165e.jpg", Name: "b"},
	}); err == nil {
		t.Fatal("duplicate attachment path must be rejected")
	}
}

func TestReplaceAndLoadBillAttachmentsKeepsScopeAndOrder(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-attachments-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	personal := []BillAttachment{
		{Path: "/uploads/2026/08/22/11111111-1111-1111-1111-111111111111.jpg", Name: "first.jpg"},
		{Path: "/uploads/2026/08/22/22222222-2222-2222-2222-222222222222.png", Name: "second.png"},
	}
	shared := []BillAttachment{{Path: "/uploads/2026/08/22/33333333-3333-3333-3333-333333333333.webp", Name: "shared.webp"}}
	if err := replaceBillAttachments(db, attachmentTypePersonal, 7, personal); err != nil {
		t.Fatal(err)
	}
	if err := replaceBillAttachments(db, attachmentTypeShared, 7, shared); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadBillAttachments(db, attachmentTypePersonal, []uint{7})
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded[7]) != 2 || loaded[7][0].Name != "first.jpg" || loaded[7][1].Name != "second.png" {
		t.Fatalf("unexpected personal attachment order: %#v", loaded[7])
	}
	if err := replaceBillAttachments(db, attachmentTypePersonal, 7, personal[1:]); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadBillAttachments(db, attachmentTypePersonal, []uint{7})
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded[7]) != 1 || loaded[7][0].Name != "second.png" {
		t.Fatalf("unexpected replaced personal attachments: %#v", loaded[7])
	}
	sharedLoaded, err := loadBillAttachments(db, attachmentTypeShared, []uint{7})
	if err != nil {
		t.Fatal(err)
	}
	if len(sharedLoaded[7]) != 1 || sharedLoaded[7][0].Name != "shared.webp" {
		t.Fatalf("shared attachments were affected: %#v", sharedLoaded[7])
	}
}

func TestSuggestTransfersMinimizesSimpleBalances(t *testing.T) {
	transfers := suggestTransfers([]balanceView{
		{UserID: 1, Username: "A", BalanceCents: 700},
		{UserID: 2, Username: "B", BalanceCents: -200},
		{UserID: 3, Username: "C", BalanceCents: -500},
	})
	if len(transfers) != 2 {
		t.Fatalf("got %d transfers, want 2", len(transfers))
	}
	var total int64
	for _, transfer := range transfers {
		if transfer.ToUserID != 1 {
			t.Fatalf("unexpected creditor: %#v", transfer)
		}
		total += transfer.AmountCents
	}
	if total != 700 {
		t.Fatalf("got transfer total %d, want 700", total)
	}
}

// 复现粉丝反馈场景:共同收入记在 A 名下、共同支出由 B 垫付并均分。
// A 代收 1000 只应得 500、支出分摊 250,因此 A 应付 750 给 B,方向不能颠倒。
func TestCalculateBalancesOffsetsSharedIncomeAndExpense(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "shared-balance-offset.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}

	users := []database.User{{Username: "alice", Password: "test"}, {Username: "bob", Password: "test"}}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	aliceID, bobID := users[0].ID, users[1].ID
	ledger := SharedLedger{Name: "家庭账本", OwnerUserID: aliceID}
	if err := db.Create(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	members := []SharedMember{
		{LedgerID: ledger.ID, UserID: aliceID, Role: MemberOwner, Status: MemberActive},
		{LedgerID: ledger.ID, UserID: bobID, Role: MemberRegular, Status: MemberActive},
	}
	if err := db.Create(&members).Error; err != nil {
		t.Fatal(err)
	}
	income := SharedTransaction{LedgerID: ledger.ID, CreatedBy: aliceID, Kind: KindIncome,
		ActorUserID: aliceID, AmountCents: 1000, Category: "共同收入", OccurredAt: time.Now()}
	if err := db.Create(&income).Error; err != nil {
		t.Fatal(err)
	}
	expense := SharedTransaction{LedgerID: ledger.ID, CreatedBy: bobID, Kind: KindExpense,
		ActorUserID: bobID, AmountCents: 500, Category: "聚餐", OccurredAt: time.Now()}
	if err := db.Create(&expense).Error; err != nil {
		t.Fatal(err)
	}
	shares := []SharedShare{
		{TransactionID: income.ID, UserID: aliceID, AmountCents: 500},
		{TransactionID: income.ID, UserID: bobID, AmountCents: 500},
		{TransactionID: expense.ID, UserID: aliceID, AmountCents: 250},
		{TransactionID: expense.ID, UserID: bobID, AmountCents: 250},
	}
	if err := db.Create(&shares).Error; err != nil {
		t.Fatal(err)
	}

	balances, err := calculateBalances(db, ledger.ID)
	if err != nil {
		t.Fatal(err)
	}
	byUser := make(map[uint]balanceView, len(balances))
	for _, view := range balances {
		byUser[view.UserID] = view
	}
	alice, bob := byUser[aliceID], byUser[bobID]
	if alice.BalanceCents != -750 || bob.BalanceCents != 750 {
		t.Fatalf("balances alice=%d bob=%d, want -750/750", alice.BalanceCents, bob.BalanceCents)
	}
	if alice.ReceivedCents != 1000 || alice.AdvancedCents != 0 || bob.ReceivedCents != 0 || bob.AdvancedCents != 500 {
		t.Fatalf("handled amounts alice=(adv %d, recv %d) bob=(adv %d, recv %d)",
			alice.AdvancedCents, alice.ReceivedCents, bob.AdvancedCents, bob.ReceivedCents)
	}

	transfers := suggestTransfers(balances)
	if len(transfers) != 1 || transfers[0].FromUserID != aliceID || transfers[0].ToUserID != bobID || transfers[0].AmountCents != 750 {
		t.Fatalf("suggested transfers = %#v, want alice->bob 750", transfers)
	}
}

func TestBillingGuardProtectsReferencedUsers(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}

	user := database.User{Username: "schema-user", Password: "test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	item := PersonalTransaction{
		UserID: user.ID, Kind: KindExpense, AmountCents: 100,
		Category: "测试", OccurredAt: time.Now(),
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	blocked, err := database.UserDeletionBlocked(db, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Fatal("expected billing data to block user deletion")
	}
	if err := userservice.DeleteUser(db, user.ID); !errors.Is(err, userservice.ErrUserHasAppData) {
		t.Fatalf("got delete error %v, want ErrUserHasAppData", err)
	}
}

func TestBillingGuardIgnoresLazyDefaultSetup(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-defaults-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	user := database.User{Username: "defaults-only", Password: "test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ensurePersonalSetup(db, user.ID); err != nil {
		t.Fatal(err)
	}
	blocked, err := hasBillingData(db, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if blocked {
		t.Fatal("lazy default ledger and categories must not block deletion of an otherwise empty account")
	}
	deletable := database.User{Username: "defaults-delete", Password: "test"}
	if err := db.Create(&deletable).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ensurePersonalSetup(db, deletable.ID); err != nil {
		t.Fatal(err)
	}
	if err := userservice.DeleteUser(db, deletable.ID); err != nil {
		t.Fatalf("delete defaults-only user: %v", err)
	}
	var orphaned int64
	if err := db.Model(&PersonalLedger{}).Where("user_id = ?", deletable.ID).Count(&orphaned).Error; err != nil {
		t.Fatal(err)
	}
	if orphaned != 0 {
		t.Fatalf("defaults-only user left %d personal ledgers behind", orphaned)
	}
	custom := PersonalCategory{UserID: user.ID, Kind: KindExpense, Name: "自定义"}
	if err := db.Create(&custom).Error; err != nil {
		t.Fatal(err)
	}
	blocked, err = hasBillingData(db, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Fatal("custom billing metadata should still protect the account")
	}
}

func TestPersonalLedgerUpgradeMigratesLegacyRows(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-legacy-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := db.Exec(`CREATE TABLE personal_transactions (
		id integer PRIMARY KEY AUTOINCREMENT,
		user_id integer NOT NULL,
		kind text NOT NULL,
		amount_cents integer NOT NULL,
		category text NOT NULL,
		note text,
		occurred_at datetime NOT NULL,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO personal_transactions (user_id, kind, amount_cents, category, occurred_at) VALUES (?, ?, ?, ?, ?)", 7, KindExpense, 888, "旧账单", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate legacy billing schema: %v", err)
	}
	user := database.User{ID: 7, Username: "legacy-upgrade", Password: "test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	ledger, err := ensurePersonalSetup(db, user.ID)
	if err != nil {
		t.Fatalf("lazy migrate legacy transaction: %v", err)
	}
	var transaction PersonalTransaction
	if err := db.First(&transaction).Error; err != nil {
		t.Fatal(err)
	}
	if transaction.LedgerID != ledger.ID || transaction.LedgerID == 0 {
		t.Fatalf("legacy transaction ledger_id = %d, want %d", transaction.LedgerID, ledger.ID)
	}
	if transaction.AccountID != 0 {
		t.Fatalf("legacy transaction account_id = %d, want 0", transaction.AccountID)
	}
}

func TestBillingGuardProtectsPersonalAccounts(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-account-guard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	user := database.User{Username: "account-owner", Password: "test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&PersonalAccount{UserID: user.ID, Type: AccountTypeWechat, Name: "日常微信", Institution: "微信"}).Error; err != nil {
		t.Fatal(err)
	}
	blocked, err := hasBillingData(db, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !blocked {
		t.Fatal("personal account should block user deletion")
	}
}

func TestPersonalLedgerMetadataUpgradePreservesExistingLedgers(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-ledger-metadata-upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := db.Exec(`CREATE TABLE personal_ledgers (
		id integer PRIMARY KEY AUTOINCREMENT,
		user_id integer NOT NULL,
		name text NOT NULL,
		currency text NOT NULL DEFAULT 'CNY',
		is_default numeric NOT NULL DEFAULT false,
		categories_seeded numeric NOT NULL DEFAULT false,
		archived numeric NOT NULL DEFAULT false,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO personal_ledgers (user_id, name, currency, is_default) VALUES (?, ?, ?, ?)", 9, "旧日常账本", "CNY", true).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate personal ledger metadata: %v", err)
	}
	var ledger PersonalLedger
	if err := db.Where("user_id = ?", 9).First(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	if ledger.Name != "旧日常账本" || ledger.Cover != defaultPersonalLedgerCover || ledger.SortOrder != 0 {
		t.Fatalf("legacy personal ledger metadata = %#v", ledger)
	}
}

func TestPersonalAccountCardUpgradePreservesLegacyAccounts(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-account-card-upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := db.Exec(`CREATE TABLE personal_accounts (
		id integer PRIMARY KEY AUTOINCREMENT,
		user_id integer NOT NULL,
		type text NOT NULL,
		name text NOT NULL,
		institution text NOT NULL,
		identifier text,
		archived numeric NOT NULL DEFAULT false,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO personal_accounts (user_id, type, name, institution, identifier) VALUES (?, ?, ?, ?, ?), (?, ?, ?, ?, ?)", 9, AccountTypeBankCard, "旧工资卡", "招商银行", "1234", 9, AccountTypeBankCard, "旧生活卡", "中国银行", "5678").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate personal account card metadata: %v", err)
	}
	var accounts []PersonalAccount
	if err := db.Order("id ASC").Find(&accounts).Error; err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || accounts[0].Identifier != "1234" || accounts[0].CardKind != "" || accounts[0].CardFingerprint != nil || accounts[1].CardFingerprint != nil {
		t.Fatalf("legacy personal accounts = %#v", accounts)
	}
}

func TestPersonalCategoryHierarchyUpgradePreservesLegacyCategories(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-category-hierarchy-upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := db.Exec(`CREATE TABLE personal_categories (
		id integer PRIMARY KEY AUTOINCREMENT,
		user_id integer NOT NULL,
		kind text NOT NULL,
		name text NOT NULL,
		is_default numeric NOT NULL DEFAULT false,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_personal_category_user_kind_name ON personal_categories(user_id, kind, name)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO personal_categories (user_id, kind, name) VALUES (?, ?, ?)", 9, KindExpense, "旧分类").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate personal category hierarchy: %v", err)
	}
	var category PersonalCategory
	if err := db.Where("user_id = ? AND name = ?", 9, "旧分类").First(&category).Error; err != nil {
		t.Fatal(err)
	}
	if category.ParentID != 0 {
		t.Fatalf("legacy category parent_id = %d, want 0", category.ParentID)
	}
}

func TestSharedLedgerFieldSettingsUpgradeEnablesExistingFields(t *testing.T) {
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-shared-fields-upgrade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.CloseDB(db)
	if err := db.Exec(`CREATE TABLE shared_ledgers (
		id integer PRIMARY KEY AUTOINCREMENT,
		name text NOT NULL,
		currency text NOT NULL DEFAULT 'CNY',
		owner_user_id integer NOT NULL,
		archived numeric NOT NULL DEFAULT false,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO shared_ledgers (name, currency, owner_user_id) VALUES (?, ?, ?)", "旧共享账本", "CNY", 7).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("auto migrate shared field settings: %v", err)
	}
	var ledger SharedLedger
	if err := db.First(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	if !ledger.ShowAccount || !ledger.ShowCategory || !ledger.ShowNote {
		t.Fatalf("legacy shared field settings = %#v, want enabled", ledger)
	}
}
