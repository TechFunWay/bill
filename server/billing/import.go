package billing

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"smallgo/server/audit"
	"smallgo/server/response"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"gorm.io/gorm"
)

const (
	importSourceAlipay = "alipay"
	importSourceWechat = "wechat"

	importMaxFileSize = 5 << 20 // 5 MiB
	importMaxRows     = 5000

	importRowStatusImport    = "import"
	importRowStatusDuplicate = "duplicate"
	importRowStatusSkipped   = "skipped"
)

var importSourceLabels = map[string]string{
	importSourceAlipay: "支付宝",
	importSourceWechat: "微信",
}

type importRow struct {
	Line            int    `json:"line"`
	OccurredAt      string `json:"occurred_at"`
	Kind            string `json:"kind"`
	AmountCents     int64  `json:"amount_cents"`
	Category        string `json:"category"`
	CategoryMatched bool   `json:"category_matched"`
	Note            string `json:"note"`
	ExternalID      string `json:"external_id"`
	Status          string `json:"status"`
	SkipReason      string `json:"skip_reason"`
	// refund marks rows generated from an Alipay refund record (不计收支 +
	// 退款成功). It stays unexported so it never leaks into the JSON payload;
	// it only feeds pairImportRefunds.
	refund bool
}

type importConfirmRow struct {
	LedgerID    uint   `json:"ledger_id"`
	AccountID   uint   `json:"account_id"`
	OccurredAt  string `json:"occurred_at"`
	Kind        string `json:"kind"`
	AmountCents int64  `json:"amount_cents"`
	Category    string `json:"category"`
	Note        string `json:"note"`
	ExternalID  string `json:"external_id"`
}

type importConfirmRequest struct {
	Source string             `json:"source"`
	Rows   []importConfirmRow `json:"rows"`
}

// decodeImportCSV normalizes an uploaded bill to UTF-8. Alipay exports
// GB18030 while WeChat exports UTF-8, optionally with a BOM.
func decodeImportCSV(raw []byte) ([]byte, error) {
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		return raw[3:], nil
	}
	if utf8.Valid(raw) {
		return raw, nil
	}
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(raw)
	if err != nil {
		return nil, errors.New("无法读取账单文件编码，请确认上传的是支付宝或微信导出的 CSV 文件")
	}
	return decoded, nil
}

// readImportXLSXRecords reads the first worksheet of a WeChat bill workbook.
// GetRows returns cell values as plain strings: numeric trade times come back
// as Excel serial numbers and amounts without the ¥ prefix, both of which the
// row parser accepts alongside their CSV spellings.
func readImportXLSXRecords(raw []byte) ([][]string, error) {
	file, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, errors.New("账单文件不是有效的 Excel 格式")
	}
	defer file.Close()
	sheets := file.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("账单文件中没有工作表")
	}
	rows, err := file.GetRows(sheets[0])
	if err != nil {
		return nil, errors.New("读取 Excel 账单内容失败")
	}
	return rows, nil
}

func cleanImportCell(value string) string {
	return strings.TrimSpace(value)
}

// importColumnIndex resolves a header to its column. Export versions spell
// the amount header differently (金额 / 金额(元) / 金额（元）), so an exact
// match is tried first and a parenthesized prefix second.
func importColumnIndex(columns map[string]int, name string) (int, bool) {
	if index, ok := columns[name]; ok {
		return index, true
	}
	wide := name + "（"
	narrow := name + "("
	for header, index := range columns {
		if strings.HasPrefix(header, wide) || strings.HasPrefix(header, narrow) {
			return index, true
		}
	}
	return 0, false
}

