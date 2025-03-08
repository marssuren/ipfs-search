package config

import (
	"time"

	"github.com/c2h5oh/datasize"

	"github.com/ipfs-search/ipfs-search/components/extractor/nsfw"
)

// NSFW 结构体保存了与 sniffer 相关的配置。
type NSFW struct {
	NSFWServerURL  string            `yaml:"url" env:"NSFW_URL"` // NSFW 服务器的 URL，从 YAML 文件或环境变量读取。
	RequestTimeout time.Duration     `yaml:"timeout"`            // 请求超时时间。
	MaxFileSize    datasize.ByteSize `yaml:"max_file_size"`      // 最大文件大小。
}

// NSFWConfig 方法从中央配置中返回组件特定的配置。
func (c *Config) NSFWConfig() *nsfw.Config {
	cfg := nsfw.Config(c.NSFW)
	return &cfg
}

// NSFWDefaults 函数返回组件配置的默认值，基于组件特定的配置。
func NSFWDefaults() NSFW {
	return NSFW(*nsfw.DefaultConfig()) // 返回 NSFW 默认配置的副本。
}
