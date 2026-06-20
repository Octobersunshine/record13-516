package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"reconciliation/alipay"
	"reconciliation/model"
	"reconciliation/pkg/timeutil"
	"reconciliation/platform"
	"reconciliation/service"
	"reconciliation/wechat"
)

type Handler struct {
	svc     *service.ReconciliationService
	diffSvc *service.DiffService
}

func NewHandler(svc *service.ReconciliationService) *Handler {
	return &Handler{
		svc:     svc,
		diffSvc: service.NewDiffService(),
	}
}

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type SummaryResponse struct {
	Date          string       `json:"date"`
	DateRange     *DateRangeInfo `json:"date_range,omitempty"`
	WechatSummary *SummaryData `json:"wechat_summary,omitempty"`
	AlipaySummary *SummaryData `json:"alipay_summary,omitempty"`
	Combined      *SummaryData `json:"combined_summary,omitempty"`
}

type DateRangeInfo struct {
	Start        string `json:"start"`
	End          string `json:"end"`
	GraceMinutes int    `json:"grace_minutes"`
	Timezone     string `json:"timezone"`
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

func normalizeDate(dateStr string) (string, error) {
	if dateStr == "" {
		return timeutil.FormatDate(timeutil.Now()), nil
	}
	t, err := timeutil.ParseDate(dateStr)
	if err != nil {
		return "", fmt.Errorf("invalid date format: %s", dateStr)
	}
	return timeutil.FormatDate(t), nil
}

func parseGraceMinutes(r *http.Request) int {
	graceStr := r.FormValue("grace_minutes")
	if graceStr == "" {
		return 0
	}
	grace, err := strconv.Atoi(graceStr)
	if err != nil || grace < 0 {
		return 0
	}
	return grace
}

func buildDateRangeInfo(dateStr string, graceMinutes int) (*DateRangeInfo, error) {
	start, end, err := timeutil.DateRangeWithGrace(dateStr, timeutil.DateRangeOption{
		GracePeriodMinutes: graceMinutes,
	})
	if err != nil {
		return nil, err
	}
	return &DateRangeInfo{
		Start:        timeutil.FormatDateTime(start),
		End:          timeutil.FormatDateTime(end),
		GraceMinutes: graceMinutes,
		Timezone:     "Asia/Shanghai (CST, UTC+8)",
	}, nil
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Code:    0,
		Message: "ok",
		Data: map[string]string{
			"status":   "running",
			"time":     timeutil.FormatDateTime(timeutil.Now()),
			"timezone": "Asia/Shanghai (CST, UTC+8)",
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

	date, err := normalizeDate(r.FormValue("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	graceMinutes := parseGraceMinutes(r)

	wechatSvc := wechat.NewReconciliationService(nil)
	records, err := wechatSvc.ParseBillReader(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("parse wechat bill failed: %v", err))
		return
	}

	filteredRecords, err := wechat.FilterByDate(records, date, graceMinutes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("filter records by date failed: %v", err))
		return
	}

	dateRangeInfo, _ := buildDateRangeInfo(date, graceMinutes)
	summary := service.SummarizeWechat(filteredRecords, date)

	writeSuccess(w, map[string]interface{}{
		"date":                date,
		"date_range":          dateRangeInfo,
		"records":             convertRecords(filteredRecords),
		"count":               len(filteredRecords),
		"total_parsed_count":  len(records),
		"filtered_out_count":  len(records) - len(filteredRecords),
		"summary":             convertSummary(summary),
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

	date, err := normalizeDate(r.FormValue("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	graceMinutes := parseGraceMinutes(r)

	alipaySvc := alipay.NewReconciliationService(nil)
	records, err := alipaySvc.ParseBillReader(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("parse alipay bill failed: %v", err))
		return
	}

	filteredRecords, err := alipay.FilterByDate(records, date, graceMinutes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("filter records by date failed: %v", err))
		return
	}

	dateRangeInfo, _ := buildDateRangeInfo(date, graceMinutes)
	summary := service.SummarizeAlipay(filteredRecords, date)

	writeSuccess(w, map[string]interface{}{
		"date":                date,
		"date_range":          dateRangeInfo,
		"records":             convertRecords(filteredRecords),
		"count":               len(filteredRecords),
		"total_parsed_count":  len(records),
		"filtered_out_count":  len(records) - len(filteredRecords),
		"summary":             convertSummary(summary),
	})
}

func (h *Handler) Reconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	date, err := normalizeDate(r.FormValue("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	graceMinutes := parseGraceMinutes(r)

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

	result := h.svc.ReconcileWithOptions(date, wechatRecords, alipayRecords, service.ReconcileOptions{
		GraceMinutes: graceMinutes,
	})

	dateRangeInfo, _ := buildDateRangeInfo(date, graceMinutes)
	resp := &SummaryResponse{
		Date:      result.Date,
		DateRange: dateRangeInfo,
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

	date, err := normalizeDate(r.FormValue("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	graceMinutes := parseGraceMinutes(r)

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

	result := h.svc.ReconcileWithOptions(date, wechatRecords, alipayRecords, service.ReconcileOptions{
		GraceMinutes: graceMinutes,
	})

	dateRangeInfo, _ := buildDateRangeInfo(date, graceMinutes)
	resp := map[string]interface{}{
		"date":       result.Date,
		"date_range": dateRangeInfo,
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
			tradeTime = timeutil.FormatDateTime(r.TradeTime)
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

type DiffOrderResponse struct {
	DiffID        string              `json:"diff_id"`
	DiffType      string              `json:"diff_type"`
	OutTradeNo    string              `json:"out_trade_no"`
	TradeType     string              `json:"trade_type"`
	Channel       string              `json:"channel"`
	PlatformTrade *TradeRecordResponse `json:"platform_trade,omitempty"`
	ChannelTrade  *TradeRecordResponse `json:"channel_trade,omitempty"`
	AmountDiff    string              `json:"amount_diff"`
	FeeDiff       string              `json:"fee_diff"`
	Description   string              `json:"description"`
	CreatedAt     string              `json:"created_at"`
}

type DiffResultResponse struct {
	Date            string            `json:"date"`
	Channel         string            `json:"channel"`
	TotalDiffCount  int64             `json:"total_diff_count"`
	TotalDiffAmount string            `json:"total_diff_amount"`
	DiffOrders      []*DiffOrderResponse `json:"diff_orders"`
	DiffSummary     map[string]int64  `json:"diff_summary"`
	PlatformCount   int64             `json:"platform_count"`
	ChannelCount    int64             `json:"channel_count"`
	MatchedCount    int64             `json:"matched_count"`
	MatchRate       string            `json:"match_rate"`
}

func (h *Handler) DiffWechat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	date, err := normalizeDate(r.FormValue("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	graceMinutes := parseGraceMinutes(r)

	platformFile, _, err := r.FormFile("platform_file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("read platform file failed: %v", err))
		return
	}
	defer platformFile.Close()

	wechatFile, _, err := r.FormFile("wechat_file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("read wechat file failed: %v", err))
		return
	}
	defer wechatFile.Close()

	platSvc := platform.NewReconciliationService()
	platRecords, err := platSvc.ParseBillReader(platformFile)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse platform bill failed: %v", err))
		return
	}

	wechatSvc := wechat.NewReconciliationService(nil)
	wechatRecords, err := wechatSvc.ParseBillReader(wechatFile)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse wechat bill failed: %v", err))
		return
	}

	filteredPlat, _ := platform.FilterByDate(platRecords, date, graceMinutes)
	filteredWechat, _ := wechat.FilterByDate(wechatRecords, date, graceMinutes)

	opts := parseDiffOptions(r)
	diffResult := h.diffSvc.Compare(filteredPlat, filteredWechat, "wechat", date, opts)

	writeSuccess(w, convertDiffResult(diffResult))
}

func (h *Handler) DiffAlipay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	date, err := normalizeDate(r.FormValue("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	graceMinutes := parseGraceMinutes(r)

	platformFile, _, err := r.FormFile("platform_file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("read platform file failed: %v", err))
		return
	}
	defer platformFile.Close()

	alipayFile, _, err := r.FormFile("alipay_file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("read alipay file failed: %v", err))
		return
	}
	defer alipayFile.Close()

	platSvc := platform.NewReconciliationService()
	platRecords, err := platSvc.ParseBillReader(platformFile)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse platform bill failed: %v", err))
		return
	}

	alipaySvc := alipay.NewReconciliationService(nil)
	alipayRecords, err := alipaySvc.ParseBillReader(alipayFile)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse alipay bill failed: %v", err))
		return
	}

	filteredPlat, _ := platform.FilterByDate(platRecords, date, graceMinutes)
	filteredAlipay, _ := alipay.FilterByDate(alipayRecords, date, graceMinutes)

	opts := parseDiffOptions(r)
	diffResult := h.diffSvc.Compare(filteredPlat, filteredAlipay, "alipay", date, opts)

	writeSuccess(w, convertDiffResult(diffResult))
}

func (h *Handler) DiffAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	date, err := normalizeDate(r.FormValue("date"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	graceMinutes := parseGraceMinutes(r)

	platformFile, _, err := r.FormFile("platform_file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("read platform file failed: %v", err))
		return
	}
	defer platformFile.Close()

	platSvc := platform.NewReconciliationService()
	platRecords, err := platSvc.ParseBillReader(platformFile)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("parse platform bill failed: %v", err))
		return
	}
	filteredPlat, _ := platform.FilterByDate(platRecords, date, graceMinutes)

	opts := parseDiffOptions(r)
	result := make(map[string]interface{})

	var wechatDiffResult *model.DiffResult
	wechatFile, _, err := r.FormFile("wechat_file")
	if err == nil {
		defer wechatFile.Close()
		wechatSvc := wechat.NewReconciliationService(nil)
		wechatRecords, err := wechatSvc.ParseBillReader(wechatFile)
		if err == nil {
			filteredWechat, _ := wechat.FilterByDate(wechatRecords, date, graceMinutes)
			wechatDiffResult = h.diffSvc.Compare(filteredPlat, filteredWechat, "wechat", date, opts)
			result["wechat_diff"] = convertDiffResult(wechatDiffResult)
		}
	}

	var alipayDiffResult *model.DiffResult
	alipayFile, _, err := r.FormFile("alipay_file")
	if err == nil {
		defer alipayFile.Close()
		alipaySvc := alipay.NewReconciliationService(nil)
		alipayRecords, err := alipaySvc.ParseBillReader(alipayFile)
		if err == nil {
			filteredAlipay, _ := alipay.FilterByDate(alipayRecords, date, graceMinutes)
			alipayDiffResult = h.diffSvc.Compare(filteredPlat, filteredAlipay, "alipay", date, opts)
			result["alipay_diff"] = convertDiffResult(alipayDiffResult)
		}
	}

	if wechatDiffResult == nil && alipayDiffResult == nil {
		writeError(w, http.StatusBadRequest, "at least one channel file is required (wechat_file or alipay_file)")
		return
	}

	combinedTotalDiff := int64(0)
	combinedDiffAmount := int64(0)
	if wechatDiffResult != nil {
		combinedTotalDiff += wechatDiffResult.TotalDiffCount
		combinedDiffAmount += wechatDiffResult.TotalDiffAmount
	}
	if alipayDiffResult != nil {
		combinedTotalDiff += alipayDiffResult.TotalDiffCount
		combinedDiffAmount += alipayDiffResult.TotalDiffAmount
	}

	result["date"] = date
	result["platform_count"] = int64(len(filteredPlat))
	result["total_diff_count"] = combinedTotalDiff
	result["total_diff_amount"] = service.FormatAmount(combinedDiffAmount)

	writeSuccess(w, result)
}

func parseDiffOptions(r *http.Request) service.DiffOptions {
	opts := service.DefaultDiffOptions()

	if val := r.FormValue("check_status"); val == "true" || val == "1" {
		opts.CheckStatus = true
	}
	if val := r.FormValue("check_fee"); val == "false" || val == "0" {
		opts.CheckFee = false
	}
	if val := r.FormValue("check_type"); val == "false" || val == "0" {
		opts.CheckType = false
	}

	if amountTolStr := r.FormValue("amount_tolerance"); amountTolStr != "" {
		if tol, err := strconv.ParseFloat(amountTolStr, 64); err == nil {
			opts.AmountTolerance = int64(tol * 100)
		}
	}
	if feeTolStr := r.FormValue("fee_tolerance"); feeTolStr != "" {
		if tol, err := strconv.ParseFloat(feeTolStr, 64); err == nil {
			opts.FeeTolerance = int64(tol * 100)
		}
	}

	return opts
}

func convertDiffResult(result *model.DiffResult) *DiffResultResponse {
	if result == nil {
		return nil
	}

	diffOrders := make([]*DiffOrderResponse, 0, len(result.DiffOrders))
	for _, d := range result.DiffOrders {
		diffOrders = append(diffOrders, convertDiffOrder(d))
	}

	diffSummary := make(map[string]int64)
	for k, v := range result.DiffSummary {
		diffSummary[string(k)] = v
	}

	return &DiffResultResponse{
		Date:            result.Date,
		Channel:         result.Channel,
		TotalDiffCount:  result.TotalDiffCount,
		TotalDiffAmount: service.FormatAmount(result.TotalDiffAmount),
		DiffOrders:      diffOrders,
		DiffSummary:     diffSummary,
		PlatformCount:   result.PlatformCount,
		ChannelCount:    result.ChannelCount,
		MatchedCount:    result.MatchedCount,
		MatchRate:       fmt.Sprintf("%.2f%%", result.MatchRate),
	}
}

func convertDiffOrder(diff *model.DiffOrder) *DiffOrderResponse {
	if diff == nil {
		return nil
	}
	resp := &DiffOrderResponse{
		DiffID:      diff.DiffID,
		DiffType:    string(diff.DiffType),
		OutTradeNo:  diff.OutTradeNo,
		TradeType:   string(diff.TradeType),
		Channel:     diff.Channel,
		AmountDiff:  service.FormatAmount(diff.AmountDiff),
		FeeDiff:     service.FormatAmount(diff.FeeDiff),
		Description: diff.Description,
		CreatedAt:   timeutil.FormatDateTime(diff.CreatedAt),
	}
	if diff.PlatformTrade != nil {
		resp.PlatformTrade = convertSingleRecord(diff.PlatformTrade)
	}
	if diff.ChannelTrade != nil {
		resp.ChannelTrade = convertSingleRecord(diff.ChannelTrade)
	}
	return resp
}

func convertSingleRecord(r *model.TradeRecord) *TradeRecordResponse {
	if r == nil {
		return nil
	}
	tradeTime := ""
	if !r.TradeTime.IsZero() {
		tradeTime = timeutil.FormatDateTime(r.TradeTime)
	}
	return &TradeRecordResponse{
		TradeNo:    r.TradeNo,
		OutTradeNo: r.OutTradeNo,
		TradeType:  string(r.TradeType),
		Amount:     service.FormatAmount(r.Amount),
		TradeTime:  tradeTime,
		Status:     r.Status,
		PayChannel: r.PayChannel,
		Fee:        service.FormatAmount(r.Fee),
		Raw:        r.Raw,
	}
}
