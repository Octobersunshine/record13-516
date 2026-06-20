package service

import (
	"fmt"
	"time"

	"reconciliation/model"
	"reconciliation/pkg/timeutil"
)

type DiffOptions struct {
	AmountTolerance int64
	FeeTolerance    int64
	CheckStatus     bool
	CheckFee        bool
	CheckType       bool
}

func DefaultDiffOptions() DiffOptions {
	return DiffOptions{
		AmountTolerance: 0,
		FeeTolerance:    0,
		CheckStatus:     false,
		CheckFee:        true,
		CheckType:       true,
	}
}

type DiffService struct {
}

func NewDiffService() *DiffService {
	return &DiffService{}
}

func (s *DiffService) Compare(platformRecords, channelRecords []*model.TradeRecord, channel string, date string, opts DiffOptions) *model.DiffResult {
	if opts.AmountTolerance == 0 && opts.FeeTolerance == 0 && !opts.CheckStatus && !opts.CheckFee && !opts.CheckType {
		opts = DefaultDiffOptions()
		opts.CheckFee = true
		opts.CheckType = true
	}

	platformMap := buildRecordMap(platformRecords)
	channelMap := buildRecordMap(channelRecords)

	diffResult := &model.DiffResult{
		Date:          date,
		Channel:       channel,
		DiffOrders:    make([]*model.DiffOrder, 0),
		DiffSummary:   make(map[model.DiffType]int64),
		PlatformCount: int64(len(platformMap)),
		ChannelCount:  int64(len(channelMap)),
	}

	matchedCount := int64(0)

	for key, platRec := range platformMap {
		chanRec, exists := channelMap[key]
		if !exists {
			diff := createPlatformOnlyDiff(platRec, channel)
			diffResult.DiffOrders = append(diffResult.DiffOrders, diff)
			diffResult.DiffSummary[model.DiffTypePlatformOnly]++
			diffResult.TotalDiffCount++
			diffResult.TotalDiffAmount += absInt64(platRec.Amount)
			continue
		}

		diffs := compareSingleRecord(platRec, chanRec, channel, opts)
		if len(diffs) > 0 {
			diffResult.DiffOrders = append(diffResult.DiffOrders, diffs...)
			for _, d := range diffs {
				diffResult.DiffSummary[d.DiffType]++
				diffResult.TotalDiffCount++
				if d.AmountDiff != 0 {
					diffResult.TotalDiffAmount += absInt64(d.AmountDiff)
				}
			}
		} else {
			matchedCount++
		}
	}

	for key, chanRec := range channelMap {
		_, exists := platformMap[key]
		if !exists {
			diff := createChannelOnlyDiff(chanRec, channel)
			diffResult.DiffOrders = append(diffResult.DiffOrders, diff)
			diffResult.DiffSummary[model.DiffTypeChannelOnly]++
			diffResult.TotalDiffCount++
			diffResult.TotalDiffAmount += absInt64(chanRec.Amount)
		}
	}

	diffResult.MatchedCount = matchedCount
	total := diffResult.PlatformCount
	if diffResult.ChannelCount > total {
		total = diffResult.ChannelCount
	}
	if total > 0 {
		diffResult.MatchRate = float64(matchedCount) / float64(total) * 100
	}

	return diffResult
}

func buildRecordMap(records []*model.TradeRecord) map[string]*model.TradeRecord {
	m := make(map[string]*model.TradeRecord)
	for _, rec := range records {
		if rec.OutTradeNo == "" {
			continue
		}
		key := buildRecordKey(rec.OutTradeNo, rec.TradeType)
		m[key] = rec
	}
	return m
}

func buildRecordKey(outTradeNo string, tradeType model.TradeType) string {
	return fmt.Sprintf("%s_%s", outTradeNo, tradeType)
}

