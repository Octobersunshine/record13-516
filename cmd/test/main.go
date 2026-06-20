package main

import (
	"fmt"
	"log"

	"reconciliation/alipay"
	"reconciliation/model"
	"reconciliation/pkg/timeutil"
	"reconciliation/service"
	"reconciliation/wechat"
)

func main() {
	testBasicParse()
	testCrossDayFiltering()
	testGracePeriod()
	testTimezoneConsistency()
	testFullReconciliation()
}

func testBasicParse() {
	fmt.Println("=== 测试1: 基础解析测试 ===")
	wechatSvc := wechat.NewReconciliationService(nil)
	records, err := wechatSvc.ParseBillFile("testdata/wechat_bill.csv")
	if err != nil {
		log.Fatalf("微信账单解析失败: %v", err)
	}

	fmt.Printf("微信解析记录数: %d\n", len(records))
	for i, r := range records {
		fmt.Printf("  记录%d: 类型=%s, 金额=%s, 订单号=%s, 时间=%s\n",
			i+1, r.TradeType, service.FormatAmount(r.Amount), r.OutTradeNo,
			timeutil.FormatDateTime(r.TradeTime))
	}

	alipaySvc := alipay.NewReconciliationService(nil)
	alipayRecords, err := alipaySvc.ParseBillFile("testdata/alipay_bill.csv")
	if err != nil {
		log.Fatalf("支付宝账单解析失败: %v", err)
	}
	fmt.Printf("支付宝解析记录数: %d\n", len(alipayRecords))
	for i, r := range alipayRecords {
		fmt.Printf("  记录%d: 类型=%s, 金额=%s, 订单号=%s, 时间=%s\n",
			i+1, r.TradeType, service.FormatAmount(r.Amount), r.OutTradeNo,
			timeutil.FormatDateTime(r.TradeTime))
	}
	fmt.Println()
}

func testCrossDayFiltering() {
	fmt.Println("=== 测试2: 跨日交易日期过滤测试 ===")
	wechatSvc := wechat.NewReconciliationService(nil)
	allRecords, err := wechatSvc.ParseBillFile("testdata/wechat_bill_crossday.csv")
	if err != nil {
		log.Fatalf("跨日微信账单解析失败: %v", err)
	}

	fmt.Printf("总解析记录数: %d\n", len(allRecords))

	filtered, err := wechat.FilterByDate(allRecords, "2024-01-15", 0)
	if err != nil {
		log.Fatalf("日期过滤失败: %v", err)
	}

	fmt.Printf("2024-01-15 过滤后记录数: %d (应过滤掉前日23:59:58和次日00:00:05的记录)\n", len(filtered))
	fmt.Println("过滤后保留的记录:")
	for i, r := range filtered {
		fmt.Printf("  [%d] %s - %s - %s\n", i+1,
			timeutil.FormatDateTime(r.TradeTime),
			r.OutTradeNo,
			service.FormatAmount(r.Amount))
	}

	expectedCount := 8
	if len(filtered) != expectedCount {
		fmt.Printf("⚠️  警告: 期望过滤后 %d 条记录, 实际 %d 条\n", expectedCount, len(filtered))
	} else {
		fmt.Printf("✅ 过滤数量正确: %d 条 (00:00:02和23:59:59都在当日范围内)\n", expectedCount)
	}
	fmt.Println()
}

func testGracePeriod() {
	fmt.Println("=== 测试3: 宽限期(grace period)处理 ===")
	wechatSvc := wechat.NewReconciliationService(nil)
	allRecords, err := wechatSvc.ParseBillFile("testdata/wechat_bill_crossday.csv")
	if err != nil {
		log.Fatalf("跨日微信账单解析失败: %v", err)
	}

	fmt.Println("--- 宽限期 0 分钟 (严格) ---")
	filteredStrict, _ := wechat.FilterByDate(allRecords, "2024-01-15", 0)
	fmt.Printf("过滤后: %d 条\n", len(filteredStrict))
	for _, r := range filteredStrict {
		fmt.Printf("  %s - %s\n", timeutil.FormatDateTime(r.TradeTime), r.OutTradeNo)
	}

	fmt.Println("--- 宽限期 5 分钟 (包含边界前后5分钟) ---")
	filteredGrace, _ := wechat.FilterByDate(allRecords, "2024-01-15", 5)
	fmt.Printf("过滤后: %d 条 (应包含23:59:58的前日记录和00:00:02的次日记录)\n", len(filteredGrace))
	for _, r := range filteredGrace {
		fmt.Printf("  %s - %s\n", timeutil.FormatDateTime(r.TradeTime), r.OutTradeNo)
	}

	expectedGraceCount := 10
	if len(filteredGrace) != expectedGraceCount {
		fmt.Printf("⚠️  警告: 期望宽限期过滤后 %d 条记录, 实际 %d 条\n", expectedGraceCount, len(filteredGrace))
	} else {
		fmt.Printf("✅ 宽限期过滤数量正确: %d 条 (含前日23:59:58和次日00:00:05, 都在5分钟宽限内)\n", expectedGraceCount)
	}
	fmt.Println()
}

