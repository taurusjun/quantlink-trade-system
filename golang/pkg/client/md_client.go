package client

import (
	"context"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"

	pb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	"github.com/nats-io/nats.go"
)

// MarketData 统一行情数据结构
type MarketData struct {
	Symbol            string
	Exchange          string
	Timestamp         uint64
	ExchangeTimestamp uint64

	BidPrice      []float64
	BidQty        []uint32
	BidOrderCount []uint32

	AskPrice      []float64
	AskQty        []uint32
	AskOrderCount []uint32

	LastPrice   float64
	LastQty     uint32
	TotalVolume uint64
	Turnover    float64

	OpenPrice     float64
	HighPrice     float64
	LowPrice      float64
	PreClosePrice float64
	UpperLimit    float64
	LowerLimit    float64
}

// MDClient gRPC行情客户端
type MDClient struct {
	conn   *grpc.ClientConn
	client pb.MDGatewayClient
}

// NewMDClient 创建MD客户端
func NewMDClient(addr string) (*MDClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &MDClient{
		conn:   conn,
		client: pb.NewMDGatewayClient(conn),
	}, nil
}

// Close 关闭连接
func (c *MDClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Subscribe 订阅行情
func (c *MDClient) Subscribe(ctx context.Context, symbols []string, exchange string, fullDepth bool) (*MDStream, error) {
	req := &pb.SubscribeRequest{
		Symbols:   symbols,
		Exchange:  exchange,
		FullDepth: fullDepth,
	}

	stream, err := c.client.SubscribeMarketData(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	return &MDStream{stream: stream}, nil
}

// GetSnapshot 获取快照
func (c *MDClient) GetSnapshot(ctx context.Context, symbol, exchange string) (*MarketData, error) {
	req := &pb.SnapshotRequest{
		Symbol:   symbol,
		Exchange: exchange,
	}

	resp, err := c.client.GetSnapshot(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	return convertFromProto(resp), nil
}

// MDStream 行情流
type MDStream struct {
	stream pb.MDGateway_SubscribeMarketDataClient
}

// Recv 接收行情数据
func (s *MDStream) Recv() (*MarketData, error) {
	pbMD, err := s.stream.Recv()
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("stream recv error: %w", err)
	}

	return convertFromProto(pbMD), nil
}

// convertFromProto 从Protobuf转换
func convertFromProto(pb *pb.MarketDataUpdate) *MarketData {
	return &MarketData{
		Symbol:            pb.Symbol,
		Exchange:          pb.Exchange,
		Timestamp:         pb.Timestamp,
		ExchangeTimestamp: pb.ExchangeTimestamp,
		BidPrice:          pb.BidPrice,
		BidQty:            pb.BidQty,
		BidOrderCount:     pb.BidOrderCount,
		AskPrice:          pb.AskPrice,
		AskQty:            pb.AskQty,
		AskOrderCount:     pb.AskOrderCount,
		LastPrice:         pb.LastPrice,
		LastQty:           pb.LastQty,
		TotalVolume:       pb.TotalVolume,
		Turnover:          pb.Turnover,
		OpenPrice:         pb.OpenPrice,
		HighPrice:         pb.HighPrice,
		LowPrice:          pb.LowPrice,
		PreClosePrice:     pb.PreClosePrice,
		UpperLimit:        pb.UpperLimit,
		LowerLimit:        pb.LowerLimit,
	}
}

// NATSClient NATS行情客户端
type NATSClient struct {
	conn *nats.Conn
	subs []*nats.Subscription
}

// NewNATSClient 创建NATS客户端
func NewNATSClient(url string) (*NATSClient, error) {
	conn, err := nats.Connect(url,
		nats.MaxReconnects(10),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	return &NATSClient{
		conn: conn,
		subs: make([]*nats.Subscription, 0),
	}, nil
}

// Close 关闭连接
func (c *NATSClient) Close() error {
	for _, sub := range c.subs {
		sub.Unsubscribe()
	}
	if c.conn != nil {
		c.conn.Close()
	}
	return nil
}

// Subscribe 订阅主题
func (c *NATSClient) Subscribe(subject string, handler func(*MarketData)) error {
	sub, err := c.conn.Subscribe(subject, func(msg *nats.Msg) {
		// 解析Protobuf
		var pbMD pb.MarketDataUpdate
		if err := proto.Unmarshal(msg.Data, &pbMD); err != nil {
			fmt.Printf("[NATS] Failed to unmarshal: %v\n", err)
			return
		}

		// 转换并回调
		md := convertFromProto(&pbMD)
		handler(md)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	c.subs = append(c.subs, sub)
	return nil
}

// SubscribePattern 订阅模式
func (c *NATSClient) SubscribePattern(pattern string, handler func(*MarketData)) error {
	sub, err := c.conn.Subscribe(pattern, func(msg *nats.Msg) {
		var pbMD pb.MarketDataUpdate
		if err := proto.Unmarshal(msg.Data, &pbMD); err != nil {
			return
		}

		md := convertFromProto(&pbMD)
		handler(md)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe pattern: %w", err)
	}

	c.subs = append(c.subs, sub)
	return nil
}
