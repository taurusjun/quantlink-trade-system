package com.quantlink.trader.shm;

import org.junit.jupiter.api.Test;

import java.lang.foreign.Arena;
import java.lang.foreign.MemoryLayout;
import java.lang.foreign.MemoryLayout.PathElement;
import java.lang.foreign.MemorySegment;
import java.lang.foreign.StructLayout;
import java.lang.foreign.ValueLayout;
import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.*;

/**
 * JUnit 5 测试，验证所有 StructLayout 的总大小和字段 offset 与 C++ (GCC x86-64) 完全一致。
 * 测试数据来源: Go tbsrc-golang/pkg/shm/types_test.go 中已验证的 sizeof/offsetof 值。
 *
 * C++ 原代码:
 *   hftbase/CommonUtils/include/marketupdateNew.h
 *   hftbase/CommonUtils/include/orderresponse.h
 */
class TypesTest {

    // =====================================================================
    // 结构体总大小验证
    // 数据来源: Go types_test.go assertSize 调用
    // =====================================================================

    @Test
    void test_BookElement_size() {
        // C++: sizeof(bookElement_t) = 16
        // Go: assertSize(t, "BookElement", unsafe.Sizeof(BookElement{}), 16)
        assertEquals(16, Types.BOOK_ELEMENT_LAYOUT.byteSize(),
            "BookElement layout size must be 16 bytes");
        assertEquals(16, Types.BOOK_ELEMENT_SIZE);
    }

    @Test
    void test_ContractDescription_size() {
        // C++: sizeof(ContractDescription) = 96
        // Go: assertSize(t, "ContractDescription", unsafe.Sizeof(ContractDescription{}), 96)
        assertEquals(96, Types.CONTRACT_DESC_LAYOUT.byteSize(),
            "ContractDescription layout size must be 96 bytes");
        assertEquals(96, Types.CONTRACT_DESC_SIZE);
    }

    @Test
    void test_MDHeaderPart_size() {
        // C++: sizeof(MDHeaderPart) = 96
        // Go: assertSize(t, "MDHeaderPart", unsafe.Sizeof(MDHeaderPart{}), 96)
        assertEquals(96, Types.MD_HEADER_LAYOUT.byteSize(),
            "MDHeaderPart layout size must be 96 bytes");
        assertEquals(96, Types.MD_HEADER_SIZE);
    }

    @Test
    void test_MDDataPart_size() {
        // C++: sizeof(MDDataPart) = 720
        // Go: assertSize(t, "MDDataPart", unsafe.Sizeof(MDDataPart{}), 720)
        assertEquals(720, Types.MD_DATA_LAYOUT.byteSize(),
            "MDDataPart layout size must be 720 bytes");
        assertEquals(720, Types.MD_DATA_SIZE);
    }

    @Test
    void test_MarketUpdateNew_size() {
        // C++: sizeof(MarketUpdateNew) = 816
        // Go: assertSize(t, "MarketUpdateNew", unsafe.Sizeof(MarketUpdateNew{}), 816)
        assertEquals(816, Types.MARKET_UPDATE_NEW_LAYOUT.byteSize(),
            "MarketUpdateNew layout size must be 816 bytes");
        assertEquals(816, Types.MARKET_UPDATE_NEW_SIZE);
    }

    @Test
    void test_RequestMsg_size() {
        // C++: sizeof(RequestMsg) = 256 (__attribute__((aligned(64))))
        // Go: assertSize(t, "RequestMsg", unsafe.Sizeof(RequestMsg{}), 256)
        assertEquals(256, Types.REQUEST_MSG_LAYOUT.byteSize(),
            "RequestMsg layout size must be 256 bytes (aligned(64))");
        assertEquals(256, Types.REQUEST_MSG_SIZE);
    }

    @Test
    void test_ResponseMsg_size() {
        // C++: sizeof(ResponseMsg) = 176
        // Go: assertSize(t, "ResponseMsg", unsafe.Sizeof(ResponseMsg{}), 176)
        assertEquals(176, Types.RESPONSE_MSG_LAYOUT.byteSize(),
            "ResponseMsg layout size must be 176 bytes");
        assertEquals(176, Types.RESPONSE_MSG_SIZE);
    }

    // =====================================================================
    // QueueElem 大小常量验证
    // Go: TestQueueElements
    // =====================================================================

    @Test
    void test_QueueElem_sizes() {
        // Go: assertSize(t, "QueueElemMD", unsafe.Sizeof(QueueElemMD{}), 824)
        assertEquals(824, Types.QUEUE_ELEM_MD_SIZE);
        // Go: assertSize(t, "QueueElemReq", unsafe.Sizeof(QueueElemReq{}), 320)
        assertEquals(320, Types.QUEUE_ELEM_REQ_SIZE);
        // Go: assertSize(t, "QueueElemResp", unsafe.Sizeof(QueueElemResp{}), 184)
        assertEquals(184, Types.QUEUE_ELEM_RESP_SIZE);
    }

    @Test
    void test_QueueElem_seqNo_offsets() {
        // Go: assertOffset(t, "QueueElemMD.SeqNo", unsafe.Offsetof(qmd.SeqNo), 816)
        assertEquals(816, Types.QUEUE_ELEM_MD_SEQNO_OFFSET);
        // Go: assertOffset(t, "QueueElemReq.SeqNo", unsafe.Offsetof(qreq.SeqNo), 256)
        assertEquals(256, Types.QUEUE_ELEM_REQ_SEQNO_OFFSET);
        // Go: assertOffset(t, "QueueElemResp.SeqNo", unsafe.Offsetof(qresp.SeqNo), 176)
        assertEquals(176, Types.QUEUE_ELEM_RESP_SEQNO_OFFSET);
    }