// detectImportSource locates the header row by content instead of a fixed
// line number, because both platforms prepend a variable block of metadata
// and Alipay ships 11- and 12-column variants.
func detectImportSource(records [][]string) (string, map[string]int, int, error) {
	for i, record := range records {
		cells := make([]string, len(record))
		for j, cell := range record {
			cells[j] = cleanImportCell(cell)
		}
		if len(cells) == 0 || cells[0] != "交易时间" {
			continue
		}
		columns := make(map[string]int, len(cells))
		for j, cell := range cells {
			if cell != "" {
				columns[cell] = j
			}
		}
		if _, ok := columns["交易单号"]; ok {
			if _, okAmount := importColumnIndex(columns, "金额"); okAmount {
				if _, okStatus := columns["当前状态"]; okStatus {
					return importSourceWechat, columns, i, nil
				}
			}
			return "", nil, 0, errors.New("微信账单格式不完整，请从微信 App 重新导出后再试")
		}
		if _, ok := columns["交易订单号"]; ok {
			if _, okAmount := importColumnIndex(columns, "金额"); okAmount {
				if _, okStatus := columns["交易状态"]; okStatus {
					return importSourceAlipay, columns, i, nil
				}
			}
			return "", nil, 0, errors.New("支付宝账单格式不完整，请从支付宝 App 重新导出后再试")
		}
	}
	return "", nil, 0, errors.New("无法识别的账单文件，请上传支付宝或微信 App 导出的 CSV 文件")
}

func parseImportAmount(raw string) (int64, error) {
	cleaned := strings.NewReplacer("¥", "", "￥", "", "CNY", "", "元", "", ",", "", " ", "", "\t", "").Replace(raw)
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(value * 100)), nil
}

var importPendingStatusKeywords = []string{"等待付款", "交易关闭", "已关闭", "失败", "已退还", "已退回", "已撤销", "已取消", "超时"}

func importStatusPending(status string) bool {
	for _, keyword := range importPendingStatusKeywords {
		if strings.Contains(status, keyword) {
			return true
		}
	}
	return false
}

// excelTimeEpoch is day 0 of Excel's 1900 date system. Serial numbers carry
// wall-clock time in UTC+08:00 (per the bill's own note); building it in the
// local zone keeps the displayed time matching the bill on a UTC+8 server.
const excelTimeEpochLayout = "1899-12-30"

func parseImportBillTime(raw string) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006/01/02 15:04", "2006/01/02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, trimmed, time.Local); err == nil {
			return t, true
		}
	}
	// WeChat xlsx exports the trade time as an Excel serial number
	// (1900-03-01 .. ~2118 keeps the duration inside int64 nanoseconds).
	if value, err := strconv.ParseFloat(trimmed, 64); err == nil && value >= 1 && value <= 80000 {
		seconds := int64(math.Round(value * 86400))
		epoch, _ := time.ParseInLocation("2006-01-02", excelTimeEpochLayout, time.Local)
		return epoch.Add(time.Duration(seconds) * time.Second), true
	}
	return time.Time{}, false
}

