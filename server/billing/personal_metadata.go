package billing

import (
	"errors"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"

	"smallgo/server/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var defaultPersonalCategories = map[string][]string{
	KindExpense: {"餐饮", "交通", "购物", "居住", "娱乐", "医疗", "教育", "人情", "旅行", "其他"},
	KindIncome:  {"工资", "奖金", "兼职", "理财", "报销", "礼金", "退款", "其他"},
}

// defaultPersonalSubcategories maps kind → top-level category name → common
// second-level categories. Parents missing on a user (deleted or renamed) are
// skipped, and "其他" intentionally has no children.
var defaultPersonalSubcategories = map[string]map[string][]string{
	KindExpense: {
		"餐饮": {"早餐", "午餐", "晚餐", "外卖", "零食饮料", "食材"},
		"交通": {"公交地铁", "打车", "加油", "停车过路", "火车", "飞机"},
		"购物": {"日用百货", "服饰鞋包", "数码家电", "美妆个护", "母婴用品"},
		"居住": {"房租房贷", "水电燃气", "物业费", "网络话费", "维修保养"},
		"娱乐": {"电影演出", "游戏充值", "运动健身"},
		"医疗": {"药品", "挂号诊疗", "体检"},
		"教育": {"学费培训", "书籍资料", "考试报名"},
		"人情": {"红包礼金", "请客送礼"},
		"旅行": {"景点门票", "旅行住宿", "旅行交通", "纪念品"},
	},
	KindIncome: {
		"工资": {"基本工资", "绩效提成", "加班费"},
		"奖金": {"年终奖", "绩效奖金"},
		"理财": {"利息", "基金收益", "股票收益"},
	},
}

const defaultPersonalLedgerCover = "coral"

var personalLedgerCoverPresets = map[string]struct{}{
	"coral":  {},
	"ocean":  {},
	"forest": {},
	"violet": {},
	"amber":  {},
	"slate":  {},
}

var personalLedgerUploadCover = regexp.MustCompile(`^/uploads/[0-9]{4}/[0-9]{2}/[0-9]{2}/[a-f0-9-]+\.(jpg|png|webp)$`)

func personalCurrency(value string) (string, error) {
	currency := strings.ToUpper(cleanText(value, 8))
	if currency == "" {
		currency = "CNY"
	}
	if currency != "CNY" {
		return "", errors.New("个人账本当前仅支持人民币 CNY")
	}
	return currency, nil
}

func personalLedgerCover(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultPersonalLedgerCover, nil
	}
	if _, ok := personalLedgerCoverPresets[value]; ok {
		return value, nil
	}
	if len(value) <= 512 && personalLedgerUploadCover.MatchString(value) && path.Clean(value) == value {
		return value, nil
	}
	return "", errors.New("请选择有效的账本封面")
}

// ensurePersonalSetup is intentionally safe to call at every personal API
// entry point. It provides both lazy migration for existing installations and
// useful defaults for newly registered users.
func ensurePersonalSetup(db *gorm.DB, userID uint) (PersonalLedger, error) {
	var ledger PersonalLedger
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND is_default = ?", userID, true).First(&ledger).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			ledger = PersonalLedger{UserID: userID, Name: "日常账本", Currency: "CNY", Cover: defaultPersonalLedgerCover, IsDefault: true}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ledger).Error; err != nil {
				return err
			}
			if ledger.ID == 0 {
				if err := tx.Where("user_id = ? AND is_default = ?", userID, true).First(&ledger).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Model(&PersonalTransaction{}).Where("user_id = ? AND ledger_id = 0", userID).Update("ledger_id", ledger.ID).Error; err != nil {
			return err
		}
		if !ledger.CategoriesSeeded {
			for kind, names := range defaultPersonalCategories {
				for _, name := range names {
					category := PersonalCategory{UserID: userID, Kind: kind, Name: name, IsDefault: true}
					if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&category).Error; err != nil {
						return err
					}
				}
			}
			if err := tx.Model(&ledger).Update("categories_seeded", true).Error; err != nil {
				return err
			}
			ledger.CategoriesSeeded = true
		}
		if !ledger.SubcategoriesSeeded {
			if err := seedPersonalSubcategories(tx, userID); err != nil {
				return err
			}
			if err := tx.Model(&ledger).Update("subcategories_seeded", true).Error; err != nil {
				return err
			}
			ledger.SubcategoriesSeeded = true
		}
		return nil
	})
	return ledger, err
}

