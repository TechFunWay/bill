package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"smallgo/server/audit"
	"smallgo/server/response"
	"smallgo/server/sysconfig"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type personalBankOption struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Short string `json:"short"`
	Color string `json:"color"`
}

var personalBankOptions = []personalBankOption{
	{Code: "ICBC", Name: "中国工商银行", Short: "工", Color: "#C81920"},
	{Code: "ABC", Name: "中国农业银行", Short: "农", Color: "#008B78"},
	{Code: "BOC", Name: "中国银行", Short: "中", Color: "#A71930"},
	{Code: "CCB", Name: "中国建设银行", Short: "建", Color: "#0B56A7"},
	{Code: "BOCOM", Name: "交通银行", Short: "交", Color: "#163F8F"},
	{Code: "PSBC", Name: "中国邮政储蓄银行", Short: "邮", Color: "#16815D"},
	{Code: "CMB", Name: "招商银行", Short: "招", Color: "#C9162E"},
	{Code: "CITIC", Name: "中信银行", Short: "信", Color: "#D71920"},
	{Code: "CEB", Name: "中国光大银行", Short: "光", Color: "#6B2C91"},
	{Code: "HXB", Name: "华夏银行", Short: "华", Color: "#D71920"},
	{Code: "CMBC", Name: "中国民生银行", Short: "民", Color: "#009B74"},
	{Code: "GDB", Name: "广发银行", Short: "广", Color: "#C71F25"},
	{Code: "PAB", Name: "平安银行", Short: "平", Color: "#F26B21"},
	{Code: "CIB", Name: "兴业银行", Short: "兴", Color: "#0B4C9B"},
	{Code: "SPDB", Name: "浦发银行", Short: "浦", Color: "#253B80"},
	{Code: "CZB", Name: "浙商银行", Short: "浙", Color: "#D51C2F"},
	{Code: "EGBANK", Name: "恒丰银行", Short: "恒", Color: "#C51826"},
	{Code: "CBHB", Name: "渤海银行", Short: "渤", Color: "#2456A6"},
	{Code: "BOB", Name: "北京银行", Short: "京", Color: "#D71920"},
	{Code: "BOS", Name: "上海银行", Short: "上", Color: "#263B80"},
	{Code: "JSB", Name: "江苏银行", Short: "苏", Color: "#1C5AA5"},
	{Code: "BOHAIB", Name: "天津银行", Short: "津", Color: "#145CA8"},
	{Code: "HEBB", Name: "河北银行", Short: "冀", Color: "#0066A4"},
	{Code: "CBB", Name: "沧州银行", Short: "沧", Color: "#0068B7"},
	{Code: "BOCD", Name: "承德银行", Short: "承", Color: "#B01F2E"},
	{Code: "SJZB", Name: "石家庄银行", Short: "石", Color: "#0B659E"},
	{Code: "JINCHB", Name: "晋城银行", Short: "晋", Color: "#C51D2A"},
	{Code: "JZCB", Name: "晋中银行", Short: "晋中", Color: "#B51E2B"},
	{Code: "BOIMC", Name: "内蒙古银行", Short: "蒙", Color: "#007F62"},
	{Code: "BOD", Name: "大连银行", Short: "连", Color: "#0068B7"},
	{Code: "SHENGJING", Name: "盛京银行", Short: "盛", Color: "#D71920"},
	{Code: "JLBANK", Name: "吉林银行", Short: "吉", Color: "#E1272D"},
	{Code: "HRBB", Name: "哈尔滨银行", Short: "哈", Color: "#0B5FAA"},
	{Code: "NBCB", Name: "宁波银行", Short: "宁", Color: "#E2232A"},
	{Code: "HZCB", Name: "杭州银行", Short: "杭", Color: "#0068B7"},
	{Code: "WZCB", Name: "温州银行", Short: "温", Color: "#D71920"},
	{Code: "TAILONG", Name: "浙江泰隆商业银行", Short: "泰", Color: "#E2232A"},
	{Code: "MINTAI", Name: "浙江民泰商业银行", Short: "民泰", Color: "#14805E"},
	{Code: "CZCB", Name: "浙江稠州商业银行", Short: "稠", Color: "#C7192A"},
	{Code: "HFB", Name: "徽商银行", Short: "徽", Color: "#B01F2E"},
	{Code: "FJHX", Name: "福建海峡银行", Short: "海", Color: "#006D5B"},
	{Code: "XMB", Name: "厦门银行", Short: "厦", Color: "#1B4E96"},
	{Code: "JXB", Name: "江西银行", Short: "赣", Color: "#D71920"},
	{Code: "JNB", Name: "济宁银行", Short: "济", Color: "#1A5AA6"},
	{Code: "QDB", Name: "青岛银行", Short: "青", Color: "#D71920"},
	{Code: "QLB", Name: "齐鲁银行", Short: "齐", Color: "#0068B7"},
	{Code: "ZZB", Name: "郑州银行", Short: "郑", Color: "#C7192A"},
	{Code: "ZMDB", Name: "中原银行", Short: "中原", Color: "#E2232A"},
	{Code: "HBC", Name: "湖北银行", Short: "鄂", Color: "#0068B7"},
	{Code: "HNB", Name: "汉口银行", Short: "汉", Color: "#C7192A"},
	{Code: "CSCB", Name: "长沙银行", Short: "长", Color: "#E2232A"},
	{Code: "GRCB", Name: "广州农村商业银行", Short: "穗农", Color: "#00876A"},
	{Code: "CGB", Name: "广州银行", Short: "穗", Color: "#D71920"},
	{Code: "DGB", Name: "东莞银行", Short: "莞", Color: "#C7192A"},
	{Code: "BGB", Name: "广西北部湾银行", Short: "桂", Color: "#0068B7"},
	{Code: "HKB", Name: "海南银行", Short: "琼", Color: "#007C67"},
	{Code: "CQB", Name: "重庆银行", Short: "渝", Color: "#D71920"},
	{Code: "CQRCB", Name: "重庆农村商业银行", Short: "渝农", Color: "#14805E"},
	{Code: "CDCB", Name: "成都银行", Short: "蓉", Color: "#D71920"},
	{Code: "SCB", Name: "四川银行", Short: "川", Color: "#D71920"},
	{Code: "GYB", Name: "贵阳银行", Short: "筑", Color: "#0068B7"},
	{Code: "GZB", Name: "贵州银行", Short: "黔", Color: "#B01F2E"},
	{Code: "FUDIAN", Name: "富滇银行", Short: "滇", Color: "#A71930"},
	{Code: "XAB", Name: "西安银行", Short: "西", Color: "#D71920"},
	{Code: "LZB", Name: "兰州银行", Short: "兰", Color: "#1A5AA6"},
	{Code: "QHB", Name: "青海银行", Short: "青海", Color: "#007C67"},
	{Code: "NXB", Name: "宁夏银行", Short: "宁夏", Color: "#0068B7"},
	{Code: "URMQB", Name: "乌鲁木齐银行", Short: "乌", Color: "#0068B7"},
	{Code: "SRCB", Name: "上海农商银行", Short: "沪农", Color: "#006A50"},
	{Code: "BJRCB", Name: "北京农商银行", Short: "京农", Color: "#16815D"},
	{Code: "JRCB", Name: "江苏农村商业银行", Short: "苏农", Color: "#14805E"},
	{Code: "MYBANK", Name: "网商银行", Short: "网", Color: "#1677FF"},
	{Code: "WEBANK", Name: "微众银行", Short: "微", Color: "#00A66A"},
	{Code: "XWBank", Name: "新网银行", Short: "新", Color: "#5A45D8"},
	{Code: "Z-BANK", Name: "众邦银行", Short: "众", Color: "#E2232A"},
	{Code: "HSBC", Name: "汇丰银行", Short: "汇", Color: "#DB0011"},
	{Code: "STANDARDCHARTERED", Name: "渣打银行", Short: "渣", Color: "#0072AA"},
	{Code: "HKBEA", Name: "东亚银行", Short: "东", Color: "#D71920"},
	{Code: "CITI", Name: "花旗银行", Short: "花", Color: "#056DAE"},
	{Code: "DBS", Name: "星展银行", Short: "星", Color: "#D71920"},
	{Code: "HANGSENG", Name: "恒生银行", Short: "恒生", Color: "#00856A"},
	{Code: "OCBC", Name: "华侨银行", Short: "侨", Color: "#D71920"},
	{Code: "UOB", Name: "大华银行", Short: "大华", Color: "#193E8C"},
	{Code: "MUFG", Name: "三菱日联银行", Short: "日联", Color: "#D71920"},
	{Code: "SMBC", Name: "三井住友银行", Short: "三井", Color: "#00856A"},
	{Code: "MIZUHO", Name: "瑞穗银行", Short: "瑞穗", Color: "#174A96"},
	{Code: "OTHER", Name: "其他银行", Short: "其他", Color: "#596174"},
}

