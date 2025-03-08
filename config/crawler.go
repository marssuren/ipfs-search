package config

import (
	"github.com/ipfs-search/ipfs-search/components/crawler"
	"time"
)

// Crawler contains configuration for a Crawler.
type Crawler struct {
	DirEntryBufferSize uint          `yaml:"direntry_buffer_size"` // 处理目录条目通道的缓冲区大小。
	MinUpdateAge       time.Duration `yaml:"min_update_age"`       // 项目更新的最小时间间隔。
	StatTimeout        time.Duration `yaml:"stat_timeout"`         // Stat() 调用的超时时间。
	DirEntryTimeout    time.Duration `yaml:"direntry_timeout"`     // 目录条目之间的超时时间。
	MaxDirSize         uint          `yaml:"max_dirsize"`          // 目录条目的最大数量。
}

// CrawlerConfig 方法从中央配置中返回组件特定的配置。
func (c *Config) CrawlerConfig() *crawler.Config {
	cfg := crawler.Config(c.Crawler)
	return &cfg
}

// CrawlerDefaults 函数封装了组件特定配置的默认值。
func CrawlerDefaults() Crawler {
	return Crawler(*crawler.DefaultConfig())
}
