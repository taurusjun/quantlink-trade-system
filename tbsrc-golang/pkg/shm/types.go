package shm

// Binary-compatible Go translations of hftbase C++ structs.
// Every struct layout must match GCC x86-64 byte-for-byte.
//
// C++ sources:
//   hftbase/CommonUtils/include/orderresponse.h
//   hftbase/CommonUtils/include/marketupdateNew.h

// --- Constants ---

const (
	InterestLevels  = 20
	MaxSymbolSize   = 50
	MaxAccntIDLen   = 10
	MaxInstrNameSz  = 32
	MaxTradeIDSize  = 21
	MaxProductSize  = 32
	MaxORSClients   = 250
)

// MD exchange name codes
const (
	ExchangeUnknown  uint8 = 0
	ChinaSHFE        uint8 = 57
	ChinaCFFEX       uint8 = 58
	ChinaZCE         uint8 = 59
	ChinaDCE         uint8 = 60
	ChinaGFEX        uint8 = 61
)

// MD side constants
const (
	SideBuy  uint8 = 'B'
	SideSell uint8 = 'S'
)

// MD feed type
const (
	FeedTBT      uint8 = 'X'
	FeedSnapshot uint8 = 'W'
)

// --- Enums (C++ enum = int32 on GCC x86-64) ---

type RequestType int32

const (
	NEWORDER       RequestType = 0
	MODIFYORDER    RequestType = 1
	CANCELORDER    RequestType = 2
	ORDERSTATUS    RequestType = 3
	SESSIONMSG     RequestType = 4
	HEARTBEAT      RequestType = 5
	OPTEXEC        RequestType = 6
	OPTEXEC_CANCEL RequestType = 7
)

type OrderType int32

const (
	LIMIT                  OrderType = 1
	MARKET                 OrderType = 2
	WEIGHTAVG              OrderType = 3
	CONDITIONAL_LIMIT_PRICE OrderType = 4
	BEST_PRICE             OrderType = 5
)

type OrderDuration int32

const (
	DAY     OrderDuration = 0
	IOC     OrderDuration = 1
	FOK     OrderDuration = 2
	COUNTER OrderDuration = 3
	FAK     OrderDuration = 4
)

type PriceType int32

const (
	PERCENTAGE PriceType = 1
	PERUNIT    PriceType = 2
	YIELD      PriceType = 9
)

type InstrumentType int32

const (
	STK InstrumentType = 0
	FUT InstrumentType = 1
	OPT InstrumentType = 2
	XXX InstrumentType = 3
)

type PositionDirection int32

const (
	POS_OPEN           PositionDirection = 10
	POS_CLOSE          PositionDirection = 11
	POS_CLOSE_INTRADAY PositionDirection = 12
	POS_ERROR          PositionDirection = 13
)

type ResponseType int32

const (
	NEW_ORDER_CONFIRM           ResponseType = 0
	NEW_ORDER_FREEZE            ResponseType = 1
	MODIFY_ORDER_CONFIRM        ResponseType = 2
	CANCEL_ORDER_CONFIRM        ResponseType = 3
	TRADE_CONFIRM               ResponseType = 4
	ORDER_ERROR                 ResponseType = 5
	MODIFY_ORDER_REJECT         ResponseType = 6
	CANCEL_ORDER_REJECT         ResponseType = 7
	ORS_REJECT                  ResponseType = 8
	RMS_REJECT                  ResponseType = 9
	SIM_REJECT                  ResponseType = 10
	BUSINESS_REJECT             ResponseType = 11
	MODIFY_ORDER_PENDING        ResponseType = 12
	CANCEL_ORDER_PENDING        ResponseType = 13
	ORDERS_PER_DAY_LIMIT_REJECT ResponseType = 14
	ORDERS_PER_DAY_LIMIT_WARNING ResponseType = 15
	ORDER_EXPIRED               ResponseType = 16
	STOP_LOSS_WARNING           ResponseType = 17
	NULL_RESPONSE               ResponseType = 18
)

type SubResponseType int32

const (
	NULL_RESPONSE_MIDDLE       SubResponseType = 0
	ORDER_REJECT_MIDDLE        SubResponseType = 1
	MODIFY_REJECT_MIDDLE       SubResponseType = 2
	CANCEL_ORDER_REJECT_MIDDLE SubResponseType = 3
)

// enum class OpenCloseType : char (int8)
type OpenCloseType int8

const (
	OC_NULL_TYPE   OpenCloseType = 0
	OC_OPEN        OpenCloseType = 1
	OC_CLOSE       OpenCloseType = 2
	OC_CLOSE_TODAY OpenCloseType = 3
)