var personalBankNames = func() map[string]bool {
	result := make(map[string]bool, len(personalBankOptions))
	for _, item := range personalBankOptions {
		result[item.Name] = true
	}
	return result
}()

type personalAccountRequest struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Institution string `json:"institution"`
	Identifier  string `json:"identifier"`
	CardKind    string `json:"card_kind"`
	CardNumber  string `json:"card_number"`
}

func normalizePersonalAccount(req personalAccountRequest) (PersonalAccount, error) {
	account := PersonalAccount{
		Type: strings.TrimSpace(req.Type), Name: cleanText(req.Name, 80),
		Institution: cleanText(req.Institution, 80), Identifier: cleanText(req.Identifier, 64), CardKind: strings.TrimSpace(req.CardKind),
	}
	if account.Name == "" {
		return PersonalAccount{}, errors.New("请输入账户名称")
	}
	switch account.Type {
	case AccountTypeBankCard:
		if !personalBankNames[account.Institution] {
			return PersonalAccount{}, errors.New("请选择有效的银行")
		}
		if account.CardKind != CardKindDebit && account.CardKind != CardKindCredit {
			return PersonalAccount{}, errors.New("请选择储蓄卡或信用卡")
		}
		account.Identifier = ""
	case AccountTypeWechat:
		account.Institution = "微信"
		account.CardKind = ""
	case AccountTypeAlipay:
		account.Institution = "支付宝"
		account.CardKind = ""
	default:
		return PersonalAccount{}, errors.New("请选择有效的账户类型")
	}
	return account, nil
}

