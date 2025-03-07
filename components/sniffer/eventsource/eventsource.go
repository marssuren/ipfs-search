// Package eventsource 这个事件源组件是IPFS嗅探器的核心部分，通过代理IPFS数据存储来监听和转发提供者事件，从而让嗅探器能够收集网络上的CID信息。
package eventsource

import (
	"context"
	"fmt"
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-eventbus"
	"time"

	"github.com/libp2p/go-libp2p-core/event"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/ipfs-search/ipfs-search/components/sniffer/proxy"
	"github.com/ipfs-search/ipfs-search/instr"
)

// 定义事件总线缓冲大小为512
const bufSize = 512

// 处理超时时间
var handleTimeout = time.Second

// 定义处理函数类型
type handleFunc func(context.Context, EvtProviderPut) error

// EventSource 通过代理 Batching datastore 在 Bus 上生成 Put 操作后的事件
type EventSource struct {
	bus     event.Bus
	emitter event.Emitter
	ds      datastore.Batching
	*instr.Instrumentation
}

// New  创建新的 EventSource 实例
func New(b event.Bus, ds datastore.Batching) (EventSource, error) {
	// 创建事件发射器
	e, err := b.Emitter(new(EvtProviderPut))
	if err != nil {
		return EventSource{}, err
	}

	s := EventSource{
		bus:             b,
		emitter:         e,
		Instrumentation: instr.New(),
	}
	// 使用代理包装数据存储
	s.ds = proxy.New(ds, s.afterPut)

	return s, nil
}

// Put 操作后的回调函数
func (s *EventSource) afterPut(k datastore.Key, v []byte, err error) error {
	// 创建追踪 span
	_, span := s.Tracer.Start(context.TODO(), "eventsource.afterPut")
	defer span.End()

	// 忽略有错误的 Put 操作
	if err != nil {
		span.RecordError(err)
		return err
	}

	// 忽略非提供者的键
	if !isProviderKey(k) {
		span.RecordError(fmt.Errorf("Non-provider key"))
		return nil
	}

	// 从键中获取 CID
	cid, err := keyToCID(k)
	if err != nil {
		span.RecordError(fmt.Errorf("cid from key '%s': %w", k, err))
		return nil
	}

	// 从键中获取 PeerID
	pid, err := keyToPeerID(k)
	if err != nil {
		span.RecordError(fmt.Errorf("pid from key '%s': %w", k, err))
		return nil
	}

	// 设置 span 属性
	span.SetAttributes(
		attribute.Stringer("cid", cid),
		attribute.Stringer("peerid", pid),
	)

	// 创建并发送事件
	e := EvtProviderPut{
		CID:         cid,
		PeerID:      pid,
		SpanContext: span.SpanContext(),
	}

	if err := s.emitter.Emit(e); err != nil {
		span.RecordError(err)
	} else {
		span.SetStatus(codes.Ok, "emitted")
	}

	// Return *original* error
	return err
}

// Batching 返回代理的数据存储实例
func (s *EventSource) Batching() datastore.Batching {
	return s.ds
}

// iterate 迭代处理事件
func (s *EventSource) iterate(ctx context.Context, c <-chan interface{}, h handleFunc) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case e, ok := <-c:
		if !ok {
			return fmt.Errorf("reading from event bus")
		}

		evt, ok := e.(EvtProviderPut)
		if !ok {
			return fmt.Errorf("casting event: %v", evt)
		}

		// Timeout handler to expose issues on the handler side
		ctx, cancel := context.WithTimeout(ctx, handleTimeout)
		defer cancel()

		return h(ctx, evt)
	}
}

// Subscribe 订阅 EvtProviderPut 事件
func (s *EventSource) Subscribe(ctx context.Context, h handleFunc) error {
	sub, err := s.bus.Subscribe(new(EvtProviderPut), eventbus.BufSize(bufSize))
	if err != nil {
		return fmt.Errorf("subscribing: %w", err)
	}
	defer sub.Close()

	c := sub.Out()

	for {
		if err := s.iterate(ctx, c, h); err != nil {
			return err
		}
	}
}
