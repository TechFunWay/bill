package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"smallgo/server/billing"
	"smallgo/server/database"
)

func billingData(t *testing.T, wCode int, body string) map[string]interface{} {
	t.Helper()
	if wCode != http.StatusOK {
		t.Fatalf("billing request status %d: %s", wCode, body)
	}
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("decode billing response: %v", err)
	}
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("billing response has unexpected data: %s", body)
	}
	return data
}

func TestPersonalLedgersCategoriesTagsAndLegacyMigration(t *testing.T) {
	r, db, _ := setupTestRouter(t)
	aliceToken := registerUser(t, r, "personal-alice", "secret123")
	bobToken := registerUser(t, r, "personal-bob", "secret123")

	var alice, bob database.User
	if err := db.Where("username = ?", "personal-alice").First(&alice).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("username = ?", "personal-bob").First(&bob).Error; err != nil {
		t.Fatal(err)
	}
	legacy := billing.PersonalTransaction{UserID: alice.ID, Kind: billing.KindExpense, AmountCents: 123, Category: "旧分类", OccurredAt: time.Now()}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}

	ledgers := doJSON(r, http.MethodGet, "/api/billing/personal/ledgers", aliceToken, nil)
	var ledgerResponse map[string]interface{}
	if err := json.Unmarshal(ledgers.Body.Bytes(), &ledgerResponse); err != nil {
		t.Fatal(err)
	}
	ledgerItems := ledgerResponse["data"].([]interface{})
	if len(ledgerItems) != 1 {
		t.Fatalf("default ledger count = %d, want 1", len(ledgerItems))
	}
	defaultLedger := ledgerItems[0].(map[string]interface{})
	defaultLedgerID := uint(defaultLedger["id"].(float64))
	if defaultLedger["name"] != "日常账本" || !defaultLedger["is_default"].(bool) || defaultLedger["cover"] != "coral" {
		t.Fatalf("unexpected default ledger: %#v", defaultLedger)
	}
	if err := db.First(&legacy, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if legacy.LedgerID != defaultLedgerID {
		t.Fatalf("legacy ledger_id = %d, want %d", legacy.LedgerID, defaultLedgerID)
	}
	var diningCategory billing.PersonalCategory
	if err := db.Where("user_id = ? AND kind = ? AND name = ?", alice.ID, billing.KindExpense, "餐饮").First(&diningCategory).Error; err != nil {
		t.Fatalf("default categories were not seeded: %v", err)
	}
	var seededSubcategory billing.PersonalCategory
	if err := db.Where("user_id = ? AND kind = ? AND name = ?", alice.ID, billing.KindExpense, "早餐").First(&seededSubcategory).Error; err != nil {
		t.Fatalf("default subcategories were not seeded: %v", err)
	}
	if seededSubcategory.ParentID != diningCategory.ID {
		t.Fatalf("seeded subcategory parent = %d, want %d", seededSubcategory.ParentID, diningCategory.ID)
	}
	// "其他" is the only expense default without second-level children, so it
	// can be deleted outright; parents with children must first remove them.
	var seededCategory billing.PersonalCategory
	if err := db.Where("user_id = ? AND kind = ? AND name = ?", alice.ID, billing.KindExpense, "其他").First(&seededCategory).Error; err != nil {
		t.Fatalf("default categories were not seeded: %v", err)
	}
	if got := doJSON(r, http.MethodDelete, fmt.Sprintf("/api/billing/personal/categories/%d", seededCategory.ID), aliceToken, nil).Code; got != http.StatusOK {
		t.Fatalf("delete seeded category = %d", got)
	}
	if got := doJSON(r, http.MethodGet, "/api/billing/personal/categories", aliceToken, nil).Code; got != http.StatusOK {
		t.Fatalf("reload categories = %d", got)
	}
	var seededCount int64
	if err := db.Model(&billing.PersonalCategory{}).Where("id = ?", seededCategory.ID).Count(&seededCount).Error; err != nil || seededCount != 0 {
		t.Fatalf("deleted category was recreated: count=%d err=%v", seededCount, err)
	}

	createdLedger := doJSON(r, http.MethodPost, "/api/billing/personal/ledgers", aliceToken, map[string]interface{}{"name": "交通账本", "currency": "CNY", "cover": "ocean"})
	createdLedgerData := billingData(t, createdLedger.Code, createdLedger.Body.String())
	transportLedgerID := uint(createdLedgerData["id"].(float64))
	if createdLedgerData["cover"] != "ocean" {
		t.Fatalf("created ledger cover = %#v, want ocean", createdLedgerData["cover"])
	}
	if got := doJSON(r, http.MethodPut, fmt.Sprintf("/api/billing/personal/ledgers/%d", transportLedgerID), aliceToken, map[string]interface{}{"name": "通勤账本", "currency": "CNY"}); got.Code != http.StatusOK || billingData(t, got.Code, got.Body.String())["cover"] != "ocean" {
		t.Fatalf("ledger update without cover did not preserve it: %d %s", got.Code, got.Body.String())
	}
	if got := doJSON(r, http.MethodPut, fmt.Sprintf("/api/billing/personal/ledgers/%d", transportLedgerID), aliceToken, map[string]interface{}{"name": "通勤账本", "currency": "CNY", "cover": "https://example.com/cover.jpg"}).Code; got != http.StatusBadRequest {
		t.Fatalf("external ledger cover = %d, want 400", got)
	}
	customCover := "/uploads/2026/08/21/12345678-1234-1234-1234-123456789abc.jpg"
	customized := doJSON(r, http.MethodPut, fmt.Sprintf("/api/billing/personal/ledgers/%d", transportLedgerID), aliceToken, map[string]interface{}{"name": "通勤账本", "currency": "CNY", "cover": customCover})
	if customized.Code != http.StatusOK || billingData(t, customized.Code, customized.Body.String())["cover"] != customCover {
		t.Fatalf("custom ledger cover was not saved: %d %s", customized.Code, customized.Body.String())
	}
	ordered := doJSON(r, http.MethodPut, "/api/billing/personal/ledgers/order", aliceToken, map[string]interface{}{"ledger_ids": []uint{transportLedgerID, defaultLedgerID}})
	if ordered.Code != http.StatusOK {
		t.Fatalf("order personal ledgers: %d %s", ordered.Code, ordered.Body.String())
	}
	orderedList := doJSON(r, http.MethodGet, "/api/billing/personal/ledgers", aliceToken, nil)
	var orderedResponse map[string]interface{}
	if err := json.Unmarshal(orderedList.Body.Bytes(), &orderedResponse); err != nil {
		t.Fatal(err)
	}
	orderedItems := orderedResponse["data"].([]interface{})
	if uint(orderedItems[0].(map[string]interface{})["id"].(float64)) != transportLedgerID {
		t.Fatalf("personal ledger order was not persisted: %s", orderedList.Body.String())
	}
	if got := doJSON(r, http.MethodPut, "/api/billing/personal/ledgers/order", aliceToken, map[string]interface{}{"ledger_ids": []uint{transportLedgerID, transportLedgerID}}).Code; got != http.StatusBadRequest {
		t.Fatalf("duplicate personal ledger order = %d, want 400", got)
	}
	if got := doJSON(r, http.MethodPut, "/api/billing/personal/ledgers/order", bobToken, map[string]interface{}{"ledger_ids": []uint{transportLedgerID, defaultLedgerID}}).Code; got != http.StatusBadRequest {
		t.Fatalf("outsider ledger order = %d, want 400", got)
	}
	if got := doJSON(r, http.MethodPost, "/api/billing/personal/ledgers", aliceToken, map[string]interface{}{"name": "美元账本", "currency": "USD"}).Code; got != http.StatusBadRequest {
		t.Fatalf("mixed-currency personal ledger = %d, want 400", got)
	}
	if got := doJSON(r, http.MethodPut, fmt.Sprintf("/api/billing/personal/ledgers/%d", transportLedgerID), bobToken, map[string]interface{}{"name": "越权"}).Code; got != http.StatusNotFound {
		t.Fatalf("outsider ledger update = %d, want 404", got)
	}

	categoryParentCreated := doJSON(r, http.MethodPost, "/api/billing/personal/categories", aliceToken, map[string]interface{}{"kind": "expense", "name": "出行方式", "parent_id": 0})
	categoryParentID := uint(billingData(t, categoryParentCreated.Code, categoryParentCreated.Body.String())["id"].(float64))
	categoryCreated := doJSON(r, http.MethodPost, "/api/billing/personal/categories", aliceToken, map[string]interface{}{"kind": "expense", "name": "通勤", "parent_id": categoryParentID})
	categoryID := uint(billingData(t, categoryCreated.Code, categoryCreated.Body.String())["id"].(float64))
	categoryList := doJSON(r, http.MethodGet, "/api/billing/personal/categories?kind=expense", aliceToken, nil)
	if categoryList.Code != http.StatusOK {
		t.Fatalf("list categories: %d %s", categoryList.Code, categoryList.Body.String())
	}
	var categoryListResponse map[string]interface{}
	if err := json.Unmarshal(categoryList.Body.Bytes(), &categoryListResponse); err != nil {
		t.Fatal(err)
	}
	categoryItems := categoryListResponse["data"].([]interface{})
	foundChild := false
	for _, raw := range categoryItems {
		item := raw.(map[string]interface{})
		if item["name"] == "通勤" && item["parent_name"] == "出行方式" {
			foundChild = true
		}
	}
	if !foundChild {
		t.Fatalf("second-level category missing parent metadata: %s", categoryList.Body.String())
	}
	if got := doJSON(r, http.MethodPost, "/api/billing/personal/categories", aliceToken, map[string]interface{}{"kind": "expense", "name": "地铁", "parent_id": categoryID}).Code; got != http.StatusBadRequest {
		t.Fatalf("third-level category = %d, want 400", got)
	}
	if got := doJSON(r, http.MethodDelete, fmt.Sprintf("/api/billing/personal/categories/%d", categoryParentID), aliceToken, nil).Code; got != http.StatusBadRequest {
		t.Fatalf("delete category parent with children = %d, want 400", got)
	}
	tagCreated := doJSON(r, http.MethodPost, "/api/billing/personal/tags", aliceToken, map[string]interface{}{"name": "报销"})
	tagID := uint(billingData(t, tagCreated.Code, tagCreated.Body.String())["id"].(float64))
	tagCreated2 := doJSON(r, http.MethodPost, "/api/billing/personal/tags", aliceToken, map[string]interface{}{"name": "地铁"})
	tagID2 := uint(billingData(t, tagCreated2.Code, tagCreated2.Body.String())["id"].(float64))
	options := doJSON(r, http.MethodGet, "/api/billing/personal/account-options", aliceToken, nil)
	optionData := billingData(t, options.Code, options.Body.String())
	if got := len(optionData["banks"].([]interface{})); got < 70 {
		t.Fatalf("bank options = %d, want at least 70", got)
	}
	firstBank := optionData["banks"].([]interface{})[0].(map[string]interface{})
	if firstBank["short"] == "" || firstBank["color"] == "" {
		t.Fatalf("bank icon metadata missing: %#v", firstBank)
	}
	if got := len(optionData["card_kinds"].([]interface{})); got != 2 {
		t.Fatalf("card kinds = %d, want 2", got)
	}
	if got := doJSON(r, http.MethodPost, "/api/billing/personal/accounts", aliceToken, map[string]interface{}{"type": "bank_card", "name": "错误卡号", "institution": "招商银行", "card_kind": "debit", "card_number": "4111111111111112"}).Code; got != http.StatusBadRequest {
		t.Fatalf("invalid bank card number = %d, want 400", got)
	}
	cardWithoutNumber := doJSON(r, http.MethodPost, "/api/billing/personal/accounts", aliceToken, map[string]interface{}{"type": "bank_card", "name": "待补卡号", "institution": "中国建设银行", "card_kind": "debit"})
	cardWithoutNumberData := billingData(t, cardWithoutNumber.Code, cardWithoutNumber.Body.String())
	cardWithoutNumberID := uint(cardWithoutNumberData["id"].(float64))
	if cardWithoutNumberData["identifier"] != "" || strings.Contains(cardWithoutNumber.Body.String(), "card_fingerprint") {
		t.Fatalf("card without number leaked or invented identity: %s", cardWithoutNumber.Body.String())
	}
	cardNumberAdded := doJSON(r, http.MethodPut, fmt.Sprintf("/api/billing/personal/accounts/%d", cardWithoutNumberID), aliceToken, map[string]interface{}{"type": "bank_card", "name": "待补卡号", "institution": "中国建设银行", "card_kind": "debit", "card_number": "6011111111111117"})
	cardNumberAddedData := billingData(t, cardNumberAdded.Code, cardNumberAdded.Body.String())
	if cardNumberAddedData["identifier"] != "1117" {
		t.Fatalf("later card number update failed: %s", cardNumberAdded.Body.String())
	}
	accountCreated := doJSON(r, http.MethodPost, "/api/billing/personal/accounts", aliceToken, map[string]interface{}{"type": "bank_card", "name": "工资卡", "institution": "招商银行", "card_kind": "debit", "card_number": "4111 1111 1111 1111"})
	accountData := billingData(t, accountCreated.Code, accountCreated.Body.String())
	accountID := uint(accountData["id"].(float64))
	if accountData["identifier"] != "1111" || accountData["card_kind"] != "debit" || strings.Contains(accountCreated.Body.String(), "4111111111111111") || strings.Contains(accountCreated.Body.String(), "card_fingerprint") {
		t.Fatalf("unsafe or incomplete bank card response: %s", accountCreated.Body.String())
	}
	if got := doJSON(r, http.MethodPost, "/api/billing/personal/accounts", aliceToken, map[string]interface{}{"type": "bank_card", "name": "重复卡", "institution": "中国银行", "card_kind": "credit", "card_number": "4111111111111111"}).Code; got != http.StatusBadRequest {
		t.Fatalf("duplicate bank card = %d, want 400", got)
	}
	secondCard := doJSON(r, http.MethodPost, "/api/billing/personal/accounts", aliceToken, map[string]interface{}{"type": "bank_card", "name": "生活卡", "institution": "中国银行", "card_kind": "credit", "card_number": "5555555555554444"})
	secondCardID := uint(billingData(t, secondCard.Code, secondCard.Body.String())["id"].(float64))
	if got := doJSON(r, http.MethodPut, fmt.Sprintf("/api/billing/personal/accounts/%d", secondCardID), aliceToken, map[string]interface{}{"type": "bank_card", "name": "生活卡", "institution": "中国银行", "card_kind": "credit", "card_number": "4111111111111111"}).Code; got != http.StatusBadRequest {
		t.Fatalf("replace with duplicate bank card = %d, want 400", got)
	}
	updatedCard := doJSON(r, http.MethodPut, fmt.Sprintf("/api/billing/personal/accounts/%d", accountID), aliceToken, map[string]interface{}{"type": "bank_card", "name": "工资卡", "institution": "招商银行", "card_kind": "credit"})
	updatedCardData := billingData(t, updatedCard.Code, updatedCard.Body.String())
	if updatedCardData["identifier"] != "1111" || updatedCardData["card_kind"] != "credit" {
		t.Fatalf("bank card metadata update lost card identity: %s", updatedCard.Body.String())
	}
	if got := doJSON(r, http.MethodPost, "/api/billing/personal/transactions", aliceToken, map[string]interface{}{"ledger_id": transportLedgerID, "kind": "expense", "amount_cents": 100, "category": "通勤"}).Code; got != http.StatusBadRequest {
		t.Fatalf("personal transaction without account = %d, want 400", got)
	}

	transactionCreated := doJSON(r, http.MethodPost, "/api/billing/personal/transactions", aliceToken, map[string]interface{}{
		"ledger_id": transportLedgerID, "account_id": accountID, "kind": "expense", "amount_cents": 500, "category": "通勤", "note": "地铁", "tag_ids": []uint{tagID, tagID2},
	})
	transaction := billingData(t, transactionCreated.Code, transactionCreated.Body.String())
	if transaction["ledger_name"] != "通勤账本" || transaction["account_name"] != "工资卡" || len(transaction["tags"].([]interface{})) != 2 {
		t.Fatalf("transaction relations missing: %#v", transaction)
	}
	if got := doJSON(r, http.MethodPut, fmt.Sprintf("/api/billing/personal/categories/%d", categoryID), aliceToken, map[string]interface{}{"kind": "income", "name": "通勤"}).Code; got != http.StatusBadRequest {
		t.Fatalf("used category kind change = %d, want 400", got)
	}

	renamed := doJSON(r, http.MethodPut, fmt.Sprintf("/api/billing/personal/categories/%d", categoryID), aliceToken, map[string]interface{}{"kind": "expense", "name": "公共交通"})
	if renamed.Code != http.StatusOK {
		t.Fatalf("rename category: %d %s", renamed.Code, renamed.Body.String())
	}
	var persisted billing.PersonalTransaction
	if err := db.Where("user_id = ? AND amount_cents = ?", alice.ID, 500).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Category != "公共交通" {
		t.Fatalf("historical category = %q", persisted.Category)
	}
	if got := doJSON(r, http.MethodDelete, fmt.Sprintf("/api/billing/personal/categories/%d", categoryID), aliceToken, nil).Code; got != http.StatusOK {
		t.Fatalf("delete category = %d", got)
	}
	if err := db.First(&persisted, persisted.ID).Error; err != nil || persisted.Category != "公共交通" {
		t.Fatalf("category deletion damaged history: %#v %v", persisted, err)
	}

	filtered := doJSON(r, http.MethodGet, fmt.Sprintf("/api/billing/personal/transactions?ledger_id=%d&account_id=%d&tag_id=%d", transportLedgerID, accountID, tagID), aliceToken, nil)
	filteredData := billingData(t, filtered.Code, filtered.Body.String())
	if filteredData["total"].(float64) != 1 {
		t.Fatalf("filtered total = %#v, want 1", filteredData["total"])
	}
	if got := doJSON(r, http.MethodDelete, fmt.Sprintf("/api/billing/personal/accounts/%d", accountID), aliceToken, nil).Code; got != http.StatusOK {
		t.Fatalf("archive account = %d, want 200", got)
	}
	if got := doJSON(r, http.MethodPost, "/api/billing/personal/transactions", aliceToken, map[string]interface{}{"ledger_id": transportLedgerID, "account_id": accountID, "kind": "expense", "amount_cents": 100, "category": "通勤"}).Code; got != http.StatusBadRequest {
		t.Fatalf("transaction with archived account = %d, want 400", got)
	}
	stats := doJSON(r, http.MethodGet, fmt.Sprintf("/api/billing/personal/stats?ledger_id=%d", transportLedgerID), aliceToken, nil)
	statsData := billingData(t, stats.Code, stats.Body.String())
	if len(statsData["ledgers"].([]interface{})) != 1 || len(statsData["tags"].([]interface{})) != 2 || len(statsData["weekdays"].([]interface{})) != 7 {
		t.Fatalf("missing stats dimensions: %#v", statsData)
	}

	if got := doJSON(r, http.MethodDelete, fmt.Sprintf("/api/billing/personal/tags/%d", tagID), aliceToken, nil).Code; got != http.StatusOK {
		t.Fatalf("delete tag = %d", got)
	}
	var transactionCount int64
	if err := db.Model(&billing.PersonalTransaction{}).Where("id = ?", persisted.ID).Count(&transactionCount).Error; err != nil || transactionCount != 1 {
		t.Fatalf("tag deletion damaged transaction: count=%d err=%v", transactionCount, err)
	}
}

