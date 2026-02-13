// offset_check Go 版本 — 输出与 C++ tools/offset_check 完全一致的格式
// 运行: go run cmd/offset_check/main.go
// 对比: diff <(./tools/offset_check) <(go run cmd/offset_check/main.go)
package main

import (
	"fmt"
	"unsafe"

	"tbsrc-golang/pkg/shm"
)

func main() {
	// BookElement (bookElement_t)
	var be shm.BookElement
	fmt.Printf("sizeof(bookElement_t) = %d\n", unsafe.Sizeof(be))
	fmt.Printf("offsetof(bookElement_t, quantity) = %d\n", unsafe.Offsetof(be.Quantity))
	fmt.Printf("offsetof(bookElement_t, orderCount) = %d\n", unsafe.Offsetof(be.OrderCount))
	fmt.Printf("offsetof(bookElement_t, price) = %d\n", unsafe.Offsetof(be.Price))

	// ContractDescription
	var cd shm.ContractDescription
	fmt.Printf("sizeof(ContractDescription) = %d\n", unsafe.Sizeof(cd))
	fmt.Printf("offsetof(ContractDescription, InstrumentName) = %d\n", unsafe.Offsetof(cd.InstrumentName))
	fmt.Printf("offsetof(ContractDescription, Symbol) = %d\n", unsafe.Offsetof(cd.Symbol))
	fmt.Printf("offsetof(ContractDescription, ExpiryDate) = %d\n", unsafe.Offsetof(cd.ExpiryDate))
	fmt.Printf("offsetof(ContractDescription, StrikePrice) = %d\n", unsafe.Offsetof(cd.StrikePrice))
	fmt.Printf("offsetof(ContractDescription, OptionType) = %d\n", unsafe.Offsetof(cd.OptionType))
	fmt.Printf("offsetof(ContractDescription, CALevel) = %d\n", unsafe.Offsetof(cd.CALevel))

	// RequestMsg
	var rm shm.RequestMsg
	fmt.Printf("sizeof(RequestMsg) = %d\n", unsafe.Sizeof(rm))
	fmt.Printf("offsetof(RequestMsg, Contract_Description) = %d\n", unsafe.Offsetof(rm.ContractDesc))
	fmt.Printf("offsetof(RequestMsg, Request_Type) = %d\n", unsafe.Offsetof(rm.Request_Type))
	fmt.Printf("offsetof(RequestMsg, OrdType) = %d\n", unsafe.Offsetof(rm.OrdType))
	fmt.Printf("offsetof(RequestMsg, Duration) = %d\n", unsafe.Offsetof(rm.Duration))
	fmt.Printf("offsetof(RequestMsg, PxType) = %d\n", unsafe.Offsetof(rm.PxType))
	fmt.Printf("offsetof(RequestMsg, PosDirection) = %d\n", unsafe.Offsetof(rm.PosDirection))
	fmt.Printf("offsetof(RequestMsg, OrderID) = %d\n", unsafe.Offsetof(rm.OrderID))
	fmt.Printf("offsetof(RequestMsg, Token) = %d\n", unsafe.Offsetof(rm.Token))
	fmt.Printf("offsetof(RequestMsg, Quantity) = %d\n", unsafe.Offsetof(rm.Quantity))
	fmt.Printf("offsetof(RequestMsg, QuantityFilled) = %d\n", unsafe.Offsetof(rm.QuantityFilled))
	fmt.Printf("offsetof(RequestMsg, DisclosedQnty) = %d\n", unsafe.Offsetof(rm.DisclosedQnty))
	fmt.Printf("offsetof(RequestMsg, Price) = %d\n", unsafe.Offsetof(rm.Price))
	fmt.Printf("offsetof(RequestMsg, TimeStamp) = %d\n", unsafe.Offsetof(rm.TimeStamp))
	fmt.Printf("offsetof(RequestMsg, AccountID) = %d\n", unsafe.Offsetof(rm.AccountID))
	fmt.Printf("offsetof(RequestMsg, Transaction_Type) = %d\n", unsafe.Offsetof(rm.TransactionType))
	fmt.Printf("offsetof(RequestMsg, Exchange_Type) = %d\n", unsafe.Offsetof(rm.ExchangeType))
	fmt.Printf("offsetof(RequestMsg, padding) = %d\n", unsafe.Offsetof(rm.Padding))
	fmt.Printf("offsetof(RequestMsg, Product) = %d\n", unsafe.Offsetof(rm.Product))
	fmt.Printf("offsetof(RequestMsg, StrategyID) = %d\n", unsafe.Offsetof(rm.StrategyID))

	// ResponseMsg
	var resp shm.ResponseMsg
	fmt.Printf("sizeof(ResponseMsg) = %d\n", unsafe.Sizeof(resp))
	fmt.Printf("offsetof(ResponseMsg, Response_Type) = %d\n", unsafe.Offsetof(resp.Response_Type))
	fmt.Printf("offsetof(ResponseMsg, Child_Response) = %d\n", unsafe.Offsetof(resp.Child_Response))
	fmt.Printf("offsetof(ResponseMsg, OrderID) = %d\n", unsafe.Offsetof(resp.OrderID))
	fmt.Printf("offsetof(ResponseMsg, ErrorCode) = %d\n", unsafe.Offsetof(resp.ErrorCode))
	fmt.Printf("offsetof(ResponseMsg, Quantity) = %d\n", unsafe.Offsetof(resp.Quantity))
	fmt.Printf("offsetof(ResponseMsg, Price) = %d\n", unsafe.Offsetof(resp.Price))
	fmt.Printf("offsetof(ResponseMsg, TimeStamp) = %d\n", unsafe.Offsetof(resp.TimeStamp))
	fmt.Printf("offsetof(ResponseMsg, Side) = %d\n", unsafe.Offsetof(resp.Side))
	fmt.Printf("offsetof(ResponseMsg, Symbol) = %d\n", unsafe.Offsetof(resp.Symbol))
	fmt.Printf("offsetof(ResponseMsg, AccountID) = %d\n", unsafe.Offsetof(resp.AccountID))
	fmt.Printf("offsetof(ResponseMsg, ExchangeOrderId) = %d\n", unsafe.Offsetof(resp.ExchangeOrderId))
	fmt.Printf("offsetof(ResponseMsg, ExchangeTradeId) = %d\n", unsafe.Offsetof(resp.ExchangeTradeId))
	fmt.Printf("offsetof(ResponseMsg, OpenClose) = %d\n", unsafe.Offsetof(resp.OpenClose))
	fmt.Printf("offsetof(ResponseMsg, ExchangeID) = %d\n", unsafe.Offsetof(resp.ExchangeID))
	fmt.Printf("offsetof(ResponseMsg, Product) = %d\n", unsafe.Offsetof(resp.Product))
	fmt.Printf("offsetof(ResponseMsg, StrategyID) = %d\n", unsafe.Offsetof(resp.StrategyID))

	// MDHeaderPart
	var hdr shm.MDHeaderPart
	fmt.Printf("sizeof(MDHeaderPart) = %d\n", unsafe.Sizeof(hdr))
	fmt.Printf("offsetof(MDHeaderPart, m_exchTS) = %d\n", unsafe.Offsetof(hdr.ExchTS))
	fmt.Printf("offsetof(MDHeaderPart, m_timestamp) = %d\n", unsafe.Offsetof(hdr.Timestamp))
	fmt.Printf("offsetof(MDHeaderPart, m_seqnum) = %d\n", unsafe.Offsetof(hdr.Seqnum))
	fmt.Printf("offsetof(MDHeaderPart, m_rptseqnum) = %d\n", unsafe.Offsetof(hdr.RptSeqnum))
	fmt.Printf("offsetof(MDHeaderPart, m_tokenId) = %d\n", unsafe.Offsetof(hdr.TokenId))
	fmt.Printf("offsetof(MDHeaderPart, m_symbol) = %d\n", unsafe.Offsetof(hdr.Symbol))
	fmt.Printf("offsetof(MDHeaderPart, m_symbolID) = %d\n", unsafe.Offsetof(hdr.SymbolID))
	fmt.Printf("offsetof(MDHeaderPart, m_exchangeName) = %d\n", unsafe.Offsetof(hdr.ExchangeName))

	// MDDataPart
	var dp shm.MDDataPart
	fmt.Printf("sizeof(MDDataPart) = %d\n", unsafe.Sizeof(dp))
	fmt.Printf("offsetof(MDDataPart, m_newPrice) = %d\n", unsafe.Offsetof(dp.NewPrice))
	fmt.Printf("offsetof(MDDataPart, m_oldPrice) = %d\n", unsafe.Offsetof(dp.OldPrice))
	fmt.Printf("offsetof(MDDataPart, m_lastTradedPrice) = %d\n", unsafe.Offsetof(dp.LastTradedPrice))
	fmt.Printf("offsetof(MDDataPart, m_lastTradedTime) = %d\n", unsafe.Offsetof(dp.LastTradedTime))
	fmt.Printf("offsetof(MDDataPart, m_totalTradedValue) = %d\n", unsafe.Offsetof(dp.TotalTradedValue))
	fmt.Printf("offsetof(MDDataPart, m_totalTradedQuantity) = %d\n", unsafe.Offsetof(dp.TotalTradedQuantity))
	fmt.Printf("offsetof(MDDataPart, m_yield) = %d\n", unsafe.Offsetof(dp.Yield))
	fmt.Printf("offsetof(MDDataPart, m_bidUpdates) = %d\n", unsafe.Offsetof(dp.BidUpdates))
	fmt.Printf("offsetof(MDDataPart, m_askUpdates) = %d\n", unsafe.Offsetof(dp.AskUpdates))
	fmt.Printf("offsetof(MDDataPart, m_newQuant) = %d\n", unsafe.Offsetof(dp.NewQuant))
	fmt.Printf("offsetof(MDDataPart, m_oldQuant) = %d\n", unsafe.Offsetof(dp.OldQuant))
	fmt.Printf("offsetof(MDDataPart, m_lastTradedQuantity) = %d\n", unsafe.Offsetof(dp.LastTradedQuantity))
	fmt.Printf("offsetof(MDDataPart, m_validBids) = %d\n", unsafe.Offsetof(dp.ValidBids))
	fmt.Printf("offsetof(MDDataPart, m_validAsks) = %d\n", unsafe.Offsetof(dp.ValidAsks))
	fmt.Printf("offsetof(MDDataPart, m_updateLevel) = %d\n", unsafe.Offsetof(dp.UpdateLevel))
	fmt.Printf("offsetof(MDDataPart, m_endPkt) = %d\n", unsafe.Offsetof(dp.EndPkt))
	fmt.Printf("offsetof(MDDataPart, m_side) = %d\n", unsafe.Offsetof(dp.Side))
	fmt.Printf("offsetof(MDDataPart, m_updateType) = %d\n", unsafe.Offsetof(dp.UpdateType))
	fmt.Printf("offsetof(MDDataPart, m_feedType) = %d\n", unsafe.Offsetof(dp.FeedType))

	// MarketUpdateNew
	fmt.Printf("sizeof(MarketUpdateNew) = %d\n", unsafe.Sizeof(shm.MarketUpdateNew{}))

	// QueueElem sizes
	fmt.Printf("sizeof(QueueElem<MarketUpdateNew>) = %d\n", unsafe.Sizeof(shm.QueueElemMD{}))
	fmt.Printf("sizeof(QueueElem<RequestMsg>) = %d\n", unsafe.Sizeof(shm.QueueElemReq{}))
	fmt.Printf("sizeof(QueueElem<ResponseMsg>) = %d\n", unsafe.Sizeof(shm.QueueElemResp{}))

	// MWMRHeader
	fmt.Printf("sizeof(MultiWriterMultiReaderShmHeader) = %d\n", unsafe.Sizeof(shm.MWMRHeader{}))
}