func compareSingleRecord(platRec, chanRec *model.TradeRecord, channel string, opts DiffOptions) []*model.DiffOrder {
	var diffs []*model.DiffOrder

	amountDiff := platRec.Amount - chanRec.Amount
	if absInt64(amountDiff) > opts.AmountTolerance {
		diffs = append(diffs, &model.DiffOrder{
			DiffID:        genDiffID(),
			DiffType:      model.DiffTypeAmountMismatch,
			OutTradeNo:    platRec.OutTradeNo,
			TradeType:     platRec.TradeType,
			Channel:       channel,
			PlatformTrade: platRec,
			ChannelTrade:  chanRec,
			AmountDiff:    amountDiff,
			Description:   fmt.Sprintf("金额不匹配: 平台 %d 分 vs 渠道 %d 分, 差额 %d 分", platRec.Amount, chanRec.Amount, amountDiff),
			CreatedAt:     timeutil.Now(),
		})
	}

	if opts.CheckFee {
		feeDiff := platRec.Fee - chanRec.Fee
		if absInt64(feeDiff) > opts.FeeTolerance {
			diffs = append(diffs, &model.DiffOrder{
				DiffID:        genDiffID(),
				DiffType:      model.DiffTypeFeeMismatch,
				OutTradeNo:    platRec.OutTradeNo,
				TradeType:     platRec.TradeType,
				Channel:       channel,
				PlatformTrade: platRec,
				ChannelTrade:  chanRec,
				FeeDiff:       feeDiff,
				Description:   fmt.Sprintf("手续费不匹配: 平台 %d 分 vs 渠道 %d 分, 差额 %d 分", platRec.Fee, chanRec.Fee, feeDiff),
				CreatedAt:     timeutil.Now(),
			})
		}
	}

	if opts.CheckStatus && platRec.Status != chanRec.Status {
		diffs = append(diffs, &model.DiffOrder{
			DiffID:        genDiffID(),
			DiffType:      model.DiffTypeStatusMismatch,
			OutTradeNo:    platRec.OutTradeNo,
			TradeType:     platRec.TradeType,
			Channel:       channel,
			PlatformTrade: platRec,
			ChannelTrade:  chanRec,
			Description:   fmt.Sprintf("状态不匹配: 平台 %s vs 渠道 %s", platRec.Status, chanRec.Status),
			CreatedAt:     timeutil.Now(),
		})
	}

	if opts.CheckType && platRec.TradeType != chanRec.TradeType {
		diffs = append(diffs, &model.DiffOrder{
			DiffID:        genDiffID(),
			DiffType:      model.DiffTypeTypeMismatch,
			OutTradeNo:    platRec.OutTradeNo,
			TradeType:     platRec.TradeType,
			Channel:       channel,
			PlatformTrade: platRec,
			ChannelTrade:  chanRec,
			Description:   fmt.Sprintf("交易类型不匹配: 平台 %s vs 渠道 %s", platRec.TradeType, chanRec.TradeType),
			CreatedAt:     timeutil.Now(),
		})
	}

	return diffs
}

func createPlatformOnlyDiff(rec *model.TradeRecord, channel string) *model.DiffOrder {
	return &model.DiffOrder{
		DiffID:        genDiffID(),
		DiffType:      model.DiffTypePlatformOnly,
		OutTradeNo:    rec.OutTradeNo,
		TradeType:     rec.TradeType,
		Channel:       channel,
		PlatformTrade: rec,
		Description:   fmt.Sprintf("平台有记录，渠道无此订单 (交易类型: %s, 金额: %d 分)", rec.TradeType, rec.Amount),
		CreatedAt:     timeutil.Now(),
	}
}

func createChannelOnlyDiff(rec *model.TradeRecord, channel string) *model.DiffOrder {
	return &model.DiffOrder{
		DiffID:       genDiffID(),
		DiffType:     model.DiffTypeChannelOnly,
		OutTradeNo:   rec.OutTradeNo,
		TradeType:    rec.TradeType,
		Channel:      channel,
		ChannelTrade: rec,
		Description:  fmt.Sprintf("渠道有记录，平台无此订单 (交易类型: %s, 金额: %d 分)", rec.TradeType, rec.Amount),
		CreatedAt:    timeutil.Now(),
	}
}

func genDiffID() string {
	return fmt.Sprintf("DIFF%d", time.Now().UnixNano())
}

func absInt64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
