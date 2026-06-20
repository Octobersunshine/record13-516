package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"reconciliation/model"
	"reconciliation/service"
	"reconciliation/wechat"
	"reconciliation/alipay"
)

type Handler struct {
	svc *service.ReconciliationService
}

func NewHandler(svc *service.ReconciliationService) *Handler {
	return &Handler{svc: svc}
}

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SummaryResponse struct {
	Date          string       `json:"date"`
	WechatSummary *SummaryData `json:"wechat_summary,omitempty"`
	AlipaySummary *SummaryData `json:"alipay_summary,omitempty"`
	Combined      *SummaryData `json:"combined_summary,omitempty"`
}

type SummaryData struct {
	Channel       string `json:"channel"`
	TotalCount    int64  `json:"total_count"`
	TotalAmount   string `json:"total_amount"`
	PaymentCount  int64  `json:"payment_count"`
	PaymentAmount string `json:"payment_amount"`
	RefundCount   int64  `json:"refund_count"`
	RefundAmount  string `json:"refund_amount"`
	TotalFee      string `json:"total_fee"`
}

type TradeRecordResponse struct {
	TradeNo    string            `json:"trade_no"`
	OutTradeNo string            `json:"out_trade_no"`
	TradeType  string            `json:"trade_type"`
	Amount     string            `json:"amount"`
	TradeTime  string            `json:"trade_time"`
	Status     string            `json:"status"`
	PayChannel string            `json:"pay_channel"`
	Fee        string            `json:"fee"`
	Raw        map[string]string `json:"raw,omitempty"`
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Code:    0,
		Message: "ok",
		Data: map[string]string{
			"status": "running",
			"time":   time.Now().Format(time.RFC3339),
		},
	})
}

func (h *Handler) ParseWechatBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("read file failed: %v", err))
		return
	}
	defer file.Close()

	wechatSvc := wechat.NewReconciliationService(nil)
	records, err := wechatSvc.ParseBillReader(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("parse wechat bill failed: %v", err))
		return
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	summary := service.SummarizeWechat(records, date)

	writeSuccess(w, map[string]interface{}{
		"date":    date,
		"records": convertRecords(records),
		"count":   len(records),
		"summary": convertSummary(summary),
	})
}

func (h *Handler) ParseAlipayBill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("read file failed: %v", err))
		return
	}
	defer file.Close()

	alipaySvc := alipay.NewReconciliationService(nil)
	records, err := alipaySvc.ParseBillReader(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("parse alipay bill failed: %v", err))
		return
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	summary := service.SummarizeAlipay(records, date)

	writeSuccess(w, map[string]interface{}{
		"date":    date,
		"records": convertRecords(records),
		"count":   len(records),
		"summary": convertSummary(summary),
	})
}

func (h *Handler) Reconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	var wechatRecords []*model.TradeRecord
	wechatFile, _, err := r.FormFile("wechat_file")
	if err == nil {
		defer wechatFile.Close()
		wechatSvc := wechat.NewReconciliationService(nil)
		wechatRecords, err = wechatSvc.ParseBillReader(wechatFile)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("parse wechat bill failed: %v", err))
			return
		}
	}

	var alipayRecords []*model.TradeRecord
	alipayFile, _, err := r.FormFile("alipay_file")
	if err == nil {
		defer alipayFile.Close()
		alipaySvc := alipay.NewReconciliationService(nil)
		alipayRecords, err = alipaySvc.ParseBillReader(alipayFile)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("parse alipay bill failed: %v", err))
			return
		}
	}

	if wechatRecords == nil && alipayRecords == nil {
		writeError(w, http.StatusBadRequest, "at least one bill file is required (wechat_file or alipay_file)")
		return
	}

	result := h.svc.Reconcile(date, wechatRecords, alipayRecords)

	resp := &SummaryResponse{
		Date: result.Date,
	}
	if result.WechatSummary != nil {
		resp.WechatSummary = convertSummary(result.WechatSummary)
	}
	if result.AlipaySummary != nil {
		resp.AlipaySummary = convertSummary(result.AlipaySummary)
	}
	if result.CombinedSummary != nil {
		resp.Combined = convertSummary(result.CombinedSummary)
	}

	writeSuccess(w, resp)
}

func (h *Handler) ReconcileDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	var wechatRecords []*model.TradeRecord
	wechatFile, _, err := r.FormFile("wechat_file")
	if err == nil {
		defer wechatFile.Close()
		wechatSvc := wechat.NewReconciliationService(nil)
		wechatRecords, err = wechatSvc.ParseBillReader(wechatFile)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("parse wechat bill failed: %v", err))
			return
		}
	}

	var alipayRecords []*model.TradeRecord
	alipayFile, _, err := r.FormFile("alipay_file")
	if err == nil {
		defer alipayFile.Close()
		alipaySvc := alipay.NewReconciliationService(nil)
		alipayRecords, err = alipaySvc.ParseBillReader(alipayFile)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("parse alipay bill failed: %v", err))
			return
		}
	}

	if wechatRecords == nil && alipayRecords == nil {
		writeError(w, http.StatusBadRequest, "at least one bill file is required")
		return
	}

	result := h.svc.Reconcile(date, wechatRecords, alipayRecords)

	resp := map[string]interface{}{
		"date": result.Date,
		"summary": map[string]interface{}{
			"wechat":   convertSummary(result.WechatSummary),
			"alipay":   convertSummary(result.AlipaySummary),
			"combined": convertSummary(result.CombinedSummary),
		},
		"wechat_records": convertRecords(result.WechatRecords),
		"alipay_records": convertRecords(result.AlipayRecords),
	}

	writeSuccess(w, resp)
}

func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(Response{
		Code:    code,
		Message: message,
	})
}

func writeSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func convertSummary(s *model.ReconciliationSummary) *SummaryData {
	if s == nil {
		return nil
	}
	return &SummaryData{
		Channel:       s.Channel,
		TotalCount:    s.TotalCount,
		TotalAmount:   service.FormatAmount(s.TotalAmount),
		PaymentCount:  s.PaymentCount,
		PaymentAmount: service.FormatAmount(s.PaymentAmount),
		RefundCount:   s.RefundCount,
		RefundAmount:  service.FormatAmount(s.RefundAmount),
		TotalFee:      service.FormatAmount(s.TotalFee),
	}
}

func convertRecords(records []*model.TradeRecord) []*TradeRecordResponse {
	if records == nil {
		return nil
	}
	result := make([]*TradeRecordResponse, 0, len(records))
	for _, r := range records {
		tradeTime := ""
		if !r.TradeTime.IsZero() {
			tradeTime = r.TradeTime.Format("2006-01-02 15:04:05")
		}
		result = append(result, &TradeRecordResponse{
			TradeNo:    r.TradeNo,
			OutTradeNo: r.OutTradeNo,
			TradeType:  string(r.TradeType),
			Amount:     service.FormatAmount(r.Amount),
			TradeTime:  tradeTime,
			Status:     r.Status,
			PayChannel: r.PayChannel,
			Fee:        service.FormatAmount(r.Fee),
			Raw:        r.Raw,
		})
	}
	return result
}