    @Test
    void test_MWMRHeader_size() {
        // Go: assertSize(t, "MWMRHeader", unsafe.Sizeof(MWMRHeader{}), 8)
        assertEquals(8, Types.MWMR_HEADER_SIZE);
    }

    @Test
    void test_ClientData_size() {
        // Go: assertSize(t, "ClientData", unsafe.Sizeof(ClientData{}), 16)
        assertEquals(16, Types.CLIENT_DATA_SIZE);
    }

    // =====================================================================
    // BookElement 字段 offset 验证
    // Go: TestBookElement
    // =====================================================================

    @Test
    void test_BookElement_field_offsets() {
        // Go: assertOffset(t, "BookElement.Quantity", unsafe.Offsetof(be.Quantity), 0)
        assertFieldOffset(Types.BOOK_ELEMENT_LAYOUT, "quantity", 0);
        // Go: assertOffset(t, "BookElement.OrderCount", unsafe.Offsetof(be.OrderCount), 4)
        assertFieldOffset(Types.BOOK_ELEMENT_LAYOUT, "orderCount", 4);
        // Go: assertOffset(t, "BookElement.Price", unsafe.Offsetof(be.Price), 8)
        assertFieldOffset(Types.BOOK_ELEMENT_LAYOUT, "price", 8);
    }

    // =====================================================================
    // ContractDescription 字段 offset 验证
    // Go: TestContractDescription
    // =====================================================================

    @Test
    void test_ContractDescription_field_offsets() {
        // Go: assertOffset(t, "ContractDescription.InstrumentName", ..., 0)
        assertFieldOffset(Types.CONTRACT_DESC_LAYOUT, "instrumentName", 0);
        // Go: assertOffset(t, "ContractDescription.Symbol", ..., 32)
        assertFieldOffset(Types.CONTRACT_DESC_LAYOUT, "symbol", 32);
        // Go: assertOffset(t, "ContractDescription.ExpiryDate", ..., 84)
        assertFieldOffset(Types.CONTRACT_DESC_LAYOUT, "expiryDate", 84);
        // Go: assertOffset(t, "ContractDescription.StrikePrice", ..., 88)
        assertFieldOffset(Types.CONTRACT_DESC_LAYOUT, "strikePrice", 88);
        // Go: assertOffset(t, "ContractDescription.OptionType", ..., 92)
        assertFieldOffset(Types.CONTRACT_DESC_LAYOUT, "optionType", 92);
        // Go: assertOffset(t, "ContractDescription.CALevel", ..., 94)
        assertFieldOffset(Types.CONTRACT_DESC_LAYOUT, "caLevel", 94);
    }

    @Test
    void test_ContractDescription_offset_constants() {
        assertEquals(0, Types.CD_INSTRUMENT_NAME_OFFSET);
        assertEquals(32, Types.CD_SYMBOL_OFFSET);
        assertEquals(84, Types.CD_EXPIRY_DATE_OFFSET);
        assertEquals(88, Types.CD_STRIKE_PRICE_OFFSET);
        assertEquals(92, Types.CD_OPTION_TYPE_OFFSET);
        assertEquals(94, Types.CD_CA_LEVEL_OFFSET);
    }

    // =====================================================================
    // MDHeaderPart 字段 offset 验证
    // Go: TestMDHeaderPart
    // =====================================================================

    @Test
    void test_MDHeaderPart_field_offsets() {
        // Go: assertOffset(t, "MDHeaderPart.ExchTS", ..., 0)
        assertFieldOffset(Types.MD_HEADER_LAYOUT, "exchTS", 0);
        // Go: assertOffset(t, "MDHeaderPart.Timestamp", ..., 8)
        assertFieldOffset(Types.MD_HEADER_LAYOUT, "timestamp", 8);
        // Go: assertOffset(t, "MDHeaderPart.Seqnum", ..., 16)
        assertFieldOffset(Types.MD_HEADER_LAYOUT, "seqnum", 16);
        // Go: assertOffset(t, "MDHeaderPart.RptSeqnum", ..., 24)
        assertFieldOffset(Types.MD_HEADER_LAYOUT, "rptSeqnum", 24);
        // Go: assertOffset(t, "MDHeaderPart.TokenId", ..., 32)
        assertFieldOffset(Types.MD_HEADER_LAYOUT, "tokenId", 32);
        // Go: assertOffset(t, "MDHeaderPart.Symbol", ..., 40)
        assertFieldOffset(Types.MD_HEADER_LAYOUT, "symbol", 40);
        // Go: assertOffset(t, "MDHeaderPart.SymbolID", ..., 88)
        assertFieldOffset(Types.MD_HEADER_LAYOUT, "symbolID", 88);
        // Go: assertOffset(t, "MDHeaderPart.ExchangeName", ..., 90)
        assertFieldOffset(Types.MD_HEADER_LAYOUT, "exchangeName", 90);
    }

    @Test
    void test_MDHeaderPart_offset_constants() {
        assertEquals(0, Types.MDH_EXCH_TS_OFFSET);
        assertEquals(8, Types.MDH_TIMESTAMP_OFFSET);
        assertEquals(16, Types.MDH_SEQNUM_OFFSET);
        assertEquals(24, Types.MDH_RPT_SEQNUM_OFFSET);
        assertEquals(32, Types.MDH_TOKEN_ID_OFFSET);
        assertEquals(40, Types.MDH_SYMBOL_OFFSET);
        assertEquals(48, Types.MDH_SYMBOL_SIZE);
        assertEquals(88, Types.MDH_SYMBOL_ID_OFFSET);
        assertEquals(90, Types.MDH_EXCHANGE_NAME_OFFSET);
    }

    // =====================================================================
    // MDDataPart 字段 offset 验证
    // Go: TestMDDataPart
    // =====================================================================

