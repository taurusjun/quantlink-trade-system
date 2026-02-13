package client

import (
	"log"

	"tbsrc-golang/pkg/connector"
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// StrategyCallback 策略回调接口，由 LegManager 实现
type StrategyCallback interface {
	MDCallBack(inst *instrument.Instrument, md *shm.MarketUpdateNew)
	ORSCallBack(resp *shm.ResponseMsg)
}

// Client 对应 C++ CommonClient
// 负责 MD/ORS 路由、RequestMsg 构造、orderID→strategy 映射
// 参考: tbsrc/main/CommonClient.cpp
type Client struct {
	conn         *connector.Connector
	instruments  map[string]*instrument.Instrument // symbol → Instrument
	strategies   map[string]StrategyCallback        // symbol → strategy callback
	orderIDMap   map[uint32]StrategyCallback         // orderID → strategy callback
	strategyID   int32
	account      string
	product      string
	exchangeType uint8 // C++: m_exchangeType（来自 FillReqInfo）
	reqMsg       shm.RequestMsg // 复用的请求缓冲区
}

// NewClient 创建 Client
func NewClient(conn *connector.Connector, strategyID int32, account, product string, exchangeType uint8) *Client {
	return &Client{
		conn:         conn,
		instruments:  make(map[string]*instrument.Instrument),
		strategies:   make(map[string]StrategyCallback),
		orderIDMap:   make(map[uint32]StrategyCallback),
		strategyID:   strategyID,
		account:      account,
		product:      product,
		exchangeType: exchangeType,
	}
}

// RegisterInstrument 注册合约
func (c *Client) RegisterInstrument(inst *instrument.Instrument) {
	c.instruments[inst.Symbol] = inst
}

// RegisterStrategy 注册策略回调（按 symbol）
func (c *Client) RegisterStrategy(symbol string, cb StrategyCallback) {
	c.strategies[symbol] = cb
}

// OnMDUpdate 作为 Connector 的 MDCallback
// 参考: CommonClient.cpp SendINDUpdate()
// 根据 symbol 查找 Instrument 并更新，然后路由到对应策略
func (c *Client) OnMDUpdate(md *shm.MarketUpdateNew) {
	symbol := extractSymbol(&md.Header)
	inst, ok := c.instruments[symbol]
	if !ok {
		return
	}

	// 更新行情簿
	inst.UpdateFromMD(md)

	// 路由到策略
	cb, ok := c.strategies[symbol]
	if !ok {
		return
	}
	cb.MDCallBack(inst, md)
}

// OnORSUpdate 作为 Connector 的 ORSCallback
// 参考: CommonClient.cpp SendInfraORSUpdate()
// 根据 orderID 查找策略并路由
func (c *Client) OnORSUpdate(resp *shm.ResponseMsg) {
	cb, ok := c.orderIDMap[resp.OrderID]
	if !ok {
		log.Printf("[Client] unknown orderID=%d responseType=%d", resp.OrderID, resp.Response_Type)
		return
	}
	cb.ORSCallBack(resp)
}

// SendNewOrder 构造 RequestMsg 并发送新订单
// 参考: CommonClient.cpp:918-989 SendNewOrder()
//
// C++: 填充 Request_Type=NEWORDER, Token, Transaction_Type, Price, Quantity,
//      AccountID, Product, StrategyID, 然后调用 FillReqInfo()+connector.SendNewOrder
func (c *Client) SendNewOrder(inst *instrument.Instrument, side types.TransactionType,
	price float64, qty int32, ordHitType types.OrderHitType, cb StrategyCallback) uint32 {

	c.clearReqMsg()

	// C++: m_reqMsg.Token = Token
	c.reqMsg.Token = inst.Token

	// C++: m_reqMsg.Transaction_Type = ConvertSide(side)
	// shm 使用 'B'/'S'，types 使用 Buy=1/Sell=2
	if side == types.Buy {
		c.reqMsg.TransactionType = shm.SideBuy
	} else {
		c.reqMsg.TransactionType = shm.SideSell
	}

	// C++: m_reqMsg.Price = price
	c.reqMsg.Price = price

	// C++: m_reqMsg.Quantity = qty
	c.reqMsg.Quantity = qty
	c.reqMsg.QuantityFilled = 0
	c.reqMsg.DisclosedQnty = qty

	// C++: memcpy(m_reqMsg.Product, execStrategy->m_product, 32)
	copyStringToBytes(c.reqMsg.Product[:], c.product)

	// C++: m_reqMsg.StrategyID = execStrategy->m_strategyID
	c.reqMsg.StrategyID = c.strategyID

	// C++: memcpy(m_reqMsg.AccountID, Account, ...)
	copyStringToBytes(c.reqMsg.AccountID[:], c.account)

	// C++: memcpy(m_reqMsg.Contract_Description.Symbol, ticker, ...)
	copyStringToBytes(c.reqMsg.ContractDesc.Symbol[:], inst.Symbol)

	// C++: m_reqMsg.Contract_Description.ExpiryDate = Exp
	c.reqMsg.ContractDesc.ExpiryDate = inst.ExpiryDate

	// C++: CROSS 使用 FAK, STANDARD 使用 DAY
	if ordHitType == types.HitCross {
		c.reqMsg.Duration = shm.FAK
	} else {
		c.reqMsg.Duration = shm.DAY
	}

	// C++: FillReqInfo() — 设置 LIMIT, PERUNIT, Exchange_Type
	c.fillReqInfo()

	// 发送
	orderID := c.conn.SendNewOrder(&c.reqMsg)

	// 注册 orderID → callback
	c.orderIDMap[orderID] = cb

	return orderID
}

// SendModifyOrder 发送改单请求
// 参考: CommonClient.cpp:991-1051 SendModifyOrder()
func (c *Client) SendModifyOrder(inst *instrument.Instrument, orderID uint32,
	side types.TransactionType, price float64, doneQty, qty int32, cb StrategyCallback) {

	c.clearReqMsg()

	c.reqMsg.OrderID = orderID
	c.reqMsg.Token = inst.Token

	if side == types.Buy {
		c.reqMsg.TransactionType = shm.SideBuy
	} else {
		c.reqMsg.TransactionType = shm.SideSell
	}

	c.reqMsg.Price = price
	c.reqMsg.Quantity = qty
	c.reqMsg.QuantityFilled = doneQty
	c.reqMsg.DisclosedQnty = qty

	copyStringToBytes(c.reqMsg.Product[:], c.product)
	c.reqMsg.StrategyID = c.strategyID
	copyStringToBytes(c.reqMsg.AccountID[:], c.account)
	copyStringToBytes(c.reqMsg.ContractDesc.Symbol[:], inst.Symbol)
	c.reqMsg.ContractDesc.ExpiryDate = inst.ExpiryDate

	c.fillReqInfo()

	c.conn.SendModifyOrder(&c.reqMsg)
}

// SendCancelOrder 发送撤单请求
// 参考: CommonClient.cpp:1053-1111 SendCancelOrder()
func (c *Client) SendCancelOrder(inst *instrument.Instrument, orderID uint32,
	side types.TransactionType, price float64, doneQty, openQty int32) {

	c.clearReqMsg()

	c.reqMsg.OrderID = orderID
	c.reqMsg.Token = inst.Token

	if side == types.Buy {
		c.reqMsg.TransactionType = shm.SideBuy
	} else {
		c.reqMsg.TransactionType = shm.SideSell
	}

	c.reqMsg.Price = price
	c.reqMsg.Quantity = openQty
	c.reqMsg.QuantityFilled = doneQty

	copyStringToBytes(c.reqMsg.Product[:], c.product)
	c.reqMsg.StrategyID = c.strategyID
	copyStringToBytes(c.reqMsg.AccountID[:], c.account)
	copyStringToBytes(c.reqMsg.ContractDesc.Symbol[:], inst.Symbol)
	c.reqMsg.ContractDesc.ExpiryDate = inst.ExpiryDate

	c.fillReqInfo()

	c.conn.SendCancelOrder(&c.reqMsg)
}

// RemoveOrderID 从 orderIDMap 中移除（订单完成后清理）
func (c *Client) RemoveOrderID(orderID uint32) {
	delete(c.orderIDMap, orderID)
}

// Connector 返回底层 Connector
func (c *Client) Connector() *connector.Connector {
	return c.conn
}

// fillReqInfo 对应 C++ CommonClient::FillReqInfo()
// 参考: CommonClient.cpp:1113
func (c *Client) fillReqInfo() {
	// C++: m_reqMsg.OrdType = LIMIT
	c.reqMsg.OrdType = shm.LIMIT
	// C++: m_reqMsg.PxType = PERUNIT
	c.reqMsg.PxType = shm.PERUNIT
	// C++: m_reqMsg.Exchange_Type = m_exchangeType
	c.reqMsg.ExchangeType = c.exchangeType
}

// clearReqMsg 清空请求缓冲区
func (c *Client) clearReqMsg() {
	c.reqMsg = shm.RequestMsg{}
}

// extractSymbol 从 MDHeaderPart 提取 symbol 字符串
func extractSymbol(h *shm.MDHeaderPart) string {
	for i, b := range h.Symbol {
		if b == 0 {
			return string(h.Symbol[:i])
		}
	}
	return string(h.Symbol[:])
}

// copyStringToBytes 将字符串复制到固定大小 byte 数组（null 终止）
func copyStringToBytes(dst []byte, src string) {
	n := copy(dst, src)
	if n < len(dst) {
		dst[n] = 0
	}
}
