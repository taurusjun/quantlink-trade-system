// hftbase_types.h — Binary-compatible RequestMsg/ResponseMsg
//
// Standalone definitions matching hftbase/CommonUtils/include/orderresponse.h
// and tbsrc-golang/pkg/shm/types.go byte-for-byte.
//
// Layout verified against Go struct comments (types.go:200-295).
// Must pass offset_check three-way verification (hftbase vs this vs Go).
//
// C++ source: hftbase/CommonUtils/include/orderresponse.h

#pragma once

#include <cstdint>
#include <cstring>

namespace illuminati {
namespace infra {

// ============================================================
// Constants (matching hftbase/CommonUtils/include/constants.h)
// ============================================================
static const int32_t ORDERID_RANGE = 1000000;
static const int MAX_ACCNTID_LEN = 10;
static const int MAX_SYMBOL_SIZE = 50;
static const int MAX_INSTRNAME_SIZE = 32;
static const int MAX_TRADE_ID_SIZE = 21;
static const int MAX_PRODUCT_SIZE = 32;

// ============================================================
// RequestType enum
// C++ source: orderresponse.h
// ============================================================
enum RequestType {
    NEWORDER = 0,
    MODIFYORDER = 1,
    CANCELORDER = 2,
    ORDERSTATUS = 3,
    SESSIONMSG = 4,
    HEARTBEAT = 5,
    OPTEXEC = 6,
    OPTEXEC_CANCEL = 7,
};

// ============================================================
// ResponseType enum
// ============================================================
enum ResponseType {
    NEW_ORDER_CONFIRM = 0,
    NEW_ORDER_FREEZE = 1,
    MODIFY_ORDER_CONFIRM = 2,
    CANCEL_ORDER_CONFIRM = 3,
    TRADE_CONFIRM = 4,
    ORDER_ERROR = 5,
    MODIFY_ORDER_REJECT = 6,
    CANCEL_ORDER_REJECT = 7,
    ORS_REJECT = 8,
    RMS_REJECT = 9,
    SIM_REJECT = 10,
    BUSINESS_REJECT = 11,
    MODIFY_ORDER_PENDING = 12,
    CANCEL_ORDER_PENDING = 13,
    ORDERS_PER_DAY_LIMIT_REJECT = 14,
    ORDERS_PER_DAY_LIMIT_WARNING = 15,
    ORDER_EXPIRED = 16,
    STOP_LOSS_WARNING = 17,
    NULL_RESPONSE = 18,
};

// ============================================================
// SubResponseType enum
// ============================================================
enum SubResponseType {
    NULL_RESPONSE_MIDDLE = 0,
    ORDER_REJECT_MIDDLE = 1,
    MODIFY_REJECT_MIDDLE = 2,
    CANCEL_ORDER_REJECT_MIDDLE = 3,
};

// ============================================================
// PositionDirection enum (int32)
// ============================================================
enum PositionDirection {
    POS_OPEN = 10,
    POS_CLOSE = 11,
    POS_CLOSE_INTRADAY = 12,
    POS_ERROR = 13,
};

// ============================================================
// OrderType enum (int32)
// ============================================================
enum OrderType {
    OT_LIMIT = 1,
    OT_MARKET = 2,
    OT_WEIGHTAVG = 3,
    OT_CONDITIONAL_LIMIT_PRICE = 4,
    OT_BEST_PRICE = 5,
};

// ============================================================
// OrderDuration enum (int32)
// ============================================================
enum OrderDuration {
    OD_DAY = 0,
    OD_IOC = 1,
    OD_FOK = 2,
    OD_COUNTER = 3,
    OD_FAK = 4,
};

// ============================================================
// PriceType enum (int32)
// ============================================================
enum PriceType {
    PT_PERCENTAGE = 1,
    PT_PERUNIT = 2,
    PT_YIELD = 9,
};

// ============================================================
// OpenCloseType (char/int8)
// ============================================================
enum OpenCloseType : char {
    OCT_NULL_TYPE = 0,
    OCT_OPEN = 1,
    OCT_CLOSE = 2,
    OCT_CLOSE_TODAY = 3,
};

// ============================================================
// TsExchangeID (char/int8)
// ============================================================
enum TsExchangeID : char {
    TSEXCH_NULL = 0,
    TSEXCH_SHFE = 1,
    TSEXCH_INE = 2,
    TSEXCH_CZCE = 3,
    TSEXCH_DCE = 4,
    TSEXCH_CFFEX = 5,
    TSEXCH_GFEX = 6,
};

// ============================================================
// Exchange_Type byte values (used in RequestMsg.Exchange_Type)
// Matches hftbase MD exchange codes
// ============================================================
static const unsigned char CHINA_SHFE  = 57;
static const unsigned char CHINA_CFFEX = 58;
static const unsigned char CHINA_ZCE   = 59;
static const unsigned char CHINA_DCE   = 60;
static const unsigned char CHINA_GFEX  = 61;

// ============================================================
// Transaction_Type byte values
// ============================================================
static const unsigned char SIDE_BUY  = 'B';
static const unsigned char SIDE_SELL = 'S';

// ============================================================
// ContractDescription
// C++ source: orderresponse.h:107-115
// Layout (GCC x86-64):
//   char InstrumentName[32]    offset 0   size 32
//   char Symbol[50]            offset 32  size 50
//   [2 bytes pad]              offset 82  size 2
//   int32_t ExpiryDate         offset 84  size 4
//   int32_t StrikePrice        offset 88  size 4
//   char OptionType[2]         offset 92  size 2
//   int16_t CALevel            offset 94  size 2
// Total: 96 bytes
// ============================================================
struct ContractDescription {
    char InstrumentName[MAX_INSTRNAME_SIZE];  // 32 bytes
    char Symbol[MAX_SYMBOL_SIZE];             // 50 bytes
    // 2 bytes implicit padding for int32_t alignment
    int32_t ExpiryDate;
    int32_t StrikePrice;
    char OptionType[2];
    int16_t CALevel;
};

// ============================================================
// RequestMsg
// C++ source: orderresponse.h:134-295
// __attribute__((aligned(64)))
//
// Layout (GCC x86-64, verified against Go types.go:227-249):
//   ContractDescription     offset 0    size 96
//   int32 Request_Type      offset 96   size 4
//   int32 OrdType           offset 100  size 4
//   int32 Duration          offset 104  size 4
//   int32 PxType            offset 108  size 4
//   int32 PosDirection      offset 112  size 4
//   uint32 OrderID          offset 116  size 4
//   int32 Token             offset 120  size 4
//   int32 Quantity          offset 124  size 4
//   int32 QuantityFilled    offset 128  size 4
//   int32 DisclosedQnty     offset 132  size 4
//   double Price            offset 136  size 8
//   uint64 TimeStamp        offset 144  size 8
//   char AccountID[11]      offset 152  size 11
//   uchar Transaction_Type  offset 163  size 1
//   uchar Exchange_Type     offset 164  size 1
//   char padding[20]        offset 165  size 20
//   char Product[32]        offset 185  size 32
//   [3 bytes pad]           offset 217  size 3
//   int StrategyID          offset 220  size 4
//   [32 bytes pad to 256]   offset 224  size 32
// Total: 256 bytes (aligned(64))
// ============================================================
struct RequestMsg {
    ContractDescription Contract_Description;  // offset 0,   96 bytes
    int32_t Request_Type;                      // offset 96,  4 bytes (enum RequestType)
    int32_t OrdType;                           // offset 100, 4 bytes (enum OrderType)
    int32_t Duration;                          // offset 104, 4 bytes (enum OrderDuration)
    int32_t PxType;                            // offset 108, 4 bytes (enum PriceType)
    int32_t PosDirection;                      // offset 112, 4 bytes (enum PositionDirection)
    uint32_t OrderID;                          // offset 116, 4 bytes
    int32_t Token;                             // offset 120, 4 bytes
    int32_t Quantity;                          // offset 124, 4 bytes
    int32_t QuantityFilled;                    // offset 128, 4 bytes
    int32_t DisclosedQnty;                     // offset 132, 4 bytes
    double Price;                              // offset 136, 8 bytes
    uint64_t TimeStamp;                        // offset 144, 8 bytes
    char AccountID[MAX_ACCNTID_LEN + 1];       // offset 152, 11 bytes
    unsigned char Transaction_Type;            // offset 163, 1 byte ('B' or 'S')
    unsigned char Exchange_Type;               // offset 164, 1 byte
    char padding[20];                          // offset 165, 20 bytes
    char Product[MAX_PRODUCT_SIZE];            // offset 185, 32 bytes
    // 3 bytes implicit padding at offset 217 for int alignment
    int StrategyID;                            // offset 220, 4 bytes
    // 32 bytes implicit padding to reach 256 (aligned(64))
} __attribute__((aligned(64)));

// ============================================================
// ResponseMsg
// C++ source: orderresponse.h:436-561
//
// Layout (GCC x86-64, verified against Go types.go:274-295):
//   int32 Response_Type        offset 0    size 4
//   int32 Child_Response       offset 4    size 4
//   uint32 OrderID             offset 8    size 4
//   uint32 ErrorCode           offset 12   size 4
//   int32 Quantity             offset 16   size 4
//   [4 bytes pad]              offset 20   size 4
//   double Price               offset 24   size 8
//   uint64 TimeStamp           offset 32   size 8
//   uchar Side                 offset 40   size 1
//   char Symbol[50]            offset 41   size 50
//   char AccountID[11]         offset 91   size 11
//   [2 bytes pad]              offset 102  size 2
//   double ExchangeOrderId     offset 104  size 8
//   char ExchangeTradeId[21]   offset 112  size 21
//   OpenCloseType OpenClose    offset 133  size 1
//   TsExchangeID ExchangeID   offset 134  size 1
//   char Product[32]           offset 135  size 32
//   [1 byte pad]               offset 167  size 1
//   int StrategyID             offset 168  size 4
//   [4 bytes tail pad]         offset 172  size 4
// Total: 176 bytes
// ============================================================
struct ResponseMsg {
    int32_t Response_Type;                     // offset 0,   4 bytes (enum ResponseType)
    int32_t Child_Response;                    // offset 4,   4 bytes (enum SubResponseType)
    uint32_t OrderID;                          // offset 8,   4 bytes
    uint32_t ErrorCode;                        // offset 12,  4 bytes
    int32_t Quantity;                          // offset 16,  4 bytes
    // 4 bytes implicit padding at offset 20 for double alignment
    double Price;                              // offset 24,  8 bytes
    uint64_t TimeStamp;                        // offset 32,  8 bytes
    unsigned char Side;                        // offset 40,  1 byte ('B' or 'S')
    char Symbol[MAX_SYMBOL_SIZE];              // offset 41,  50 bytes
    char AccountID[MAX_ACCNTID_LEN + 1];       // offset 91,  11 bytes
    // 2 bytes implicit padding at offset 102 for double alignment
    double ExchangeOrderId;                    // offset 104, 8 bytes
    char ExchangeTradeId[MAX_TRADE_ID_SIZE];   // offset 112, 21 bytes
    OpenCloseType OpenClose;                   // offset 133, 1 byte
    TsExchangeID ExchangeID;                  // offset 134, 1 byte
    char Product[MAX_PRODUCT_SIZE];            // offset 135, 32 bytes
    // 1 byte implicit padding at offset 167 for int alignment
    int StrategyID;                            // offset 168, 4 bytes
    // 4 bytes implicit tail padding to align struct size to 8
};

// ============================================================
// Compile-time size checks
// ============================================================
static_assert(sizeof(ContractDescription) == 96,
    "ContractDescription must be 96 bytes");
static_assert(sizeof(RequestMsg) == 256,
    "RequestMsg must be 256 bytes (aligned(64))");
static_assert(sizeof(ResponseMsg) == 176,
    "ResponseMsg must be 176 bytes");

} // namespace infra
} // namespace illuminati
