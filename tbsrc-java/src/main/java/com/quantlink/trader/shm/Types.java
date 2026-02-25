package com.quantlink.trader.shm;

import java.lang.foreign.MemoryLayout;
import java.lang.foreign.MemoryLayout.PathElement;
import java.lang.foreign.StructLayout;
import java.lang.foreign.ValueLayout;
import java.lang.invoke.VarHandle;

/**
 * Panama StructLayout 定义，与 C++ hftbase 共享内存结构体逐字节二进制兼容。
 *
 * C++ 原代码:
 *   hftbase/CommonUtils/include/marketupdateNew.h — MarketUpdateNew, MDHeaderPart, MDDataPart, bookElement_t
 *   hftbase/CommonUtils/include/orderresponse.h   — RequestMsg, ResponseMsg, ContractDescription
 *
 * 所有 offset 已通过 Go types_test.go 和 C++ sizeof/offsetof 交叉验证。
 * 目标平台: GCC x86-64 ABI（与 hftbase 编译目标一致）
 */
public final class Types {

    private Types() {} // 不可实例化

    // =====================================================================
    // 1. BookElement (16 bytes)
    // C++: struct bookElement_t : public orderQtPair_t { double price; }
    // C++: struct orderQtPair_t { int32_t quantity; int32_t orderCount; }
    // Ref: hftbase/CommonUtils/include/marketupdateNew.h:134-158
    // =====================================================================
    public static final StructLayout BOOK_ELEMENT_LAYOUT = MemoryLayout.structLayout(
        ValueLayout.JAVA_INT.withName("quantity"),       // offset 0,  size 4  — C++: int32_t quantity
        ValueLayout.JAVA_INT.withName("orderCount"),     // offset 4,  size 4  — C++: int32_t orderCount
        ValueLayout.JAVA_DOUBLE.withName("price")        // offset 8,  size 8  — C++: double price
    ).withName("BookElement");
    // total: 16 bytes — C++: sizeof(bookElement_t) = 16

    /** C++: bookElement_t::quantity (offset 0) */
    public static final VarHandle BE_QUANTITY_VH = BOOK_ELEMENT_LAYOUT.varHandle(
        PathElement.groupElement("quantity"));
    /** C++: bookElement_t::orderCount (offset 4) */
    public static final VarHandle BE_ORDER_COUNT_VH = BOOK_ELEMENT_LAYOUT.varHandle(
        PathElement.groupElement("orderCount"));
    /** C++: bookElement_t::price (offset 8) */
    public static final VarHandle BE_PRICE_VH = BOOK_ELEMENT_LAYOUT.varHandle(
        PathElement.groupElement("price"));

    // =====================================================================
    // 2. ContractDescription (96 bytes)
    // C++: struct ContractDescription (orderresponse.h:107-115)
    // Layout:
    //   char InstrumentName[32]  offset 0   size 32
    //   char Symbol[50]          offset 32  size 50
    //   [2 bytes padding]        offset 82  size 2  — C++: implicit padding for int32_t alignment
    //   int32_t ExpiryDate       offset 84  size 4
    //   int32_t StrikePrice      offset 88  size 4
    //   char OptionType[2]       offset 92  size 2
    //   int16_t CALevel          offset 94  size 2
    // total: 96 bytes
    // =====================================================================
    public static final StructLayout CONTRACT_DESC_LAYOUT = MemoryLayout.structLayout(
        MemoryLayout.sequenceLayout(32, ValueLayout.JAVA_BYTE).withName("instrumentName"),  // offset 0,  size 32
        MemoryLayout.sequenceLayout(50, ValueLayout.JAVA_BYTE).withName("symbol"),          // offset 32, size 50
        MemoryLayout.paddingLayout(2).withName("_pad0"),                                    // offset 82, size 2  — C++: implicit padding
        ValueLayout.JAVA_INT.withName("expiryDate"),                                        // offset 84, size 4
        ValueLayout.JAVA_INT.withName("strikePrice"),                                       // offset 88, size 4
        MemoryLayout.sequenceLayout(2, ValueLayout.JAVA_BYTE).withName("optionType"),       // offset 92, size 2
        ValueLayout.JAVA_SHORT.withName("caLevel")                                          // offset 94, size 2
    ).withName("ContractDescription");
    // total: 96 bytes

    /** C++: ContractDescription::ExpiryDate (offset 84) */
    public static final VarHandle CD_EXPIRY_DATE_VH = CONTRACT_DESC_LAYOUT.varHandle(
        PathElement.groupElement("expiryDate"));
    /** C++: ContractDescription::StrikePrice (offset 88) */
    public static final VarHandle CD_STRIKE_PRICE_VH = CONTRACT_DESC_LAYOUT.varHandle(
        PathElement.groupElement("strikePrice"));
    /** C++: ContractDescription::CALevel (offset 94) */
    public static final VarHandle CD_CA_LEVEL_VH = CONTRACT_DESC_LAYOUT.varHandle(
        PathElement.groupElement("caLevel"));

