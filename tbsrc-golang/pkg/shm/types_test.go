package shm

import (
	"testing"
	"unsafe"
)

// TestBookElement verifies sizeof and field offsets for BookElement.
// C++: bookElement_t : orderQtPair_t { int32_t quantity; int32_t orderCount; double price; }
func TestBookElement(t *testing.T) {
	assertSize(t, "BookElement", unsafe.Sizeof(BookElement{}), 16)
	var be BookElement
	assertOffset(t, "BookElement.Quantity", unsafe.Offsetof(be.Quantity), 0)
	assertOffset(t, "BookElement.OrderCount", unsafe.Offsetof(be.OrderCount), 4)
	assertOffset(t, "BookElement.Price", unsafe.Offsetof(be.Price), 8)
}

// TestContractDescription verifies sizeof and field offsets.
func TestContractDescription(t *testing.T) {
	assertSize(t, "ContractDescription", unsafe.Sizeof(ContractDescription{}), 96)
	var cd ContractDescription
	assertOffset(t, "ContractDescription.InstrumentName", unsafe.Offsetof(cd.InstrumentName), 0)
	assertOffset(t, "ContractDescription.Symbol", unsafe.Offsetof(cd.Symbol), 32)
	assertOffset(t, "ContractDescription.ExpiryDate", unsafe.Offsetof(cd.ExpiryDate), 84)
	assertOffset(t, "ContractDescription.StrikePrice", unsafe.Offsetof(cd.StrikePrice), 88)
	assertOffset(t, "ContractDescription.OptionType", unsafe.Offsetof(cd.OptionType), 92)
	assertOffset(t, "ContractDescription.CALevel", unsafe.Offsetof(cd.CALevel), 94)
}

// TestRequestMsg verifies sizeof and field offsets.
// C++: sizeof(RequestMsg) = 256 due to __attribute__((aligned(64)))
func TestRequestMsg(t *testing.T) {
	assertSize(t, "RequestMsg", unsafe.Sizeof(RequestMsg{}), 256)
	var rm RequestMsg
	assertOffset(t, "RequestMsg.ContractDesc", unsafe.Offsetof(rm.ContractDesc), 0)
	assertOffset(t, "RequestMsg.Request_Type", unsafe.Offsetof(rm.Request_Type), 96)
	assertOffset(t, "RequestMsg.OrdType", unsafe.Offsetof(rm.OrdType), 100)
	assertOffset(t, "RequestMsg.Duration", unsafe.Offsetof(rm.Duration), 104)
	assertOffset(t, "RequestMsg.PxType", unsafe.Offsetof(rm.PxType), 108)
	assertOffset(t, "RequestMsg.PosDirection", unsafe.Offsetof(rm.PosDirection), 112)
	assertOffset(t, "RequestMsg.OrderID", unsafe.Offsetof(rm.OrderID), 116)
	assertOffset(t, "RequestMsg.Token", unsafe.Offsetof(rm.Token), 120)
	assertOffset(t, "RequestMsg.Quantity", unsafe.Offsetof(rm.Quantity), 124)
	assertOffset(t, "RequestMsg.QuantityFilled", unsafe.Offsetof(rm.QuantityFilled), 128)
	assertOffset(t, "RequestMsg.DisclosedQnty", unsafe.Offsetof(rm.DisclosedQnty), 132)
	assertOffset(t, "RequestMsg.Price", unsafe.Offsetof(rm.Price), 136)
	assertOffset(t, "RequestMsg.TimeStamp", unsafe.Offsetof(rm.TimeStamp), 144)
	assertOffset(t, "RequestMsg.AccountID", unsafe.Offsetof(rm.AccountID), 152)
	assertOffset(t, "RequestMsg.TransactionType", unsafe.Offsetof(rm.TransactionType), 163)
	assertOffset(t, "RequestMsg.ExchangeType", unsafe.Offsetof(rm.ExchangeType), 164)
	assertOffset(t, "RequestMsg.Padding", unsafe.Offsetof(rm.Padding), 165)
	assertOffset(t, "RequestMsg.Product", unsafe.Offsetof(rm.Product), 185)
	assertOffset(t, "RequestMsg.StrategyID", unsafe.Offsetof(rm.StrategyID), 220)
}

