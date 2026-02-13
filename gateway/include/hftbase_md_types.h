// hftbase_md_types.h — Binary-compatible MarketUpdateNew
//
// Standalone definition matching hftbase/CommonUtils/include/marketupdateNew.h
// and tbsrc-golang/pkg/shm/types.go byte-for-byte.
//
// The original C++ uses multiple inheritance (MDHeaderPart + MDDataPart).
// We use a flat struct with explicit offsets to match the in-memory layout.
//
// C++ source: hftbase/CommonUtils/include/marketupdateNew.h

#pragma once

#include <cstdint>
#include <cstring>

namespace illuminati {
namespace md {

// ============================================================
// Constants
// ============================================================
static const int32_t INTEREST_LEVELS = 20;
static const int32_t MAX_SYMBOL_SIZE = 50;

// Exchange name codes (m_exchangeName)
// C++ source: marketupdateNew.h:24-98
static const unsigned char EXCHANGE_UNKNOWN = 0;
static const unsigned char CHINA_SHFE = 57;
static const unsigned char CHINA_CFFEX = 58;
static const unsigned char CHINA_ZCE = 59;
static const unsigned char CHINA_DCE = 60;
static const unsigned char CHINA_GFEX = 61;
static const unsigned char CHINA_SH = 70;
static const unsigned char CHINA_SZ = 71;

// Feed type
static const unsigned char FEED_TBT = 'X';
static const unsigned char FEED_SNAPSHOT = 'W';

// Side
static const unsigned char MD_SIDE_BUY = 'B';
static const unsigned char MD_SIDE_SELL = 'S';
static const unsigned char MD_SIDE_NONE = 'N';

// Update type
static const unsigned char MDUPDTYPE_ADD = 'A';
static const unsigned char MDUPDTYPE_NONE = 'N';
static const unsigned char MDUPDTYPE_TRADE_INFO = 'I';

// ============================================================
// bookElement_t — 16 bytes
// C++ source: marketupdateNew.h:134-158
// Layout:
//   int32_t quantity     offset 0   size 4
//   int32_t orderCount   offset 4   size 4
//   double  price        offset 8   size 8
// Total: 16 bytes
// ============================================================
struct bookElement_t {
    int32_t quantity;
    int32_t orderCount;
    double price;
};

// ============================================================
// MDHeaderPart — 96 bytes
// C++ source: marketupdateNew.h:166-229
// Layout:
//   uint64_t m_exchTS         offset 0   size 8
//   uint64_t m_timestamp      offset 8   size 8
//   uint64_t m_seqnum         offset 16  size 8
//   uint64_t m_rptseqnum      offset 24  size 8
//   uint64_t m_tokenId        offset 32  size 8
//   char m_symbol[48]         offset 40  size 48  (MAX_SYMBOL_SIZE-2)
//   uint16_t m_symbolID       offset 88  size 2
//   uchar m_exchangeName      offset 90  size 1
//   [5 bytes pad]             offset 91  size 5
// Total: 96 bytes
// ============================================================
struct MDHeaderPart {
    uint64_t m_exchTS;
    uint64_t m_timestamp;
    uint64_t m_seqnum;
    uint64_t m_rptseqnum;
    uint64_t m_tokenId;
    char m_symbol[MAX_SYMBOL_SIZE - 2];   // 48 bytes
    uint16_t m_symbolID;
    unsigned char m_exchangeName;
    // 5 bytes implicit padding to align to 8
};

// ============================================================
// MDDataPart — 720 bytes
// C++ source: marketupdateNew.h:231-475
// Layout:
//   double m_newPrice                      offset 0    size 8
//   double m_oldPrice                      offset 8    size 8
//   double m_lastTradedPrice               offset 16   size 8
//   uint64_t m_lastTradedTime              offset 24   size 8
//   double m_totalTradedValue              offset 32   size 8
//   int64_t m_totalTradedQuantity          offset 40   size 8
//   double m_yield                         offset 48   size 8
//   bookElement_t m_bidUpdates[20]         offset 56   size 320
//   bookElement_t m_askUpdates[20]         offset 376  size 320
//   int32_t m_newQuant                     offset 696  size 4
//   int32_t m_oldQuant                     offset 700  size 4
//   int32_t m_lastTradedQuantity           offset 704  size 4
//   int8_t m_validBids                     offset 708  size 1
//   int8_t m_validAsks                     offset 709  size 1
//   int8_t m_updateLevel                   offset 710  size 1
//   uint8_t m_endPkt                       offset 711  size 1
//   uchar m_side                           offset 712  size 1
//   uchar m_updateType                     offset 713  size 1
//   uchar m_feedType                       offset 714  size 1
//   [5 bytes pad]                          offset 715  size 5
// Total: 720 bytes
// ============================================================
struct MDDataPart {
    double m_newPrice;
    double m_oldPrice;
    double m_lastTradedPrice;
    uint64_t m_lastTradedTime;
    double m_totalTradedValue;
    int64_t m_totalTradedQuantity;
    double m_yield;
    bookElement_t m_bidUpdates[INTEREST_LEVELS];   // 20 * 16 = 320
    bookElement_t m_askUpdates[INTEREST_LEVELS];   // 20 * 16 = 320
    int32_t m_newQuant;
    int32_t m_oldQuant;
    int32_t m_lastTradedQuantity;
    int8_t m_validBids;
    int8_t m_validAsks;
    int8_t m_updateLevel;
    uint8_t m_endPkt;
    unsigned char m_side;
    unsigned char m_updateType;
    unsigned char m_feedType;
    // 5 bytes implicit padding to align to 8
};

// ============================================================
// MarketUpdateNew — 816 bytes
// C++ source: marketupdateNew.h:477-557
// In original C++: struct MarketUpdateNew : public MDHeaderPart, MDDataPart {}
// For binary compatibility we use a flat struct with the same field layout.
//
// Layout:
//   MDHeaderPart   offset 0    size 96
//   MDDataPart     offset 96   size 720
// Total: 816 bytes
//
// Go match: tbsrc-golang/pkg/shm/types.go MarketUpdateNew
// ============================================================
struct MarketUpdateNew {
    // --- MDHeaderPart (96 bytes) ---
    uint64_t m_exchTS;
    uint64_t m_timestamp;
    uint64_t m_seqnum;
    uint64_t m_rptseqnum;
    uint64_t m_tokenId;
    char m_symbol[MAX_SYMBOL_SIZE - 2];   // 48 bytes
    uint16_t m_symbolID;
    unsigned char m_exchangeName;
    char _headerPad[5];                   // explicit padding

