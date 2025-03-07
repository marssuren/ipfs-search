package crawler

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	index_types "github.com/ipfs-search/ipfs-search/components/index/types"
	t "github.com/ipfs-search/ipfs-search/types"
)

// 添加引用，返回更新后的引用列表和是否有新增
func appendReference(refs index_types.References, r *t.Reference) (index_types.References, bool) {
	if r.Parent == nil {
		// 没有新引用，不更新
		// 注意：这种情况不应该发生，可能是一个 bug
		return refs, false
	}

	// 检查是否已存在相同引用
	for _, indexedRef := range refs {
		if indexedRef.ParentHash == r.Parent.ID && indexedRef.Name == r.Name {
			// Existing reference, not updating
			return refs, false
		}
	}

	// 添加新引用
	return append(refs, index_types.Reference{
		ParentHash: r.Parent.ID,
		Name:       r.Name,
	}), true
}

// updateExisting 更新已知存在的项目
func (c *Crawler) updateExisting(ctx context.Context, i *existingItem) error {
	ctx, span := c.Tracer.Start(ctx, "crawler.updateExisting")
	defer span.End()

	switch i.Source {
	case t.DirectorySource:
		// 从目录引用的Item, 考虑更新引用（但不更新最后访问时间）
		refs, refsUpdated := appendReference(i.References, &i.AnnotatedResource.Reference)

		if refsUpdated {
			span.AddEvent("Updating",
				trace.WithAttributes(
					attribute.String("reason", "reference-added"),
					attribute.Stringer("new-reference", &i.AnnotatedResource.Reference),
				))

			return i.Index.Update(ctx, i.AnnotatedResource.ID, &index_types.Update{
				References: refs,
			})
		}

	case t.SnifferSource, t.UnknownSource:
		// TODO: Remove UnknownSource after sniffer is updated and queue is flushed.
		// Item sniffed, conditionally update last-seen.
		now := time.Now()

		// 去除毫秒以适配旧的 ES 索引格式
		// This can be safely removed after the next reindex with _nomillis removed from time format.
		now = now.Truncate(time.Second)

		var isRecent bool
		if i.LastSeen == nil {
			log.Printf("LastSeen 为空，强制设置 isRecent 为 true")
			isRecent = true
		} else {
			isRecent = now.Sub(*i.LastSeen) > c.config.MinUpdateAge
		}

		if isRecent {
			span.AddEvent("Updating",
				trace.WithAttributes(
					attribute.String("reason", "is-recent")))
			// TODO: This causes a panic when LastSeen is nil.
			// attribute.Stringer("last-seen", i.LastSeen),

			return i.Index.Update(ctx, i.AnnotatedResource.ID, &index_types.Update{
				LastSeen: &now,
			})
		}

	case t.ManualSource, t.UserSource:
		// 不基于手动或用户输入进行更新

	default:
		// Panic for unexpected Source values, instead of hard failing.
		panic(fmt.Sprintf("Unexpected source %s for item %+v", i.Source, i))
	}

	span.AddEvent("Not updating")

	return nil
}

// deletePartial 删除部分项目
func (c *Crawler) deletePartial(ctx context.Context, i *existingItem) error {
	return c.indexes.Partials.Delete(ctx, i.ID)
}

// processPartial 处理索引中发现的部分项目
func (c *Crawler) processPartial(ctx context.Context, i *existingItem) (bool, error) {
	if i.Reference.Parent == nil {
		log.Printf("快速跳过未引用的部分项目 %v", i)

		// Skip unreferenced partial
		return true, nil
	}

	// Referenced partial; delete as partial
	if err := c.deletePartial(ctx, i); err != nil {
		return true, err
	}

	// Index item as new
	return false, nil
}

// 处理已存在的项目
func (c *Crawler) processExisting(ctx context.Context, i *existingItem) (bool, error) {
	switch i.Index {
	case c.indexes.Invalids:
		// 已标记为无效，处理完成
		return true, nil
	case c.indexes.Partials:
		return c.processPartial(ctx, i)
	}

	//  更新项目并完成
	if err := c.updateExisting(ctx, i); err != nil {
		return true, err
	}

	return true, nil
}

// updateMaybeExisting 检查并更新可能存在的项目
func (c *Crawler) updateMaybeExisting(ctx context.Context, r *t.AnnotatedResource) (bool, error) {
	ctx, span := c.Tracer.Start(ctx, "crawler.updateMaybeExisting")
	defer span.End()

	existing, err := c.getExistingItem(ctx, r)
	if err != nil {
		return false, err
	}

	// Process existing item
	if existing != nil {
		if span.IsRecording() {
			span.AddEvent("existing") //, trace.WithAttributes(attribute.Stringer("index", existing.Index)))
		}

		return c.processExisting(ctx, existing)
	}

	return false, nil
}