// enum class TsExchangeID : char (int8)
type TsExchangeID int8

const (
	TS_NULL_EXCHANGE TsExchangeID = 0
	TS_SHFE          TsExchangeID = 1
	TS_INE           TsExchangeID = 2
	TS_CZCE          TsExchangeID = 3
	TS_DCE           TsExchangeID = 4
	TS_CFFEX         TsExchangeID = 5
	TS_GFEX          TsExchangeID = 6
)

type ExchangeType int32

const (
	ET_NSE_FO            ExchangeType = 0
	ET_NSE_CM            ExchangeType = 1
	ET_NSE_CDS           ExchangeType = 2
	ET_MICEX_FOND        ExchangeType = 3
	ET_MICEX_CURR        ExchangeType = 4
	ET_MCX               ExchangeType = 5
	ET_CME               ExchangeType = 6
	ET_LME               ExchangeType = 7
	ET_NYSE              ExchangeType = 8
	ET_ARCA              ExchangeType = 9
	ET_NOT_NSE           ExchangeType = 10
	ET_REQUEST_MSG_EXCHG ExchangeType = 11
	ET_RESPONSE_MSG_EXCHG ExchangeType = 12
)

// --- Structs (binary-compatible with GCC x86-64) ---

// ContractDescription matches C++ ContractDescription.
// C++ layout:
//   char InstrumentName[32]    offset 0   size 32
//   char Symbol[50]            offset 32  size 50
//   int32_t ExpiryDate         offset 84  size 4   (needs 2 bytes pad after Symbol)
//   int32_t StrikePrice        offset 88  size 4
//   char OptionType[2]         offset 92  size 2
//   int16_t CALevel            offset 94  size 2
// Total: 96 bytes (with 2 bytes padding after Symbol for int32_t alignment)
type ContractDescription struct {
	InstrumentName [MaxInstrNameSz]byte // offset 0, size 32
	Symbol         [MaxSymbolSize]byte  // offset 32, size 50
	_pad0          [2]byte              // offset 82, size 2 (align ExpiryDate to 4)
	ExpiryDate     int32                // offset 84, size 4
	StrikePrice    int32                // offset 88, size 4
	OptionType     [2]byte              // offset 92, size 2
	CALevel        int16                // offset 94, size 2
}

// RequestMsg matches C++ RequestMsg __attribute__((aligned(64))).
// C++ layout (GCC x86-64):
//   ContractDescription Contract_Description  offset 0    size 96
//   RequestType Request_Type                  offset 96   size 4
//   OrderType OrdType                         offset 100  size 4
//   OrderDuration Duration                    offset 104  size 4
//   PriceType PxType                          offset 108  size 4
//   PositionDirection PosDirection             offset 112  size 4
//   uint32_t OrderID                          offset 116  size 4
//   int32_t Token                             offset 120  size 4
//   int32_t Quantity                          offset 124  size 4
//   int32_t QuantityFilled                    offset 128  size 4
//   int32_t DisclosedQnty                     offset 132  size 4
//   double Price                              offset 136  size 8
//   uint64_t TimeStamp                        offset 144  size 8
//   char AccountID[11]                        offset 152  size 11
//   unsigned char Transaction_Type            offset 163  size 1
//   unsigned char Exchange_Type               offset 164  size 1
//   char padding[20]                          offset 165  size 20
//   char Product[32]                          offset 185  size 32
//   int StrategyID                            offset 220  size 4  (needs 3 bytes pad for int alignment? No, 217 is not aligned)
//   --- natural end: 224 bytes; aligned(64) rounds to 256 ---
//   _pad to 256
//
// Wait: offset 165+20=185, 185+32=217, 217 is not 4-aligned.
// At offset 217, we need int (4 bytes). 217 % 4 = 1, so compiler adds 3 bytes padding.
// StrategyID at offset 220, ends at 224. aligned(64): 224 → 256.
type RequestMsg struct {
	ContractDesc    ContractDescription // offset 0,   size 96
	Request_Type    RequestType         // offset 96,  size 4
	OrdType         OrderType           // offset 100, size 4
	Duration        OrderDuration       // offset 104, size 4
	PxType          PriceType           // offset 108, size 4
	PosDirection    PositionDirection   // offset 112, size 4
	OrderID         uint32              // offset 116, size 4
	Token           int32               // offset 120, size 4
	Quantity        int32               // offset 124, size 4
	QuantityFilled  int32               // offset 128, size 4
	DisclosedQnty   int32               // offset 132, size 4
	Price           float64             // offset 136, size 8
	TimeStamp       uint64              // offset 144, size 8
	AccountID       [MaxAccntIDLen + 1]byte // offset 152, size 11
	TransactionType uint8               // offset 163, size 1
	ExchangeType    uint8               // offset 164, size 1
	Padding         [20]byte            // offset 165, size 20
	Product         [MaxProductSize]byte // offset 185, size 32
	_pad1           [3]byte             // offset 217, size 3 (align StrategyID to 4)
	StrategyID      int32               // offset 220, size 4
	_pad2           [32]byte            // offset 224, size 32 (pad to 256 for aligned(64))
}

