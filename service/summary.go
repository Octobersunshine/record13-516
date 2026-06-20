package service

import (
	"reconciliation/alipay"
	"reconciliation/model"
	"reconciliation/wechat"
)

func SummarizeWechat(records []*model.TradeRecord, date string) *model.ReconciliationSummary {
	return wechat.Summarize(records, date)
}

func SummarizeAlipay(records []*model.TradeRecord, date string) *model.ReconciliationSummary {
	return alipay.Summarize(records, date)
}