    @Test
    void test_MDDataPart_field_offsets() {
        // Go: assertOffset(t, "MDDataPart.NewPrice", ..., 0)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "newPrice", 0);
        // Go: assertOffset(t, "MDDataPart.OldPrice", ..., 8)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "oldPrice", 8);
        // Go: assertOffset(t, "MDDataPart.LastTradedPrice", ..., 16)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "lastTradedPrice", 16);
        // Go: assertOffset(t, "MDDataPart.LastTradedTime", ..., 24)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "lastTradedTime", 24);
        // Go: assertOffset(t, "MDDataPart.TotalTradedValue", ..., 32)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "totalTradedValue", 32);
        // Go: assertOffset(t, "MDDataPart.TotalTradedQuantity", ..., 40)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "totalTradedQuantity", 40);
        // Go: assertOffset(t, "MDDataPart.Yield", ..., 48)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "yield", 48);
        // Go: assertOffset(t, "MDDataPart.BidUpdates", ..., 56)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "bidUpdates", 56);
        // Go: assertOffset(t, "MDDataPart.AskUpdates", ..., 376)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "askUpdates", 376);
        // Go: assertOffset(t, "MDDataPart.NewQuant", ..., 696)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "newQuant", 696);
        // Go: assertOffset(t, "MDDataPart.OldQuant", ..., 700)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "oldQuant", 700);
        // Go: assertOffset(t, "MDDataPart.LastTradedQuantity", ..., 704)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "lastTradedQuantity", 704);
        // Go: assertOffset(t, "MDDataPart.ValidBids", ..., 708)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "validBids", 708);
        // Go: assertOffset(t, "MDDataPart.ValidAsks", ..., 709)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "validAsks", 709);
        // Go: assertOffset(t, "MDDataPart.UpdateLevel", ..., 710)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "updateLevel", 710);
        // Go: assertOffset(t, "MDDataPart.EndPkt", ..., 711)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "endPkt", 711);
        // Go: assertOffset(t, "MDDataPart.Side", ..., 712)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "side", 712);
        // Go: assertOffset(t, "MDDataPart.UpdateType", ..., 713)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "updateType", 713);
        // Go: assertOffset(t, "MDDataPart.FeedType", ..., 714)
        assertFieldOffset(Types.MD_DATA_LAYOUT, "feedType", 714);
    }

    @Test
    void test_MDDataPart_offset_constants() {
        assertEquals(0, Types.MDD_NEW_PRICE_OFFSET);
        assertEquals(8, Types.MDD_OLD_PRICE_OFFSET);
        assertEquals(16, Types.MDD_LAST_TRADED_PRICE_OFFSET);
        assertEquals(24, Types.MDD_LAST_TRADED_TIME_OFFSET);
        assertEquals(32, Types.MDD_TOTAL_TRADED_VALUE_OFFSET);
        assertEquals(40, Types.MDD_TOTAL_TRADED_QTY_OFFSET);
        assertEquals(48, Types.MDD_YIELD_OFFSET);
        assertEquals(56, Types.MDD_BID_UPDATES_OFFSET);
        assertEquals(376, Types.MDD_ASK_UPDATES_OFFSET);
        assertEquals(696, Types.MDD_NEW_QUANT_OFFSET);
        assertEquals(700, Types.MDD_OLD_QUANT_OFFSET);
        assertEquals(704, Types.MDD_LAST_TRADED_QTY_OFFSET);
        assertEquals(708, Types.MDD_VALID_BIDS_OFFSET);
        assertEquals(709, Types.MDD_VALID_ASKS_OFFSET);
        assertEquals(710, Types.MDD_UPDATE_LEVEL_OFFSET);
        assertEquals(711, Types.MDD_END_PKT_OFFSET);
        assertEquals(712, Types.MDD_SIDE_OFFSET);
        assertEquals(713, Types.MDD_UPDATE_TYPE_OFFSET);
        assertEquals(714, Types.MDD_FEED_TYPE_OFFSET);
    }

    // =====================================================================
    // MarketUpdateNew 字段 offset 验证
    // Go: TestMarketUpdateNew
    // =====================================================================

    @Test
    void test_MarketUpdateNew_field_offsets() {
        // Go: assertOffset(t, "MarketUpdateNew.Header", ..., 0)
        assertFieldOffset(Types.MARKET_UPDATE_NEW_LAYOUT, "header", 0);
        // Go: assertOffset(t, "MarketUpdateNew.Data", ..., 96)
        assertFieldOffset(Types.MARKET_UPDATE_NEW_LAYOUT, "data", 96);
    }

    @Test
    void test_MarketUpdateNew_offset_constants() {
        assertEquals(0, Types.MU_HEADER_OFFSET);
        assertEquals(96, Types.MU_DATA_OFFSET);
    }

    // =====================================================================
    // RequestMsg 字段 offset 验证
    // Go: TestRequestMsg
    // =====================================================================