// importCategoryRules map bill text to app categories, first match wins.
// Each rule carries an expense and an income target; a suggestion is only
// used when the user actually owns that category for the row's kind.
var importCategoryRules = []struct {
	pattern *regexp.Regexp
	expense string
	income  string
}{
	{regexp.MustCompile(`退款`), "退款", "退款"},
	{regexp.MustCompile(`工资|薪资|薪酬|加班费`), "", "工资"},
	{regexp.MustCompile(`房租|租金|物业费|水费|电费|燃气|热力|供暖|宽带|话费|网费`), "居住", ""},
	{regexp.MustCompile(`外卖|饿了么|美团外卖|肯德基|麦当劳|必胜客|汉堡|披萨|星巴克|瑞幸|喜茶|蜜雪|奶茶|咖啡|餐厅|餐饮|食堂|烧烤|火锅|早餐|午餐|晚餐|夜宵|小吃|米线|饺子|面包`), "餐饮", ""},
	{regexp.MustCompile(`滴滴|高德|打车|出行|地铁|公交|乘车|骑行|单车|哈啰|加油|停车|过路|高速|铁路|12306|火车|机票|航班|航空|机场|客运|船票`), "交通", ""},
	{regexp.MustCompile(`酒店|民宿|门票|景区|旅行|旅游|度假|民宿`), "旅行", ""},
	{regexp.MustCompile(`电影|影票|演出|游戏|视频会员|腾讯视频|爱奇艺|优酷|音乐|健身|运动|游泳|KTV|密室|桌游|彩票`), "娱乐", ""},
	{regexp.MustCompile(`医院|药店|药房|医药|挂号|诊所|体检|口腔|疫苗`), "医疗", ""},
	{regexp.MustCompile(`学费|培训|教育|网课|考试|书店|图书|文具|课程`), "教育", ""},
	{regexp.MustCompile(`淘宝|天猫|京东|拼多多|唯品会|苏宁|抖音商城|超市|百货|商城|旗舰店|专卖店|服饰|穿搭`), "购物", ""},
	{regexp.MustCompile(`红包|亲属卡|群收款|赞赏|份子|随礼`), "人情", "礼金"},
	{regexp.MustCompile(`理财|基金|零钱通|余额宝|收益|利息|黄金|股票`), "理财", "理财"},
}

// importAlipayCategoryAliases translate Alipay's own trade categories to the
// app's default category names. Substring matching handles the rest.
var importAlipayCategoryAliases = map[string]string{
	"餐饮美食": "餐饮",
	"交通出行": "交通",
	"服饰装扮": "购物",
	"日用百货": "购物",
	"美妆个护": "购物",
	"住房物业": "居住",
	"水电煤":   "居住",
	"充值缴费": "居住",
	"文化休闲": "娱乐",
	"娱乐消费": "娱乐",
	"医疗健康": "医疗",
	"医药医疗": "医疗",
	"教育培训": "教育",
	"人情往来": "人情",
	"旅行度假": "旅行",
	"文体旅游": "旅行",
	"酒店旅游": "旅行",
	"数码电器": "数码家电",
	"家居家装": "居住",
	"投资理财": "理财",
}

type importCategoryMatcher struct {
	names map[string]map[string]bool // kind -> category names owned by the user
}

func loadImportCategoryMatcher(db *gorm.DB, userID uint) (*importCategoryMatcher, error) {
	var categories []PersonalCategory
	if err := db.Where("user_id = ?", userID).Find(&categories).Error; err != nil {
		return nil, err
	}
	matcher := &importCategoryMatcher{names: map[string]map[string]bool{}}
	for _, category := range categories {
		if matcher.names[category.Kind] == nil {
			matcher.names[category.Kind] = map[string]bool{}
		}
		matcher.names[category.Kind][category.Name] = true
	}
	return matcher, nil
}

func (m *importCategoryMatcher) has(kind, name string) bool {
	return name != "" && m.names[kind][name]
}

// suggestImportCategory returns a category owned by the user plus whether the
// match is confident. Unconfident suggestions fall back to 其他 so the preview
// can flag them for review instead of leaving the row unbookable.
func (m *importCategoryMatcher) suggest(kind, platformCategory, hintText string) (string, bool) {
	if m.has(kind, platformCategory) {
		return platformCategory, true
	}
	if platformCategory != "" {
		for name := range m.names[kind] {
			if strings.Contains(platformCategory, name) || strings.Contains(name, platformCategory) {
				return name, true
			}
		}
		if alias, ok := importAlipayCategoryAliases[platformCategory]; ok && m.has(kind, alias) {
			return alias, true
		}
	}
	text := platformCategory + " " + hintText
	for _, rule := range importCategoryRules {
		if !rule.pattern.MatchString(text) {
			continue
		}
		target := rule.expense
		if kind == KindIncome {
			target = rule.income
		}
		if m.has(kind, target) {
			return target, true
		}
	}
	if m.has(kind, "其他") {
		return "其他", false
	}
	return "", false
}

