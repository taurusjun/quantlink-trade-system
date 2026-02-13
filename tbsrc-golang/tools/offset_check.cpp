// offset_check.cpp — 输出 C++ 结构体 sizeof + offsetof 基准值
// 编译: g++ -std=c++11 -I<hftbase>/CommonUtils/include -o offset_check offset_check.cpp
//
// 输出格式与 Go cmd/offset_check/main.go 完全一致，
// 通过 diff 验证 Go 结构体的二进制兼容性。

#include <cstdio>
#include <cstddef>
#include "marketupdateNew.h"
#include "orderresponse.h"

using namespace illuminati::md;
using namespace illuminati::infra;

#define PRINT_SIZE(T) printf("sizeof(%s) = %zu\n", #T, sizeof(T))
#define PRINT_OFFSET(T, F) printf("offsetof(%s, %s) = %zu\n", #T, #F, offsetof(T, F))

int main() {
    // BookElement (bookElement_t)
    PRINT_SIZE(bookElement_t);
    PRINT_OFFSET(bookElement_t, quantity);
    PRINT_OFFSET(bookElement_t, orderCount);
    PRINT_OFFSET(bookElement_t, price);

    // ContractDescription
    PRINT_SIZE(ContractDescription);
    PRINT_OFFSET(ContractDescription, InstrumentName);
    PRINT_OFFSET(ContractDescription, Symbol);
    PRINT_OFFSET(ContractDescription, ExpiryDate);
    PRINT_OFFSET(ContractDescription, StrikePrice);
    PRINT_OFFSET(ContractDescription, OptionType);
    PRINT_OFFSET(ContractDescription, CALevel);

    // RequestMsg
    PRINT_SIZE(RequestMsg);
    PRINT_OFFSET(RequestMsg, Contract_Description);
    PRINT_OFFSET(RequestMsg, Request_Type);
    PRINT_OFFSET(RequestMsg, OrdType);
    PRINT_OFFSET(RequestMsg, Duration);
    PRINT_OFFSET(RequestMsg, PxType);
    PRINT_OFFSET(RequestMsg, PosDirection);
    PRINT_OFFSET(RequestMsg, OrderID);
    PRINT_OFFSET(RequestMsg, Token);
    PRINT_OFFSET(RequestMsg, Quantity);
    PRINT_OFFSET(RequestMsg, QuantityFilled);
    PRINT_OFFSET(RequestMsg, DisclosedQnty);
    PRINT_OFFSET(RequestMsg, Price);
    PRINT_OFFSET(RequestMsg, TimeStamp);
    PRINT_OFFSET(RequestMsg, AccountID);
    PRINT_OFFSET(RequestMsg, Transaction_Type);
    PRINT_OFFSET(RequestMsg, Exchange_Type);
    PRINT_OFFSET(RequestMsg, padding);
    PRINT_OFFSET(RequestMsg, Product);
    PRINT_OFFSET(RequestMsg, StrategyID);

    // ResponseMsg
    PRINT_SIZE(ResponseMsg);
    PRINT_OFFSET(ResponseMsg, Response_Type);
    PRINT_OFFSET(ResponseMsg, Child_Response);
    PRINT_OFFSET(ResponseMsg, OrderID);
    PRINT_OFFSET(ResponseMsg, ErrorCode);
    PRINT_OFFSET(ResponseMsg, Quantity);
    PRINT_OFFSET(ResponseMsg, Price);
    PRINT_OFFSET(ResponseMsg, TimeStamp);
    PRINT_OFFSET(ResponseMsg, Side);
    PRINT_OFFSET(ResponseMsg, Symbol);
    PRINT_OFFSET(ResponseMsg, AccountID);
    PRINT_OFFSET(ResponseMsg, ExchangeOrderId);
    PRINT_OFFSET(ResponseMsg, ExchangeTradeId);
    PRINT_OFFSET(ResponseMsg, OpenClose);
    PRINT_OFFSET(ResponseMsg, ExchangeID);
    PRINT_OFFSET(ResponseMsg, Product);
    PRINT_OFFSET(ResponseMsg, StrategyID);

    // MDHeaderPart
    PRINT_SIZE(MDHeaderPart);
    PRINT_OFFSET(MDHeaderPart, m_exchTS);
    PRINT_OFFSET(MDHeaderPart, m_timestamp);
    PRINT_OFFSET(MDHeaderPart, m_seqnum);
    PRINT_OFFSET(MDHeaderPart, m_rptseqnum);
    PRINT_OFFSET(MDHeaderPart, m_tokenId);
    PRINT_OFFSET(MDHeaderPart, m_symbol);
    PRINT_OFFSET(MDHeaderPart, m_symbolID);
    PRINT_OFFSET(MDHeaderPart, m_exchangeName);

    // MDDataPart
    PRINT_SIZE(MDDataPart);
    PRINT_OFFSET(MDDataPart, m_newPrice);
    PRINT_OFFSET(MDDataPart, m_oldPrice);
    PRINT_OFFSET(MDDataPart, m_lastTradedPrice);
    PRINT_OFFSET(MDDataPart, m_lastTradedTime);
    PRINT_OFFSET(MDDataPart, m_totalTradedValue);
    PRINT_OFFSET(MDDataPart, m_totalTradedQuantity);
    PRINT_OFFSET(MDDataPart, m_yield);
    PRINT_OFFSET(MDDataPart, m_bidUpdates);
    PRINT_OFFSET(MDDataPart, m_askUpdates);
    PRINT_OFFSET(MDDataPart, m_newQuant);
    PRINT_OFFSET(MDDataPart, m_oldQuant);
    PRINT_OFFSET(MDDataPart, m_lastTradedQuantity);
    PRINT_OFFSET(MDDataPart, m_validBids);
    PRINT_OFFSET(MDDataPart, m_validAsks);
    PRINT_OFFSET(MDDataPart, m_updateLevel);
    PRINT_OFFSET(MDDataPart, m_endPkt);
    PRINT_OFFSET(MDDataPart, m_side);
    PRINT_OFFSET(MDDataPart, m_updateType);
    PRINT_OFFSET(MDDataPart, m_feedType);

    // MarketUpdateNew
    PRINT_SIZE(MarketUpdateNew);

    // QueueElem sizes
    printf("sizeof(QueueElem<MarketUpdateNew>) = %zu\n", sizeof(QueueElem<MarketUpdateNew>));
    printf("sizeof(QueueElem<RequestMsg>) = %zu\n", sizeof(QueueElem<RequestMsg>));
    printf("sizeof(QueueElem<ResponseMsg>) = %zu\n", sizeof(QueueElem<ResponseMsg>));

    // MWMRHeader
    printf("sizeof(MultiWriterMultiReaderShmHeader) = %zu\n", sizeof(MultiWriterMultiReaderShmHeader));

    return 0;
}