// seedPersonalSubcategories attaches the common default second-level
// categories to whichever matching top-level categories the user still has.
// It is safe to run repeatedly: missing parents are skipped and the unique
// user/kind/name index turns duplicates into no-ops, so categories the user
// already created or deleted are never recreated or moved.
func seedPersonalSubcategories(tx *gorm.DB, userID uint) error {
	var categories []PersonalCategory
	if err := tx.Where("user_id = ?", userID).Find(&categories).Error; err != nil {
		return err
	}
	roots := make(map[string]map[string]uint, 2)
	for _, category := range categories {
		if category.ParentID != 0 {
			continue
		}
		if roots[category.Kind] == nil {
			roots[category.Kind] = make(map[string]uint)
		}
		roots[category.Kind][category.Name] = category.ID
	}
	for kind, parents := range defaultPersonalSubcategories {
		for parentName, names := range parents {
			parentID, ok := roots[kind][parentName]
			if !ok {
				continue
			}
			for _, name := range names {
				category := PersonalCategory{UserID: userID, Kind: kind, Name: name, ParentID: parentID, IsDefault: true}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&category).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func ownedPersonalLedger(db *gorm.DB, userID, ledgerID uint, writable bool) (PersonalLedger, error) {
	var ledger PersonalLedger
	if err := db.Where("id = ? AND user_id = ?", ledgerID, userID).First(&ledger).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ledger, errNotFound
		}
		return ledger, err
	}
	if writable && ledger.Archived {
		return ledger, errors.New("该个人账本已归档，不能继续记账")
	}
	return ledger, nil
}

func handleListPersonalLedgers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := ensurePersonalSetup(db, c.GetUint("userID")); err != nil {
			response.ErrorInternal(c, "获取账本失败")
			return
		}
		query := db.Where("user_id = ?", c.GetUint("userID"))
		if c.Query("include_archived") != "true" {
			query = query.Where("archived = ?", false)
		}
		var items []PersonalLedger
		if err := query.Order("archived ASC, sort_order ASC, is_default DESC, created_at ASC").Find(&items).Error; err != nil {
			response.ErrorInternal(c, "获取账本失败")
			return
		}
		for i := range items {
			if err := db.Model(&PersonalTransaction{}).Where("user_id = ? AND ledger_id = ?", c.GetUint("userID"), items[i].ID).Count(&items[i].TransactionCount).Error; err != nil {
				response.ErrorInternal(c, "获取账本失败")
				return
			}
		}
		response.Success(c, items)
	}
}

type personalLedgerRequest struct {
	Name     string  `json:"name"`
	Currency string  `json:"currency"`
	Cover    *string `json:"cover"`
}

func handleCreatePersonalLedger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := ensurePersonalSetup(db, c.GetUint("userID")); err != nil {
			response.ErrorInternal(c, "创建账本失败")
			return
		}
		var req personalLedgerRequest
		if c.ShouldBindJSON(&req) != nil {
			response.ErrorBadRequest(c, "请输入账本名称")
			return
		}
		name := cleanText(req.Name, 100)
		if name == "" {
			response.ErrorBadRequest(c, "请输入账本名称")
			return
		}
		currency, err := personalCurrency(req.Currency)
		if err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		cover := defaultPersonalLedgerCover
		if req.Cover != nil {
			cover, err = personalLedgerCover(*req.Cover)
			if err != nil {
				response.ErrorBadRequest(c, err.Error())
				return
			}
		}
		var maxOrder int
		if err := db.Model(&PersonalLedger{}).Where("user_id = ? AND archived = ?", c.GetUint("userID"), false).Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder).Error; err != nil {
			response.ErrorInternal(c, "创建账本失败")
			return
		}
		item := PersonalLedger{UserID: c.GetUint("userID"), Name: name, Currency: currency, Cover: cover, SortOrder: maxOrder + 1}
		if err := db.Create(&item).Error; err != nil {
			response.ErrorBadRequest(c, "账本名称已存在")
			return
		}
		response.Success(c, item)
	}
}

func handleUpdatePersonalLedger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			response.ErrorBadRequest(c, "无效的账本 ID")
			return
		}
		ledger, err := ownedPersonalLedger(db, c.GetUint("userID"), uint(id), false)
		if err != nil {
			respondPersonalError(c, err, "账本不存在")
			return
		}
		var req personalLedgerRequest
		if c.ShouldBindJSON(&req) != nil {
			response.ErrorBadRequest(c, "请输入账本信息")
			return
		}
		name := cleanText(req.Name, 100)
		if name == "" {
			response.ErrorBadRequest(c, "请输入账本名称")
			return
		}
		currency, err := personalCurrency(req.Currency)
		if err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		updates := map[string]interface{}{"name": name, "currency": currency}
		if req.Cover != nil {
			cover, coverErr := personalLedgerCover(*req.Cover)
			if coverErr != nil {
				response.ErrorBadRequest(c, coverErr.Error())
				return
			}
			updates["cover"] = cover
		}
		if err := db.Model(&ledger).Updates(updates).Error; err != nil {
			response.ErrorBadRequest(c, "账本名称已存在")
			return
		}
		db.First(&ledger, ledger.ID)
		response.Success(c, ledger)
	}
}

