package billing

import (
	"bytes"
	"encoding/csv"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"smallgo/server/database"

	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/simplifiedchinese"
	"gorm.io/gorm"
)

const testWechatCSV = `微信支付账单明细,,,,,,,,,,
微信昵称：[测试],,,,,,,,,,
起始时间：[2024-01-01 00:00:00] 终止时间：[2024-03-31 23:59:59],,,,,,,,,,
导出类型：[全部],,,,,,,,,,
导出时间：[2024-04-01 08:00:00],,,,,,,,,,
,,,,,,,,,,
共7笔记录,,,,,,,,,,
收入：2笔 23.35元,,,,,,,,,,
支出：2笔 628.16元,,,,,,,,,,
中性交易：2笔 2100.00元,,,,,,,,,,
注：,,,,,,,,,,
1. 充值/提现/理财通购买/零钱通存取/信用卡还款等交易，将计入中性交易,,,,,,,,,,
2. 本明细仅展示当前账单中的交易，不包括已删除的记录,,,,,,,,,,
3. 本明细仅供个人对账使用,,,,,,,,,,
,,,,,,,,,,
----------------------微信支付账单明细列表--------------------,,,,,,,,,,
交易时间,交易类型,交易对方,商品,收/支,金额(元),支付方式,当前状态,交易单号,商户单号,备注
2024-03-20 08:53:26,商户消费,云膳过桥米线,总共消费:28.16,支出,¥28.16,零钱,支付成功,4200002403201406599120	,20200000085313893125	,/
2024-03-21 10:10:11,微信红包,同性好友,/,收入,¥0.35,/,已存入零钱,4200002403211547583210	,129847129	,/
2024-03-22 12:34:56,转账,房东,转账备注:微信转账,支出,¥600.00,零钱通,朋友已收钱,4200002403220812345678	,1000220322567890	,/
2024-03-23 09:00:00,商户消费,某店铺,商品A,支出,¥100.00,零钱,已全额退款(退款),4200002403230911111111	,1000220321111111	,/
2024-03-24 19:29:00,二维码收款,用户A,收款方备注:二维码收款,收入,¥23.00,零钱,已收钱,4200002403241922222222	,/	,/
2024-03-25 10:07:31,转入零钱通-来自工商银行(9876),/,/,/,¥2000.00,工商银行(9876),支付成功,4200002403251033333333	,129847129	,/
2024-03-26 16:29:37,零钱提现,工商银行(),"/",/,¥100.10,工商银行,提现已到账,207210715100077148235523883175	,/	,"服务费¥0.10"
2024-03-27 09:00:00,商户消费,某电商,预付款,支出,¥88.00,零钱,等待付款,4200002403270944444444	,1000220324444444	,/
`

const testAlipayCSV = `------------------------------------------------------------------------------------
导出信息：
姓名：测试
支付宝账户：test@example.com
起始时间：[2024-03-01 00:00:00]    终止时间：[2024-03-31 23:59:59]
导出交易类型：[全部]
导出时间：[2024-04-01 08:58:35]
共5笔记录
收入：1笔 222.50元
支出：2笔 26.50元
不计收支：2笔 115.37元

特别提示：
1.本回单内容可表明支付宝受理了相应支付交易申请。
2.请勿将本回单作为收款方发货的凭据使用。
3.部分账单如：充值提现、账户转存等不计入为收入或者支出，记为不计收支类。

------------------------支付宝（中国）网络技术有限公司  电子客户回单------------------------
交易时间            ,交易分类                ,交易对方                ,对方账号                ,商品说明                ,收/支                 ,金额                  ,收/付款方式              ,交易状态                ,交易订单号     ,商家订单号           ,备注                  ,
2024-03-20 08:40:44 ,交通出行                ,大连明珠公用卡股份有限公司 ,zfb***@126.com ,公交-9404路 ,支出 ,1.00 ,余额宝 ,交易成功 ,2024032001440003121414081572	 ,20240328167300956962817	 ,                    ,
2024-03-21 10:10:10 ,餐饮美食                ,肯德基 ,/ ,早餐 ,支出 ,25.50 ,余额 ,交易成功 ,2024032111440003121414081573	 ,20240321167300956962818	 ,                    ,
2024-03-22 14:16:52 ,退款                ,某店铺 ,/ ,退款-商品A ,不计收支 ,16.03 ,交通银行信用卡(7449) ,退款成功 ,2024032215440003121414081574	 ,20240322167300956962819	 ,                    ,
2024-03-23 18:21:04 ,投资理财                ,蚂蚁财富 ,/ ,蚂蚁财富-卖出至余额宝 ,不计收支 ,99.34 ,余额宝 ,交易成功 ,2024032318440003121414081575	 ,                    	 ,                    ,
2024-03-24 20:17:29 ,转账红包                ,朋友 ,friend***@163.com ,转账 ,收入 ,222.50 ,余额 ,交易成功 ,2024032420440003121414081576	 ,                    	 ,                    ,
`

