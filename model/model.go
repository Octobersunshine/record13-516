package model

import "time"

type TradeType string

const (
	TradeTypePayment TradeType = "PAYMENT"
	TradeTypeRefund  TradeType = "REFUND"
)

type TradeRecord struct {
	TradeNo      string
	OutTradeNo   string
	TradeType    TradeType
	Amount       int64
	TradeTime    time.Time
	Status       string
	PayChannel   string
	Fee          int64
	Raw          map[string]string
}

type ReconciliationSummary struct {
	Date           string
	TotalCount     int64
	TotalAmount    int64
	PaymentCount   int64
	PaymentAmount  int64
	RefundCount    int64
	RefundAmount   int64
	TotalFee       int64
	Channel        string
}

type ReconciliationResult struct {
	WechatSummary    *ReconciliationSummary
	AlipaySummary    *ReconciliationSummary
	CombinedSummary  *ReconciliationSummary
	WechatRecords    []*TradeRecord
	AlipayRecords    []*TradeRecord
	Date             string
}

type ReconciliationConfig struct {
	Wechat *WechatConfig
	Alipay *AlipayConfig
}

type WechatConfig struct {
	AppID      string
	MchID      string
	APIKey     string
	CertPath   string
	KeyPath    string
	APIv3Key   string
	MchCertSerial string
}

type AlipayConfig struct {
	AppID      string
	PrivateKey string
	PublicKey  string
	AppCertSN  string
	AlipayRootCertSN string
}

type DiffType string

const (
	DiffTypeAmountMismatch   DiffType = "AMOUNT_MISMATCH"
	DiffTypeStatusMismatch   DiffType = "STATUS_MISMATCH"
	DiffTypeFeeMismatch      DiffType = "FEE_MISMATCH"
	DiffTypePlatformOnly     DiffType = "PLATFORM_ONLY"
	DiffTypeChannelOnly      DiffType = "CHANNEL_ONLY"
	DiffTypeTypeMismatch     DiffType = "TYPE_MISMATCH"
)

type DiffOrder struct {
	DiffID       string
	DiffType     DiffType
	OutTradeNo   string
	TradeType    TradeType
	Channel      string
	PlatformTrade *TradeRecord
	ChannelTrade  *TradeRecord
	AmountDiff   int64
	FeeDiff      int64
	Description  string
	CreatedAt    time.Time
}

type DiffResult struct {
	Date           string
	Channel        string
	TotalDiffCount int64
	TotalDiffAmount int64
	DiffOrders     []*DiffOrder
	DiffSummary    map[DiffType]int64
	PlatformCount  int64
	ChannelCount   int64
	MatchedCount   int64
	MatchRate      float64
}