// TestResponseMsg verifies sizeof and field offsets.
func TestResponseMsg(t *testing.T) {
	assertSize(t, "ResponseMsg", unsafe.Sizeof(ResponseMsg{}), 176)
	var rm ResponseMsg
	assertOffset(t, "ResponseMsg.Response_Type", unsafe.Offsetof(rm.Response_Type), 0)
	assertOffset(t, "ResponseMsg.Child_Response", unsafe.Offsetof(rm.Child_Response), 4)
	assertOffset(t, "ResponseMsg.OrderID", unsafe.Offsetof(rm.OrderID), 8)
	assertOffset(t, "ResponseMsg.ErrorCode", unsafe.Offsetof(rm.ErrorCode), 12)
	assertOffset(t, "ResponseMsg.Quantity", unsafe.Offsetof(rm.Quantity), 16)
	assertOffset(t, "ResponseMsg.Price", unsafe.Offsetof(rm.Price), 24)
	assertOffset(t, "ResponseMsg.TimeStamp", unsafe.Offsetof(rm.TimeStamp), 32)
	assertOffset(t, "ResponseMsg.Side", unsafe.Offsetof(rm.Side), 40)
	assertOffset(t, "ResponseMsg.Symbol", unsafe.Offsetof(rm.Symbol), 41)
	assertOffset(t, "ResponseMsg.AccountID", unsafe.Offsetof(rm.AccountID), 91)
	assertOffset(t, "ResponseMsg.ExchangeOrderId", unsafe.Offsetof(rm.ExchangeOrderId), 104)
	assertOffset(t, "ResponseMsg.ExchangeTradeId", unsafe.Offsetof(rm.ExchangeTradeId), 112)
	assertOffset(t, "ResponseMsg.OpenClose", unsafe.Offsetof(rm.OpenClose), 133)
	assertOffset(t, "ResponseMsg.ExchangeID", unsafe.Offsetof(rm.ExchangeID), 134)
	assertOffset(t, "ResponseMsg.Product", unsafe.Offsetof(rm.Product), 135)
	assertOffset(t, "ResponseMsg.StrategyID", unsafe.Offsetof(rm.StrategyID), 168)
}

// TestMDHeaderPart verifies sizeof and field offsets.
func TestMDHeaderPart(t *testing.T) {
	assertSize(t, "MDHeaderPart", unsafe.Sizeof(MDHeaderPart{}), 96)
	var h MDHeaderPart
	assertOffset(t, "MDHeaderPart.ExchTS", unsafe.Offsetof(h.ExchTS), 0)
	assertOffset(t, "MDHeaderPart.Timestamp", unsafe.Offsetof(h.Timestamp), 8)
	assertOffset(t, "MDHeaderPart.Seqnum", unsafe.Offsetof(h.Seqnum), 16)
	assertOffset(t, "MDHeaderPart.RptSeqnum", unsafe.Offsetof(h.RptSeqnum), 24)
	assertOffset(t, "MDHeaderPart.TokenId", unsafe.Offsetof(h.TokenId), 32)
	assertOffset(t, "MDHeaderPart.Symbol", unsafe.Offsetof(h.Symbol), 40)
	assertOffset(t, "MDHeaderPart.SymbolID", unsafe.Offsetof(h.SymbolID), 88)
	assertOffset(t, "MDHeaderPart.ExchangeName", unsafe.Offsetof(h.ExchangeName), 90)
}