    // ContractDescription 字段 offset 常量（用于数组字段的手动读取）
    public static final long CD_INSTRUMENT_NAME_OFFSET = 0;   // char[32]
    public static final long CD_SYMBOL_OFFSET = 32;            // char[50]
    public static final long CD_EXPIRY_DATE_OFFSET = 84;       // int32_t
    public static final long CD_STRIKE_PRICE_OFFSET = 88;      // int32_t
    public static final long CD_OPTION_TYPE_OFFSET = 92;        // char[2]
    public static final long CD_CA_LEVEL_OFFSET = 94;           // int16_t

    // =====================================================================
    // 3. MDHeaderPart (96 bytes)
    // C++: struct MDHeaderPart (marketupdateNew.h:166-229)
    // Layout:
    //   uint64_t m_exchTS         offset 0,  size 8
    //   uint64_t m_timestamp      offset 8,  size 8
    //   uint64_t m_seqnum         offset 16, size 8
    //   uint64_t m_rptseqnum      offset 24, size 8
    //   uint64_t m_tokenId        offset 32, size 8
    //   char m_symbol[48]         offset 40, size 48  — C++: MAX_SYMBOL_SIZE - 2 = 48
    //   uint16_t m_symbolID       offset 88, size 2
    //   unsigned char m_exchangeName offset 90, size 1
    //   [5 bytes padding]         offset 91, size 5  — C++: struct tail padding to align to 8
    // total: 96 bytes
    // =====================================================================
    public static final StructLayout MD_HEADER_LAYOUT = MemoryLayout.structLayout(
        ValueLayout.JAVA_LONG.withName("exchTS"),                                            // offset 0,  size 8  — C++: uint64_t m_exchTS
        ValueLayout.JAVA_LONG.withName("timestamp"),                                         // offset 8,  size 8  — C++: uint64_t m_timestamp
        ValueLayout.JAVA_LONG.withName("seqnum"),                                            // offset 16, size 8  — C++: uint64_t m_seqnum
        ValueLayout.JAVA_LONG.withName("rptSeqnum"),                                         // offset 24, size 8  — C++: uint64_t m_rptseqnum
        ValueLayout.JAVA_LONG.withName("tokenId"),                                           // offset 32, size 8  — C++: uint64_t m_tokenId
        MemoryLayout.sequenceLayout(48, ValueLayout.JAVA_BYTE).withName("symbol"),           // offset 40, size 48 — C++: char m_symbol[MAX_SYMBOL_SIZE-2]
        ValueLayout.JAVA_SHORT.withName("symbolID"),                                         // offset 88, size 2  — C++: uint16_t m_symbolID
        ValueLayout.JAVA_BYTE.withName("exchangeName"),                                      // offset 90, size 1  — C++: unsigned char m_exchangeName
        MemoryLayout.paddingLayout(5).withName("_pad0")                                      // offset 91, size 5  — C++: struct tail padding
    ).withName("MDHeaderPart");
    // total: 96 bytes

    /** C++: m_exchTS (offset 0) */
    public static final VarHandle MDH_EXCH_TS_VH = MD_HEADER_LAYOUT.varHandle(
        PathElement.groupElement("exchTS"));
    /** C++: m_timestamp (offset 8) */
    public static final VarHandle MDH_TIMESTAMP_VH = MD_HEADER_LAYOUT.varHandle(
        PathElement.groupElement("timestamp"));
    /** C++: m_seqnum (offset 16) */
    public static final VarHandle MDH_SEQNUM_VH = MD_HEADER_LAYOUT.varHandle(
        PathElement.groupElement("seqnum"));
    /** C++: m_rptseqnum (offset 24) */
    public static final VarHandle MDH_RPT_SEQNUM_VH = MD_HEADER_LAYOUT.varHandle(
        PathElement.groupElement("rptSeqnum"));
    /** C++: m_tokenId (offset 32) */
    public static final VarHandle MDH_TOKEN_ID_VH = MD_HEADER_LAYOUT.varHandle(
        PathElement.groupElement("tokenId"));
    /** C++: m_symbolID (offset 88) */
    public static final VarHandle MDH_SYMBOL_ID_VH = MD_HEADER_LAYOUT.varHandle(
        PathElement.groupElement("symbolID"));
    /** C++: m_exchangeName (offset 90) */
    public static final VarHandle MDH_EXCHANGE_NAME_VH = MD_HEADER_LAYOUT.varHandle(
        PathElement.groupElement("exchangeName"));

    // MDHeaderPart 字段 offset 常量
    public static final long MDH_EXCH_TS_OFFSET = 0;
    public static final long MDH_TIMESTAMP_OFFSET = 8;
    public static final long MDH_SEQNUM_OFFSET = 16;
    public static final long MDH_RPT_SEQNUM_OFFSET = 24;
    public static final long MDH_TOKEN_ID_OFFSET = 32;
    public static final long MDH_SYMBOL_OFFSET = 40;         // char[48], 需用 MemorySegment.asSlice 读取
    public static final long MDH_SYMBOL_SIZE = 48;            // MAX_SYMBOL_SIZE - 2
    public static final long MDH_SYMBOL_ID_OFFSET = 88;
    public static final long MDH_EXCHANGE_NAME_OFFSET = 90;