func TestSharedLedgerArchiveRestorePaginationAndInvitations(t *testing.T) {
	r, db, _ := setupTestRouter(t)
	aliceToken := registerUser(t, r, "alice", "secret123")
	bobToken := registerUser(t, r, "bob", "secret123")
	charlieToken := registerUser(t, r, "charlie", "secret123")
	registerUser(t, r, "disabled", "secret123")
	if err := db.Model(&database.User{}).Where("username = ?", "disabled").Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}

	// A single invalid (or disabled) invite rejects the whole request, so no
	// owner member or ledger is left behind.
	invalid := doJSON(r, http.MethodPost, "/api/billing/ledgers", aliceToken, map[string]interface{}{
		"name": "should not exist", "usernames": []string{"missing-user"},
	})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid invite status = %d, want 400: %s", invalid.Code, invalid.Body.String())
	}
	var ledgerCount int64
	if err := db.Model(&billing.SharedLedger{}).Count(&ledgerCount).Error; err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 0 {
		t.Fatalf("invalid invitation created %d ledgers", ledgerCount)
	}
	disabled := doJSON(r, http.MethodPost, "/api/billing/ledgers", aliceToken, map[string]interface{}{
		"name": "also should not exist", "usernames": []string{"disabled"},
	})
	if disabled.Code != http.StatusBadRequest {
		t.Fatalf("disabled invite status = %d, want 400: %s", disabled.Code, disabled.Body.String())
	}
	if err := db.Model(&billing.SharedLedger{}).Count(&ledgerCount).Error; err != nil {
		t.Fatal(err)
	}
	if ledgerCount != 0 {
		t.Fatalf("disabled invitation created %d ledgers", ledgerCount)
	}

	created := doJSON(r, http.MethodPost, "/api/billing/ledgers", aliceToken, map[string]interface{}{
		"name": "trip", "currency": "cny", "usernames": []string{"alice", "bob", "bob"},
	})
	data := billingData(t, created.Code, created.Body.String())
	ledger := data
	ledgerID := uint(ledger["id"].(float64))
	ledgerPath := fmt.Sprintf("/api/billing/ledgers/%d", ledgerID)

	// Pending invitees and outsiders are isolated from the ledger until an
	// invitation is accepted.
	if got := doJSON(r, http.MethodGet, ledgerPath, bobToken, nil).Code; got != http.StatusForbidden {
		t.Fatalf("pending invitee get ledger = %d, want 403", got)
	}
	if got := doJSON(r, http.MethodGet, ledgerPath, charlieToken, nil).Code; got != http.StatusForbidden {
		t.Fatalf("outsider get ledger = %d, want 403", got)
	}
	accepted := doJSON(r, http.MethodPost, ledgerPath+"/invitations/respond", bobToken, map[string]bool{"accept": true})
	if accepted.Code != http.StatusOK {
		t.Fatalf("accept invitation: %d %s", accepted.Code, accepted.Body.String())
	}
	aliceAccountCreated := doJSON(r, http.MethodPost, "/api/billing/personal/accounts", aliceToken, map[string]interface{}{"type": "wechat", "name": "共享支出微信", "identifier": "测试"})
	aliceAccountID := uint(billingData(t, aliceAccountCreated.Code, aliceAccountCreated.Body.String())["id"].(float64))
	bobAccountCreated := doJSON(r, http.MethodPost, "/api/billing/personal/accounts", bobToken, map[string]interface{}{"type": "alipay", "name": "Bob 支付宝"})
	bobAccountID := uint(billingData(t, bobAccountCreated.Code, bobAccountCreated.Body.String())["id"].(float64))

	// The API stores the final cents values, irrespective of whether the UI
	// derived them from equal, fixed-amount, percentage, or share-weight modes.
	transactions := []map[string]interface{}{
		{"kind": "expense", "actor_user_id": 1, "account_id": aliceAccountID, "amount_cents": 100, "category": "equal", "shares": []map[string]int{}},
		{"kind": "expense", "actor_user_id": 1, "amount_cents": 100, "category": "fixed", "shares": []map[string]int{{"user_id": 1, "amount_cents": 30}, {"user_id": 2, "amount_cents": 70}}},
		{"kind": "expense", "actor_user_id": 1, "amount_cents": 100, "category": "percent", "shares": []map[string]int{{"user_id": 1, "amount_cents": 25}, {"user_id": 2, "amount_cents": 75}}},
		{"kind": "expense", "actor_user_id": 1, "amount_cents": 100, "category": "weight", "shares": []map[string]int{{"user_id": 1, "amount_cents": 20}, {"user_id": 2, "amount_cents": 80}}},
	}
	wrongAccount := map[string]interface{}{"kind": "expense", "actor_user_id": 1, "account_id": bobAccountID, "amount_cents": 100, "category": "wrong", "shares": []map[string]int{}}
	if got := doJSON(r, http.MethodPost, ledgerPath+"/transactions", aliceToken, wrongAccount).Code; got != http.StatusBadRequest {
		t.Fatalf("shared transaction with another member account = %d, want 400", got)
	}
	for _, transaction := range transactions {
		w := doJSON(r, http.MethodPost, ledgerPath+"/transactions", aliceToken, transaction)
		if w.Code != http.StatusOK {
			t.Fatalf("create transaction: %d %s", w.Code, w.Body.String())
		}
	}
	var persisted []billing.SharedTransaction
	if err := db.Where("ledger_id = ?", ledgerID).Order("id ASC").Find(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	wantShares := [][]int64{{50, 50}, {30, 70}, {25, 75}, {20, 80}}
	for i, transaction := range persisted {
		var shares []billing.SharedShare
		if err := db.Where("transaction_id = ?", transaction.ID).Order("id ASC").Find(&shares).Error; err != nil {
			t.Fatal(err)
		}
		if len(shares) != len(wantShares[i]) {
			t.Fatalf("transaction %d has %d shares, want %d", transaction.ID, len(shares), len(wantShares[i]))
		}
		for j, share := range shares {
			if share.AmountCents != wantShares[i][j] {
				t.Fatalf("transaction %d share %d = %d, want %d", transaction.ID, j, share.AmountCents, wantShares[i][j])
			}
		}
	}

	// Alice paid all four hundred cents and owes 125 cents; Bob owes 275.
	settlement := doJSON(r, http.MethodPost, ledgerPath+"/settlements", bobToken, map[string]interface{}{
		"from_user_id": 2, "to_user_id": 1, "amount_cents": 50, "note": "partial settlement",
	})
	if settlement.Code != http.StatusOK {
		t.Fatalf("create settlement: %d %s", settlement.Code, settlement.Body.String())
	}

	page1 := doJSON(r, http.MethodGet, ledgerPath+"?transaction_page=1&transaction_page_size=2", aliceToken, nil)
	pageData := billingData(t, page1.Code, page1.Body.String())
	if pageData["transactions_total"].(float64) != 4 || pageData["transactions_page"].(float64) != 1 || pageData["transactions_page_size"].(float64) != 2 {
		t.Fatalf("unexpected page metadata: %#v", pageData)
	}
	totals := pageData["totals"].(map[string]interface{})
	if totals["expense_cents"].(float64) != 400 || totals["expense_count"].(float64) != 4 {
		t.Fatalf("pagination changed full-ledger totals: %#v", totals)
	}
	if got := len(pageData["transactions"].([]interface{})); got != 2 {
		t.Fatalf("page one transaction count = %d, want 2", got)
	}
	balances := pageData["balances"].([]interface{})
	if len(balances) != 2 {
		t.Fatalf("got %d balances, want 2", len(balances))
	}
	gotBalances := map[string]float64{}
	for _, raw := range balances {
		balance := raw.(map[string]interface{})
		gotBalances[balance["username"].(string)] = balance["balance_cents"].(float64)
	}
	if gotBalances["alice"] != 225 || gotBalances["bob"] != -225 {
		t.Fatalf("settlement produced balances %#v, want alice=225 bob=-225", gotBalances)
	}

	page2 := doJSON(r, http.MethodGet, ledgerPath+"?transaction_page=2&transaction_page_size=2", aliceToken, nil)
	if got := len(billingData(t, page2.Code, page2.Body.String())["transactions"].([]interface{})); got != 2 {
		t.Fatalf("page two transaction count = %d, want 2", got)
	}
	page3 := doJSON(r, http.MethodGet, ledgerPath+"?transaction_page=99&transaction_page_size=999", aliceToken, nil)
	page3Data := billingData(t, page3.Code, page3.Body.String())
	if page3Data["transactions_page_size"].(float64) != 100 || len(page3Data["transactions"].([]interface{})) != 0 {
		t.Fatalf("pagination bounds not applied: %#v", page3Data)
	}
	defaultPage := doJSON(r, http.MethodGet, ledgerPath+"?transaction_page=0&transaction_page_size=0", aliceToken, nil)
	defaultPageData := billingData(t, defaultPage.Code, defaultPage.Body.String())
	if defaultPageData["transactions_page"].(float64) != 1 || defaultPageData["transactions_page_size"].(float64) != 50 {
		t.Fatalf("invalid pagination did not use defaults: %#v", defaultPageData)
	}

	if got := doJSON(r, http.MethodPost, ledgerPath+"/restore", bobToken, nil).Code; got != http.StatusForbidden {
		t.Fatalf("member restore status = %d, want 403", got)
	}
	if got := doJSON(r, http.MethodDelete, ledgerPath, aliceToken, nil).Code; got != http.StatusOK {
		t.Fatalf("archive ledger status = %d", got)
	}
	if got := doJSON(r, http.MethodPost, ledgerPath+"/transactions", aliceToken, transactions[0]).Code; got != http.StatusBadRequest {
		t.Fatalf("archived ledger write status = %d, want 400", got)
	}
	if got := doJSON(r, http.MethodPost, ledgerPath+"/restore", aliceToken, nil).Code; got != http.StatusOK {
		t.Fatalf("owner restore ledger status = %d", got)
	}

	var restored billing.SharedLedger
	if err := db.First(&restored, ledgerID).Error; err != nil {
		t.Fatal(err)
	}
	if restored.Archived {
		t.Fatal("ledger remained archived after restore")
	}
	settings := doJSON(r, http.MethodPut, ledgerPath, aliceToken, map[string]interface{}{"name": "trip", "currency": "CNY", "show_account": true, "show_category": false, "show_note": false})
	if settings.Code != http.StatusOK {
		t.Fatalf("update shared transaction fields: %d %s", settings.Code, settings.Body.String())
	}
	if err := db.First(&restored, ledgerID).Error; err != nil {
		t.Fatal(err)
	}
	if !restored.ShowAccount || restored.ShowCategory || restored.ShowNote {
		t.Fatalf("unexpected shared field settings: %#v", restored)
	}
}
