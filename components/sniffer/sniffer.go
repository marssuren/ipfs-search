/*
Package sniffer 包含嗅探器组件，可通过代理数据存储接入libp2p DHT节点。
规范实现参考：https://github.com/ipfs-search/ipfs-sniffer
如需嗅探CID，建议使用`factory`简化配置。
*/
/* 数据流图示意：
IPFS DHT 数据存储 (datastore.Batching)
       ↓ 触发事件
事件源 (eventsource.EventSource)
       ↓ 订阅事件
subscribe() → 写入 sniffed 通道
       ↓ 从 sniffed 读取
filter() → 写入 filtered 通道
       ↓ 从 filtered 读取
queue() → 发布到消息队列
*/
package sniffer

import (
	"context" // 上下文管理
	"fmt"     // 格式化输出
	"log"     // 基础日志
	"time"    // 时间处理

	"golang.org/x/sync/errgroup" // 错误处理组

	// "go.opentelemetry.io/otel/codes"
	"github.com/ipfs/go-datastore"  // IPFS数据存储接口
	"github.com/libp2p/go-eventbus" // 事件总线

	"github.com/ipfs-search/ipfs-search/components/queue"                           // 队列组件
	"github.com/ipfs-search/ipfs-search/components/sniffer/eventsource"             // 事件源
	"github.com/ipfs-search/ipfs-search/components/sniffer/handler"                 // 事件处理器
	filters "github.com/ipfs-search/ipfs-search/components/sniffer/providerfilters" // 过滤器
	"github.com/ipfs-search/ipfs-search/components/sniffer/queuer"                  // 队列处理器
	filter "github.com/ipfs-search/ipfs-search/components/sniffer/streamfilter"     // 流过滤器

	"github.com/ipfs-search/ipfs-search/instr"   // 性能监控工具
	t "github.com/ipfs-search/ipfs-search/types" // 类型定义
)

// Sniffer 允许嗅探批处理数据存储的事件，从而有效地嗅探 IPFS DHT（分布式哈希表）。
// 为了有效地使用 Sniffer，需要通过调用 Sniffer 上的 Batching() 来获取代理的数据存储。
type Sniffer struct {
	cfg *Config                 // 配置信息
	es  eventsource.EventSource // 事件源
	pub queue.PublisherFactory  // 消息队列工厂

	*instr.Instrumentation // 监控组件
}

// New 基于一个数据存储创建一个新的 Sniffer，或者返回一个错误。
func New(cfg *Config, ds datastore.Batching, pub queue.PublisherFactory, i *instr.Instrumentation) (*Sniffer, error) {
	bus := eventbus.NewBus() // 创建新的事件总线

	es, err := eventsource.New(bus, ds) // 创建事件源
	if err != nil {
		return nil, fmt.Errorf("failed to get eventsource: %w", err)
	}

	s := Sniffer{ // 初始化Sniffer实例
		cfg:             cfg,
		es:              es,
		pub:             pub,
		Instrumentation: i,
	}

	return &s, nil
}

// Batching 返回一个被嗅探钩子包装的数据存储。
func (s *Sniffer) Batching() datastore.Batching {
	return s.es.Batching() // 返回带有嗅探钩子的数据存储
}

// subscribe 订阅数据存储事件，将事件转换为Provider类型写入通道
func (s *Sniffer) subscribe(ctx context.Context, c chan<- t.Provider) error {
	// ctx, span := s.Tracer.Start(ctx, "sniffer.subscribe")
	// defer span.End()

	h := handler.New(c) // 创建事件处理器

	err := s.es.Subscribe(ctx, h.HandleFunc) // 订阅事件源
	// span.RecordError(err)
	// span.SetStatus(codes.Internal, err.Error())
	return err
}

// filter 对事件进行双重过滤（去重和CID过滤），防止重复处理
func (s *Sniffer) filter(ctx context.Context, in <-chan t.Provider, out chan<- t.Provider) error {
	// ctx, span := s.Tracer.Start(ctx, "sniffer.filter")
	// defer span.End()

	// 初始化两个过滤器：最近看到的内容过滤和CID过滤
	lastSeenFilter := filters.NewLastSeenFilter(s.cfg.LastSeenExpiration, s.cfg.LastSeenPruneLen)
	cidFilter := filters.NewCidFilter()
	// 组合过滤器
	mutliFilter := filters.NewMultiFilter(lastSeenFilter, cidFilter)
	// 创建过滤流处理器
	f := filter.New(mutliFilter, in, out)

	err := f.Filter(ctx) // 执行过滤
	// span.RecordError(err)
	// span.SetStatus(codes.Internal, err.Error())
	return err
}

// queue 将过滤后的内容发布到消息队列
func (s *Sniffer) queue(ctx context.Context, c <-chan t.Provider) error {
	// ctx, span := s.Tracer.Start(ctx, "sniffer.Queue")
	// defer span.End()

	publisher, err := s.pub.NewPublisher(ctx) // 创建队列发布者
	if err != nil {
		return err
	}

	q := queuer.New(publisher, c) // 创建队列处理器

	err = q.Queue(ctx) // 开始入队操作
	// span.RecordError(err)
	// span.SetStatus(codes.Internal, err.Error())
	return err
}

// iterate 使用错误处理组并发运行订阅、过滤和入队列流程
func (s *Sniffer) iterate(ctx context.Context, sniffed, filtered chan t.Provider) error {
	// ctx, span := s.Tracer.Start(ctx, "sniffer.iterate")
	// defer span.End()

	// Create error group and context
	errg, ctx := errgroup.WithContext(ctx) // 创建错误处理组
	// 并发执行三个核心流程
	errg.Go(func() error { return s.subscribe(ctx, sniffed) })        // 数据源
	errg.Go(func() error { return s.filter(ctx, sniffed, filtered) }) // 中间处理
	errg.Go(func() error { return s.queue(ctx, filtered) })           // 最终输出

	// Wait until all contexts are closed, then return *first* error
	err := errg.Wait() // 等待所有协程完成

	// span.RecordError(err)
	// span.SetStatus(codes.Internal, err.Error())

	return err
}

// Sniff 会一直嗅探，直到上下文被关闭——它会在间歇性错误发生时重新启动。
func (s *Sniffer) Sniff(ctx context.Context) error {
	// ctx, span := s.Tracer.Start(ctx, "sniffer.Sniff")
	// defer span.End()

	// 初始化缓冲通道
	sniffed := make(chan t.Provider, s.cfg.BufferSize)
	filtered := make(chan t.Provider, s.cfg.BufferSize)

	for {
		err := s.iterate(ctx, sniffed, filtered) // 运行核心流程

		// 检查上下文是否被取消
		// 关闭父上下文应该导致返回，其他错误则会导致重新启动。
		if err := ctx.Err(); err != nil {
			log.Printf("Parent context closed with error '%s', returning error", err)
			// span.RecordError(err)
			// span.SetStatus(codes.Internal, err.Error())
			return err
		}

		log.Printf("Wait group exited with error '%s', restarting", err)

		//  错误处理与重启逻辑
		// TODO: Add circuit breaker here
		log.Printf("Stubbornly restarting in 1s")
		time.Sleep(time.Second) // 简单粗暴的重试间隔
	}
}