    // --- MDDataPart (720 bytes) ---
    double m_newPrice;
    double m_oldPrice;
    double m_lastTradedPrice;
    uint64_t m_lastTradedTime;
    double m_totalTradedValue;
    int64_t m_totalTradedQuantity;
    double m_yield;
    bookElement_t m_bidUpdates[INTEREST_LEVELS];
    bookElement_t m_askUpdates[INTEREST_LEVELS];
    int32_t m_newQuant;
    int32_t m_oldQuant;
    int32_t m_lastTradedQuantity;
    int8_t m_validBids;
    int8_t m_validAsks;
    int8_t m_updateLevel;
    uint8_t m_endPkt;
    unsigned char m_side;
    unsigned char m_updateType;
    unsigned char m_feedType;
    char _dataPad[5];                     // explicit padding
};

// ============================================================
// Compile-time size checks
// ============================================================
static_assert(sizeof(bookElement_t) == 16,
    "bookElement_t must be 16 bytes");
static_assert(sizeof(MDHeaderPart) == 96,
    "MDHeaderPart must be 96 bytes");
static_assert(sizeof(MDDataPart) == 720,
    "MDDataPart must be 720 bytes");
static_assert(sizeof(MarketUpdateNew) == 816,
    "MarketUpdateNew must be 816 bytes");

} // namespace md
} // namespace illuminati
