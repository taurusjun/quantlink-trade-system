package client

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// ORSClient ORS Gateway客户端
type ORSClient struct {
	// gRPC连接
	conn   *grpc.ClientConn
	client orspb.ORSGatewayClient

	// NATS连接（用于订单回报）
	natsConn *nats.Conn
	natsSub  *nats.Subscription

	// 订单回调
	onOrderUpdate func(*orspb.OrderUpdate)

	// 状态
	mu        sync.RWMutex
	connected bool
	strategyID string

	// 统计
	ordersSent    int64
	ordersAccepted int64
	ordersRejected int64
}

// ORSClientConfig 客户端配置
type ORSClientConfig struct {
	GatewayAddr string // ORS Gateway地址 (例如: localhost:50052)
	NATSAddr    string // NATS服务器地址 (例如: nats://localhost:4222)
	StrategyID  string // 策略ID
}

// NewORSClient 创建ORS客户端
func NewORSClient(config ORSClientConfig) (*ORSClient, error) {
	client := &ORSClient{
		strategyID: config.StrategyID,
	}

	// 1. 连接gRPC
	conn, err := grpc.NewClient(
		config.GatewayAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ORS Gateway: %w", err)
	}

	client.conn = conn
	client.client = orspb.NewORSGatewayClient(conn)

	// 2. 连接NATS（可选）
	if config.NATSAddr != "" {
		nc, err := nats.Connect(config.NATSAddr)
		if err != nil {
			log.Printf("[ORSClient] Warning: failed to connect to NATS: %v", err)
		} else {
			client.natsConn = nc
			log.Printf("[ORSClient] Connected to NATS: %s", config.NATSAddr)
		}
	}

	client.connected = true
	log.Printf("[ORSClient] Connected to ORS Gateway: %s", config.GatewayAddr)

	return client, nil
}

// Close 关闭客户端
func (c *ORSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false

	// 取消NATS订阅
	if c.natsSub != nil {
		c.natsSub.Unsubscribe()
	}

	// 关闭NATS连接
	if c.natsConn != nil {
		c.natsConn.Close()
	}

	// 关闭gRPC连接
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// SendOrder 发送订单
func (c *ORSClient) SendOrder(ctx context.Context, req *orspb.OrderRequest) (*orspb.OrderResponse, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client not connected")
	}
	c.mu.RUnlock()

	// 设置策略ID
	if req.StrategyId == "" {
		req.StrategyId = c.strategyID
	}

	// 调用gRPC接口
	resp, err := c.client.SendOrder(ctx, req)
	if err != nil {
		c.ordersRejected++
		return nil, fmt.Errorf("failed to send order: %w", err)
	}

	c.ordersSent++
	if resp.ErrorCode == orspb.ErrorCode_SUCCESS {
		c.ordersAccepted++
	} else {
		c.ordersRejected++
	}

	return resp, nil
}

// CancelOrder 撤销订单
func (c *ORSClient) CancelOrder(ctx context.Context, req *orspb.CancelRequest) (*orspb.CancelResponse, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client not connected")
	}
	c.mu.RUnlock()

	resp, err := c.client.CancelOrder(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel order: %w", err)
	}

	return resp, nil
}

// QueryOrders 查询订单
func (c *ORSClient) QueryOrders(ctx context.Context, req *orspb.OrderQuery) ([]*orspb.OrderData, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client not connected")
	}
	c.mu.RUnlock()

	// 设置策略ID
	if req.StrategyId == "" {
		req.StrategyId = c.strategyID
	}

	stream, err := c.client.QueryOrders(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}

	var orders []*orspb.OrderData
	for {
		orderData, err := stream.Recv()
		if err != nil {
			break
		}
		orders = append(orders, orderData)
	}

	return orders, nil
}

// QueryPosition 查询仓位
func (c *ORSClient) QueryPosition(ctx context.Context, req *orspb.PositionQuery) ([]*orspb.PositionData, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client not connected")
	}
	c.mu.RUnlock()

	stream, err := c.client.QueryPosition(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to query position: %w", err)
	}

	var positions []*orspb.PositionData
	for {
		posData, err := stream.Recv()
		if err != nil {
			break
		}
		positions = append(positions, posData)
	}

	return positions, nil
}

// SubscribeOrderUpdates 订阅订单回报（通过NATS）
func (c *ORSClient) SubscribeOrderUpdates(callback func(*orspb.OrderUpdate)) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.natsConn == nil {
		return fmt.Errorf("NATS connection not available")
	}

	c.onOrderUpdate = callback

	// 订阅主题：order.{strategy_id}.>
	subject := fmt.Sprintf("order.%s.>", c.strategyID)
	sub, err := c.natsConn.Subscribe(subject, func(msg *nats.Msg) {
		var update orspb.OrderUpdate
		if err := proto.Unmarshal(msg.Data, &update); err != nil {
			log.Printf("[ORSClient] Failed to unmarshal order update: %v", err)
			return
		}

		if c.onOrderUpdate != nil {
			c.onOrderUpdate(&update)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to order updates: %w", err)
	}

	c.natsSub = sub
	log.Printf("[ORSClient] Subscribed to order updates: %s", subject)

	return nil
}

// GetStatistics 获取统计信息
func (c *ORSClient) GetStatistics() map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]int64{
		"orders_sent":     c.ordersSent,
		"orders_accepted": c.ordersAccepted,
		"orders_rejected": c.ordersRejected,
	}
}

// SendOrderSync 同步发送订单（简化版）
func (c *ORSClient) SendOrderSync(symbol string, side orspb.OrderSide, price float64, quantity int64) (*orspb.OrderResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &orspb.OrderRequest{
		StrategyId:  c.strategyID,
		Symbol:      symbol,
		Side:        side,
		OrderType:   orspb.OrderType_LIMIT,
		Price:       price,
		Quantity:    quantity,
		TimeInForce: orspb.TimeInForce_GTC,
		OpenClose:   orspb.OpenClose_OPEN,
	}

	return c.SendOrder(ctx, req)
}
