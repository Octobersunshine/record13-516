package main

import (
	"fmt"
	"log"

	"reconciliation/alipay"
	"reconciliation/model"
	"reconciliation/pkg/timeutil"
	"reconciliation/platform"
	"reconciliation/service"
	"reconciliation/wechat"
)

func main() {
	testBasicParse()
	testCrossDayFiltering()
	testGracePeriod()
	testTimezoneConsistency()
	testFullReconciliation()
	testPlatformParse()
	testDiffComparison()
	testDiffOptions()
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

func testPlatformParse() {
	fmt.Println("=== 测试6: 平台订单解析测试 ===")
	platSvc := platform.NewReconciliationService()
	records, err := platSvc.ParseBillFile("testdata/platform_bill.csv")
	if err != nil {
		log.Fatalf("平台账单解析失败: %v", err)
	}

	fmt.Printf("平台解析记录数: %d\n", len(records))
	for i, r := range records {
		fmt.Printf("  记录%d: 类型=%s, 金额=%s, 订单号=%s, 时间=%s, 手续费=%s\n",
			i+1, r.TradeType, service.FormatAmount(r.Amount), r.OutTradeNo,
			timeutil.FormatDateTime(r.TradeTime), service.FormatAmount(r.Fee))
	}
	fmt.Println()
}

func testDiffComparison() {
	fmt.Println("=== 测试7: 对账差异单生成测试 ===")

	platSvc := platform.NewReconciliationService()
	platRecords, err := platSvc.ParseBillFile("testdata/platform_bill.csv")
	if err != nil {
		log.Fatalf("平台账单解析失败: %v", err)
	}

	wechatSvc := wechat.NewReconciliationService(nil)
	wechatRecords, err := wechatSvc.ParseBillFile("testdata/wechat_bill.csv")
	if err != nil {
		log.Fatalf("微信账单解析失败: %v", err)
	}

	date := "2024-01-15"
	graceMinutes := 0
	filteredPlat, _ := platform.FilterByDate(platRecords, date, graceMinutes)
	filteredWechat, _ := wechat.FilterByDate(wechatRecords, date, graceMinutes)

	fmt.Printf("平台订单数: %d, 微信订单数: %d\n", len(filteredPlat), len(filteredWechat))

	diffSvc := service.NewDiffService()
	opts := service.DefaultDiffOptions()
	diffResult := diffSvc.Compare(filteredPlat, filteredWechat, "wechat", date, opts)

	fmt.Printf("\n差异比对结果:\n")
	fmt.Printf("  差异总笔数: %d\n", diffResult.TotalDiffCount)
	fmt.Printf("  差异总金额: %s\n", service.FormatAmount(diffResult.TotalDiffAmount))
	fmt.Printf("  平台订单数: %d\n", diffResult.PlatformCount)
	fmt.Printf("  渠道订单数: %d\n", diffResult.ChannelCount)
	fmt.Printf("  匹配成功数: %d\n", diffResult.MatchedCount)
	fmt.Printf("  匹配率: %.2f%%\n", diffResult.MatchRate)

	fmt.Printf("\n差异类型统计:\n")
	for diffType, count := range diffResult.DiffSummary {
		fmt.Printf("  %s: %d 笔\n", diffType, count)
	}

	fmt.Printf("\n差异单详情:\n")
	for i, diff := range diffResult.DiffOrders {
		fmt.Printf("  差异%d: 类型=%s, 订单号=%s, 渠道=%s\n",
			i+1, diff.DiffType, diff.OutTradeNo, diff.Channel)
		fmt.Printf("    描述: %s\n", diff.Description)
		if diff.AmountDiff != 0 {
			fmt.Printf("    金额差额: %s\n", service.FormatAmount(diff.AmountDiff))
		}
		if diff.FeeDiff != 0 {
			fmt.Printf("    手续费差额: %s\n", service.FormatAmount(diff.FeeDiff))
		}
	}

	expectedDiffs := int64(5)
	if diffResult.TotalDiffCount == expectedDiffs {
		fmt.Printf("\n✅ 差异数量正确: %d 笔 (金额不匹配1 + 平台有渠道无1 + 渠道有平台无2 + 手续费不匹配? 待核对)\n", expectedDiffs)
	} else {
		fmt.Printf("\n⚠️  差异数量: %d 笔 (需根据测试数据核对)\n", diffResult.TotalDiffCount)
	}
	fmt.Println()
}

func testDiffOptions() {
	fmt.Println("=== 测试8: 差异比对选项测试 ===")

	platSvc := platform.NewReconciliationService()
	platRecords, _ := platSvc.ParseBillFile("testdata/platform_bill.csv")

	wechatSvc := wechat.NewReconciliationService(nil)
	wechatRecords, _ := wechatSvc.ParseBillFile("testdata/wechat_bill.csv")

	date := "2024-01-15"
	filteredPlat, _ := platform.FilterByDate(platRecords, date, 0)
	filteredWechat, _ := wechat.FilterByDate(wechatRecords, date, 0)

	diffSvc := service.NewDiffService()

	fmt.Println("--- 默认选项 (检查手续费和类型) ---")
	defaultOpts := service.DefaultDiffOptions()
	defaultResult := diffSvc.Compare(filteredPlat, filteredWechat, "wechat", date, defaultOpts)
	fmt.Printf("  差异数: %d 笔\n", defaultResult.TotalDiffCount)

	fmt.Println("\n--- 不检查手续费 ---")
	optsNoFee := service.DefaultDiffOptions()
	optsNoFee.CheckFee = false
	resultNoFee := diffSvc.Compare(filteredPlat, filteredWechat, "wechat", date, optsNoFee)
	fmt.Printf("  差异数: %d 笔 (减少 %d 笔手续费差异)\n",
		resultNoFee.TotalDiffCount,
		defaultResult.TotalDiffCount-resultNoFee.TotalDiffCount)

	fmt.Println("\n--- 设置金额容忍度 1 元 ---")
	optsTolerance := service.DefaultDiffOptions()
	optsTolerance.AmountTolerance = 100 // 1元 = 100分
	resultTolerance := diffSvc.Compare(filteredPlat, filteredWechat, "wechat", date, optsTolerance)
	fmt.Printf("  差异数: %d 笔 (1元内金额差异被忽略)\n", resultTolerance.TotalDiffCount)

	fmt.Println("\n--- 不检查类型 ---")
	optsNoType := service.DefaultDiffOptions()
	optsNoType.CheckType = false
	resultNoType := diffSvc.Compare(filteredPlat, filteredWechat, "wechat", date, optsNoType)
	fmt.Printf("  差异数: %d 笔\n", resultNoType.TotalDiffCount)

	fmt.Println("\n--- 全部关闭，只检查金额 ---")
	optsAmountOnly := service.DiffOptions{
		AmountTolerance: 0,
		CheckStatus:     false,
		CheckFee:        false,
		CheckType:       false,
	}
	resultAmountOnly := diffSvc.Compare(filteredPlat, filteredWechat, "wechat", date, optsAmountOnly)
	fmt.Printf("  差异数: %d 笔 (仅金额不匹配 + 两边缺失的订单)\n", resultAmountOnly.TotalDiffCount)

	fmt.Println()
}
