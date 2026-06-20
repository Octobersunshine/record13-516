package platform

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
}

func NewReconciliationService() *ReconciliationService {
	return &ReconciliationService{}
}

func (s *ReconciliationService) ParseBillFile(filePath string) ([]*model.TradeRecord, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open platform bill file failed: %w", err)
	}
	defer file.Close()

	return s.ParseBillReader(file)
}

func (s *ReconciliationService) ParseBillReader(r io.Reader) ([]*model.TradeRecord, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	var lines []string
	headerFound := false

	possibleHeaders := []string{
		"订单号", "商户订单号", "平台订单号", "交易号",
		"交易类型", "类型", "订单类型",
		"金额", "交易金额", "订单金额", "实付金额",
		"手续费", "服务费",
		"交易时间", "创建时间", "支付时间", "下单时间",
		"状态", "交易状态", "订单状态",
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
				headerFound = true
				lines = append(lines, line)
				continue
			}
		}

		if headerFound && trimmed == "" {
			continue
		}

		if headerFound && trimmed != "" {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read platform bill file failed: %w", err)
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

	outTradeNo := getField("商户订单号", "商家订单号", "平台订单号", "订单号")
	tradeNo := getField("交易号", "平台交易号", "支付单号")
	tradeTypeStr := getField("交易类型", "类型", "订单类型")
	tradeStatus := getField("状态", "交易状态", "订单状态", "支付状态")
	totalAmountStr := getField("金额", "交易金额", "订单金额", "实付金额", "支付金额")
	feeStr := getField("手续费", "服务费", "平台手续费")
	tradeTimeStr := getField("交易时间", "创建时间", "支付时间", "下单时间", "完成时间")

	if outTradeNo == "" && tradeNo == "" {
		return nil, fmt.Errorf("no order number")
	}

	tradeTime, err := parsePlatformTime(tradeTimeStr)
	if err != nil {
		tradeTime = time.Time{}
	}

	var tradeType model.TradeType
	isRefund := false
	if strings.Contains(tradeTypeStr, "退款") ||
		strings.Contains(tradeStatus, "退款") ||
		strings.Contains(tradeTypeStr, "REFUND") {
		isRefund = true
	}

	if isRefund {
		tradeType = model.TradeTypeRefund
	} else {
		tradeType = model.TradeTypePayment
	}

	amount := parseAmount(totalAmountStr)
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
		TradeType:  tradeType,
		Amount:     amount,
		TradeTime:  tradeTime,
		Status:     tradeStatus,
		PayChannel: "platform",
		Fee:        fee,
		Raw:        raw,
	}, nil
}

func parsePlatformTime(timeStr string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
	}
	return timeutil.ParseTimeMultiLayout(timeStr, layouts)
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
		Channel: "platform",
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