    @Test
    void test_RequestMsg_field_offsets() {
        // Go: assertOffset(t, "RequestMsg.ContractDesc", ..., 0)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "contractDesc", 0);
        // Go: assertOffset(t, "RequestMsg.Request_Type", ..., 96)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "requestType", 96);
        // Go: assertOffset(t, "RequestMsg.OrdType", ..., 100)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "ordType", 100);
        // Go: assertOffset(t, "RequestMsg.Duration", ..., 104)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "duration", 104);
        // Go: assertOffset(t, "RequestMsg.PxType", ..., 108)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "pxType", 108);
        // Go: assertOffset(t, "RequestMsg.PosDirection", ..., 112)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "posDirection", 112);
        // Go: assertOffset(t, "RequestMsg.OrderID", ..., 116)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "orderID", 116);
        // Go: assertOffset(t, "RequestMsg.Token", ..., 120)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "token", 120);
        // Go: assertOffset(t, "RequestMsg.Quantity", ..., 124)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "quantity", 124);
        // Go: assertOffset(t, "RequestMsg.QuantityFilled", ..., 128)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "quantityFilled", 128);
        // Go: assertOffset(t, "RequestMsg.DisclosedQnty", ..., 132)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "disclosedQnty", 132);
        // Go: assertOffset(t, "RequestMsg.Price", ..., 136)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "price", 136);
        // Go: assertOffset(t, "RequestMsg.TimeStamp", ..., 144)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "timeStamp", 144);
        // Go: assertOffset(t, "RequestMsg.AccountID", ..., 152)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "accountID", 152);
        // Go: assertOffset(t, "RequestMsg.TransactionType", ..., 163)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "transactionType", 163);
        // Go: assertOffset(t, "RequestMsg.ExchangeType", ..., 164)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "exchangeType", 164);
        // Go: assertOffset(t, "RequestMsg.Padding", ..., 165)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "padding", 165);
        // Go: assertOffset(t, "RequestMsg.Product", ..., 185)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "product", 185);
        // Go: assertOffset(t, "RequestMsg.StrategyID", ..., 220)
        assertFieldOffset(Types.REQUEST_MSG_LAYOUT, "strategyID", 220);
    }

    @Test
    void test_RequestMsg_offset_constants() {
        assertEquals(0, Types.REQ_CONTRACT_DESC_OFFSET);
        assertEquals(96, Types.REQ_REQUEST_TYPE_OFFSET);
        assertEquals(100, Types.REQ_ORD_TYPE_OFFSET);
        assertEquals(104, Types.REQ_DURATION_OFFSET);
        assertEquals(108, Types.REQ_PX_TYPE_OFFSET);
        assertEquals(112, Types.REQ_POS_DIRECTION_OFFSET);
        assertEquals(116, Types.REQ_ORDER_ID_OFFSET);
        assertEquals(120, Types.REQ_TOKEN_OFFSET);
        assertEquals(124, Types.REQ_QUANTITY_OFFSET);
        assertEquals(128, Types.REQ_QUANTITY_FILLED_OFFSET);
        assertEquals(132, Types.REQ_DISCLOSED_QNTY_OFFSET);
        assertEquals(136, Types.REQ_PRICE_OFFSET);
        assertEquals(144, Types.REQ_TIMESTAMP_OFFSET);
        assertEquals(152, Types.REQ_ACCOUNT_ID_OFFSET);
        assertEquals(163, Types.REQ_TRANSACTION_TYPE_OFFSET);
        assertEquals(164, Types.REQ_EXCHANGE_TYPE_OFFSET);
        assertEquals(165, Types.REQ_PADDING_OFFSET);
        assertEquals(185, Types.REQ_PRODUCT_OFFSET);
        assertEquals(220, Types.REQ_STRATEGY_ID_OFFSET);
    }

    // =====================================================================
    // ResponseMsg 字段 offset 验证
    // Go: TestResponseMsg
    // =====================================================================

    @Test
    void test_ResponseMsg_field_offsets() {
        // Go: assertOffset(t, "ResponseMsg.Response_Type", ..., 0)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "responseType", 0);
        // Go: assertOffset(t, "ResponseMsg.Child_Response", ..., 4)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "childResponse", 4);
        // Go: assertOffset(t, "ResponseMsg.OrderID", ..., 8)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "orderID", 8);
        // Go: assertOffset(t, "ResponseMsg.ErrorCode", ..., 12)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "errorCode", 12);
        // Go: assertOffset(t, "ResponseMsg.Quantity", ..., 16)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "quantity", 16);
        // Go: assertOffset(t, "ResponseMsg.Price", ..., 24)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "price", 24);
        // Go: assertOffset(t, "ResponseMsg.TimeStamp", ..., 32)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "timeStamp", 32);
        // Go: assertOffset(t, "ResponseMsg.Side", ..., 40)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "side", 40);
        // Go: assertOffset(t, "ResponseMsg.Symbol", ..., 41)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "symbol", 41);
        // Go: assertOffset(t, "ResponseMsg.AccountID", ..., 91)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "accountID", 91);
        // Go: assertOffset(t, "ResponseMsg.ExchangeOrderId", ..., 104)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "exchangeOrderId", 104);
        // Go: assertOffset(t, "ResponseMsg.ExchangeTradeId", ..., 112)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "exchangeTradeId", 112);
        // Go: assertOffset(t, "ResponseMsg.OpenClose", ..., 133)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "openClose", 133);
        // Go: assertOffset(t, "ResponseMsg.ExchangeID", ..., 134)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "exchangeID", 134);
        // Go: assertOffset(t, "ResponseMsg.Product", ..., 135)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "product", 135);
        // Go: assertOffset(t, "ResponseMsg.StrategyID", ..., 168)
        assertFieldOffset(Types.RESPONSE_MSG_LAYOUT, "strategyID", 168);
    }

    @Test
    void test_ResponseMsg_offset_constants() {
        assertEquals(0, Types.RESP_RESPONSE_TYPE_OFFSET);
        assertEquals(4, Types.RESP_CHILD_RESPONSE_OFFSET);
        assertEquals(8, Types.RESP_ORDER_ID_OFFSET);
        assertEquals(12, Types.RESP_ERROR_CODE_OFFSET);
        assertEquals(16, Types.RESP_QUANTITY_OFFSET);
        assertEquals(24, Types.RESP_PRICE_OFFSET);
        assertEquals(32, Types.RESP_TIMESTAMP_OFFSET);
        assertEquals(40, Types.RESP_SIDE_OFFSET);
        assertEquals(41, Types.RESP_SYMBOL_OFFSET);
        assertEquals(91, Types.RESP_ACCOUNT_ID_OFFSET);
        assertEquals(104, Types.RESP_EXCHANGE_ORDER_ID_OFFSET);
        assertEquals(112, Types.RESP_EXCHANGE_TRADE_ID_OFFSET);
        assertEquals(133, Types.RESP_OPEN_CLOSE_OFFSET);
        assertEquals(134, Types.RESP_EXCHANGE_ID_OFFSET);
        assertEquals(135, Types.RESP_PRODUCT_OFFSET);
        assertEquals(168, Types.RESP_STRATEGY_ID_OFFSET);
    }

