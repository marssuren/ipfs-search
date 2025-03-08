package config

import (
	"runtime"
	"time"

	"github.com/c2h5oh/datasize"
)

// OpenSearch 结构体保存了 OpenSearch 的配置。
type OpenSearch struct {
	URL                     string            `yaml:"url" env:"OPENSEARCH_URL"`  // OpenSearch 的 URL 地址，从 YAML 文件或环境变量读取。
	BulkIndexerWorkers      int               `yaml:"bulk_indexer_workers"`      // 用于索引器的工作线程数量。
	BulkIndexerFlushBytes   datasize.ByteSize `yaml:"bulk_flush_bytes"`          // 在达到这个字节数后刷新索引缓冲区。
	BulkIndexerFlushTimeout time.Duration     `yaml:"bulk_flush_timeout"`        // 在达到这个时间后刷新索引缓冲区。
	BulkGetterBatchSize     int               `yaml:"bulk_getter_batch_size"`    // 批量获取操作的最大批次大小。
	BulkGetterBatchTimeout  time.Duration     `yaml:"bulk_getter_batch_timeout"` // 在达到这个时间后执行批量操作。
}

// OpenSearchDefaults 函数返回 OpenSearch 的默认配置。
func OpenSearchDefaults() OpenSearch {
	return OpenSearch{
		URL:                     "http://localhost:9200", // 默认的 OpenSearch URL 地址。
		BulkIndexerWorkers:      runtime.NumCPU(),        // 默认的索引器工作线程数量为系统的 CPU 核心数。
		BulkIndexerFlushTimeout: 5 * time.Minute,         // 默认的索引缓冲区刷新时间为 5 分钟。
		BulkIndexerFlushBytes:   5e+6,                    // 默认的索引缓冲区刷新字节数为 5MB。
		BulkGetterBatchSize:     48,                      // 默认的批量获取操作的最大批次大小为 48。
		BulkGetterBatchTimeout:  150 * time.Millisecond,  // 默认的批量获取操作的最大等待时间为 150 毫秒。
	}
}