// parseImportRow converts one CSV record into a preview row. Rows that should
// not be booked by default (neutral transfers, pending or refunded trades)
// come back with a skip status and a Chinese reason shown in the UI.
func parseImportRow(matcher *importCategoryMatcher, source string, columns map[string]int, record []string, line int) *importRow {
	cells := make([]string, len(record))
	blank := true
	for j, cell := range record {
		cells[j] = cleanImportCell(cell)
		if cells[j] != "" {
			blank = false
		}
	}
	if blank {
		return nil
	}
	cellAt := func(name string) string {
		index, ok := columns[name]
		if !ok || index >= len(cells) {
			return ""
		}
		return cells[index]
	}
	occurredRaw := cellAt("交易时间")
	occurredAt, ok := parseImportBillTime(occurredRaw)
	if !ok {
		return &importRow{Line: line, OccurredAt: occurredRaw, Status: importRowStatusSkipped, SkipReason: "时间无法解析"}
	}
	amountRaw := ""
	if index, ok := importColumnIndex(columns, "金额"); ok && index < len(cells) {
		amountRaw = cells[index]
	}
	amountCents, err := parseImportAmount(amountRaw)
	if err != nil {
		return &importRow{Line: line, OccurredAt: occurredAt.Format("2006-01-02 15:04:05"), Status: importRowStatusSkipped, SkipReason: "金额无法解析"}
	}
	if amountCents <= 0 {
		// e.g. 医保统筹 rows where no cash changes hands; the app only books
		// positive amounts, so surface them for review instead of failing.
		return &importRow{Line: line, OccurredAt: occurredAt.Format("2006-01-02 15:04:05"), AmountCents: amountCents, Status: importRowStatusSkipped, SkipReason: "零金额账单"}
	}
	externalID := cellAt("交易订单号")
	if source == importSourceWechat {
		externalID = cellAt("交易单号")
	}
	direction := cellAt("收/支")
	kind := ""
	switch direction {
	case "支出":
		kind = KindExpense
	case "收入":
		kind = KindIncome
	}
	statusText := cellAt("交易状态")
	if source == importSourceWechat {
		statusText = cellAt("当前状态")
	}
	tradeType := cellAt("交易分类")
	if source == importSourceWechat {
		tradeType = cellAt("交易类型")
	}
	// Skipped rows still carry time, amount, direction and the platform order
	// id so the preview can show them and refund pairing can match them.
	skipped := func(reason string) *importRow {
		return &importRow{Line: line, OccurredAt: occurredAt.Format("2006-01-02 15:04:05"), Kind: kind, AmountCents: amountCents, ExternalID: externalID, Status: importRowStatusSkipped, SkipReason: reason}
	}

	counterparty := cellAt("交易对方")
	product := cellAt("商品说明")
	if source == importSourceWechat {
		product = cellAt("商品")
	}
	remark := cellAt("备注")

	// Refunds: Alipay books them as neutral rows with 退款成功 status, WeChat
	// as income rows; both mean money came back, so import them as income.
	isRefund := false
	if kind == "" && (strings.Contains(statusText, "退款成功") || tradeType == "退款") {
		kind = KindIncome
		tradeType = "退款"
		isRefund = true
	}
	if kind == "" {
		return skipped("中性交易")
	}
	if importStatusPending(statusText) {
		return skipped("未完成交易")
	}
	if kind == KindExpense && strings.Contains(statusText, "退款") {
		return skipped("已退款")
	}

	hintParts := make([]string, 0, 3)
	for _, part := range []string{tradeType, counterparty, product, remark} {
		if part != "" && part != "/" {
			hintParts = append(hintParts, part)
		}
	}
	hintText := strings.Join(hintParts, " ")
	category, matched := matcher.suggest(kind, tradeType, hintText)

	noteParts := make([]string, 0, 2)
	if product != "" && product != "/" {
		noteParts = append(noteParts, product)
	}
	if counterparty != "" && counterparty != "/" {
		noteParts = append(noteParts, counterparty)
	}
	note := strings.Join(noteParts, " · ")
	if remark != "" && remark != "/" {
		note = strings.TrimSpace(note + "（" + remark + "）")
	}

	return &importRow{
		Line:            line,
		OccurredAt:      occurredAt.Format("2006-01-02 15:04:05"),
		Kind:            kind,
		AmountCents:     amountCents,
		Category:        category,
		CategoryMatched: matched,
		Note:            note,
		ExternalID:      externalID,
		Status:          importRowStatusImport,
		refund:          isRefund,
	}
}