    // =====================================================================
    // 4. MDDataPart (720 bytes)
    // C++: struct MDDataPart (marketupdateNew.h:231-475)
    // Layout:
    //   double m_newPrice                offset 0,   size 8
    //   double m_oldPrice                offset 8,   size 8
    //   double m_lastTradedPrice         offset 16,  size 8
    //   uint64_t m_lastTradedTime        offset 24,  size 8
    //   double m_totalTradedValue        offset 32,  size 8
    //   int64_t m_totalTradedQuantity    offset 40,  size 8
    //   double m_yield                   offset 48,  size 8
    //   bookElement_t m_bidUpdates[20]   offset 56,  size 320
    //   bookElement_t m_askUpdates[20]   offset 376, size 320
    //   int32_t m_newQuant               offset 696, size 4
    //   int32_t m_oldQuant               offset 700, size 4
    //   int32_t m_lastTradedQuantity     offset 704, size 4
    //   int8_t m_validBids               offset 708, size 1
    //   int8_t m_validAsks               offset 709, size 1
    //   int8_t m_updateLevel             offset 710, size 1
    //   uint8_t m_endPkt                 offset 711, size 1
    //   unsigned char m_side             offset 712, size 1
    //   unsigned char m_updateType       offset 713, size 1
    //   unsigned char m_feedType         offset 714, size 1
    //   [5 bytes padding]               offset 715, size 5  — C++: struct tail padding to align to 8
    // total: 720 bytes
    // =====================================================================
    public static final StructLayout MD_DATA_LAYOUT = MemoryLayout.structLayout(
        ValueLayout.JAVA_DOUBLE.withName("newPrice"),                                        // offset 0,   size 8  — C++: double m_newPrice
        ValueLayout.JAVA_DOUBLE.withName("oldPrice"),                                        // offset 8,   size 8  — C++: double m_oldPrice
        ValueLayout.JAVA_DOUBLE.withName("lastTradedPrice"),                                 // offset 16,  size 8  — C++: double m_lastTradedPrice
        ValueLayout.JAVA_LONG.withName("lastTradedTime"),                                    // offset 24,  size 8  — C++: uint64_t m_lastTradedTime
        ValueLayout.JAVA_DOUBLE.withName("totalTradedValue"),                                // offset 32,  size 8  — C++: double m_totalTradedValue
        ValueLayout.JAVA_LONG.withName("totalTradedQuantity"),                               // offset 40,  size 8  — C++: int64_t m_totalTradedQuantity
        ValueLayout.JAVA_DOUBLE.withName("yield"),                                           // offset 48,  size 8  — C++: double m_yield
        MemoryLayout.sequenceLayout(20, BOOK_ELEMENT_LAYOUT).withName("bidUpdates"),         // offset 56,  size 320 — C++: bookElement_t m_bidUpdates[20]
        MemoryLayout.sequenceLayout(20, BOOK_ELEMENT_LAYOUT).withName("askUpdates"),         // offset 376, size 320 — C++: bookElement_t m_askUpdates[20]
        ValueLayout.JAVA_INT.withName("newQuant"),                                           // offset 696, size 4  — C++: int32_t m_newQuant
        ValueLayout.JAVA_INT.withName("oldQuant"),                                           // offset 700, size 4  — C++: int32_t m_oldQuant
        ValueLayout.JAVA_INT.withName("lastTradedQuantity"),                                 // offset 704, size 4  — C++: int32_t m_lastTradedQuantity
        ValueLayout.JAVA_BYTE.withName("validBids"),                                         // offset 708, size 1  — C++: int8_t m_validBids
        ValueLayout.JAVA_BYTE.withName("validAsks"),                                         // offset 709, size 1  — C++: int8_t m_validAsks
        ValueLayout.JAVA_BYTE.withName("updateLevel"),                                       // offset 710, size 1  — C++: int8_t m_updateLevel
        ValueLayout.JAVA_BYTE.withName("endPkt"),                                            // offset 711, size 1  — C++: uint8_t m_endPkt
        ValueLayout.JAVA_BYTE.withName("side"),                                              // offset 712, size 1  — C++: unsigned char m_side
        ValueLayout.JAVA_BYTE.withName("updateType"),                                        // offset 713, size 1  — C++: unsigned char m_updateType
        ValueLayout.JAVA_BYTE.withName("feedType"),                                          // offset 714, size 1  — C++: unsigned char m_feedType
        MemoryLayout.paddingLayout(5).withName("_pad0")                                      // offset 715, size 5  — C++: struct tail padding
    ).withName("MDDataPart");
    // total: 720 bytes