    // =====================================================================
    // VarHandle 读写测试 — RequestMsg
    // =====================================================================

    @Test
    void test_RequestMsg_VarHandle_readWrite() {
        try (Arena arena = Arena.ofConfined()) {
            MemorySegment seg = arena.allocate(Types.REQUEST_MSG_LAYOUT);
            seg.fill((byte) 0);
            long base = 0L;

            // 写入字段
            Types.REQ_REQUEST_TYPE_VH.set(seg, base, Constants.REQUEST_NEWORDER);
            Types.REQ_ORD_TYPE_VH.set(seg, base, Constants.ORD_LIMIT);
            Types.REQ_DURATION_VH.set(seg, base, Constants.DUR_IOC);
            Types.REQ_PX_TYPE_VH.set(seg, base, Constants.PX_PERUNIT);
            Types.REQ_POS_DIRECTION_VH.set(seg, base, Constants.POS_OPEN);
            Types.REQ_ORDER_ID_VH.set(seg, base, 1000001);
            Types.REQ_TOKEN_VH.set(seg, base, 42);
            Types.REQ_QUANTITY_VH.set(seg, base, 10);
            Types.REQ_QUANTITY_FILLED_VH.set(seg, base, 0);
            Types.REQ_DISCLOSED_QNTY_VH.set(seg, base, 0);
            Types.REQ_PRICE_VH.set(seg, base, 5800.0);
            Types.REQ_TIMESTAMP_VH.set(seg, base, 1708891234567890L);
            Types.REQ_TRANSACTION_TYPE_VH.set(seg, base, Constants.SIDE_BUY);
            Types.REQ_EXCHANGE_TYPE_VH.set(seg, base, (byte) 0);
            Types.REQ_STRATEGY_ID_VH.set(seg, base, 92201);

            // 写入 Symbol 到 ContractDescription 内部
            byte[] symbolBytes = "ag2603".getBytes(StandardCharsets.US_ASCII);
            MemorySegment.copy(MemorySegment.ofArray(symbolBytes), 0, seg,
                Types.REQ_CONTRACT_DESC_OFFSET + Types.CD_SYMBOL_OFFSET, symbolBytes.length);

            // 读取验证
            assertEquals(Constants.REQUEST_NEWORDER, (int) Types.REQ_REQUEST_TYPE_VH.get(seg, base));
            assertEquals(Constants.ORD_LIMIT, (int) Types.REQ_ORD_TYPE_VH.get(seg, base));
            assertEquals(Constants.DUR_IOC, (int) Types.REQ_DURATION_VH.get(seg, base));
            assertEquals(Constants.PX_PERUNIT, (int) Types.REQ_PX_TYPE_VH.get(seg, base));
            assertEquals(Constants.POS_OPEN, (int) Types.REQ_POS_DIRECTION_VH.get(seg, base));
            assertEquals(1000001, (int) Types.REQ_ORDER_ID_VH.get(seg, base));
            assertEquals(42, (int) Types.REQ_TOKEN_VH.get(seg, base));
            assertEquals(10, (int) Types.REQ_QUANTITY_VH.get(seg, base));
            assertEquals(5800.0, (double) Types.REQ_PRICE_VH.get(seg, base), 1e-10);
            assertEquals(1708891234567890L, (long) Types.REQ_TIMESTAMP_VH.get(seg, base));
            assertEquals(Constants.SIDE_BUY, (byte) Types.REQ_TRANSACTION_TYPE_VH.get(seg, base));
            assertEquals(92201, (int) Types.REQ_STRATEGY_ID_VH.get(seg, base));
        }
    }

    // =====================================================================
    // VarHandle 读写测试 — ResponseMsg
    // =====================================================================