func testTimezoneConsistency() {
	fmt.Println("=== 测试4: 时区一致性验证 ===")

	fmt.Printf("统一时区: Asia/Shanghai (CST, UTC+8)\n")

	now := timeutil.Now()
	fmt.Printf("当前时间: %s (时区: %s)\n", timeutil.FormatDateTime(now), now.Location().String())

	testDate := "2024-01-15"
	start, end, err := timeutil.DateRange(testDate)
	if err != nil {
		log.Fatalf("日期范围解析失败: %v", err)
	}
	fmt.Printf("日期范围: %s\n", testDate)
	fmt.Printf("  起始: %s (Unix: %d)\n", timeutil.FormatDateTime(start), start.Unix())
	fmt.Printf("  结束: %s (Unix: %d)\n", timeutil.FormatDateTime(end), end.Unix())
	fmt.Printf("  起始时区: %s\n", start.Location().String())
	fmt.Printf("  结束时区: %s\n", end.Location().String())

	if start.Location().String() != end.Location().String() {
		fmt.Println("❌ 错误: 起始和结束时区不一致")
	} else {
		fmt.Println("✅ 时区一致性验证通过")
	}

	graceStart, graceEnd, _ := timeutil.DateRangeWithGrace(testDate, timeutil.DateRangeOption{
		GracePeriodMinutes: 10,
	})
	fmt.Printf("宽限期10分钟范围:\n")
	fmt.Printf("  起始: %s\n", timeutil.FormatDateTime(graceStart))
	fmt.Printf("  结束: %s\n", timeutil.FormatDateTime(graceEnd))
	fmt.Println()
}

func testFullReconciliation() {
	fmt.Println("=== 测试5: 完整对账流程(含日期过滤) ===")
	cfg := &model.ReconciliationConfig{
		Wechat: &model.WechatConfig{},
		Alipay: &model.AlipayConfig{},
	}
	svc := service.NewReconciliationService(cfg)

	wechatRecords, _ := svc.ParseWechatBillFile("testdata/wechat_bill_crossday.csv")
	alipayRecords, _ := svc.ParseAlipayBillFile("testdata/alipay_bill.csv")

	fmt.Printf("--- 使用 ReconcileWithOptions, 宽限期5分钟 ---\n")
	result := svc.ReconcileWithOptions("2024-01-15", wechatRecords, alipayRecords, service.ReconcileOptions{
		GraceMinutes: 5,
	})

	fmt.Printf("对账日期: %s\n", result.Date)
	fmt.Printf("\n微信汇总 (宽限期5分钟, 包含前日23:59:58和次日00:00:02):\n")
	if result.WechatSummary != nil {
		fmt.Printf("  总笔数: %d, 净金额: %s\n", result.WechatSummary.TotalCount, service.FormatAmount(result.WechatSummary.TotalAmount))
		fmt.Printf("  支付笔数: %d, 支付金额: %s\n", result.WechatSummary.PaymentCount, service.FormatAmount(result.WechatSummary.PaymentAmount))
		fmt.Printf("  退款笔数: %d, 退款金额: %s\n", result.WechatSummary.RefundCount, service.FormatAmount(result.WechatSummary.RefundAmount))
		fmt.Printf("  手续费: %s\n", service.FormatAmount(result.WechatSummary.TotalFee))
	}

	fmt.Printf("\n支付宝汇总:\n")
	if result.AlipaySummary != nil {
		fmt.Printf("  总笔数: %d, 净金额: %s\n", result.AlipaySummary.TotalCount, service.FormatAmount(result.AlipaySummary.TotalAmount))
	}

	fmt.Printf("\n合并汇总:\n")
	if result.CombinedSummary != nil {
		fmt.Printf("  总笔数: %d\n", result.CombinedSummary.TotalCount)
		fmt.Printf("  净金额: %s\n", service.FormatAmount(result.CombinedSummary.TotalAmount))
		fmt.Printf("  支付笔数: %d, 支付金额: %s\n", result.CombinedSummary.PaymentCount, service.FormatAmount(result.CombinedSummary.PaymentAmount))
		fmt.Printf("  退款笔数: %d, 退款金额: %s\n", result.CombinedSummary.RefundCount, service.FormatAmount(result.CombinedSummary.RefundAmount))
		fmt.Printf("  手续费: %s\n", service.FormatAmount(result.CombinedSummary.TotalFee))
	}

	fmt.Printf("\n--- 使用严格模式 (宽限期0分钟) 对比 ---\n")
	strictResult := svc.ReconcileWithOptions("2024-01-15", wechatRecords, alipayRecords, service.ReconcileOptions{
		GraceMinutes: 0,
	})
	if strictResult.WechatSummary != nil && result.WechatSummary != nil {
		fmt.Printf("  微信严格模式笔数: %d vs 宽限期模式: %d (差异: %d)\n",
			strictResult.WechatSummary.TotalCount,
			result.WechatSummary.TotalCount,
			result.WechatSummary.TotalCount-strictResult.WechatSummary.TotalCount)
		fmt.Printf("  微信严格模式金额: %s vs 宽限期模式: %s\n",
			service.FormatAmount(strictResult.WechatSummary.TotalAmount),
			service.FormatAmount(result.WechatSummary.TotalAmount))
	}

	fmt.Println("\n=== 所有测试完成 ===")
}