    /** C++: m_newPrice (offset 0) */
    public static final VarHandle MDD_NEW_PRICE_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("newPrice"));
    /** C++: m_oldPrice (offset 8) */
    public static final VarHandle MDD_OLD_PRICE_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("oldPrice"));
    /** C++: m_lastTradedPrice (offset 16) */
    public static final VarHandle MDD_LAST_TRADED_PRICE_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("lastTradedPrice"));
    /** C++: m_lastTradedTime (offset 24) */
    public static final VarHandle MDD_LAST_TRADED_TIME_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("lastTradedTime"));
    /** C++: m_totalTradedValue (offset 32) */
    public static final VarHandle MDD_TOTAL_TRADED_VALUE_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("totalTradedValue"));
    /** C++: m_totalTradedQuantity (offset 40) */
    public static final VarHandle MDD_TOTAL_TRADED_QTY_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("totalTradedQuantity"));
    /** C++: m_yield (offset 48) */
    public static final VarHandle MDD_YIELD_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("yield"));
    /** C++: m_newQuant (offset 696) */
    public static final VarHandle MDD_NEW_QUANT_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("newQuant"));
    /** C++: m_oldQuant (offset 700) */
    public static final VarHandle MDD_OLD_QUANT_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("oldQuant"));
    /** C++: m_lastTradedQuantity (offset 704) */
    public static final VarHandle MDD_LAST_TRADED_QTY_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("lastTradedQuantity"));
    /** C++: m_validBids (offset 708) */
    public static final VarHandle MDD_VALID_BIDS_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("validBids"));
    /** C++: m_validAsks (offset 709) */
    public static final VarHandle MDD_VALID_ASKS_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("validAsks"));
    /** C++: m_updateLevel (offset 710) */
    public static final VarHandle MDD_UPDATE_LEVEL_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("updateLevel"));
    /** C++: m_endPkt (offset 711) */
    public static final VarHandle MDD_END_PKT_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("endPkt"));
    /** C++: m_side (offset 712) */
    public static final VarHandle MDD_SIDE_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("side"));
    /** C++: m_updateType (offset 713) */
    public static final VarHandle MDD_UPDATE_TYPE_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("updateType"));
    /** C++: m_feedType (offset 714) */
    public static final VarHandle MDD_FEED_TYPE_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("feedType"));

    // BookElement 数组访问 VarHandle (bidUpdates/askUpdates)
    // 使用路径: groupElement("bidUpdates") -> sequenceElement(i) -> groupElement("price")
    /** C++: m_bidUpdates[i].quantity */
    public static final VarHandle MDD_BID_QUANTITY_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("bidUpdates"),
        PathElement.sequenceElement(),
        PathElement.groupElement("quantity"));
    /** C++: m_bidUpdates[i].orderCount */
    public static final VarHandle MDD_BID_ORDER_COUNT_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("bidUpdates"),
        PathElement.sequenceElement(),
        PathElement.groupElement("orderCount"));
    /** C++: m_bidUpdates[i].price */
    public static final VarHandle MDD_BID_PRICE_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("bidUpdates"),
        PathElement.sequenceElement(),
        PathElement.groupElement("price"));
    /** C++: m_askUpdates[i].quantity */
    public static final VarHandle MDD_ASK_QUANTITY_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("askUpdates"),
        PathElement.sequenceElement(),
        PathElement.groupElement("quantity"));
    /** C++: m_askUpdates[i].orderCount */
    public static final VarHandle MDD_ASK_ORDER_COUNT_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("askUpdates"),
        PathElement.sequenceElement(),
        PathElement.groupElement("orderCount"));
    /** C++: m_askUpdates[i].price */
    public static final VarHandle MDD_ASK_PRICE_VH = MD_DATA_LAYOUT.varHandle(
        PathElement.groupElement("askUpdates"),
        PathElement.sequenceElement(),
        PathElement.groupElement("price"));

    // MDDataPart 字段 offset 常量（用于手动偏移计算）
    public static final long MDD_NEW_PRICE_OFFSET = 0;
    public static final long MDD_OLD_PRICE_OFFSET = 8;
    public static final long MDD_LAST_TRADED_PRICE_OFFSET = 16;
    public static final long MDD_LAST_TRADED_TIME_OFFSET = 24;
    public static final long MDD_TOTAL_TRADED_VALUE_OFFSET = 32;
    public static final long MDD_TOTAL_TRADED_QTY_OFFSET = 40;
    public static final long MDD_YIELD_OFFSET = 48;
    public static final long MDD_BID_UPDATES_OFFSET = 56;     // bookElement_t[20], 320 bytes
    public static final long MDD_ASK_UPDATES_OFFSET = 376;    // bookElement_t[20], 320 bytes
    public static final long MDD_NEW_QUANT_OFFSET = 696;
    public static final long MDD_OLD_QUANT_OFFSET = 700;
    public static final long MDD_LAST_TRADED_QTY_OFFSET = 704;
    public static final long MDD_VALID_BIDS_OFFSET = 708;
    public static final long MDD_VALID_ASKS_OFFSET = 709;
    public static final long MDD_UPDATE_LEVEL_OFFSET = 710;
    public static final long MDD_END_PKT_OFFSET = 711;
    public static final long MDD_SIDE_OFFSET = 712;
    public static final long MDD_UPDATE_TYPE_OFFSET = 713;
    public static final long MDD_FEED_TYPE_OFFSET = 714;

    // =====================================================================
    // 5. MarketUpdateNew (816 bytes)
    // C++: struct MarketUpdateNew : public MDHeaderPart, MDDataPart
    //      (marketupdateNew.h:477-501)
    // Layout: MDHeaderPart(96) + MDDataPart(720) = 816 bytes
    // =====================================================================
    public static final StructLayout MARKET_UPDATE_NEW_LAYOUT = MemoryLayout.structLayout(
        MD_HEADER_LAYOUT.withName("header"),   // offset 0,  size 96  — C++: MDHeaderPart
        MD_DATA_LAYOUT.withName("data")        // offset 96, size 720 — C++: MDDataPart
    ).withName("MarketUpdateNew");
    // total: 816 bytes

    // MarketUpdateNew 复合 offset 常量（header + data 内部字段的绝对 offset）
    public static final long MU_HEADER_OFFSET = 0;
    public static final long MU_DATA_OFFSET = 96;

    // =====================================================================
    // 6. RequestMsg (256 bytes, __attribute__((aligned(64))))
    // C++: struct RequestMsg __attribute__((aligned(64))) (orderresponse.h:134-295)
    // Layout (GCC x86-64):
    //   ContractDescription Contract_Description   offset 0,   size 96
    //   RequestType Request_Type                   offset 96,  size 4   — C++: enum (int32)
    //   OrderType OrdType                          offset 100, size 4
    //   OrderDuration Duration                     offset 104, size 4
    //   PriceType PxType                           offset 108, size 4
    //   PositionDirection PosDirection              offset 112, size 4
    //   uint32_t OrderID                           offset 116, size 4
    //   int32_t Token                              offset 120, size 4
    //   int32_t Quantity                           offset 124, size 4
    //   int32_t QuantityFilled                     offset 128, size 4
    //   int32_t DisclosedQnty                      offset 132, size 4
    //   double Price                               offset 136, size 8
    //   uint64_t TimeStamp                         offset 144, size 8
    //   char AccountID[11]                         offset 152, size 11  — MAX_ACCNTID_LEN + 1
    //   unsigned char Transaction_Type             offset 163, size 1
    //   unsigned char Exchange_Type                offset 164, size 1
    //   char padding[20]                           offset 165, size 20  — C++: explicit padding
    //   char Product[32]                           offset 185, size 32
    //   [3 bytes implicit padding]                 offset 217, size 3   — C++: align int to 4
    //   int StrategyID                             offset 220, size 4
    //   [32 bytes tail padding]                    offset 224, size 32  — C++: aligned(64) → 256
    // total: 256 bytes
    // =====================================================================
    public static final StructLayout REQUEST_MSG_LAYOUT = MemoryLayout.structLayout(
        CONTRACT_DESC_LAYOUT.withName("contractDesc"),                                        // offset 0,   size 96
        ValueLayout.JAVA_INT.withName("requestType"),                                        // offset 96,  size 4  — C++: RequestType Request_Type
        ValueLayout.JAVA_INT.withName("ordType"),                                            // offset 100, size 4  — C++: OrderType OrdType
        ValueLayout.JAVA_INT.withName("duration"),                                           // offset 104, size 4  — C++: OrderDuration Duration
        ValueLayout.JAVA_INT.withName("pxType"),                                             // offset 108, size 4  — C++: PriceType PxType
        ValueLayout.JAVA_INT.withName("posDirection"),                                       // offset 112, size 4  — C++: PositionDirection PosDirection
        ValueLayout.JAVA_INT.withName("orderID"),                                            // offset 116, size 4  — C++: uint32_t OrderID (Java int, unsigned semantics via Integer.toUnsignedLong)
        ValueLayout.JAVA_INT.withName("token"),                                              // offset 120, size 4  — C++: int32_t Token
        ValueLayout.JAVA_INT.withName("quantity"),                                           // offset 124, size 4  — C++: int32_t Quantity
        ValueLayout.JAVA_INT.withName("quantityFilled"),                                     // offset 128, size 4  — C++: int32_t QuantityFilled
        ValueLayout.JAVA_INT.withName("disclosedQnty"),                                      // offset 132, size 4  — C++: int32_t DisclosedQnty
        ValueLayout.JAVA_DOUBLE.withName("price"),                                           // offset 136, size 8  — C++: double Price
        ValueLayout.JAVA_LONG.withName("timeStamp"),                                         // offset 144, size 8  — C++: uint64_t TimeStamp
        MemoryLayout.sequenceLayout(11, ValueLayout.JAVA_BYTE).withName("accountID"),        // offset 152, size 11 — C++: char AccountID[MAX_ACCNTID_LEN+1]
        ValueLayout.JAVA_BYTE.withName("transactionType"),                                   // offset 163, size 1  — C++: unsigned char Transaction_Type
        ValueLayout.JAVA_BYTE.withName("exchangeType"),                                      // offset 164, size 1  — C++: unsigned char Exchange_Type
        MemoryLayout.sequenceLayout(20, ValueLayout.JAVA_BYTE).withName("padding"),          // offset 165, size 20 — C++: char padding[20]
        MemoryLayout.sequenceLayout(32, ValueLayout.JAVA_BYTE).withName("product"),          // offset 185, size 32 — C++: char Product[MAX_PRODUCT_SIZE]
        MemoryLayout.paddingLayout(3).withName("_pad1"),                                     // offset 217, size 3  — C++: implicit padding for int alignment
        ValueLayout.JAVA_INT.withName("strategyID"),                                         // offset 220, size 4  — C++: int StrategyID
        MemoryLayout.paddingLayout(32).withName("_pad2")                                     // offset 224, size 32 — C++: aligned(64) tail padding → 256
    ).withName("RequestMsg");
    // total: 256 bytes

    /** C++: Request_Type (offset 96) */
    public static final VarHandle REQ_REQUEST_TYPE_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("requestType"));
    /** C++: OrdType (offset 100) */
    public static final VarHandle REQ_ORD_TYPE_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("ordType"));
    /** C++: Duration (offset 104) */
    public static final VarHandle REQ_DURATION_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("duration"));
    /** C++: PxType (offset 108) */
    public static final VarHandle REQ_PX_TYPE_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("pxType"));
    /** C++: PosDirection (offset 112) */
    public static final VarHandle REQ_POS_DIRECTION_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("posDirection"));
    /** C++: OrderID (offset 116) — uint32_t, Java 中用 int 表示 */
    public static final VarHandle REQ_ORDER_ID_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("orderID"));
    /** C++: Token (offset 120) */
    public static final VarHandle REQ_TOKEN_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("token"));
    /** C++: Quantity (offset 124) */
    public static final VarHandle REQ_QUANTITY_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("quantity"));
    /** C++: QuantityFilled (offset 128) */
    public static final VarHandle REQ_QUANTITY_FILLED_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("quantityFilled"));
    /** C++: DisclosedQnty (offset 132) */
    public static final VarHandle REQ_DISCLOSED_QNTY_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("disclosedQnty"));
    /** C++: Price (offset 136) */
    public static final VarHandle REQ_PRICE_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("price"));
    /** C++: TimeStamp (offset 144) — uint64_t, Java 中用 long 表示 */
    public static final VarHandle REQ_TIMESTAMP_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("timeStamp"));
    /** C++: Transaction_Type (offset 163) */
    public static final VarHandle REQ_TRANSACTION_TYPE_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("transactionType"));
    /** C++: Exchange_Type (offset 164) */
    public static final VarHandle REQ_EXCHANGE_TYPE_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("exchangeType"));
    /** C++: StrategyID (offset 220) */
    public static final VarHandle REQ_STRATEGY_ID_VH = REQUEST_MSG_LAYOUT.varHandle(
        PathElement.groupElement("strategyID"));

    // RequestMsg 字段 offset 常量
    public static final long REQ_CONTRACT_DESC_OFFSET = 0;
    public static final long REQ_REQUEST_TYPE_OFFSET = 96;
    public static final long REQ_ORD_TYPE_OFFSET = 100;
    public static final long REQ_DURATION_OFFSET = 104;
    public static final long REQ_PX_TYPE_OFFSET = 108;
    public static final long REQ_POS_DIRECTION_OFFSET = 112;
    public static final long REQ_ORDER_ID_OFFSET = 116;
    public static final long REQ_TOKEN_OFFSET = 120;
    public static final long REQ_QUANTITY_OFFSET = 124;
    public static final long REQ_QUANTITY_FILLED_OFFSET = 128;
    public static final long REQ_DISCLOSED_QNTY_OFFSET = 132;
    public static final long REQ_PRICE_OFFSET = 136;
    public static final long REQ_TIMESTAMP_OFFSET = 144;
    public static final long REQ_ACCOUNT_ID_OFFSET = 152;     // char[11]
    public static final long REQ_TRANSACTION_TYPE_OFFSET = 163;
    public static final long REQ_EXCHANGE_TYPE_OFFSET = 164;
    public static final long REQ_PADDING_OFFSET = 165;         // char[20]
    public static final long REQ_PRODUCT_OFFSET = 185;         // char[32]
    public static final long REQ_STRATEGY_ID_OFFSET = 220;

    // =====================================================================
    // 7. ResponseMsg (176 bytes)
    // C++: struct ResponseMsg (orderresponse.h:436-561)
    // Layout (GCC x86-64):
    //   ResponseType Response_Type           offset 0,   size 4   — C++: enum (int32)
    //   SubResponseType Child_Response       offset 4,   size 4
    //   uint32_t OrderID                     offset 8,   size 4
    //   uint32_t ErrorCode                   offset 12,  size 4
    //   int32_t Quantity                     offset 16,  size 4
    //   [4 bytes padding]                    offset 20,  size 4   — C++: implicit padding for double alignment
    //   double Price                         offset 24,  size 8
    //   uint64_t TimeStamp                   offset 32,  size 8
    //   unsigned char Side                   offset 40,  size 1
    //   char Symbol[50]                      offset 41,  size 50
    //   char AccountID[11]                   offset 91,  size 11
    //   [2 bytes padding]                    offset 102, size 2   — C++: implicit padding for double alignment
    //   double ExchangeOrderId               offset 104, size 8
    //   char ExchangeTradeId[21]             offset 112, size 21
    //   OpenCloseType OpenClose              offset 133, size 1   — C++: enum class : char
    //   TsExchangeID ExchangeID              offset 134, size 1   — C++: enum class : char
    //   char Product[32]                     offset 135, size 32
    //   [1 byte padding]                     offset 167, size 1   — C++: implicit padding for int alignment
    //   int StrategyID                       offset 168, size 4
    //   [4 bytes tail padding]               offset 172, size 4   — C++: struct tail padding to align to 8
    // total: 176 bytes
    // =====================================================================
    public static final StructLayout RESPONSE_MSG_LAYOUT = MemoryLayout.structLayout(
        ValueLayout.JAVA_INT.withName("responseType"),                                       // offset 0,   size 4  — C++: ResponseType Response_Type
        ValueLayout.JAVA_INT.withName("childResponse"),                                      // offset 4,   size 4  — C++: SubResponseType Child_Response
        ValueLayout.JAVA_INT.withName("orderID"),                                            // offset 8,   size 4  — C++: uint32_t OrderID
        ValueLayout.JAVA_INT.withName("errorCode"),                                          // offset 12,  size 4  — C++: uint32_t ErrorCode
        ValueLayout.JAVA_INT.withName("quantity"),                                           // offset 16,  size 4  — C++: int32_t Quantity
        MemoryLayout.paddingLayout(4).withName("_pad0"),                                     // offset 20,  size 4  — C++: implicit padding for double
        ValueLayout.JAVA_DOUBLE.withName("price"),                                           // offset 24,  size 8  — C++: double Price
        ValueLayout.JAVA_LONG.withName("timeStamp"),                                         // offset 32,  size 8  — C++: uint64_t TimeStamp
        ValueLayout.JAVA_BYTE.withName("side"),                                              // offset 40,  size 1  — C++: unsigned char Side
        MemoryLayout.sequenceLayout(50, ValueLayout.JAVA_BYTE).withName("symbol"),           // offset 41,  size 50 — C++: char Symbol[MAX_SYMBOL_SIZE]
        MemoryLayout.sequenceLayout(11, ValueLayout.JAVA_BYTE).withName("accountID"),        // offset 91,  size 11 — C++: char AccountID[MAX_ACCNTID_LEN+1]
        MemoryLayout.paddingLayout(2).withName("_pad1"),                                     // offset 102, size 2  — C++: implicit padding for double
        ValueLayout.JAVA_DOUBLE.withName("exchangeOrderId"),                                 // offset 104, size 8  — C++: double ExchangeOrderId
        MemoryLayout.sequenceLayout(21, ValueLayout.JAVA_BYTE).withName("exchangeTradeId"),  // offset 112, size 21 — C++: char ExchangeTradeId[MAX_TRADE_ID_SIZE]
        ValueLayout.JAVA_BYTE.withName("openClose"),                                         // offset 133, size 1  — C++: OpenCloseType OpenClose (enum class : char)
        ValueLayout.JAVA_BYTE.withName("exchangeID"),                                        // offset 134, size 1  — C++: TsExchangeID ExchangeID (enum class : char)
        MemoryLayout.sequenceLayout(32, ValueLayout.JAVA_BYTE).withName("product"),          // offset 135, size 32 — C++: char Product[MAX_PRODUCT_SIZE]
        MemoryLayout.paddingLayout(1).withName("_pad2"),                                     // offset 167, size 1  — C++: implicit padding for int
        ValueLayout.JAVA_INT.withName("strategyID"),                                         // offset 168, size 4  — C++: int StrategyID
        MemoryLayout.paddingLayout(4).withName("_pad3")                                      // offset 172, size 4  — C++: struct tail padding to align to 8
    ).withName("ResponseMsg");
    // total: 176 bytes

    /** C++: Response_Type (offset 0) */
    public static final VarHandle RESP_RESPONSE_TYPE_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("responseType"));
    /** C++: Child_Response (offset 4) */
    public static final VarHandle RESP_CHILD_RESPONSE_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("childResponse"));
    /** C++: OrderID (offset 8) — uint32_t, Java 中用 int 表示 */
    public static final VarHandle RESP_ORDER_ID_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("orderID"));
    /** C++: ErrorCode (offset 12) — uint32_t, Java 中用 int 表示 */
    public static final VarHandle RESP_ERROR_CODE_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("errorCode"));
    /** C++: Quantity (offset 16) */
    public static final VarHandle RESP_QUANTITY_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("quantity"));
    /** C++: Price (offset 24) */
    public static final VarHandle RESP_PRICE_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("price"));
    /** C++: TimeStamp (offset 32) — uint64_t, Java 中用 long 表示 */
    public static final VarHandle RESP_TIMESTAMP_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("timeStamp"));
    /** C++: Side (offset 40) */
    public static final VarHandle RESP_SIDE_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("side"));
    /** C++: ExchangeOrderId (offset 104) */
    public static final VarHandle RESP_EXCHANGE_ORDER_ID_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("exchangeOrderId"));
    /** C++: OpenClose (offset 133) — enum class : char */
    public static final VarHandle RESP_OPEN_CLOSE_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("openClose"));
    /** C++: ExchangeID (offset 134) — enum class : char */
    public static final VarHandle RESP_EXCHANGE_ID_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("exchangeID"));
    /** C++: StrategyID (offset 168) */
    public static final VarHandle RESP_STRATEGY_ID_VH = RESPONSE_MSG_LAYOUT.varHandle(
        PathElement.groupElement("strategyID"));

    // ResponseMsg 字段 offset 常量
    public static final long RESP_RESPONSE_TYPE_OFFSET = 0;
    public static final long RESP_CHILD_RESPONSE_OFFSET = 4;
    public static final long RESP_ORDER_ID_OFFSET = 8;
    public static final long RESP_ERROR_CODE_OFFSET = 12;
    public static final long RESP_QUANTITY_OFFSET = 16;
    public static final long RESP_PRICE_OFFSET = 24;
    public static final long RESP_TIMESTAMP_OFFSET = 32;
    public static final long RESP_SIDE_OFFSET = 40;
    public static final long RESP_SYMBOL_OFFSET = 41;           // char[50]
    public static final long RESP_ACCOUNT_ID_OFFSET = 91;       // char[11]
    public static final long RESP_EXCHANGE_ORDER_ID_OFFSET = 104;
    public static final long RESP_EXCHANGE_TRADE_ID_OFFSET = 112; // char[21]
    public static final long RESP_OPEN_CLOSE_OFFSET = 133;
    public static final long RESP_EXCHANGE_ID_OFFSET = 134;
    public static final long RESP_PRODUCT_OFFSET = 135;          // char[32]
    public static final long RESP_STRATEGY_ID_OFFSET = 168;

    // =====================================================================
    // QueueElem 大小常量 (MWMR SHM 队列元素大小)
    // C++: template<T> struct QueueElem { T data; uint64_t seqNo; }
    //      实际大小受 T 的 alignment 属性影响
    // =====================================================================

    /** C++: sizeof(QueueElem<MarketUpdateNew>) = 816 + 8(seqNo) = 824 bytes */
    public static final long QUEUE_ELEM_MD_SIZE = 824;

    /**
     * C++: sizeof(QueueElem<RequestMsg>) = 320 bytes.
     * RequestMsg has __attribute__((aligned(64))), 所以 QueueElem<RequestMsg>
     * 从 264 (=256+8) 被 padding 到 320 (=5*64) bytes.
     * 注意: seqNo 在 offset 256 (sizeof(RequestMsg)), 剩余 56 bytes 是尾部 padding.
     */
    public static final long QUEUE_ELEM_REQ_SIZE = 320;

    /** C++: sizeof(QueueElem<ResponseMsg>) = 176 + 8(seqNo) = 184 bytes */
    public static final long QUEUE_ELEM_RESP_SIZE = 184;

    /** C++: sizeof(MultiWriterMultiReaderShmHeader) = 8 bytes — atomic<int64_t> head */
    public static final long MWMR_HEADER_SIZE = 8;

    /**
     * C++: sizeof(LocklessShmClientStore::ClientData) = 16 bytes.
     * Layout: atomic<uint64_t> data (8) + uint64_t firstClientId (8)
     */
    public static final long CLIENT_DATA_SIZE = 16;

    // QueueElem 内部 seqNo 偏移
    /** QueueElem<MarketUpdateNew>.seqNo offset = sizeof(MarketUpdateNew) = 816 */
    public static final long QUEUE_ELEM_MD_SEQNO_OFFSET = 816;
    /** QueueElem<RequestMsg>.seqNo offset = sizeof(RequestMsg) = 256 */
    public static final long QUEUE_ELEM_REQ_SEQNO_OFFSET = 256;
    /** QueueElem<ResponseMsg>.seqNo offset = sizeof(ResponseMsg) = 176 */
    public static final long QUEUE_ELEM_RESP_SEQNO_OFFSET = 176;

    // =====================================================================
    // 结构体总大小常量 (便于断言和分配)
    // =====================================================================
    public static final long BOOK_ELEMENT_SIZE = BOOK_ELEMENT_LAYOUT.byteSize();          // 16
    public static final long CONTRACT_DESC_SIZE = CONTRACT_DESC_LAYOUT.byteSize();        // 96
    public static final long MD_HEADER_SIZE = MD_HEADER_LAYOUT.byteSize();                // 96
    public static final long MD_DATA_SIZE = MD_DATA_LAYOUT.byteSize();                    // 720
    public static final long MARKET_UPDATE_NEW_SIZE = MARKET_UPDATE_NEW_LAYOUT.byteSize();// 816
    public static final long REQUEST_MSG_SIZE = REQUEST_MSG_LAYOUT.byteSize();            // 256
    public static final long RESPONSE_MSG_SIZE = RESPONSE_MSG_LAYOUT.byteSize();          // 176
}
