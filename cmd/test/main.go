package main

import (
	"fmt"
	"log"

	"reconciliation/alipay"
	"reconciliation/model"
	"reconciliation/service"
	"reconciliation/wechat"
)

func main() {
	testWechatParse()
	testAlipayParse()
	testReconcile()
}

func testWechatParse() {
	fmt.Println("=== 测试微信对账文件解析 ===")
	wechatSvc := wechat.NewReconciliationService(nil)
	records, err := wechatSvc.ParseBillFile("testdata/wechat_bill.csv")
	if err != nil {
		log.Fatalf("微信账单解析失败: %v", err)
	}

	fmt.Printf("解析记录数: %d\n", len(records))
	for i, r := range records {
		fmt.Printf("  记录%d: 类型=%s, 金额=%d分(%.2f元), 订单号=%s\n",
			i+1, r.TradeType, r.Amount, float64(r.Amount)/100, r.OutTradeNo)
	}

	summary := wechat.Summarize(records, "2024-01-15")
	fmt.Printf("\n微信汇总:\n")
	fmt.Printf("  总笔数: %d\n", summary.TotalCount)
	fmt.Printf("  总金额: %.2f元\n", float64(summary.TotalAmount)/100)
	fmt.Printf("  支付笔数: %d, 支付金额: %.2f元\n", summary.PaymentCount, float64(summary.PaymentAmount)/100)
	fmt.Printf("  退款笔数: %d, 退款金额: %.2f元\n", summary.RefundCount, float64(summary.RefundAmount)/100)
	fmt.Printf("  手续费: %.2f元\n", float64(summary.TotalFee)/100)
	fmt.Println()
}

func testAlipayParse() {
	fmt.Println("=== 测试支付宝对账文件解析 ===")
	alipaySvc := alipay.NewReconciliationService(nil)
	records, err := alipaySvc.ParseBillFile("testdata/alipay_bill.csv")
	if err != nil {
		log.Fatalf("支付宝账单解析失败: %v", err)
	}

	fmt.Printf("解析记录数: %d\n", len(records))
	for i, r := range records {
		fmt.Printf("  记录%d: 类型=%s, 金额=%d分(%.2f元), 订单号=%s\n",
			i+1, r.TradeType, r.Amount, float64(r.Amount)/100, r.OutTradeNo)
	}

	summary := alipay.Summarize(records, "2024-01-15")
	fmt.Printf("\n支付宝汇总:\n")
	fmt.Printf("  总笔数: %d\n", summary.TotalCount)
	fmt.Printf("  净金额: %.2f元\n", float64(summary.TotalAmount)/100)
	fmt.Printf("  支付笔数: %d, 支付金额: %.2f元\n", summary.PaymentCount, float64(summary.PaymentAmount)/100)
	fmt.Printf("  退款笔数: %d, 退款金额: %.2f元\n", summary.RefundCount, float64(summary.RefundAmount)/100)
	fmt.Println()
}

func testReconcile() {
	fmt.Println("=== 测试对账汇总 ===")
	cfg := &model.ReconciliationConfig{
		Wechat: &model.WechatConfig{},
		Alipay: &model.AlipayConfig{},
	}
	svc := service.NewReconciliationService(cfg)

	wechatRecords, _ := svc.ParseWechatBillFile("testdata/wechat_bill.csv")
	alipayRecords, _ := svc.ParseAlipayBillFile("testdata/alipay_bill.csv")

	result := svc.Reconcile("2024-01-15", wechatRecords, alipayRecords)

	fmt.Printf("对账日期: %s\n", result.Date)
	fmt.Printf("\n微信汇总:\n")
	if result.WechatSummary != nil {
		fmt.Printf("  总笔数: %d, 净金额: %s\n", result.WechatSummary.TotalCount, service.FormatAmount(result.WechatSummary.TotalAmount))
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
	fmt.Println("\n=== 测试完成 ===")
}