type personalLedgerOrderRequest struct {
	LedgerIDs []uint `json:"ledger_ids"`
}

func handleOrderPersonalLedgers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("userID")
		if _, err := ensurePersonalSetup(db, userID); err != nil {
			response.ErrorInternal(c, "账本排序失败")
			return
		}
		var req personalLedgerOrderRequest
		if c.ShouldBindJSON(&req) != nil || len(req.LedgerIDs) == 0 {
			response.ErrorBadRequest(c, "请提供完整的账本顺序")
			return
		}
		var ledgers []PersonalLedger
		if err := db.Where("user_id = ? AND archived = ?", userID, false).Find(&ledgers).Error; err != nil {
			response.ErrorInternal(c, "账本排序失败")
			return
		}
		if len(req.LedgerIDs) != len(ledgers) {
			response.ErrorBadRequest(c, "账本顺序与当前账本不一致，请刷新后重试")
			return
		}
		owned := make(map[uint]struct{}, len(ledgers))
		for _, ledger := range ledgers {
			owned[ledger.ID] = struct{}{}
		}
		seen := make(map[uint]struct{}, len(req.LedgerIDs))
		for _, id := range req.LedgerIDs {
			if _, ok := owned[id]; !ok {
				response.ErrorBadRequest(c, "账本顺序包含无效账本")
				return
			}
			if _, duplicate := seen[id]; duplicate {
				response.ErrorBadRequest(c, "账本顺序不能重复")
				return
			}
			seen[id] = struct{}{}
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			for index, id := range req.LedgerIDs {
				if err := tx.Model(&PersonalLedger{}).Where("id = ? AND user_id = ? AND archived = ?", id, userID, false).Update("sort_order", index).Error; err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			response.ErrorInternal(c, "账本排序失败")
			return
		}
		response.Success(c, gin.H{"ledger_ids": req.LedgerIDs})
	}
}

func handleArchivePersonalLedger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			response.ErrorBadRequest(c, "无效的账本 ID")
			return
		}
		ledger, err := ownedPersonalLedger(db, c.GetUint("userID"), uint(id), false)
		if err != nil {
			respondPersonalError(c, err, "账本不存在")
			return
		}
		if ledger.IsDefault {
			response.ErrorBadRequest(c, "默认账本不能归档")
			return
		}
		if err := db.Model(&ledger).Update("archived", true).Error; err != nil {
			response.ErrorInternal(c, "归档账本失败")
			return
		}
		ledger.Archived = true
		response.Success(c, ledger)
	}
}

func handleRestorePersonalLedger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			response.ErrorBadRequest(c, "无效的账本 ID")
			return
		}
		ledger, err := ownedPersonalLedger(db, c.GetUint("userID"), uint(id), false)
		if err != nil {
			respondPersonalError(c, err, "账本不存在")
			return
		}
		var maxOrder int
		if err := db.Model(&PersonalLedger{}).Where("user_id = ? AND archived = ?", c.GetUint("userID"), false).Select("COALESCE(MAX(sort_order), -1)").Scan(&maxOrder).Error; err != nil {
			response.ErrorInternal(c, "恢复账本失败")
			return
		}
		if err := db.Model(&ledger).Updates(map[string]interface{}{"archived": false, "sort_order": maxOrder + 1}).Error; err != nil {
			response.ErrorInternal(c, "恢复账本失败")
			return
		}
		ledger.Archived = false
		ledger.SortOrder = maxOrder + 1
		response.Success(c, ledger)
	}
}

type personalCategoryRequest struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	ParentID *uint  `json:"parent_id"`
}