    @Test
    void test_ResponseMsg_VarHandle_readWrite() {
        try (Arena arena = Arena.ofConfined()) {
            MemorySegment seg = arena.allocate(Types.RESPONSE_MSG_LAYOUT);
            seg.fill((byte) 0);
            long base = 0L;

            // 写入字段
            Types.RESP_RESPONSE_TYPE_VH.set(seg, base, Constants.RESP_TRADE_CONFIRM);
            Types.RESP_CHILD_RESPONSE_VH.set(seg, base, Constants.SUB_NULL_RESPONSE_MIDDLE);
            Types.RESP_ORDER_ID_VH.set(seg, base, 1000001);
            Types.RESP_ERROR_CODE_VH.set(seg, base, 0);
            Types.RESP_QUANTITY_VH.set(seg, base, 5);
            Types.RESP_PRICE_VH.set(seg, base, 5810.0);
            Types.RESP_TIMESTAMP_VH.set(seg, base, 1708891234567999L);
            Types.RESP_SIDE_VH.set(seg, base, Constants.SIDE_BUY);
            Types.RESP_EXCHANGE_ORDER_ID_VH.set(seg, base, 123456789.0);
            Types.RESP_OPEN_CLOSE_VH.set(seg, base, Constants.OC_OPEN);
            Types.RESP_EXCHANGE_ID_VH.set(seg, base, Constants.TS_SHFE);
            Types.RESP_STRATEGY_ID_VH.set(seg, base, 92201);

            // 写入 Symbol
            byte[] symbolBytes = "ag2603".getBytes(StandardCharsets.US_ASCII);
            MemorySegment.copy(MemorySegment.ofArray(symbolBytes), 0, seg, Types.RESP_SYMBOL_OFFSET, symbolBytes.length);

            // 写入 Product
            byte[] productBytes = "ag".getBytes(StandardCharsets.US_ASCII);
            MemorySegment.copy(MemorySegment.ofArray(productBytes), 0, seg, Types.RESP_PRODUCT_OFFSET, productBytes.length);

            // 读取验证
            assertEquals(Constants.RESP_TRADE_CONFIRM, (int) Types.RESP_RESPONSE_TYPE_VH.get(seg, base));
            assertEquals(Constants.SUB_NULL_RESPONSE_MIDDLE, (int) Types.RESP_CHILD_RESPONSE_VH.get(seg, base));
            assertEquals(1000001, (int) Types.RESP_ORDER_ID_VH.get(seg, base));
            assertEquals(0, (int) Types.RESP_ERROR_CODE_VH.get(seg, base));
            assertEquals(5, (int) Types.RESP_QUANTITY_VH.get(seg, base));
            assertEquals(5810.0, (double) Types.RESP_PRICE_VH.get(seg, base), 1e-10);
            assertEquals(1708891234567999L, (long) Types.RESP_TIMESTAMP_VH.get(seg, base));
            assertEquals(Constants.SIDE_BUY, (byte) Types.RESP_SIDE_VH.get(seg, base));
            assertEquals(123456789.0, (double) Types.RESP_EXCHANGE_ORDER_ID_VH.get(seg, base), 1e-10);
            assertEquals(Constants.OC_OPEN, (byte) Types.RESP_OPEN_CLOSE_VH.get(seg, base));
            assertEquals(Constants.TS_SHFE, (byte) Types.RESP_EXCHANGE_ID_VH.get(seg, base));
            assertEquals(92201, (int) Types.RESP_STRATEGY_ID_VH.get(seg, base));
        }
    }

    // =====================================================================
    // VarHandle 读写测试 — MDHeaderPart
    // =====================================================================

    @Test
    void test_MDHeaderPart_VarHandle_readWrite() {
        try (Arena arena = Arena.ofConfined()) {
            MemorySegment seg = arena.allocate(Types.MD_HEADER_LAYOUT);
            seg.fill((byte) 0);
            long base = 0L;

            Types.MDH_EXCH_TS_VH.set(seg, base, 1708891200000000L);
            Types.MDH_TIMESTAMP_VH.set(seg, base, 1708891200001000L);
            Types.MDH_SEQNUM_VH.set(seg, base, 12345L);
            Types.MDH_RPT_SEQNUM_VH.set(seg, base, 67890L);
            Types.MDH_TOKEN_ID_VH.set(seg, base, 100L);
            Types.MDH_SYMBOL_ID_VH.set(seg, base, (short) 1);
            Types.MDH_EXCHANGE_NAME_VH.set(seg, base, Constants.CHINA_SHFE);

            // 写入 Symbol
            byte[] symbolBytes = "ag2603".getBytes(StandardCharsets.US_ASCII);
            MemorySegment.copy(MemorySegment.ofArray(symbolBytes), 0, seg, Types.MDH_SYMBOL_OFFSET, symbolBytes.length);

            assertEquals(1708891200000000L, (long) Types.MDH_EXCH_TS_VH.get(seg, base));
            assertEquals(1708891200001000L, (long) Types.MDH_TIMESTAMP_VH.get(seg, base));
            assertEquals(12345L, (long) Types.MDH_SEQNUM_VH.get(seg, base));
            assertEquals(67890L, (long) Types.MDH_RPT_SEQNUM_VH.get(seg, base));
            assertEquals(100L, (long) Types.MDH_TOKEN_ID_VH.get(seg, base));
            assertEquals((short) 1, (short) Types.MDH_SYMBOL_ID_VH.get(seg, base));
            assertEquals(Constants.CHINA_SHFE, (byte) Types.MDH_EXCHANGE_NAME_VH.get(seg, base));
        }
    }

    // =====================================================================
    // VarHandle 读写测试 — MDDataPart (含 BookElement 数组)
    // =====================================================================

