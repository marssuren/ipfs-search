// Package crawler 围绕 Crawler 组件构建，用于从 AnnotatedResource 爬取和索引内容
package crawler

import (
	"context"
	"errors"
	"log"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ipfs-search/ipfs-search/components/extractor"
	"github.com/ipfs-search/ipfs-search/components/protocol"

	"github.com/ipfs-search/ipfs-search/instr"
	t "github.com/ipfs-search/ipfs-search/types"
)

// Crawler 允许爬取资源
type Crawler struct {
	config     *Config               // 配置信息
	indexes    *Indexes              // 索引管理
	queues     *Queues               // 队列管理
	protocol   protocol.Protocol     // 协议处理
	extractors []extractor.Extractor // 提取器列表

	*instr.Instrumentation // 插桩工具
}

// isSupportedType 检查资源类型是否支持
func isSupportedType(rType t.ResourceType) bool {
	switch rType {
	case t.UndefinedType, t.FileType, t.DirectoryType:
		return true
	default:
		return false
	}
}

// Crawl 更新现有资源或爬取新资源，并在适用时提取元数据
func (c *Crawler) Crawl(ctx context.Context, r *t.AnnotatedResource) error {
	// 创建追踪 span
	ctx, span := c.Tracer.Start(ctx, "crawler.Crawl",
		trace.WithAttributes(attribute.String("cid", r.ID)),
	)
	defer span.End()

	var err error
	// 检查协议有效性
	if r.Protocol == t.InvalidProtocol {
		// Sending items with an invalid protocol to Crawl() is a programming error and
		// should never happen.
		panic("invalid protocol")
	}
	// 检查资源类型是否支持
	if !isSupportedType(r.Type) {
		// Calling crawler with unsupported types is undefined behaviour.
		panic("invalid type for crawler")
	}
	// 检查并更新可能存在的资源
	exists, err := c.updateMaybeExisting(ctx, r)
	if err != nil {
		span.RecordError(err)
		return err
	}
	// 如果资源已存在，直接返回
	if exists {
		log.Printf("Done processing existing resource: %v", r)
		span.AddEvent("existing resource")
		return nil
	}
	// 确保资源类型已定义
	if err := c.ensureType(ctx, r); err != nil {
		if errors.Is(err, t.ErrInvalidResource) {
			// Resource is invalid, index as such, throwing away ErrInvalidResource in favor of the result of indexing operation.
			log.Printf("Indexing invalid resource %v", r)
			span.AddEvent("Indexing invalid resource")

			err = c.indexInvalid(ctx, r, err)
		}

		// Errors from ensureType imply that no type could be found, hence we can't index.
		if err != nil {
			span.RecordError(err)
		}
		return err
	}
	// 索引新资源
	log.Printf("Indexing new item %v", r)
	err = c.index(ctx, r)
	if err != nil {
		span.RecordError(err)
	}
	return err
}

// New 创建一个新的 Crawler 实例
func New(config *Config, indexes *Indexes, queues *Queues, protocol protocol.Protocol, extractors []extractor.Extractor, i *instr.Instrumentation) *Crawler {
	return &Crawler{
		config,
		indexes,
		queues,
		protocol,
		extractors,
		i,
	}
}

// 确保资源类型已定义
func (c *Crawler) ensureType(ctx context.Context, r *t.AnnotatedResource) error {
	ctx, span := c.Tracer.Start(ctx, "crawler.ensureType")
	defer span.End()

	var err error
	// 如果类型未定义，则通过协议获取状态
	if r.Type == t.UndefinedType {
		ctx, cancel := context.WithTimeout(ctx, c.config.StatTimeout)
		defer cancel()

		err = c.protocol.Stat(ctx, r)
		if err != nil {
			span.RecordError(err)
		}
	}

	return err
}