// ResponseMsg matches C++ ResponseMsg.
// C++ layout (GCC x86-64):
//   ResponseType Response_Type         offset 0    size 4
//   SubResponseType Child_Response     offset 4    size 4
//   uint32_t OrderID                   offset 8    size 4
//   uint32_t ErrorCode                 offset 12   size 4
//   int32_t Quantity                   offset 16   size 4
//   [4 bytes padding for double align] offset 20   size 4
//   double Price                       offset 24   size 8
//   uint64_t TimeStamp                 offset 32   size 8
//   unsigned char Side                 offset 40   size 1
//   char Symbol[50]                    offset 41   size 50
//   char AccountID[11]                 offset 91   size 11
//   [2 bytes padding for double align] offset 102  size 2
//   double ExchangeOrderId             offset 104  size 8
//   char ExchangeTradeId[21]           offset 112  size 21
//   OpenCloseType OpenClose            offset 133  size 1
//   TsExchangeID ExchangeID           offset 134  size 1
//   char Product[32]                   offset 135  size 32
//   [1 byte padding for int align]    offset 167  size 1
//   int StrategyID                     offset 168  size 4
//   [4 bytes tail padding to align struct to 8]  offset 172 size 4
// Total: 176 bytes
type ResponseMsg struct {
	Response_Type   ResponseType    // offset 0,   size 4
	Child_Response  SubResponseType // offset 4,   size 4
	OrderID         uint32          // offset 8,   size 4
	ErrorCode       uint32          // offset 12,  size 4
	Quantity        int32           // offset 16,  size 4
	_pad0           [4]byte         // offset 20,  size 4 (align Price to 8)
	Price           float64         // offset 24,  size 8
	TimeStamp       uint64          // offset 32,  size 8
	Side            uint8           // offset 40,  size 1
	Symbol          [MaxSymbolSize]byte // offset 41, size 50
	AccountID       [MaxAccntIDLen + 1]byte // offset 91, size 11
	_pad1           [2]byte         // offset 102, size 2 (align ExchangeOrderId to 8)
	ExchangeOrderId float64         // offset 104, size 8
	ExchangeTradeId [MaxTradeIDSize]byte // offset 112, size 21
	OpenClose       OpenCloseType   // offset 133, size 1
	ExchangeID      TsExchangeID    // offset 134, size 1
	Product         [MaxProductSize]byte // offset 135, size 32
	_pad2           [1]byte         // offset 167, size 1 (align StrategyID to 4)
	StrategyID      int32           // offset 168, size 4
	_pad3           [4]byte         // offset 172, size 4 (struct tail padding to align 8)
}

// BookElement matches C++ bookElement_t (inherits orderQtPair_t).
// Layout: quantity(4) + orderCount(4) + price(8) = 16 bytes
type BookElement struct {
	Quantity   int32   // offset 0, size 4
	OrderCount int32   // offset 4, size 4
	Price      float64 // offset 8, size 8
}

// MDHeaderPart matches C++ MDHeaderPart.
// Layout:
//   uint64_t m_exchTS        offset 0,  size 8
//   uint64_t m_timestamp     offset 8,  size 8
//   uint64_t m_seqnum        offset 16, size 8
//   uint64_t m_rptseqnum     offset 24, size 8
//   uint64_t m_tokenId       offset 32, size 8
//   char m_symbol[48]        offset 40, size 48
//   uint16_t m_symbolID      offset 88, size 2
//   unsigned char m_exchangeName offset 90, size 1
//   [5 bytes padding to align struct to 8] offset 91, size 5
// Total: 96 bytes
type MDHeaderPart struct {
	ExchTS       uint64           // offset 0
	Timestamp    uint64           // offset 8
	Seqnum       uint64           // offset 16
	RptSeqnum    uint64           // offset 24
	TokenId      uint64           // offset 32
	Symbol       [MaxSymbolSize - 2]byte // offset 40, size 48
	SymbolID     uint16           // offset 88
	ExchangeName uint8            // offset 90
	_pad0        [5]byte          // offset 91, pad to 96
}