func readImportTestRecords(t *testing.T, content []byte) [][]string {
	t.Helper()
	reader := csv.NewReader(bytes.NewReader(content))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read test csv: %v", err)
	}
	return records
}

func setupImportTestUser(t *testing.T) (*gorm.DB, uint) {
	t.Helper()
	db, err := database.InitDB(filepath.Join(t.TempDir(), "billing-import-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.CloseDB(db) })
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	user := database.User{Username: "import-user", Password: "test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ensurePersonalSetup(db, user.ID); err != nil {
		t.Fatal(err)
	}
	return db, user.ID
}

func TestDecodeImportCSVHandlesGB18030AndBOM(t *testing.T) {
	gbk, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(testAlipayCSV))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeImportCSV(gbk)
	if err != nil {
		t.Fatalf("decode gb18030: %v", err)
	}
	if !strings.Contains(string(decoded), "支付宝（中国）网络技术有限公司") {
		t.Fatal("gb18030 content was not decoded")
	}
	if _, err := decodeImportCSV([]byte(testWechatCSV)); err != nil {
		t.Fatalf("utf-8 content should pass through: %v", err)
	}
	bom := append([]byte{0xEF, 0xBB, 0xBF}, []byte(testWechatCSV)...)
	decoded, err = decodeImportCSV(bom)
	if err != nil {
		t.Fatalf("decode bom: %v", err)
	}
	if strings.HasPrefix(string(decoded), "\ufeff") {
		t.Fatal("BOM was not stripped")
	}
}

func TestDetectImportSource(t *testing.T) {
	wechatRecords := readImportTestRecords(t, []byte(testWechatCSV))
	source, _, headerIndex, err := detectImportSource(wechatRecords)
	if err != nil {
		t.Fatal(err)
	}
	if source != importSourceWechat || headerIndex != 16 {
		t.Fatalf("source=%s headerIndex=%d, want wechat/16", source, headerIndex)
	}

	gbk, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(testAlipayCSV))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeImportCSV(gbk)
	if err != nil {
		t.Fatal(err)
	}
	alipayRecords := readImportTestRecords(t, decoded)
	source, _, headerIndex, err = detectImportSource(alipayRecords)
	if err != nil {
		t.Fatal(err)
	}
	if source != importSourceAlipay || headerIndex != 16 {
		t.Fatalf("source=%s headerIndex=%d, want alipay/16", source, headerIndex)
	}

	if _, _, _, err := detectImportSource(readImportTestRecords(t, []byte("a,b,c\n1,2,3\n"))); err == nil {
		t.Fatal("unknown csv should be rejected")
	}
}

func parseImportTestRows(t *testing.T, content []byte, source string) []importRow {
	t.Helper()
	records := readImportTestRecords(t, content)
	_, columns, headerIndex, err := detectImportSource(records)
	if err != nil {
		t.Fatal(err)
	}
	db, userID := setupImportTestUser(t)
	matcher, err := loadImportCategoryMatcher(db, userID)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]importRow, 0)
	for i := headerIndex + 1; i < len(records); i++ {
		row := parseImportRow(matcher, source, columns, records[i], i+1)
		if row != nil {
			rows = append(rows, *row)
		}
	}
	return rows
}

