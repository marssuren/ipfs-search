package opensearch

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jpillora/backoff"
	opensearch "github.com/opensearch-project/opensearch-go/v2"
	opensearchtransport "github.com/opensearch-project/opensearch-go/v2/opensearchtransport"
	opensearchutil "github.com/opensearch-project/opensearch-go/v2/opensearchutil"
	"go.opentelemetry.io/otel/trace"

	"github.com/ipfs-search/ipfs-search/components/index"
	"github.com/ipfs-search/ipfs-search/components/index/opensearch/bulkgetter"
	"github.com/ipfs-search/ipfs-search/instr"
)

// Client 搜索索引的客户端.
type Client struct {
	searchClient *opensearch.Client
	bulkIndexer  opensearchutil.BulkIndexer
	bulkGetter   bulkgetter.AsyncGetter

	*instr.Instrumentation
}

// ClientConfig 配置搜索索引。
type ClientConfig struct {
	URL       string
	Transport http.RoundTripper
	Debug     bool

	BulkIndexerWorkers      int
	BulkIndexerFlushBytes   int
	BulkIndexerFlushTimeout time.Duration

	BulkGetterBatchSize    int
	BulkGetterBatchTimeout time.Duration
}

// NewClient 返回一个配置好的搜索索引客户端，或者一个错误。
func NewClient(cfg *ClientConfig, i *instr.Instrumentation) (*Client, error) {
	var (
		c   *opensearch.Client
		bi  opensearchutil.BulkIndexer
		bg  bulkgetter.AsyncGetter
		err error
	)

	if cfg == nil {
		panic("NewClient ClientConfig cannot be nil.") // 如果配置为空，抛出异常。
	}

	if i == nil {
		panic("NewCLient Instrumentation cannot be nil.") // 如果Instrumentation为空，抛出异常。
	}

	if c, err = getSearchClient(cfg, i); err != nil {
		return nil, err // 获取搜索客户端，如果出错，返回错误。
	}

	if bi, err = getBulkIndexer(c, cfg, i); err != nil {
		return nil, err // 获取批量索引器，如果出错，返回错误。
	}

	if bg, err = getBulkGetter(c, cfg, i); err != nil {
		return nil, err // 获取批量获取器，如果出错，返回错误。
	}

	return &Client{
		searchClient:    c,
		bulkIndexer:     bi,
		bulkGetter:      bg,
		Instrumentation: i,
	}, nil // 返回配置好的客户端。
}

// Work 启动（并关闭）一个客户端工作器。
func (c *Client) Work(ctx context.Context) error {
	// 在上下文关闭时刷新索引缓冲区。
	// 使用后台上下文，因为当前上下文已经关闭。
	defer c.bulkIndexer.Close(context.Background())

	return c.bulkGetter.Work(ctx) // 启动批量获取器的工作。
}

// NewIndex 根据给定的名称返回一个新索引。
func (c *Client) NewIndex(name string) index.Index {
	return New(
		c,
		&Config{Name: name},
	)
}

func getSearchClient(cfg *ClientConfig, i *instr.Instrumentation) (*opensearch.Client, error) {
	b := backoff.Backoff{
		Factor: 2.0,
		Jitter: true,
	}

	// 参考：https://pkg.go.dev/github.com/opensearch-project/opensearch-go@v1.0.0#Config
	clientConfig := opensearch.Config{
		Addresses:    []string{cfg.URL},
		Transport:    cfg.Transport,
		DisableRetry: cfg.Debug,
		// 重试/退避管理
		// https://www.elastic.co/guide/en/opensearch/reference/master/tune-for-indexing-speed.html#multiple-workers-threads
		RetryOnStatus:        []int{429, 502, 503, 504},
		EnableRetryOnTimeout: true,
		RetryBackoff:         func(i int) time.Duration { return b.ForAttempt(float64(i)) },
		// 分散查询/负载；在启动时发现节点，并每5分钟再次执行。
		DiscoverNodesOnStart:  true,
		DiscoverNodesInterval: 5 * time.Minute,
	}

	if cfg.Debug {
		clientConfig.Logger = &opensearchtransport.TextLogger{
			Output:             log.Default().Writer(),
			EnableRequestBody:  cfg.Debug,
			EnableResponseBody: cfg.Debug,
		}
	}

	return opensearch.NewClient(clientConfig)
}

// getBulkIndexer 返回一个配置好的 BulkIndexer 或者一个错误。
func getBulkIndexer(client *opensearch.Client, cfg *ClientConfig, i *instr.Instrumentation) (opensearchutil.BulkIndexer, error) {
	iCfg := opensearchutil.BulkIndexerConfig{
		Client:        client,                      // 设置 OpenSearch 客户端。
		NumWorkers:    cfg.BulkIndexerWorkers,      // 设置批量索引器的工作线程数。
		FlushBytes:    cfg.BulkIndexerFlushBytes,   // 设置批量索引器的刷新字节数。
		FlushInterval: cfg.BulkIndexerFlushTimeout, // 设置批量索引器的刷新时间间隔。
		OnFlushStart: func(ctx context.Context) context.Context {
			// 在刷新开始时，启动一个新的追踪 span。
			newCtx, _ := i.Tracer.Start(ctx, "index.opensearch.BulkIndexerFlush")
			return newCtx
		},
		OnError: func(ctx context.Context, err error) {
			// 在发生错误时，记录错误并打印日志。
			span := trace.SpanFromContext(ctx)
			span.RecordError(err)
			log.Printf("Error flushing index buffer: %s", err)
		},
		OnFlushEnd: func(ctx context.Context) {
			// 在刷新结束时，结束追踪 span 并打印日志。
			span := trace.SpanFromContext(ctx)
			log.Println("Flushed index buffer")

			// log.Printf("ES stats: %+v", )
			span.End()
		},
	}

	// 如果在调试模式下，将刷新字节数设置为1，并禁用刷新时间间隔。
	if cfg.Debug {
		iCfg.FlushBytes = 1
		iCfg.FlushInterval = 0
	}

	// 返回配置好的 BulkIndexer。
	return opensearchutil.NewBulkIndexer(iCfg)
}

// getBulkGetter 返回一个配置好的 AsyncGetter 或者一个错误。
func getBulkGetter(client *opensearch.Client, cfg *ClientConfig, i *instr.Instrumentation) (bulkgetter.AsyncGetter, error) {
	// 配置 BulkGetter 的配置。
	bgCfg := bulkgetter.Config{
		Client:       client,                     // 设置 OpenSearch 客户端。
		BatchSize:    cfg.BulkGetterBatchSize,    // 设置批量获取器的批处理大小。
		BatchTimeout: cfg.BulkGetterBatchTimeout, // 设置批量获取器的批处理超时时间。
	}

	// 返回配置好的 BulkGetter。
	return bulkgetter.New(bgCfg), nil
}
