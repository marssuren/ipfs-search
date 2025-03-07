package crawler

import (
	"context"
	"errors"
	"log"
	"math/rand"

	"golang.org/x/sync/errgroup" // 用于并发控制

	indexTypes "github.com/ipfs-search/ipfs-search/components/index/types"
	t "github.com/ipfs-search/ipfs-search/types"
)

var (
	// ErrDirectoryTooLarge 公开错误，表示目录过大
	ErrDirectoryTooLarge = t.WrappedError{Err: t.ErrInvalidResource, Msg: "directory too large"}

	// errEndOfLs 内部错误，用作列表结束信号
	errEndOfLs = errors.New("end of list")
)

// Crawl 核心目录爬取方法。
func (c *Crawler) crawlDir(ctx context.Context, r *t.AnnotatedResource, properties *indexTypes.Directory) error {
	ctx, span := c.Tracer.Start(ctx, "crawler.crawlDir") // 开始跟踪
	defer span.End()

	entries := make(chan *t.AnnotatedResource, c.config.DirEntryBufferSize) // 带缓冲的条目通道

	wg, ctx := errgroup.WithContext(ctx) // 创建错误组

	var panicVar interface{} // panic捕获变量

	defer func() { // 确保panic传播
		// Propagate panic
		if panicVar != nil {
			panic(panicVar)
		}
	}()

	// 启动条目处理协程
	wg.Go(func() error {
		defer func() { // panic捕获
			if r := recover(); r != nil {
				panicVar = r
			}
		}()
		return c.processDirEntries(ctx, entries, properties)
	})

	// 启动目录列表协程
	wg.Go(func() error {
		defer close(entries) // 确保通道关闭
		defer func() {       // panic捕获
			if r := recover(); r != nil {
				panicVar = r
			}
		}()
		return c.protocol.Ls(ctx, r, entries) // tocheck: protocol.Ls 的具体实现
	})

	return wg.Wait() // 等待所有协程完成
}

// 资源类型转换
func resourceToLinkType(r *t.AnnotatedResource) indexTypes.LinkType {
	switch r.Type {
	case t.FileType:
		return indexTypes.FileLinkType
	case t.DirectoryType:
		return indexTypes.DirectoryLinkType
	case t.UndefinedType:
		return indexTypes.UnknownLinkType
	case t.UnsupportedType:
		return indexTypes.UnsupportedLinkType
	default:
		panic("unexpected type") // 确保类型处理完整
	}
}

// 添加目录链接
func addLink(e *t.AnnotatedResource, properties *indexTypes.Directory) {
	properties.Links = append(properties.Links, indexTypes.Link{
		Hash: e.ID,
		Name: e.Reference.Name,
		Size: e.Size,
		Type: resourceToLinkType(e),
	})
}

// 处理目录条目（核心逻辑）
func (c *Crawler) processDirEntries(ctx context.Context, entries <-chan *t.AnnotatedResource, properties *indexTypes.Directory) error {
	ctx, span := c.Tracer.Start(ctx, "crawler.processDirEntries")
	defer span.End()

	var (
		dirCnt  uint = 0     // 条目计数器
		isLarge bool = false // 大目录标记
	)

	// Question: do we need a maximum entry cutoff point? E.g. 10^6 entries or something?
	processNextDirEntry := func() error { // 闭包函数处理单个条目
		// Create (and cancel!) a new timeout context for every entry.
		ctx, cancel := context.WithTimeout(ctx, c.config.DirEntryTimeout) // 超时控制
		defer cancel()

		select {
		case <-ctx.Done():
			return ctx.Err() // 上下文取消或超时
		case entry, ok := <-entries:
			if !ok {
				return errEndOfLs // 通道关闭
			}
			// 定期日志输出（每1024个条目）
			if dirCnt > 0 && dirCnt%1024 == 0 {
				log.Printf("Processed %d directory entries in %v.", dirCnt, entry.Parent)
				log.Printf("Latest entry: %v", entry)
			}

			// Only add to properties up to limit (preventing oversized directory entries) - but queue entries nonetheless.
			// 大目录处理逻辑
			if dirCnt == c.config.MaxDirSize {
				span.AddEvent("large-directory")
				log.Printf("Directory %v is large, crawling entries but not directory itself.", entry.Parent)
				isLarge = true // 标记但继续处理
			}

			if !isLarge {
				addLink(entry, properties) // 正常目录添加链接
			}

			return c.queueDirEntry(ctx, entry) // 条目入队
		}
	}

	var err error

	for err == nil { // 循环处理直到出错
		err = processNextDirEntry()
		dirCnt++
	}

	// 错误后处理
	if errors.Is(err, errEndOfLs) { // 正常结束
		// Normal exit of loop, reset error condition
		err = nil

		if isLarge {
			err = ErrDirectoryTooLarge // 大目录特殊错误
		}
	} else {
		// 异常错误
		// Unknown error situation: fail hard
		// Prefer less over incomplete or inconsistent data.
		log.Printf("Unexpected error processing directory entries: %v", err)
	}

	if err != nil {
		span.RecordError(err)
	}

	return err
}

// 队列分发逻辑
func (c *Crawler) queueDirEntry(ctx context.Context, r *t.AnnotatedResource) error {
	// Generate random lower priority for items in this directory
	// Rationale; directories might have different availability but
	// within a directory, items are likely to have similar availability.
	// We want consumers to get a varied mixture of availability, for
	// consistent overall indexing load.
	priority := uint8(1 + rand.Intn(7)) // 生成1-7随机优先级

	switch r.Type { // 根据类型分发队列
	case t.UndefinedType:
		return c.queues.Hashes.Publish(ctx, r, priority)
	case t.FileType:
		return c.queues.Files.Publish(ctx, r, priority)
	case t.DirectoryType:
		return c.queues.Directories.Publish(ctx, r, priority)
	case t.UnsupportedType:
		// Index right away as invalid.
		// Rationale: as no additional protocol request is required and queue'ing returns
		// similarly fast as indexing.
		return c.indexInvalid(ctx, r, t.ErrUnsupportedType) // 直接索引无效类型
	default:
		panic("unexpected type") // 类型安全防护
	}
}