// pairImportRefunds drops Alipay refund rows whose original expense never
// entered the books. A refund's 交易订单号 carries the original order id as a
// prefix (e.g. ...8655_INP...), so a refund pairing with a skipped original
// (交易关闭 / 等待付款) must be skipped too, otherwise the refund would be
// booked as income without the matching expense. Refunds paired with an
// imported original, or with no original in the file, stay importable.
func pairImportRefunds(rows []importRow) {
	for i := range rows {
		refund := &rows[i]
		if !refund.refund || refund.ExternalID == "" {
			continue
		}
		for j := range rows {
			original := &rows[j]
			if i == j || original.refund || original.Kind != KindExpense || original.ExternalID == "" {
				continue
			}
			if original.AmountCents != refund.AmountCents || !strings.HasPrefix(refund.ExternalID, original.ExternalID) {
				continue
			}
			if original.Status != importRowStatusImport {
				refund.Status = importRowStatusSkipped
				refund.SkipReason = "原单未计支出，退款不重复入账"
			}
			break
		}
	}
}

type importDedupKey struct {
	ExternalID string
	Kind       string
	Amount     int64
	At         int64
}

// markImportDuplicates flags rows already stored by a previous import of the
// same platform, plus duplicates inside the file itself. The lookup is scoped
// to the file's time span so users with long histories stay cheap. WeChat
// refund rows reuse the original order id, hence Kind in the key.
func markImportDuplicates(db *gorm.DB, userID uint, source string, rows []importRow) {
	bound := time.Hour * 24
	var minAt, maxAt time.Time
	seen := map[importDedupKey]bool{}
	for i := range rows {
		row := &rows[i]
		if row.Status == importRowStatusSkipped || row.ExternalID == "" {
			continue
		}
		occurredAt, ok := parseImportBillTime(row.OccurredAt)
		if !ok {
			continue
		}
		key := importDedupKey{row.ExternalID, row.Kind, row.AmountCents, occurredAt.Unix()}
		if seen[key] {
			row.Status = importRowStatusDuplicate
			row.SkipReason = "重复账单"
			continue
		}
		seen[key] = true
		if minAt.IsZero() || occurredAt.Before(minAt) {
			minAt = occurredAt
		}
		if maxAt.IsZero() || occurredAt.After(maxAt) {
			maxAt = occurredAt
		}
	}
	if minAt.IsZero() {
		return
	}
	var existing []PersonalTransaction
	if err := db.Select("external_id, kind, amount_cents, occurred_at").
		Where("user_id = ? AND source = ? AND external_id <> '' AND occurred_at >= ? AND occurred_at <= ?",
			userID, source, minAt.Add(-bound), maxAt.Add(bound)).
		Find(&existing).Error; err != nil {
		return
	}
	stored := make(map[importDedupKey]bool, len(existing))
	for _, item := range existing {
		stored[importDedupKey{item.ExternalID, item.Kind, item.AmountCents, item.OccurredAt.Unix()}] = true
	}
	for i := range rows {
		row := &rows[i]
		if row.Status != importRowStatusImport || row.ExternalID == "" {
			continue
		}
		occurredAt, ok := parseImportBillTime(row.OccurredAt)
		if !ok {
			continue
		}
		if stored[importDedupKey{row.ExternalID, row.Kind, row.AmountCents, occurredAt.Unix()}] {
			row.Status = importRowStatusDuplicate
			row.SkipReason = "重复账单"
		}
	}
}

