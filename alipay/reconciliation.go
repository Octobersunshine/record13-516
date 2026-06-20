package alipay

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
	"reconciliation/pkg/timeutil"
)

type ReconciliationService struct {
	config *model.AlipayConfig
}

func NewReconciliationService(cfg *model.AlipayConfig) *ReconciliationService {
	return &ReconciliationService{
		config: cfg,
	}
}

func (s *ReconciliationService) DownloadBill(date string, billType string) (string, error) {
	return "", fmt.Errorf("alipay bill download requires real API credentials, use ParseBillFile instead")
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
	possibleHeaders := []string{
		"交易创建时间", "交易时间", "创建时间", "付款时间",
		"商户订单号", "商家订单号", "交易订单号",
		"交易号", "支付宝交易号",
		"类型", "收支类型",
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if !headerFound {
			headerCount := 0
			for _, h := range possibleHeaders {
				if strings.Contains(trimmed, h) {
					headerCount++
				}
			}
			if headerCount >= 2 {
				inDataSection = true
				headerFound = true
				lines = append(lines, line)
				continue
			}
		}

		if headerFound && trimmed == "" {
			continue
		}

		if headerFound && (strings.HasPrefix(trimmed, "总收入") ||
			strings.HasPrefix(trimmed, "总支出") ||
			strings.HasPrefix(trimmed, "账户") ||
			strings.HasPrefix(trimmed, "备注") ||
			strings.Contains(trimmed, "共") && strings.Contains(trimmed, "笔")) {
			if inDataSection {
				break
			}
			continue
		}

		if inDataSection && trimmed != "" {
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
	getField := func(names ...string) string {
		for _, name := range names {
			idx, ok := headerIndex[name]
			if ok && idx < len(record) {
				val := strings.TrimSpace(record[idx])
				if val != "" {
					return val
				}
			}
		}
		return ""
	}

	tradeTimeStr := getField("交易创建时间", "付款时间", "交易时间", "创建时间")
	tradeNo := getField("交易号", "支付宝交易号", "交易订单号")
	outTradeNo := getField("商户订单号", "商家订单号", "商户网站唯一订单号")
	tradeType := getField("类型", "收支类型", "商品名称")
	tradeStatus := getField("交易状态", "状态")
	totalAmountStr := getField("金额", "金额（元）", "交易金额")
	feeStr := getField("服务费", "服务费（元）", "手续费")
	inOut := getField("收/支", "收支")
	refundAmountStr := getField("成功退款", "成功退款（元）", "退款金额")

	if tradeNo == "" {
		return nil, fmt.Errorf("no trade number")
	}

	tradeTime, err := parseAlipayTime(tradeTimeStr)
	if err != nil {
		tradeTime = time.Time{}
	}

	var amount int64
	var recTradeType model.TradeType

	isRefund := false
	if strings.Contains(tradeType, "退款") ||
		strings.Contains(tradeStatus, "退款") ||
		inOut == "支出" ||
		(refundAmountStr != "" && refundAmountStr != "0.00" && refundAmountStr != "0") {
		isRefund = true
	}

	if isRefund {
		recTradeType = model.TradeTypeRefund
		amount = parseAmount(refundAmountStr)
		if amount == 0 {
			amount = parseAmount(totalAmountStr)
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
		PayChannel: "alipay",
		Fee:        fee,
		Raw:        raw,
	}, nil
}

func parseAlipayTime(timeStr string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02 15:04",
		"2006年01月02日 15:04:05",
	}
	return timeutil.ParseTimeMultiLayout(timeStr, layouts)
}

func FilterByDate(records []*model.TradeRecord, dateStr string, graceMinutes int) ([]*model.TradeRecord, error) {
	start, end, err := timeutil.DateRangeWithGrace(dateStr, timeutil.DateRangeOption{
		GracePeriodMinutes: graceMinutes,
	})
	if err != nil {
		return nil, fmt.Errorf("parse date failed: %w", err)
	}

	filtered := make([]*model.TradeRecord, 0, len(records))
	for _, rec := range records {
		if timeutil.IsInRange(rec.TradeTime, start, end) {
			filtered = append(filtered, rec)
		}
	}
	return filtered, nil
}

func parseAmount(amountStr string) int64 {
	amountStr = strings.TrimSpace(amountStr)
	amountStr = strings.ReplaceAll(amountStr, "¥", "")
	amountStr = strings.ReplaceAll(amountStr, "￥", "")
	amountStr = strings.ReplaceAll(amountStr, ",", "")
	amountStr = strings.ReplaceAll(amountStr, " ", "")
	amountStr = strings.TrimSpace(amountStr)

	if amountStr == "" || amountStr == "-" || amountStr == "--" {
		return 0
	}

	if strings.HasPrefix(amountStr, "-") {
		absStr := strings.TrimPrefix(amountStr, "-")
		absAmount := parseAmount(absStr)
		return -absAmount
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
		Channel: "alipay",
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