func normalizeCardNumber(value string) (string, error) {
	value = strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(value))
	if len(value) < 12 || len(value) > 19 {
		return "", errors.New("请输入 12 至 19 位银行卡号")
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return "", errors.New("银行卡号只能包含数字")
		}
	}
	if !validLuhn(value) {
		return "", errors.New("银行卡号校验失败，请检查后重试")
	}
	return value, nil
}

func validLuhn(value string) bool {
	sum, double := 0, false
	for i := len(value) - 1; i >= 0; i-- {
		digit := int(value[i] - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}
	return sum%10 == 0
}

func cardFingerprint(db *gorm.DB, cardNumber string) (string, error) {
	secret, err := sysconfig.GetConfig(db, "jwt_secret", 0)
	if err != nil || secret == "" {
		return "", errors.New("读取银行卡校验密钥失败")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(cardNumber))
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func prepareBankCard(db *gorm.DB, account *PersonalAccount, cardNumber string) error {
	normalized, err := normalizeCardNumber(cardNumber)
	if err != nil {
		return err
	}
	fingerprint, err := cardFingerprint(db, normalized)
	if err != nil {
		return err
	}
	account.Identifier = normalized[len(normalized)-4:]
	account.CardFingerprint = &fingerprint
	return nil
}

func duplicateCard(db *gorm.DB, userID uint, fingerprint string, excludeID uint) (bool, error) {
	query := db.Model(&PersonalAccount{}).Where("user_id = ? AND card_fingerprint = ?", userID, fingerprint)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func personalAccountConflictMessage(err error) string {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "card_fingerprint") || strings.Contains(message, "idx_personal_account_user_card") {
		return "这张银行卡已经添加过了"
	}
	if strings.Contains(message, "unique") {
		return "账户名称已存在"
	}
	return ""
}

func handlePersonalAccountOptions() gin.HandlerFunc {
	return func(c *gin.Context) {
		response.Success(c, gin.H{"banks": personalBankOptions, "card_kinds": []gin.H{{"value": CardKindDebit, "label": "储蓄卡"}, {"value": CardKindCredit, "label": "信用卡"}}})
	}
}

func ownedPersonalAccount(db *gorm.DB, userID, accountID uint, writable bool) (PersonalAccount, error) {
	if accountID == 0 {
		return PersonalAccount{}, errors.New("账户不存在")
	}
	query := db.Where("id = ? AND user_id = ?", accountID, userID)
	if writable {
		query = query.Where("archived = ?", false)
	}
	var account PersonalAccount
	if err := query.First(&account).Error; err != nil {
		return PersonalAccount{}, err
	}
	return account, nil
}

func handleListPersonalAccounts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db.Where("user_id = ?", c.GetUint("userID"))
		if c.Query("include_archived") != "true" {
			query = query.Where("archived = ?", false)
		}
		var items []PersonalAccount
		if err := query.Order("archived ASC, updated_at DESC, id DESC").Find(&items).Error; err != nil {
			response.ErrorInternal(c, "获取账户失败")
			return
		}
		for i := range items {
			if err := db.Model(&PersonalTransaction{}).Where("user_id = ? AND account_id = ?", c.GetUint("userID"), items[i].ID).Count(&items[i].TransactionCount).Error; err != nil {
				response.ErrorInternal(c, "获取账户失败")
				return
			}
		}
		response.Success(c, items)
	}
}

func handleCreatePersonalAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req personalAccountRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入完整账户信息")
			return
		}
		account, err := normalizePersonalAccount(req)
		if err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		account.UserID = c.GetUint("userID")
		if account.Type == AccountTypeBankCard {
			if strings.TrimSpace(req.CardNumber) != "" {
				if err := prepareBankCard(db, &account, req.CardNumber); err != nil {
					response.ErrorBadRequest(c, err.Error())
					return
				}
				duplicate, err := duplicateCard(db, account.UserID, *account.CardFingerprint, 0)
				if err != nil {
					response.ErrorInternal(c, "校验银行卡失败")
					return
				}
				if duplicate {
					response.ErrorBadRequest(c, "这张银行卡已经添加过了")
					return
				}
			}
		}
		if err := db.Create(&account).Error; err != nil {
			if message := personalAccountConflictMessage(err); message != "" {
				response.ErrorBadRequest(c, message)
				return
			}
			response.ErrorInternal(c, "创建账户失败")
			return
		}
		audit.Log(db, c, "personal_account_create", "personal_account", account.ID, "新增记账账户")
		response.Success(c, account)
	}
}

func handleUpdatePersonalAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			response.ErrorBadRequest(c, "无效的账户 ID")
			return
		}
		existing, err := ownedPersonalAccount(db, c.GetUint("userID"), uint(id), false)
		if err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "账户不存在")
			return
		}
		var req personalAccountRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请输入完整账户信息")
			return
		}
		account, err := normalizePersonalAccount(req)
		if err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		if account.Type == AccountTypeBankCard {
			if strings.TrimSpace(req.CardNumber) != "" {
				if err := prepareBankCard(db, &account, req.CardNumber); err != nil {
					response.ErrorBadRequest(c, err.Error())
					return
				}
				duplicate, err := duplicateCard(db, c.GetUint("userID"), *account.CardFingerprint, existing.ID)
				if err != nil {
					response.ErrorInternal(c, "校验银行卡失败")
					return
				}
				if duplicate {
					response.ErrorBadRequest(c, "这张银行卡已经添加过了")
					return
				}
			} else if existing.Type == AccountTypeBankCard {
				account.Identifier = existing.Identifier
				account.CardFingerprint = existing.CardFingerprint
			}
		} else {
			account.CardFingerprint = nil
		}
		updates := map[string]interface{}{"type": account.Type, "name": account.Name, "institution": account.Institution, "identifier": account.Identifier, "card_kind": account.CardKind, "card_fingerprint": account.CardFingerprint}
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			if message := personalAccountConflictMessage(err); message != "" {
				response.ErrorBadRequest(c, message)
				return
			}
			response.ErrorInternal(c, "更新账户失败")
			return
		}
		db.First(&existing, existing.ID)
		response.Success(c, existing)
	}
}

func handleArchivePersonalAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			response.ErrorBadRequest(c, "无效的账户 ID")
			return
		}
		account, err := ownedPersonalAccount(db, c.GetUint("userID"), uint(id), false)
		if err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "账户不存在")
			return
		}
		if err := db.Model(&account).Update("archived", true).Error; err != nil {
			response.ErrorInternal(c, "归档账户失败")
			return
		}
		audit.Log(db, c, "personal_account_archive", "personal_account", account.ID, "归档记账账户")
		response.Success(c, nil)
	}
}

func handleRestorePersonalAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			response.ErrorBadRequest(c, "无效的账户 ID")
			return
		}
		account, err := ownedPersonalAccount(db, c.GetUint("userID"), uint(id), false)
		if err != nil {
			response.Error(c, http.StatusNotFound, response.CodeNotFound, "账户不存在")
			return
		}
		if err := db.Model(&account).Update("archived", false).Error; err != nil {
			response.ErrorInternal(c, "恢复账户失败")
			return
		}
		response.Success(c, nil)
	}
}
