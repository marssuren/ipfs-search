/* Handler实现了一个高效的事件转换管道，是连接底层IPFS事件与上层处理逻辑的关键桥梁
执行流程图解:
 IPFS DHT操作
	↓ 生成 EvtProviderPut 事件
 Handler.HandleFunc 被调用
	↓ 提取 CID、PeerID
 构造 t.Provider 对象
	↓ 带追踪上下文
 写入 providers 通道
	↓ 下游处理流程（过滤、入队列等）
*/

package handler // Package handler

import (
	"context" // 上下文管理
	"time"    // 时间处理

	"go.opentelemetry.io/otel/attribute" // OpenTelemetry属性
	"go.opentelemetry.io/otel/trace"     // 分布式追踪

	"github.com/ipfs-search/ipfs-search/components/sniffer/eventsource" // 事件源定义

	"github.com/ipfs-search/ipfs-search/instr"   // 可观测性工具
	t "github.com/ipfs-search/ipfs-search/types" // 类型定义
)

// Handler 处理EvtProviderPut事件，将Provider写入通道
type Handler struct {
	providers              chan<- t.Provider // 只写通道，用于传递处理后的Provider数据
	*instr.Instrumentation                   // 集成可观测性工具（埋点、监控等）
}

// New 创建新Handler实例，绑定到指定通道
func New(providers chan<- t.Provider) Handler {
	return Handler{
		providers:       providers,   // 注入输出通道
		Instrumentation: instr.New(), // 初始化监控组件
	}
}

// HandleFunc 处理EvtProviderPut事件，构造Provider并写入 Handler 的providers 通道。
func (h *Handler) HandleFunc(ctx context.Context, e eventsource.EvtProviderPut) error {
	// 将事件携带的SpanContext注入当前上下文（分布式追踪）
	ctx = trace.ContextWithRemoteSpanContext(ctx, e.SpanContext)
	// 创建新的追踪Span，记录关键属性
	ctx, span := h.Tracer.Start(ctx, "handler.HandleFunc", trace.WithAttributes(
		attribute.Stringer("cid", e.CID),       // 记录CID
		attribute.Stringer("peerid", e.PeerID), // 记录PeerID
	), trace.WithSpanKind(trace.SpanKindConsumer)) // 标记为消费者端Span
	defer span.End() // 确保Span结束

	// 构造Provider数据结构
	p := t.Provider{
		Resource: &t.Resource{
			Protocol: t.IPFSProtocol, // 固定协议为IPFS
			ID:       e.CID.String(), // 转换CID为字符串
		},
		Date:        time.Now(),         // 记录处理时间
		Provider:    e.PeerID.String(),  // 转换PeerID为字符串
		SpanContext: span.SpanContext(), // 保存当前Span上下文
	}

	// 非阻塞式写入通道（带上下文监听）
	select {
	case <-ctx.Done(): // 上下文取消时返回错误
		return ctx.Err()
	case h.providers <- p: // 成功写入通道
		return nil
	}
}
