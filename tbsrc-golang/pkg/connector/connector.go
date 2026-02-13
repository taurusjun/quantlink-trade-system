package connector

import (
	"fmt"
	"log"
	"runtime"
	"sync/atomic"

	"tbsrc-golang/pkg/shm"
)

const (
	// OrderIDRange defines the range per client for order ID generation.
	// C++: OrderID = clientID * ORDERID_RANGE + seq
	OrderIDRange = 1_000_000
)

// MDCallback is invoked for each incoming market data update.
type MDCallback func(md *shm.MarketUpdateNew)

// ORSCallback is invoked for each incoming ORS response.
type ORSCallback func(resp *shm.ResponseMsg)

// Config holds SHM keys and sizes for the Connector.
type Config struct {
	MDShmKey   int
	MDQueueSz  int
	ReqShmKey  int
	ReqQueueSz int
	RespShmKey int
	RespQueueSz int
	ClientStoreShmKey int
}

// Connector manages three SHM queues (MD, Request, Response)
// and a ClientStore for atomic client ID allocation.
// It mirrors the C++ strategy's interaction with hftbase ShmManager.
type Connector struct {
	mdQueue     *shm.MWMRQueue[shm.MarketUpdateNew]
	reqQueue    *shm.MWMRQueue[shm.RequestMsg]
	respQueue   *shm.MWMRQueue[shm.ResponseMsg]
	clientStore *shm.ClientStore

	clientID    uint32
	orderCount  atomic.Uint32

	mdCallback  MDCallback
	orsCallback ORSCallback
	running     atomic.Bool
}

// New creates a Connector that attaches to existing SHM queues.
// mdCb and orsCb are invoked from polling goroutines.
func New(cfg Config, mdCb MDCallback, orsCb ORSCallback) (*Connector, error) {
	mdQ, err := shm.NewMWMRQueue[shm.MarketUpdateNew](cfg.MDShmKey, cfg.MDQueueSz)
	if err != nil {
		return nil, fmt.Errorf("connector: MD queue: %w", err)
	}

	reqQ, err := shm.NewMWMRQueue[shm.RequestMsg](cfg.ReqShmKey, cfg.ReqQueueSz)
	if err != nil {
		mdQ.Close()
		return nil, fmt.Errorf("connector: Req queue: %w", err)
	}

	respQ, err := shm.NewMWMRQueue[shm.ResponseMsg](cfg.RespShmKey, cfg.RespQueueSz)
	if err != nil {
		mdQ.Close()
		reqQ.Close()
		return nil, fmt.Errorf("connector: Resp queue: %w", err)
	}

	cs, err := shm.NewClientStore(cfg.ClientStoreShmKey)
	if err != nil {
		mdQ.Close()
		reqQ.Close()
		respQ.Close()
		return nil, fmt.Errorf("connector: ClientStore: %w", err)
	}

	clientID := uint32(cs.GetClientIDAndIncrement())
	log.Printf("[Connector] allocated clientID=%d", clientID)

	c := &Connector{
		mdQueue:     mdQ,
		reqQueue:    reqQ,
		respQueue:   respQ,
		clientStore: cs,
		clientID:    clientID,
		mdCallback:  mdCb,
		orsCallback: orsCb,
	}

	return c, nil
}

// NewForTest creates a Connector that creates new SHM segments (for tests).
func NewForTest(cfg Config, mdCb MDCallback, orsCb ORSCallback) (*Connector, error) {
	mdQ, err := shm.NewMWMRQueueCreate[shm.MarketUpdateNew](cfg.MDShmKey, cfg.MDQueueSz)
	if err != nil {
		return nil, fmt.Errorf("connector: MD queue: %w", err)
	}

	reqQ, err := shm.NewMWMRQueueCreate[shm.RequestMsg](cfg.ReqShmKey, cfg.ReqQueueSz)
	if err != nil {
		mdQ.Destroy()
		return nil, fmt.Errorf("connector: Req queue: %w", err)
	}

	respQ, err := shm.NewMWMRQueueCreate[shm.ResponseMsg](cfg.RespShmKey, cfg.RespQueueSz)
	if err != nil {
		mdQ.Destroy()
		reqQ.Destroy()
		return nil, fmt.Errorf("connector: Resp queue: %w", err)
	}

	cs, err := shm.NewClientStoreCreate(cfg.ClientStoreShmKey, 1)
	if err != nil {
		mdQ.Destroy()
		reqQ.Destroy()
		respQ.Destroy()
		return nil, fmt.Errorf("connector: ClientStore: %w", err)
	}

	clientID := uint32(cs.GetClientIDAndIncrement())

	c := &Connector{
		mdQueue:     mdQ,
		reqQueue:    reqQ,
		respQueue:   respQ,
		clientStore: cs,
		clientID:    clientID,
		mdCallback:  mdCb,
		orsCallback: orsCb,
	}

	return c, nil
}