    @Test
    void test_MDDataPart_VarHandle_readWrite() {
        try (Arena arena = Arena.ofConfined()) {
            MemorySegment seg = arena.allocate(Types.MD_DATA_LAYOUT);
            seg.fill((byte) 0);
            long base = 0L;

            // 标量字段
            Types.MDD_NEW_PRICE_VH.set(seg, base, 5800.0);
            Types.MDD_OLD_PRICE_VH.set(seg, base, 5799.0);
            Types.MDD_LAST_TRADED_PRICE_VH.set(seg, base, 5800.0);
            Types.MDD_LAST_TRADED_TIME_VH.set(seg, base, 1708891200000000L);
            Types.MDD_TOTAL_TRADED_VALUE_VH.set(seg, base, 1.5e9);
            Types.MDD_TOTAL_TRADED_QTY_VH.set(seg, base, 25000L);
            Types.MDD_YIELD_VH.set(seg, base, 0.0);
            Types.MDD_NEW_QUANT_VH.set(seg, base, 100);
            Types.MDD_OLD_QUANT_VH.set(seg, base, 90);
            Types.MDD_LAST_TRADED_QTY_VH.set(seg, base, 10);
            Types.MDD_VALID_BIDS_VH.set(seg, base, (byte) 5);
            Types.MDD_VALID_ASKS_VH.set(seg, base, (byte) 5);
            Types.MDD_UPDATE_LEVEL_VH.set(seg, base, (byte) 0);
            Types.MDD_END_PKT_VH.set(seg, base, (byte) 1);
            Types.MDD_SIDE_VH.set(seg, base, Constants.SIDE_BUY);
            Types.MDD_UPDATE_TYPE_VH.set(seg, base, Constants.MDUPDTYPE_ADD);
            Types.MDD_FEED_TYPE_VH.set(seg, base, Constants.FEED_TBT);

            // BookElement 数组访问 — bid[0]
            Types.MDD_BID_QUANTITY_VH.set(seg, base, 0L, 50);
            Types.MDD_BID_ORDER_COUNT_VH.set(seg, base, 0L, 3);
            Types.MDD_BID_PRICE_VH.set(seg, base, 0L, 5800.0);
            // BookElement 数组访问 — bid[1]
            Types.MDD_BID_QUANTITY_VH.set(seg, base, 1L, 30);
            Types.MDD_BID_ORDER_COUNT_VH.set(seg, base, 1L, 2);
            Types.MDD_BID_PRICE_VH.set(seg, base, 1L, 5799.0);
            // BookElement 数组访问 — ask[0]
            Types.MDD_ASK_QUANTITY_VH.set(seg, base, 0L, 40);
            Types.MDD_ASK_ORDER_COUNT_VH.set(seg, base, 0L, 4);
            Types.MDD_ASK_PRICE_VH.set(seg, base, 0L, 5801.0);

            // 验证标量字段
            assertEquals(5800.0, (double) Types.MDD_NEW_PRICE_VH.get(seg, base), 1e-10);
            assertEquals(5799.0, (double) Types.MDD_OLD_PRICE_VH.get(seg, base), 1e-10);
            assertEquals(5800.0, (double) Types.MDD_LAST_TRADED_PRICE_VH.get(seg, base), 1e-10);
            assertEquals(1708891200000000L, (long) Types.MDD_LAST_TRADED_TIME_VH.get(seg, base));
            assertEquals(1.5e9, (double) Types.MDD_TOTAL_TRADED_VALUE_VH.get(seg, base), 1e-10);
            assertEquals(25000L, (long) Types.MDD_TOTAL_TRADED_QTY_VH.get(seg, base));
            assertEquals(100, (int) Types.MDD_NEW_QUANT_VH.get(seg, base));
            assertEquals(90, (int) Types.MDD_OLD_QUANT_VH.get(seg, base));
            assertEquals(10, (int) Types.MDD_LAST_TRADED_QTY_VH.get(seg, base));
            assertEquals((byte) 5, (byte) Types.MDD_VALID_BIDS_VH.get(seg, base));
            assertEquals((byte) 5, (byte) Types.MDD_VALID_ASKS_VH.get(seg, base));
            assertEquals((byte) 0, (byte) Types.MDD_UPDATE_LEVEL_VH.get(seg, base));
            assertEquals((byte) 1, (byte) Types.MDD_END_PKT_VH.get(seg, base));
            assertEquals(Constants.SIDE_BUY, (byte) Types.MDD_SIDE_VH.get(seg, base));
            assertEquals(Constants.MDUPDTYPE_ADD, (byte) Types.MDD_UPDATE_TYPE_VH.get(seg, base));
            assertEquals(Constants.FEED_TBT, (byte) Types.MDD_FEED_TYPE_VH.get(seg, base));

            // 验证 BookElement 数组
            assertEquals(50, (int) Types.MDD_BID_QUANTITY_VH.get(seg, base, 0L));
            assertEquals(3, (int) Types.MDD_BID_ORDER_COUNT_VH.get(seg, base, 0L));
            assertEquals(5800.0, (double) Types.MDD_BID_PRICE_VH.get(seg, base, 0L), 1e-10);
            assertEquals(30, (int) Types.MDD_BID_QUANTITY_VH.get(seg, base, 1L));
            assertEquals(2, (int) Types.MDD_BID_ORDER_COUNT_VH.get(seg, base, 1L));
            assertEquals(5799.0, (double) Types.MDD_BID_PRICE_VH.get(seg, base, 1L), 1e-10);
            assertEquals(40, (int) Types.MDD_ASK_QUANTITY_VH.get(seg, base, 0L));
            assertEquals(4, (int) Types.MDD_ASK_ORDER_COUNT_VH.get(seg, base, 0L));
            assertEquals(5801.0, (double) Types.MDD_ASK_PRICE_VH.get(seg, base, 0L), 1e-10);
        }
    }

    // =====================================================================
    // 完整 MarketUpdateNew 端到端测试
    // =====================================================================

    @Test
    void test_MarketUpdateNew_endToEnd() {
        try (Arena arena = Arena.ofConfined()) {
            MemorySegment seg = arena.allocate(Types.MARKET_UPDATE_NEW_LAYOUT);
            seg.fill((byte) 0);

            // 写入 header 字段 (offset = MU_HEADER_OFFSET + field offset)
            long headerBase = Types.MU_HEADER_OFFSET;
            Types.MDH_EXCH_TS_VH.set(seg, headerBase, 1708891200000000L);
            Types.MDH_EXCHANGE_NAME_VH.set(seg, headerBase, Constants.CHINA_SHFE);

            byte[] symbolBytes = "ag2603".getBytes(StandardCharsets.US_ASCII);
            MemorySegment.copy(MemorySegment.ofArray(symbolBytes), 0, seg,
                headerBase + Types.MDH_SYMBOL_OFFSET, symbolBytes.length);

            // 写入 data 字段 (offset = MU_DATA_OFFSET + field offset)
            long dataBase = Types.MU_DATA_OFFSET;
            Types.MDD_LAST_TRADED_PRICE_VH.set(seg, dataBase, 5800.0);
            Types.MDD_VALID_BIDS_VH.set(seg, dataBase, (byte) 5);
            Types.MDD_VALID_ASKS_VH.set(seg, dataBase, (byte) 5);
            Types.MDD_BID_PRICE_VH.set(seg, dataBase, 0L, 5800.0);
            Types.MDD_ASK_PRICE_VH.set(seg, dataBase, 0L, 5801.0);

            // 读取验证
            assertEquals(1708891200000000L, (long) Types.MDH_EXCH_TS_VH.get(seg, headerBase));
            assertEquals(Constants.CHINA_SHFE, (byte) Types.MDH_EXCHANGE_NAME_VH.get(seg, headerBase));
            assertEquals(5800.0, (double) Types.MDD_LAST_TRADED_PRICE_VH.get(seg, dataBase), 1e-10);
            assertEquals(5800.0, (double) Types.MDD_BID_PRICE_VH.get(seg, dataBase, 0L), 1e-10);
            assertEquals(5801.0, (double) Types.MDD_ASK_PRICE_VH.get(seg, dataBase, 0L), 1e-10);
        }
    }