func TestParseImportWechatRows(t *testing.T) {
	rows := parseImportTestRows(t, []byte(testWechatCSV), importSourceWechat)
	if len(rows) != 8 {
		t.Fatalf("got %d rows, want 8", len(rows))
	}
	first := rows[0]
	if first.Kind != KindExpense || first.AmountCents != 2816 || first.Category != "餐饮" || !first.CategoryMatched {
		t.Fatalf("first wechat row = %#v", first)
	}
	if first.ExternalID != "4200002403201406599120" || strings.ContainsAny(first.ExternalID, "\t ") {
		t.Fatalf("external id was not cleaned: %q", first.ExternalID)
	}
	if !strings.Contains(first.Note, "云膳过桥米线") {
		t.Fatalf("note should carry counterparty: %q", first.Note)
	}

	if rows[1].Kind != KindIncome || rows[1].AmountCents != 35 || rows[1].Category != "礼金" {
		t.Fatalf("red packet row = %#v", rows[1])
	}
	if rows[2].Kind != KindExpense || rows[2].AmountCents != 60000 || rows[2].Category != "其他" || rows[2].CategoryMatched {
		t.Fatalf("transfer row should fall back to 其他: %#v", rows[2])
	}
	if rows[3].Status != importRowStatusSkipped || rows[3].SkipReason != "已退款" {
		t.Fatalf("refunded expense row = %#v", rows[3])
	}
	if rows[3].OccurredAt != "2024-03-23 09:00:00" || rows[3].AmountCents != 10000 {
		t.Fatalf("skipped rows should keep time and amount: %#v", rows[3])
	}
	if rows[4].Kind != KindIncome || rows[4].AmountCents != 2300 {
		t.Fatalf("qr income row = %#v", rows[4])
	}
	if rows[5].Status != importRowStatusSkipped || rows[5].SkipReason != "中性交易" {
		t.Fatalf("zero-money-fund row = %#v", rows[5])
	}
	if rows[6].Status != importRowStatusSkipped || rows[6].SkipReason != "中性交易" {
		t.Fatalf("withdraw row = %#v", rows[6])
	}
	if rows[7].Status != importRowStatusSkipped || rows[7].SkipReason != "未完成交易" {
		t.Fatalf("pending row = %#v", rows[7])
	}
}

func TestParseImportAlipayRows(t *testing.T) {
	gbk, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(testAlipayCSV))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeImportCSV(gbk)
	if err != nil {
		t.Fatal(err)
	}
	rows := parseImportTestRows(t, decoded, importSourceAlipay)
	if len(rows) != 5 {
		t.Fatalf("got %d rows, want 5", len(rows))
	}
	if rows[0].Kind != KindExpense || rows[0].AmountCents != 100 || rows[0].Category != "交通" {
		t.Fatalf("bus row = %#v", rows[0])
	}
	if rows[0].ExternalID != "2024032001440003121414081572" {
		t.Fatalf("external id was not cleaned: %q", rows[0].ExternalID)
	}
	if rows[1].Category != "餐饮" {
		t.Fatalf("kfc row category = %q, want 餐饮", rows[1].Category)
	}
	if rows[2].Kind != KindIncome || rows[2].AmountCents != 1603 || rows[2].Category != "退款" || !rows[2].CategoryMatched {
		t.Fatalf("refund row should import as income: %#v", rows[2])
	}
	if rows[3].Status != importRowStatusSkipped || rows[3].SkipReason != "中性交易" {
		t.Fatalf("fund row = %#v", rows[3])
	}
	if rows[4].Kind != KindIncome || rows[4].AmountCents != 22250 || rows[4].Category != "礼金" {
		t.Fatalf("transfer income row = %#v", rows[4])
	}
}

