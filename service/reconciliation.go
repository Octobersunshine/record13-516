package service

import (
	"fmt"

	"reconciliation/alipay"
	"reconciliation/model"
	"reconciliation/wechat"
)

type ReconciliationService struct {
	wechatSvc *wechat.ReconciliationService
	alipaySvc *alipay.ReconciliationService
}

func NewReconciliationService(cfg *model.ReconciliationConfig) *ReconciliationService {
	svc := &ReconciliationService{}
	if cfg.Wechat != nil {
		svc.wechatSvc = wechat.NewReconciliationService(cfg.Wechat)
	}
	if cfg.Alipay != nil {
		svc.alipaySvc = alipay.NewReconciliationService(cfg.Alipay)
	}
	return svc
}

func (s *ReconciliationService) ParseWechatBillFile(filePath string) ([]*model.TradeRecord, error) {
	if s.wechatSvc == nil {
		return nil, fmt.Errorf("wechat service not configured")
	}
	return s.wechatSvc.ParseBillFile(filePath)
}

func (s *ReconciliationService) ParseAlipayBillFile(filePath string) ([]*model.TradeRecord, error) {
	if s.alipaySvc == nil {
		return nil, fmt.Errorf("alipay service not configured")
	}
	return s.alipaySvc.ParseBillFile(filePath)
}

func (s *ReconciliationService) Reconcile(date string, wechatRecords, alipayRecords []*model.TradeRecord) *model.ReconciliationResult {
	result := &model.ReconciliationResult{
		Date:          date,
		WechatRecords: wechatRecords,
		AlipayRecords: alipayRecords,
	}

	if wechatRecords != nil {
		result.WechatSummary = wechat.Summarize(wechatRecords, date)
	}

	if alipayRecords != nil {
		result.AlipaySummary = alipay.Summarize(alipayRecords, date)
	}

	result.CombinedSummary = s.combineSummaries(result.WechatSummary, result.AlipaySummary, date)

	return result
}

func (s *ReconciliationService) combineSummaries(wechatSum, alipaySum *model.ReconciliationSummary, date string) *model.ReconciliationSummary {
	combined := &model.ReconciliationSummary{
		Date:    date,
		Channel: "combined",
	}

	if wechatSum != nil {
		combined.TotalCount += wechatSum.TotalCount
		combined.TotalAmount += wechatSum.TotalAmount
		combined.PaymentCount += wechatSum.PaymentCount
		combined.PaymentAmount += wechatSum.PaymentAmount
		combined.RefundCount += wechatSum.RefundCount
		combined.RefundAmount += wechatSum.RefundAmount
		combined.TotalFee += wechatSum.TotalFee
	}

	if alipaySum != nil {
		combined.TotalCount += alipaySum.TotalCount
		combined.TotalAmount += alipaySum.TotalAmount
		combined.PaymentCount += alipaySum.PaymentCount
		combined.PaymentAmount += alipaySum.PaymentAmount
		combined.RefundCount += alipaySum.RefundCount
		combined.RefundAmount += alipaySum.RefundAmount
		combined.TotalFee += alipaySum.TotalFee
	}

	return combined
}

func FormatAmount(amount int64) string {
	if amount == 0 {
		return "0.00"
	}
	sign := ""
	if amount < 0 {
		sign = "-"
		amount = -amount
	}
	yuan := amount / 100
	fen := amount % 100
	return fmt.Sprintf("%s%d.%02d", sign, yuan, fen)
}