func handleParsePersonalImport(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("userID")
		if _, err := ensurePersonalSetup(db, userID); err != nil {
			response.ErrorInternal(c, "解析账单失败")
			return
		}
		fileHeader, err := c.FormFile("file")
		if err != nil {
			response.ErrorBadRequest(c, "请选择要导入的账单文件")
			return
		}
		if fileHeader.Size > importMaxFileSize {
			response.ErrorBadRequest(c, "账单文件不能超过 5MB")
			return
		}
		fileName := strings.ToLower(strings.TrimSpace(fileHeader.Filename))
		var records [][]string
		switch {
		case strings.HasSuffix(fileName, ".csv"):
			file, err := fileHeader.Open()
			if err != nil {
				response.ErrorBadRequest(c, "读取账单文件失败")
				return
			}
			raw, err := io.ReadAll(io.LimitReader(file, importMaxFileSize+1))
			file.Close()
			if err != nil {
				response.ErrorBadRequest(c, "读取账单文件失败")
				return
			}
			if len(raw) > importMaxFileSize {
				response.ErrorBadRequest(c, "账单文件不能超过 5MB")
				return
			}
			content, err := decodeImportCSV(raw)
			if err != nil {
				response.ErrorBadRequest(c, err.Error())
				return
			}
			reader := csv.NewReader(bytes.NewReader(content))
			reader.FieldsPerRecord = -1
			reader.TrimLeadingSpace = true
			records, err = reader.ReadAll()
			if err != nil {
				response.ErrorBadRequest(c, "账单文件不是有效的 CSV 格式")
				return
			}
		case strings.HasSuffix(fileName, ".xlsx"):
			file, err := fileHeader.Open()
			if err != nil {
				response.ErrorBadRequest(c, "读取账单文件失败")
				return
			}
			raw, err := io.ReadAll(io.LimitReader(file, importMaxFileSize+1))
			file.Close()
			if err != nil {
				response.ErrorBadRequest(c, "读取账单文件失败")
				return
			}
			if len(raw) > importMaxFileSize {
				response.ErrorBadRequest(c, "账单文件不能超过 5MB")
				return
			}
			records, err = readImportXLSXRecords(raw)
			if err != nil {
				response.ErrorBadRequest(c, err.Error())
				return
			}
		default:
			response.ErrorBadRequest(c, "请上传解压后的 CSV 或 xlsx 账单文件，暂不支持直接上传加密压缩包")
			return
		}
		source, columns, headerIndex, err := detectImportSource(records)
		if err != nil {
			response.ErrorBadRequest(c, err.Error())
			return
		}
		matcher, err := loadImportCategoryMatcher(db, userID)
		if err != nil {
			response.ErrorInternal(c, "解析账单失败")
			return
		}
		rows := make([]importRow, 0, len(records))
		for i := headerIndex + 1; i < len(records); i++ {
			row := parseImportRow(matcher, source, columns, records[i], i+1)
			if row == nil {
				continue
			}
			rows = append(rows, *row)
			if len(rows) > importMaxRows {
				response.ErrorBadRequest(c, fmt.Sprintf("账单笔数超过上限 %d 笔，请分时间段分批导出", importMaxRows))
				return
			}
		}
		if len(rows) == 0 {
			response.ErrorBadRequest(c, "账单文件中没有解析到交易记录")
			return
		}
		pairImportRefunds(rows)
		markImportDuplicates(db, userID, source, rows)
		response.Success(c, gin.H{"source": source, "total": len(rows), "rows": rows})
	}
}

