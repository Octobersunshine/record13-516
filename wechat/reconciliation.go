package wechat

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"reconciliation/model"
)

type ReconciliationService struct {
	config *model.WechatConfig
}

func NewReconciliationService(cfg *model.WechatConfig) *ReconciliationService {
	return &ReconciliationService{
		config: cfg,
	}
}

func (s *ReconciliationService) DownloadBill(date string, billType string) (string, error) {
	return "", fmt.Errorf("wechat bill download requires real API credentials, use ParseBillFile instead")
}

func (s *ReconciliationService) ParseBillFile(filePath string) ([]*model.TradeRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open bill file failed: %w", err)
	}
	defer file.Close()

	return s.ParseBillReader(file)
}

func (s *ReconciliationService) ParseBillReader(r io.Reader) ([]*model.TradeRecord, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	var lines []string
	inDataSection := false
	headerFound := false

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "交易时间") ||
			strings.Contains(trimmed, "微信订单号") ||
			strings.Contains(trimmed, "交易类型") {
			inDataSection = true
			headerFound = true
			lines = append(lines, line)
			continue
		}

		if strings.HasPrefix(trimmed, "总交易单数") ||
			strings.HasPrefix(trimmed, "应结订单总金额") ||
			strings.HasPrefix(trimmed, "退款总金额") {
			if inDataSection {
				break
			}
			continue
		}

		if inDataSection && headerFound && trimmed != "" {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read bill file failed: %w", err)
	}

	if len(lines) < 2 {
		return nil, fmt.Errorf("no valid bill data found")
	}

	csvReader := csv.NewReader(strings.NewReader(strings.Join(lines, "\n")))
	csvReader.FieldsPerRecord = -1
	csvReader.Comma = ','
	csvReader.LazyQuotes = true

	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv failed: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("no bill records found")
	}

	headers := records[0]
	headerIndex := make(map[string]int)
	for i, h := range headers {
		headerIndex[strings.TrimSpace(h)] = i
	}

	var tradeRecords []*model.TradeRecord

	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) == 0 {
			continue
		}

		tradeRec, err := s.parseRecord(record, headerIndex)
		if err != nil {
			continue
		}
		if tradeRec != nil {
			tradeRecords = append(tradeRecords, tradeRec)
		}
	}

	return tradeRecords, nil
}

func (s *ReconciliationService) parseRecord(record []string, headerIndex map[string]int) (*model.TradeRecord, error) {
	getField := func(name string) string {
		idx, ok := headerIndex[name]
		if !ok || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	tradeTimeStr := getField("交易时间")
	tradeNo := getField("微信订单号")
	outTradeNo := getField("商户订单号")
	tradeType := getField("交易类型")
	tradeStatus := getField("交易状态")
	totalAmountStr := getField("总金额")
	refundAmountStr := getField("退款金额")
	feeStr := getField("手续费")
	refundNo := getField("微信退款单号")
	outRefundNo := getField("商户退款单号")

	if tradeNo == "" && refundNo == "" {
		return nil, fmt.Errorf("no trade number")
	}

	tradeTime, err := parseWechatTime(tradeTimeStr)
	if err != nil {
		tradeTime = time.Time{}
	}

	var amount int64
	var recTradeType model.TradeType

	if refundNo != "" || strings.Contains(tradeType, "退款") ||
		(tradeStatus == "退款成功") || (refundAmountStr != "" && refundAmountStr != "0.00" && refundAmountStr != "0") {
		recTradeType = model.TradeTypeRefund
		amount = parseAmount(refundAmountStr)
		if amount == 0 {
			amount = parseAmount(totalAmountStr)
		}
		if refundNo != "" {
			tradeNo = refundNo
		}
		if outRefundNo != "" {
			outTradeNo = outRefundNo
		}
	} else {
		recTradeType = model.TradeTypePayment
		amount = parseAmount(totalAmountStr)
	}

	fee := parseAmount(feeStr)

	raw := make(map[string]string)
	for name, idx := range headerIndex {
		if idx < len(record) {
			raw[name] = strings.TrimSpace(record[idx])
		}
	}

	return &model.TradeRecord{
		TradeNo:    tradeNo,
		OutTradeNo: outTradeNo,
		TradeType:  recTradeType,
		Amount:     amount,
		TradeTime:  tradeTime,
		Status:     tradeStatus,
		PayChannel: "wechat",
		Fee:        fee,
		Raw:        raw,
	}, nil
}

func parseWechatTime(timeStr string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, layout := range layouts {
		t, err := time.ParseInLocation(layout, timeStr, time.Local)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse time failed: %s", timeStr)
}

func parseAmount(amountStr string) int64 {
	amountStr = strings.TrimSpace(amountStr)
	amountStr = strings.ReplaceAll(amountStr, "¥", "")
	amountStr = strings.ReplaceAll(amountStr, "￥", "")
	amountStr = strings.ReplaceAll(amountStr, ",", "")
	amountStr = strings.TrimSpace(amountStr)

	if amountStr == "" || amountStr == "-" {
		return 0
	}

	if strings.Contains(amountStr, ".") {
		f, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			return 0
		}
		return int64(f * 100)
	}

	i, err := strconv.ParseInt(amountStr, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

func Summarize(records []*model.TradeRecord, date string) *model.ReconciliationSummary {
	summary := &model.ReconciliationSummary{
		Date:    date,
		Channel: "wechat",
	}

	for _, rec := range records {
		summary.TotalCount++
		summary.TotalFee += rec.Fee

		switch rec.TradeType {
		case model.TradeTypePayment:
			summary.PaymentCount++
			summary.PaymentAmount += rec.Amount
			summary.TotalAmount += rec.Amount
		case model.TradeTypeRefund:
			summary.RefundCount++
			summary.RefundAmount += rec.Amount
			summary.TotalAmount -= rec.Amount
		}
	}

	return summary
}