func handleListPersonalCategories(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := ensurePersonalSetup(db, c.GetUint("userID")); err != nil {
			response.ErrorInternal(c, "获取分类失败")
			return
		}
		query := db.Where("user_id = ?", c.GetUint("userID"))
		if validKind(c.Query("kind")) {
			query = query.Where("kind = ?", c.Query("kind"))
		}
		var items []PersonalCategory
		if err := query.Order("kind ASC, parent_id ASC, name ASC").Find(&items).Error; err != nil {
			response.ErrorInternal(c, "获取分类失败")
			return
		}
		parentNames := make(map[uint]string, len(items))
		for _, item := range items {
			if item.ParentID == 0 {
				parentNames[item.ID] = item.Name
			}
		}
		for i := range items {
			if items[i].ParentID > 0 {
				items[i].ParentName = parentNames[items[i].ParentID]
			}
			if err := db.Model(&PersonalTransaction{}).Where("user_id = ? AND kind = ? AND category = ?", c.GetUint("userID"), items[i].Kind, items[i].Name).Count(&items[i].TransactionCount).Error; err != nil {
				response.ErrorInternal(c, "获取分类失败")
				return
			}
		}
		response.Success(c, items)
	}
}

func validateCategoryRequest(req personalCategoryRequest) (string, error) {
	if !validKind(req.Kind) {
		return "", errInvalid
	}
	name := cleanText(req.Name, 64)
	if name == "" {
		return "", errInvalid
	}
	return name, nil
}

func validateCategoryParent(db *gorm.DB, userID uint, kind string, parentID, currentID uint) error {
	if parentID == 0 {
		return nil
	}
	if parentID == currentID {
		return errors.New("分类不能设为自己的上级")
	}
	var parent PersonalCategory
	if err := db.Where("id = ? AND user_id = ?", parentID, userID).First(&parent).Error; err != nil {
		return errors.New("上级分类不存在")
	}
	if parent.ParentID != 0 {
		return errors.New("最多支持二级分类")
	}
	if parent.Kind != kind {
		return errors.New("上级分类的收支类型不一致")
	}
	return nil
}

func requestedCategoryParentID(req personalCategoryRequest, fallback uint) uint {
	if req.ParentID == nil {
		return fallback
	}
	return *req.ParentID
}

func handleCreatePersonalCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, err := ensurePersonalSetup(db, c.GetUint("userID")); err != nil {
			response.ErrorInternal(c, "创建分类失败")
			return
		}
		var req personalCategoryRequest
		if c.ShouldBindJSON(&req) != nil {
			response.ErrorBadRequest(c, "请输入分类信息")
			return
		}
		name, err := validateCategoryRequest(req)
		if err != nil {
			response.ErrorBadRequest(c, "请输入有效分类")
			return
		}
		parentID := requestedCategoryParentID(req, 0)
		if err := validateCategoryParent(db, c.GetUint("userID"), req.Kind, parentID, 0); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		item := PersonalCategory{UserID: c.GetUint("userID"), Kind: req.Kind, Name: name, ParentID: parentID}
		if err := db.Create(&item).Error; err != nil {
			response.ErrorBadRequest(c, "分类已存在")
			return
		}
		response.Success(c, item)
	}
}

func handleUpdatePersonalCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			response.ErrorBadRequest(c, "无效的分类 ID")
			return
		}
		var current PersonalCategory
		if err := db.Where("id = ? AND user_id = ?", uint(id), c.GetUint("userID")).First(&current).Error; err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "分类不存在")
			return
		}
		var req personalCategoryRequest
		if c.ShouldBindJSON(&req) != nil {
			response.ErrorBadRequest(c, "请输入分类信息")
			return
		}
		name, err := validateCategoryRequest(req)
		if err != nil {
			response.ErrorBadRequest(c, "请输入有效分类")
			return
		}
		parentID := requestedCategoryParentID(req, current.ParentID)
		if err := validateCategoryParent(db, current.UserID, req.Kind, parentID, current.ID); err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		var childCount int64
		if err := db.Model(&PersonalCategory{}).Where("user_id = ? AND parent_id = ?", current.UserID, current.ID).Count(&childCount).Error; err != nil {
			response.ErrorInternal(c, "检查下级分类失败")
			return
		}
		if childCount > 0 && parentID != 0 {
			response.ErrorBadRequest(c, "包含下级分类的分类不能设为二级分类")
			return
		}
		if childCount > 0 && req.Kind != current.Kind {
			response.ErrorBadRequest(c, "包含下级分类时不能修改收支类型")
			return
		}
		if req.Kind != current.Kind {
			var used int64
			if err := db.Model(&PersonalTransaction{}).
				Where("user_id = ? AND kind = ? AND category = ?", current.UserID, current.Kind, current.Name).
				Count(&used).Error; err != nil {
				response.ErrorInternal(c, "检查分类使用情况失败")
				return
			}
			if used > 0 {
				response.ErrorBadRequest(c, "已有账单使用该分类，不能修改收支类型")
				return
			}
		}
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&PersonalCategory{}).Where("id = ? AND user_id = ?", current.ID, current.UserID).Updates(map[string]interface{}{"kind": req.Kind, "name": name, "parent_id": parentID}).Error; err != nil {
				return err
			}
			return tx.Model(&PersonalTransaction{}).Where("user_id = ? AND kind = ? AND category = ?", current.UserID, current.Kind, current.Name).Update("category", name).Error
		})
		if err != nil {
			response.ErrorBadRequest(c, "分类已存在")
			return
		}
		db.First(&current, current.ID)
		response.Success(c, current)
	}
}