func handleConfirmPersonalImport(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetUint("userID")
		if _, err := ensurePersonalSetup(db, userID); err != nil {
			response.ErrorInternal(c, "导入账单失败")
			return
		}
		var req importConfirmRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			response.ErrorBadRequest(c, "请提供完整的导入信息")
			return
		}
		if req.Source != importSourceAlipay && req.Source != importSourceWechat {
			response.ErrorBadRequest(c, "导入来源无效")
			return
		}
		if len(req.Rows) == 0 {
			response.ErrorBadRequest(c, "请选择要导入的账单")
			return
		}
		if len(req.Rows) > importMaxRows {
			response.ErrorBadRequest(c, fmt.Sprintf("单次最多导入 %d 笔账单", importMaxRows))
			return
		}
		// Each row carries its own ledger and account (editable in the import
		// preview), so ownership checks run against the user's writable sets
		// loaded once instead of per-row queries.
		var ledgers []PersonalLedger
		if err := db.Where("user_id = ? AND archived = ?", userID, false).Find(&ledgers).Error; err != nil {
			response.ErrorInternal(c, "导入账单失败")
			return
		}
		validLedgers := make(map[uint]bool, len(ledgers))
		for _, ledger := range ledgers {
			validLedgers[ledger.ID] = true
		}
		var accounts []PersonalAccount
		if err := db.Where("user_id = ? AND archived = ?", userID, false).Find(&accounts).Error; err != nil {
			response.ErrorInternal(c, "导入账单失败")
			return
		}
		validAccounts := make(map[uint]bool, len(accounts))
		for _, account := range accounts {
			validAccounts[account.ID] = true
		}
		var categories []PersonalCategory
		if err := db.Where("user_id = ?", userID).Find(&categories).Error; err != nil {
			response.ErrorInternal(c, "导入账单失败")
			return
		}
		validCategories := make(map[string]bool, len(categories))
		for _, category := range categories {
			validCategories[category.Kind+"/"+category.Name] = true
		}
		items := make([]PersonalTransaction, 0, len(req.Rows))
		for index, row := range req.Rows {
			if !validLedgers[row.LedgerID] {
				response.ErrorBadRequest(c, fmt.Sprintf("第 %d 行未选择有效账本", index+1))
				return
			}
			if !validAccounts[row.AccountID] {
				response.ErrorBadRequest(c, fmt.Sprintf("第 %d 行未选择有效收支账户", index+1))
				return
			}
			if !validKind(row.Kind) || row.AmountCents <= 0 || row.AmountCents > 999999999999 {
				response.ErrorBadRequest(c, fmt.Sprintf("第 %d 行金额或类型无效", index+1))
				return
			}
			category := cleanText(row.Category, 64)
			if !validCategories[row.Kind+"/"+category] {
				response.ErrorBadRequest(c, fmt.Sprintf("第 %d 行分类无效，请重新选择分类", index+1))
				return
			}
			occurredAt, err := parseOccurredAt(row.OccurredAt)
			if err != nil {
				response.ErrorBadRequest(c, fmt.Sprintf("第 %d 行时间格式不正确", index+1))
				return
			}
			items = append(items, PersonalTransaction{
				UserID:      userID,
				LedgerID:    row.LedgerID,
				AccountID:   row.AccountID,
				Kind:        row.Kind,
				AmountCents: row.AmountCents,
				Category:    category,
				Note:        cleanText(row.Note, 500),
				OccurredAt:  occurredAt,
				Source:      req.Source,
				ExternalID:  cleanText(row.ExternalID, 64),
			})
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			return tx.CreateInBatches(&items, 500).Error
		}); err != nil {
			response.ErrorInternal(c, "导入账单失败")
			return
		}
		audit.Log(db, c, "personal_transaction_import", "personal_transaction", 0,
			fmt.Sprintf("导入%s账单 %d 笔", importSourceLabels[req.Source], len(items)))
		response.Success(c, gin.H{"imported": len(items)})
	}
}