    // =====================================================================
    // Constants 枚举值验证
    // =====================================================================

    @Test
    void test_Constants_enumValues() {
        // RequestType — 验证与 C++ 一致
        assertEquals(0, Constants.REQUEST_NEWORDER);
        assertEquals(1, Constants.REQUEST_MODIFYORDER);
        assertEquals(2, Constants.REQUEST_CANCELORDER);
        assertEquals(7, Constants.REQUEST_OPTEXEC_CANCEL);

        // ResponseType — 验证关键值
        assertEquals(0, Constants.RESP_NEW_ORDER_CONFIRM);
        assertEquals(4, Constants.RESP_TRADE_CONFIRM);
        assertEquals(5, Constants.RESP_ORDER_ERROR);
        assertEquals(18, Constants.RESP_NULL_RESPONSE);

        // OrderType — 注意从 1 开始
        assertEquals(1, Constants.ORD_LIMIT);
        assertEquals(2, Constants.ORD_MARKET);
        assertEquals(5, Constants.ORD_BEST_PRICE);

        // PositionDirection — 注意从 10 开始
        assertEquals(10, Constants.POS_OPEN);
        assertEquals(11, Constants.POS_CLOSE);
        assertEquals(12, Constants.POS_CLOSE_INTRADAY);
        assertEquals(13, Constants.POS_ERROR);

        // OpenCloseType
        assertEquals(0, Constants.OC_NULL_TYPE);
        assertEquals(1, Constants.OC_OPEN);
        assertEquals(2, Constants.OC_CLOSE);
        assertEquals(3, Constants.OC_CLOSE_TODAY);

        // TsExchangeID
        assertEquals(0, Constants.TS_NULL_EXCHANGE);
        assertEquals(1, Constants.TS_SHFE);
        assertEquals(6, Constants.TS_GFEX);

        // 交易所代码
        assertEquals(57, Constants.CHINA_SHFE);
        assertEquals(58, Constants.CHINA_CFFEX);
        assertEquals(59, Constants.CHINA_ZCE);
        assertEquals(60, Constants.CHINA_DCE);
        assertEquals(61, Constants.CHINA_GFEX);

        // ORDERID_RANGE
        assertEquals(1_000_000, Constants.ORDERID_RANGE);
    }

    @Test
    void test_Constants_stringLookups() {
        assertEquals("NEWORDER", Constants.requestTypeStr(0));
        assertEquals("CANCELORDER", Constants.requestTypeStr(2));
        assertEquals("UNKNOWN", Constants.requestTypeStr(99));

        assertEquals("NEW_ORDER_CONFIRM", Constants.responseTypeStr(0));
        assertEquals("TRADE_CONFIRM", Constants.responseTypeStr(4));
        assertEquals("NULL_RESPONSE", Constants.responseTypeStr(18));
        assertEquals("UNKNOWN", Constants.responseTypeStr(-1));

        assertEquals("LIMIT", Constants.orderTypeStr(1));
        assertEquals("MARKET", Constants.orderTypeStr(2));
        assertEquals("UNKNOWN", Constants.orderTypeStr(99));

        assertEquals("DAY", Constants.orderDurationStr(0));
        assertEquals("FAK", Constants.orderDurationStr(4));
        assertEquals("UNKNOWN", Constants.orderDurationStr(99));

        assertEquals("OPEN", Constants.positionDirectionStr(10));
        assertEquals("CLOSE", Constants.positionDirectionStr(11));
        assertEquals("CLOSE_INTRADAY", Constants.positionDirectionStr(12));
    }

    @Test
    void test_Constants_sizeConstants() {
        assertEquals(20, Constants.INTEREST_LEVELS);
        assertEquals(50, Constants.MAX_SYMBOL_SIZE);
        assertEquals(10, Constants.MAX_ACCNT_ID_LEN);
        assertEquals(11, Constants.ACCOUNT_ID_SIZE);
        assertEquals(32, Constants.MAX_INSTR_NAME_SZ);
        assertEquals(21, Constants.MAX_TRADE_ID_SIZE);
        assertEquals(32, Constants.MAX_PRODUCT_SIZE);
        assertEquals(250, Constants.MAX_ORS_CLIENTS);
        assertEquals(72, Constants.MAX_EXCHANGE_COUNT);
    }

    // =====================================================================
    // 工具方法
    // =====================================================================

    /**
     * 断言 StructLayout 中指定字段的 byteOffset 与期望值一致。
     */
    private void assertFieldOffset(StructLayout layout, String fieldName, long expectedOffset) {
        long actualOffset = layout.byteOffset(PathElement.groupElement(fieldName));
        assertEquals(expectedOffset, actualOffset,
            String.format("offset of %s.%s: expected %d, got %d",
                layout.name().orElse("?"), fieldName, expectedOffset, actualOffset));
    }
}