// Start launches the MD and ORS polling goroutines.
func (c *Connector) Start() {
	c.running.Store(true)
	go c.pollMD()
	go c.pollORS()
}

// Stop signals both polling goroutines to exit.
func (c *Connector) Stop() {
	c.running.Store(false)
}

// Close stops polling and detaches all SHM segments.
func (c *Connector) Close() error {
	c.Stop()
	var firstErr error
	if err := c.mdQueue.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := c.reqQueue.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := c.respQueue.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := c.clientStore.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// Destroy stops and removes all SHM segments (for tests).
func (c *Connector) Destroy() error {
	c.Stop()
	var firstErr error
	if err := c.mdQueue.Destroy(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := c.reqQueue.Destroy(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := c.respQueue.Destroy(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := c.clientStore.Destroy(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// ClientID returns the allocated client ID for this connector.
func (c *Connector) ClientID() uint32 {
	return c.clientID
}

// SendNewOrder enqueues a new order request to ORS.
// It sets OrderID, Request_Type=NEWORDER, and enqueues.
// C++: OrderID = clientID * ORDERID_RANGE + seq
func (c *Connector) SendNewOrder(req *shm.RequestMsg) uint32 {
	orderID := c.nextOrderID()
	req.OrderID = orderID
	req.Request_Type = shm.NEWORDER
	c.reqQueue.Enqueue(req)
	return orderID
}

// SendCancelOrder enqueues a cancel order request to ORS.
func (c *Connector) SendCancelOrder(req *shm.RequestMsg) {
	req.Request_Type = shm.CANCELORDER
	c.reqQueue.Enqueue(req)
}

// SendModifyOrder enqueues a modify order request to ORS.
func (c *Connector) SendModifyOrder(req *shm.RequestMsg) {
	req.Request_Type = shm.MODIFYORDER
	c.reqQueue.Enqueue(req)
}

// EnqueueMD enqueues a market data update (for tests / simulator).
func (c *Connector) EnqueueMD(md *shm.MarketUpdateNew) {
	c.mdQueue.Enqueue(md)
}

// EnqueueResponse enqueues a response (for tests / simulator).
func (c *Connector) EnqueueResponse(resp *shm.ResponseMsg) {
	c.respQueue.Enqueue(resp)
}

// nextOrderID generates a unique order ID.
// C++: clientID * ORDERID_RANGE + atomic_seq++
func (c *Connector) nextOrderID() uint32 {
	seq := c.orderCount.Add(1)
	return c.clientID*OrderIDRange + seq
}

// pollMD continuously reads MD queue and invokes callback.
func (c *Connector) pollMD() {
	var md shm.MarketUpdateNew
	for c.running.Load() {
		if c.mdQueue.Dequeue(&md) {
			c.mdCallback(&md)
		} else {
			runtime.Gosched()
		}
	}
}

// pollORS continuously reads response queue and invokes callback for our orders.
// C++: filter by resp.OrderID / ORDERID_RANGE == clientID
func (c *Connector) pollORS() {
	var resp shm.ResponseMsg
	for c.running.Load() {
		if c.respQueue.Dequeue(&resp) {
			// Filter: only process responses belonging to this client
			if resp.OrderID/OrderIDRange == c.clientID {
				c.orsCallback(&resp)
			}
		} else {
			runtime.Gosched()
		}
	}
}