// TestMDDataPart verifies sizeof and field offsets.
func TestMDDataPart(t *testing.T) {
	assertSize(t, "MDDataPart", unsafe.Sizeof(MDDataPart{}), 720)
	var d MDDataPart
	assertOffset(t, "MDDataPart.NewPrice", unsafe.Offsetof(d.NewPrice), 0)
	assertOffset(t, "MDDataPart.OldPrice", unsafe.Offsetof(d.OldPrice), 8)
	assertOffset(t, "MDDataPart.LastTradedPrice", unsafe.Offsetof(d.LastTradedPrice), 16)
	assertOffset(t, "MDDataPart.LastTradedTime", unsafe.Offsetof(d.LastTradedTime), 24)
	assertOffset(t, "MDDataPart.TotalTradedValue", unsafe.Offsetof(d.TotalTradedValue), 32)
	assertOffset(t, "MDDataPart.TotalTradedQuantity", unsafe.Offsetof(d.TotalTradedQuantity), 40)
	assertOffset(t, "MDDataPart.Yield", unsafe.Offsetof(d.Yield), 48)
	assertOffset(t, "MDDataPart.BidUpdates", unsafe.Offsetof(d.BidUpdates), 56)
	assertOffset(t, "MDDataPart.AskUpdates", unsafe.Offsetof(d.AskUpdates), 376)
	assertOffset(t, "MDDataPart.NewQuant", unsafe.Offsetof(d.NewQuant), 696)
	assertOffset(t, "MDDataPart.OldQuant", unsafe.Offsetof(d.OldQuant), 700)
	assertOffset(t, "MDDataPart.LastTradedQuantity", unsafe.Offsetof(d.LastTradedQuantity), 704)
	assertOffset(t, "MDDataPart.ValidBids", unsafe.Offsetof(d.ValidBids), 708)
	assertOffset(t, "MDDataPart.ValidAsks", unsafe.Offsetof(d.ValidAsks), 709)
	assertOffset(t, "MDDataPart.UpdateLevel", unsafe.Offsetof(d.UpdateLevel), 710)
	assertOffset(t, "MDDataPart.EndPkt", unsafe.Offsetof(d.EndPkt), 711)
	assertOffset(t, "MDDataPart.Side", unsafe.Offsetof(d.Side), 712)
	assertOffset(t, "MDDataPart.UpdateType", unsafe.Offsetof(d.UpdateType), 713)
	assertOffset(t, "MDDataPart.FeedType", unsafe.Offsetof(d.FeedType), 714)
}

// TestMarketUpdateNew verifies total size.
func TestMarketUpdateNew(t *testing.T) {
	assertSize(t, "MarketUpdateNew", unsafe.Sizeof(MarketUpdateNew{}), 816)
	var mu MarketUpdateNew
	assertOffset(t, "MarketUpdateNew.Header", unsafe.Offsetof(mu.Header), 0)
	assertOffset(t, "MarketUpdateNew.Data", unsafe.Offsetof(mu.Data), 96)
}

// TestQueueElements verifies queue element wrapper sizes.
func TestQueueElements(t *testing.T) {
	assertSize(t, "QueueElemMD", unsafe.Sizeof(QueueElemMD{}), 824)
	// C++: QueueElem<RequestMsg> = 320 bytes due to __attribute__((aligned(64)))
	assertSize(t, "QueueElemReq", unsafe.Sizeof(QueueElemReq{}), 320)
	assertSize(t, "QueueElemResp", unsafe.Sizeof(QueueElemResp{}), 184)

	var qmd QueueElemMD
	assertOffset(t, "QueueElemMD.Data", unsafe.Offsetof(qmd.Data), 0)
	assertOffset(t, "QueueElemMD.SeqNo", unsafe.Offsetof(qmd.SeqNo), 816)

	var qreq QueueElemReq
	assertOffset(t, "QueueElemReq.Data", unsafe.Offsetof(qreq.Data), 0)
	assertOffset(t, "QueueElemReq.SeqNo", unsafe.Offsetof(qreq.SeqNo), 256)

	var qresp QueueElemResp
	assertOffset(t, "QueueElemResp.Data", unsafe.Offsetof(qresp.Data), 0)
	assertOffset(t, "QueueElemResp.SeqNo", unsafe.Offsetof(qresp.SeqNo), 176)
}

// TestMWMRHeader verifies SHM header sizes.
func TestMWMRHeader(t *testing.T) {
	assertSize(t, "MWMRHeader", unsafe.Sizeof(MWMRHeader{}), 8)
}

// TestClientData verifies ClientStore SHM layout.
func TestClientData(t *testing.T) {
	assertSize(t, "ClientData", unsafe.Sizeof(ClientData{}), 16)
	var cd ClientData
	assertOffset(t, "ClientData.Data", unsafe.Offsetof(cd.Data), 0)
	assertOffset(t, "ClientData.FirstClientId", unsafe.Offsetof(cd.FirstClientId), 8)
}

func assertSize(t *testing.T, name string, got, want uintptr) {
	t.Helper()
	if got != want {
		t.Errorf("sizeof(%s) = %d, want %d", name, got, want)
	}
}

func assertOffset(t *testing.T, name string, got, want uintptr) {
	t.Helper()
	if got != want {
		t.Errorf("offsetof(%s) = %d, want %d", name, got, want)
	}
}