// MDDataPart matches C++ MDDataPart.
// Layout:
//   double m_newPrice               offset 0,   size 8
//   double m_oldPrice               offset 8,   size 8
//   double m_lastTradedPrice        offset 16,  size 8
//   uint64_t m_lastTradedTime       offset 24,  size 8
//   double m_totalTradedValue       offset 32,  size 8
//   int64_t m_totalTradedQuantity   offset 40,  size 8
//   double m_yield                  offset 48,  size 8
//   bookElement_t m_bidUpdates[20]  offset 56,  size 320
//   bookElement_t m_askUpdates[20]  offset 376, size 320
//   int32_t m_newQuant              offset 696, size 4
//   int32_t m_oldQuant              offset 700, size 4
//   int32_t m_lastTradedQuantity    offset 704, size 4
//   int8_t m_validBids              offset 708, size 1
//   int8_t m_validAsks              offset 709, size 1
//   int8_t m_updateLevel            offset 710, size 1
//   uint8_t m_endPkt                offset 711, size 1
//   unsigned char m_side            offset 712, size 1
//   unsigned char m_updateType      offset 713, size 1
//   unsigned char m_feedType        offset 714, size 1
//   [5 bytes padding to align struct to 8] offset 715, size 5
// Total: 720 bytes
type MDDataPart struct {
	NewPrice            float64              // offset 0
	OldPrice            float64              // offset 8
	LastTradedPrice     float64              // offset 16
	LastTradedTime      uint64               // offset 24
	TotalTradedValue    float64              // offset 32
	TotalTradedQuantity int64                // offset 40
	Yield               float64              // offset 48
	BidUpdates          [InterestLevels]BookElement // offset 56, size 320
	AskUpdates          [InterestLevels]BookElement // offset 376, size 320
	NewQuant            int32                // offset 696
	OldQuant            int32                // offset 700
	LastTradedQuantity  int32                // offset 704
	ValidBids           int8                 // offset 708
	ValidAsks           int8                 // offset 709
	UpdateLevel         int8                 // offset 710
	EndPkt              uint8                // offset 711
	Side                uint8                // offset 712
	UpdateType          uint8                // offset 713
	FeedType            uint8                // offset 714
	_pad0               [5]byte              // offset 715, pad to 720
}

// MarketUpdateNew matches C++ MarketUpdateNew : public MDHeaderPart, MDDataPart.
// Layout: MDHeaderPart(96) + MDDataPart(720) = 816 bytes
type MarketUpdateNew struct {
	Header MDHeaderPart // offset 0,  size 96
	Data   MDDataPart   // offset 96, size 720
}

// --- Queue element wrappers (used in MWMRQueue SHM layout) ---

// ReqQueueElemSize is the C++ sizeof(QueueElem<RequestMsg>) = 320 bytes.
// C++: RequestMsg has __attribute__((aligned(64))), which causes the compiler
// to pad QueueElem<RequestMsg> from 264 (=256+8) to 320 (=5*64) bytes.
// This must be passed as elemSizeOverride when creating/opening the request queue.
const ReqQueueElemSize uintptr = 320

// QueueElemMD is QueueElem<MarketUpdateNew>: data + seqNo
type QueueElemMD struct {
	Data  MarketUpdateNew
	SeqNo uint64
}

// QueueElemReq is QueueElem<RequestMsg>: data + seqNo
// C++: RequestMsg has __attribute__((aligned(64))), so QueueElem<RequestMsg>
// is padded to 320 bytes (= next multiple of 64 after 256+8=264).
// The seqNo is at offset 256 (sizeof(RequestMsg)), and the remaining
// 56 bytes (320-264) are tail padding for alignment.
type QueueElemReq struct {
	Data  RequestMsg
	SeqNo uint64
	_pad  [56]byte // pad to 320 bytes to match C++ aligned(64) on RequestMsg
}

// QueueElemResp is QueueElem<ResponseMsg>: data + seqNo
type QueueElemResp struct {
	Data  ResponseMsg
	SeqNo uint64
}

// MWMRHeader matches C++ MultiWriterMultiReaderShmHeader.
// Layout: atomic<int64_t> head (8 bytes)
type MWMRHeader struct {
	Head int64
}

// ClientData matches C++ LocklessShmClientStore::ClientData.
// Layout: atomic<uint64_t> data (8 bytes) + uint64_t firstCliendId (8 bytes) = 16 bytes
type ClientData struct {
	Data          int64 // atomic<uint64_t> — we use int64 to match C++ fetch_add semantics
	FirstClientId int64
}
