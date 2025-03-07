package crawler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ipfs-search/ipfs-search/components/extractor" // 元数据提取器
	"github.com/ipfs-search/ipfs-search/components/index"     // 索引接口
	indexTypes "github.com/ipfs-search/ipfs-search/components/index/types"
	t "github.com/ipfs-search/ipfs-search/types" // 类型定义
)

// 创建基础文档结构（核心公共字段）
func makeDocument(r *t.AnnotatedResource) indexTypes.Document {
	now := time.Now().UTC()

	// 截断到秒级兼容ES格式
	// This can be safely removed after the next reindex with _nomillis removed from time format.
	now = now.Truncate(time.Second)

	var references []indexTypes.Reference
	if r.Reference.Parent != nil {
		references = []indexTypes.Reference{
			{
				ParentHash: r.Reference.Parent.ID,
				Name:       r.Reference.Name,
			},
		}
	}

	// Common Document properties
	return indexTypes.Document{
		FirstSeen:  now,        // 首次发现时间
		LastSeen:   now,        // 最后发现时间
		References: references, // 引用
		Size:       r.Size,     // 资源大小
	}
}

// 索引无效资源
func (c *Crawler) indexInvalid(ctx context.Context, r *t.AnnotatedResource, err error) error {
	// 将错误信息存入Invalid索引
	return c.indexes.Invalids.Index(ctx, r.ID, &indexTypes.Invalid{
		Error: err.Error(), // 存储错误信息字符串
	})
}

// 获取文件属性（含元数据提取）
func (c *Crawler) getFileProperties(ctx context.Context, r *t.AnnotatedResource) (interface{}, error) {
	var err error

	span := trace.SpanFromContext(ctx) // 获取当前追踪span

	properties := &indexTypes.File{
		Document: makeDocument(r), // 基础文档
	}

	// 顺序执行提取器（可能存在依赖关系）
	for _, e := range c.extractors {
		err = e.Extract(ctx, r, properties)
		if errors.Is(err, extractor.ErrFileTooLarge) { // 处理过大文件
			// Interpret files which are too large as invalid resources; prevent repeated attempts.
			span.RecordError(err)
			return nil, fmt.Errorf("%w: %v", t.ErrInvalidResource, err) // 包装错误
		}
	}

	return properties, err
}

// 获取目录属性（触发目录爬取）
func (c *Crawler) getDirectoryProperties(ctx context.Context, r *t.AnnotatedResource) (interface{}, error) {
	properties := &indexTypes.Directory{
		Document: makeDocument(r),
	}
	err := c.crawlDir(ctx, r, properties) // 触发目录爬取（需确认crawlDir实现）

	return properties, err
}

// 资源类型分发器（核心路由逻辑）
func (c *Crawler) getProperties(ctx context.Context, r *t.AnnotatedResource) (index.Index, interface{}, error) {
	var err error

	span := trace.SpanFromContext(ctx)

	switch r.Type {
	case t.FileType:
		f, err := c.getFileProperties(ctx, r)

		return c.indexes.Files, f, err

	case t.DirectoryType:
		d, err := c.getDirectoryProperties(ctx, r)

		return c.indexes.Directories, d, err

	case t.UnsupportedType: // 不支持的资源类型
		// Index unsupported items as invalid.
		err = t.ErrUnsupportedType
		span.RecordError(err)

		return nil, nil, err

	case t.PartialType: // 部分资源
		// Index partial (no properties)
		return c.indexes.Partials, &indexTypes.Partial{}, nil // 空属性

	case t.UndefinedType: // 未定义类型（异常情况）
		panic("undefined type after Stat call")

	default:
		panic("unexpected type")
	}
}

// 主索引入口方法
func (c *Crawler) index(ctx context.Context, r *t.AnnotatedResource) error {
	// 创建带资源类型的追踪span
	ctx, span := c.Tracer.Start(ctx, "crawler.index",
		trace.WithAttributes(attribute.Stringer("type", r.Type)),
	)
	defer span.End()
	// 获取索引类型和属性
	index, properties, err := c.getProperties(ctx, r)

	if err != nil {
		if errors.Is(err, t.ErrInvalidResource) { // 无效资源特殊处理
			log.Printf("Indexing invalid '%v', err: %v", r, err)
			span.RecordError(err)
			return c.indexInvalid(ctx, r, err) // 存入invalid索引
		}

		return err
	}

	// 执行实际索引操作
	return index.Index(ctx, r.ID, properties)
}