func TestMarkImportDuplicatesSkipsExistingOrders(t *testing.T) {
	db, userID := setupImportTestUser(t)
	occurredAt, err := time.ParseInLocation("2006-01-02 15:04:05", "2024-03-20 08:53:26", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	existing := PersonalTransaction{
		UserID: userID, Kind: KindExpense, AmountCents: 2816, Category: "餐饮",
		OccurredAt: occurredAt, Source: importSourceWechat, ExternalID: "4200002403201406599120",
	}
	if err := db.Create(&existing).Error; err != nil {
		t.Fatal(err)
	}
	rows := parseImportTestRows(t, []byte(testWechatCSV), importSourceWechat)
	markImportDuplicates(db, userID, importSourceWechat, rows)
	if rows[0].Status != importRowStatusDuplicate || rows[0].SkipReason != "重复账单" {
		t.Fatalf("imported order should be marked duplicate: %#v", rows[0])
	}
	if rows[1].Status != importRowStatusImport {
		t.Fatalf("new order should stay importable: %#v", rows[1])
	}
	// 微信退款行与原单共用交易单号，方向不同不能误判为重复。
	refundLike := []importRow{{
		Line: 2, OccurredAt: "2024-03-23 09:30:00", Kind: KindIncome, AmountCents: 10000,
		Category: "退款", ExternalID: "4200002403230911111111", Status: importRowStatusImport,
	}}
	markImportDuplicates(db, userID, importSourceWechat, refundLike)
	if refundLike[0].Status != importRowStatusImport {
		t.Fatalf("refund sharing the order id must not be a duplicate: %#v", refundLike[0])
	}
}

// TestParseImportRefundPairingEndToEnd 走完整解析路径复现真实账单场景：
// 原单交易关闭（默认跳过）+ 独立退款行（不计收支+退款成功），退款必须一并跳过。
func TestParseImportRefundPairingEndToEnd(t *testing.T) {
	csvContent := "交易时间,交易分类,交易对方,对方账号,商品说明,收/支,金额,收/付款方式,交易状态,交易订单号,商家订单号,备注\n" +
		"2026-08-22 22:23:19,文化休闲,抖音电商商家,/,抖音电商-订单编号1,支出,11.90,交通银行信用卡(2959),交易关闭,2026082223001410671423179835\t,\t,\n" +
		"2026-08-22 22:26:21,退款,抖音电商商家,/,退款-抖音电商-订单编号1,不计收支,11.90,交通银行信用卡(2959)&优惠,退款成功,2026082223001410671423179835_INP26082222260619014041254923455\t,\t,\n"
	records := readImportTestRecords(t, []byte(csvContent))
	source, columns, headerIndex, err := detectImportSource(records)
	if err != nil {
		t.Fatal(err)
	}
	db, userID := setupImportTestUser(t)
	matcher, err := loadImportCategoryMatcher(db, userID)
	if err != nil {
		t.Fatal(err)
	}
	rows := make([]importRow, 0)
	for i := headerIndex + 1; i < len(records); i++ {
		if row := parseImportRow(matcher, source, columns, records[i], i+1); row != nil {
			rows = append(rows, *row)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	pairImportRefunds(rows)
	if rows[0].Status != importRowStatusSkipped || rows[0].SkipReason != "未完成交易" {
		t.Fatalf("closed original should stay skipped: %#v", rows[0])
	}
	if rows[1].Status != importRowStatusSkipped || rows[1].SkipReason != "原单未计支出，退款不重复入账" {
		t.Fatalf("refund of a closed order should be skipped: %#v", rows[1])
	}
}

func TestParseImportAmount(t *testing.T) {
	tests := []struct {
		raw  string
		want int64
	}{
		{"28.16", 2816},
		{"¥28.16", 2816},
		{"¥1,234.56", 123456},
		{" 1.00\t", 100},
		{"0.35", 35},
	}
	for _, test := range tests {
		got, err := parseImportAmount(test.raw)
		if err != nil || got != test.want {
			t.Fatalf("parseImportAmount(%q) = %d, %v; want %d", test.raw, got, err, test.want)
		}
	}
	if _, err := parseImportAmount("/"); err == nil {
		t.Fatal("placeholder amount should fail")
	}
}

// TestParseImportXLSXRows 复现微信 App 导出的 xlsx 流水：交易时间是 Excel
// 序列日期数值、金额是无符号数字，表头行前还有大量说明行。
func TestParseImportXLSXRows(t *testing.T) {
	file := excelize.NewFile()
	sheet := file.GetSheetName(0)
	expect := []string{
		"2026-09-12 11:25:31", "商户消费", "贵阳体彩展示体验中心", "/", "支出", "6",
		"华夏银行储蓄卡(9519)", "支付成功", "4500000379202609123957310403", "10425294928917891835240193230601", "/",
	}
	serial, err := time.ParseInLocation("2006-01-02 15:04:05", expect[0], time.Local)
	if err != nil {
		t.Fatal(err)
	}
	// Excel 1900 日期系统：天数自 1899-12-30 起，用浮点天数避免时长溢出。
	epoch, _ := time.ParseInLocation("2006-01-02", "1899-12-30", time.Local)
	excelDays := serial.Sub(epoch).Hours() / 24
	values := []interface{}{excelDays, expect[1], expect[2], expect[3], expect[4], 6.0,
		expect[6], expect[7], expect[8], expect[9], expect[10]}
	header := []interface{}{"交易时间", "交易类型", "交易对方", "商品", "收/支", "金额(元)", "支付方式", "当前状态", "交易单号", "商户单号", "备注"}
	if err := file.SetSheetRow(sheet, "A18", &header); err != nil {
		t.Fatal(err)
	}
	if err := file.SetSheetRow(sheet, "A19", &values); err != nil {
		t.Fatal(err)
	}
	if err := file.SetSheetRow(sheet, "A1", &[]interface{}{"微信支付账单明细"}); err != nil {
		t.Fatal(err)
	}
	buffer, err := file.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	records, err := readImportXLSXRecords(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	source, columns, headerIndex, err := detectImportSource(records)
	if err != nil {
		t.Fatal(err)
	}
	if source != importSourceWechat || headerIndex != 17 {
		t.Fatalf("source=%s headerIndex=%d, want wechat/17", source, headerIndex)
	}
	db, userID := setupImportTestUser(t)
	matcher, err := loadImportCategoryMatcher(db, userID)
	if err != nil {
		t.Fatal(err)
	}
	row := parseImportRow(matcher, source, columns, records[18], 19)
	if row == nil || row.Kind != KindExpense || row.AmountCents != 600 {
		t.Fatalf("xlsx row = %#v", row)
	}
	if row.ExternalID != expect[8] || row.Category != "其他" {
		t.Fatalf("xlsx row fields = %#v", row)
	}
	parsed, ok := parseImportBillTime(row.OccurredAt)
	if !ok || parsed.Format("2006-01-02 15:04:05") != expect[0] {
		t.Fatalf("excel serial time = %v, want %s", parsed, expect[0])
	}
}

func TestPairImportRefunds(t *testing.T) {
	rows := []importRow{
		// 原单交易关闭（默认跳过），退款与原单单号前缀一致且金额相等 → 退款一并跳过。
		{Line: 2, Kind: KindExpense, AmountCents: 1190, ExternalID: "2026082223001410671423179835", Status: importRowStatusSkipped, SkipReason: "未完成交易"},
		{Line: 3, Kind: KindIncome, AmountCents: 1190, ExternalID: "2026082223001410671423179835_INP26082222260619014041254923455", Status: importRowStatusImport, refund: true},
		// 原单交易成功（导入），退款保持导入，净额为零。
		{Line: 5, Kind: KindExpense, AmountCents: 7700, ExternalID: "2026081923001110671409348655", Status: importRowStatusImport},
		{Line: 6, Kind: KindIncome, AmountCents: 7700, ExternalID: "2026081923001110671409348655_13180601326081975909538088276", Status: importRowStatusImport, refund: true},
		// 文件里找不到原单的退款 → 保留导入（真实退回的钱）。
		{Line: 8, Kind: KindIncome, AmountCents: 500, ExternalID: "2026082000000000000000000000_X", Status: importRowStatusImport, refund: true},
	}
	pairImportRefunds(rows)
	if rows[1].Status != importRowStatusSkipped || rows[1].SkipReason != "原单未计支出，退款不重复入账" {
		t.Fatalf("refund of a closed order should be skipped: %#v", rows[1])
	}
	if rows[3].Status != importRowStatusImport || rows[2].Status != importRowStatusImport {
		t.Fatalf("refund of an imported order should stay importable: %#v / %#v", rows[2], rows[3])
	}
	if rows[4].Status != importRowStatusImport {
		t.Fatalf("unpaired refund should stay importable: %#v", rows[4])
	}
}

func TestParseImportZeroAmountRow(t *testing.T) {
	db, userID := setupImportTestUser(t)
	matcher, err := loadImportCategoryMatcher(db, userID)
	if err != nil {
		t.Fatal(err)
	}
	columns := map[string]int{
		"交易时间": 0, "交易分类": 1, "交易对方": 2, "对方账号": 3, "商品说明": 4, "收/支": 5,
		"金额": 6, "收/付款方式": 7, "交易状态": 8, "交易订单号": 9,
	}
	record := []string{"2026-08-17 20:12:52", "医疗健康", "药房", "/", "医保支付(不含自费)", "支出", "0.00", "", "支付成功", "20260817106784458848"}
	row := parseImportRow(matcher, importSourceAlipay, columns, record, 2)
	if row == nil || row.Status != importRowStatusSkipped || row.SkipReason != "零金额账单" {
		t.Fatalf("zero amount row = %#v", row)
	}
}
