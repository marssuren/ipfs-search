package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"

	opensearchutil "github.com/opensearch-project/opensearch-go/v2/opensearchutil"
	"go.opentelemetry.io/otel/codes"

	"github.com/ipfs-search/ipfs-search/components/index"
	"github.com/ipfs-search/ipfs-search/components/index/opensearch/bulkgetter"
)

const debug bool = false

// Index 包装了一个 OpenSearch 索引用于存储文档
type Index struct {
	cfg *Config
	c   *Client
}

// New 返回一个新索引。
func New(client *Client, cfg *Config) index.Index {
	if client == nil {
		panic("Index.New Client cannot be nil.") // 如果客户端为空，抛出异常。
	}

	if cfg == nil {
		panic("Index.New Config cannot be nil.") // 如果配置为空，抛出异常。
	}

	index := &Index{
		c:   client,
		cfg: cfg,
	}

	return index
}

// String 返回索引的名称，便于日志记录。
func (i *Index) String() string {
	return i.cfg.Name
}

// getBody 将一个 interface{} 序列化为 io.ReadSeeker。
func getBody(v interface{}) (io.ReadSeeker, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return bytes.NewReader(b), nil
}

// index 包装了 BulkIndexer.Add() 方法。
func (i *Index) index(
	ctx context.Context,
	action string,
	id string,
	properties interface{},
) error {
	ctx, span := i.c.Tracer.Start(ctx, "index.opensearch.index")
	defer span.End()

	var (
		body io.ReadSeeker
		err  error
	)

	if properties != nil {
		if action == "update" {
			// 对于更新操作，更新的字段需要包装在 `doc` 字段中。
			body, err = getBody(struct {
				Doc interface{} `json:"doc"`
			}{properties})
		} else {
			body, err = getBody(properties) // 序列化 properties。
		}
		if err != nil {
			panic(err)
		}
	}

	item := opensearchutil.BulkIndexerItem{
		Index:      i.cfg.Name,
		Action:     action,
		Body:       body,
		DocumentID: id,
		Version:    nil,
		OnFailure: func(
			ctx context.Context,
			item opensearchutil.BulkIndexerItem,
			res opensearchutil.BulkIndexerResponseItem, err error,
		) {
			if err == nil {
				err = fmt.Errorf("Error flushing: %+v (%s)", res, id)
			}

			span.RecordError(err)
			log.Println(err)

		},
	}

	ctx, span = i.c.Tracer.Start(ctx, "index.opensearch.bulkIndexer.Add")
	defer span.End()

	err = i.c.bulkIndexer.Add(ctx, item)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Error adding to BulkIndexer.")
	}

	return err
}

// Index 根据 id 索引文档的属性。
func (i *Index) Index(ctx context.Context, id string, properties interface{}) error {
	return i.index(ctx, "create", id, properties)
}

// Update 根据 id 更新文档的属性。
func (i *Index) Update(ctx context.Context, id string, properties interface{}) error {
	return i.index(ctx, "update", id, properties)
}

// Delete 从索引中删除项目。
func (i *Index) Delete(ctx context.Context, id string) error {
	return i.index(ctx, "delete", id, nil)
}

// Get 从索引中检索具有 `id` 的文档的 `fields`，返回：
// - (true, decoding_error) 如果找到（当 JSON 解码出错时设置解码错误）
// - (false, nil) 如果未找到
// - (false, error) 否则
func (i *Index) Get(ctx context.Context, id string, dst interface{}, fields ...string) (bool, error) {
	ctx, span := i.c.Tracer.Start(ctx, "index.opensearch.Get")
	defer span.End()

	req := bulkgetter.GetRequest{
		Index:      i.cfg.Name,
		DocumentID: id,
		Fields:     fields,
	}

	resp := <-i.c.bulkGetter.Get(ctx, &req, dst) // 异步获取文档。

	if debug {
		if resp.Found {
			log.Printf("opensearch: found %s in %s", id, i)
		} else {
			if resp.Error != nil {
				log.Printf("opensearch: error getting %s in %s: %v", id, i, resp.Error)
			}
		}
	}

	return resp.Found, resp.Error
}

// 编译时保证实现满足接口要求。
var _ index.Index = &Index{}
