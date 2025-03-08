package config

import (
	"time"

	"github.com/c2h5oh/datasize"

	"github.com/ipfs-search/ipfs-search/components/extractor/tika"
)

// Tika 结构体保存了与 sniffer 相关的配置。
type Tika struct {
	TikaExtractorURL string            `yaml:"url" env:"TIKA_EXTRACTOR"` // Tika 提取器的 URL，从 YAML 文件或环境变量读取。
	RequestTimeout   time.Duration     `yaml:"timeout"`                  // 请求超时时间。
	MaxFileSize      datasize.ByteSize `yaml:"max_file_size"`            // 最大文件大小。
}

// TikaConfig 方法从中央配置中返回组件特定的配置。
func (c *Config) TikaConfig() *tika.Config {
	cfg := tika.Config(c.Tika) // 将当前配置转换为 Tika 配置。
	return &cfg
}

// TikaDefaults 函数返回组件配置的默认值，基于组件特定的配置。
func TikaDefaults() Tika {
	return Tika(*tika.DefaultConfig()) // 返回 Tika 默认配置的副本。
}