func handleDeletePersonalCategory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			response.ErrorBadRequest(c, "无效的分类 ID")
			return
		}
		var current PersonalCategory
		if err := db.Where("id = ? AND user_id = ?", uint(id), c.GetUint("userID")).First(&current).Error; err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "分类不存在")
			return
		}
		var childCount int64
		if err := db.Model(&PersonalCategory{}).Where("user_id = ? AND parent_id = ?", current.UserID, current.ID).Count(&childCount).Error; err != nil {
			response.ErrorInternal(c, "检查下级分类失败")
			return
		}
		if childCount > 0 {
			response.ErrorBadRequest(c, "请先删除或移动该分类下的二级分类")
			return
		}
		result := db.Delete(&current)
		if result.Error != nil {
			response.ErrorInternal(c, "删除分类失败")
			return
		}
		response.Success(c, nil)
	}
}

type personalTagRequest struct {
	Name string `json:"name"`
}

func handleListPersonalTags(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var items []PersonalTag
		if err := db.Where("user_id = ?", c.GetUint("userID")).Order("name ASC").Find(&items).Error; err != nil {
			response.ErrorInternal(c, "获取标签失败")
			return
		}
		for i := range items {
			if err := db.Table("personal_transaction_tags").Joins("JOIN personal_transactions ON personal_transactions.id = personal_transaction_tags.transaction_id").Where("personal_transaction_tags.tag_id = ? AND personal_transactions.user_id = ?", items[i].ID, c.GetUint("userID")).Count(&items[i].TransactionCount).Error; err != nil {
				response.ErrorInternal(c, "获取标签失败")
				return
			}
		}
		response.Success(c, items)
	}
}
func handleCreatePersonalTag(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req personalTagRequest
		if c.ShouldBindJSON(&req) != nil {
			response.ErrorBadRequest(c, "请输入标签名称")
			return
		}
		name := cleanText(req.Name, 50)
		if name == "" {
			response.ErrorBadRequest(c, "请输入标签名称")
			return
		}
		item := PersonalTag{UserID: c.GetUint("userID"), Name: name}
		if err := db.Create(&item).Error; err != nil {
			response.ErrorBadRequest(c, "标签已存在")
			return
		}
		response.Success(c, item)
	}
}
func handleUpdatePersonalTag(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			response.ErrorBadRequest(c, "无效的标签 ID")
			return
		}
		var item PersonalTag
		if err := db.Where("id = ? AND user_id = ?", uint(id), c.GetUint("userID")).First(&item).Error; err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "标签不存在")
			return
		}
		var req personalTagRequest
		if c.ShouldBindJSON(&req) != nil {
			response.ErrorBadRequest(c, "请输入标签名称")
			return
		}
		name := cleanText(req.Name, 50)
		if name == "" {
			response.ErrorBadRequest(c, "请输入标签名称")
			return
		}
		if err := db.Model(&item).Update("name", name).Error; err != nil {
			response.ErrorBadRequest(c, "标签已存在")
			return
		}
		item.Name = name
		response.Success(c, item)
	}
}
func handleDeletePersonalTag(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			response.ErrorBadRequest(c, "无效的标签 ID")
			return
		}
		var item PersonalTag
		if err := db.Where("id = ? AND user_id = ?", uint(id), c.GetUint("userID")).First(&item).Error; err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "标签不存在")
			return
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("tag_id = ?", item.ID).Delete(&PersonalTransactionTag{}).Error; err != nil {
				return err
			}
			return tx.Delete(&item).Error
		}); err != nil {
			response.ErrorInternal(c, "删除标签失败")
			return
		}
		response.Success(c, nil)
	}
}

func respondPersonalError(c *gin.Context, err error, notFoundMessage string) {
	if errors.Is(err, errNotFound) {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, notFoundMessage)
		return
	}
	if errors.Is(err, errForbidden) {
		response.Error(c, http.StatusForbidden, response.CodeForbidden, err.Error())
		return
	}
	response.ErrorBadRequest(c, err.Error())
}
