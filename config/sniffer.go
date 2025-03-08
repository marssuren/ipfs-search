package config

import (
	"github.com/ipfs-search/ipfs-search/components/sniffer"
	"time"
)

// Sniffer is configuration pertaining to the sniffer
type Sniffer struct {
	LastSeenExpiration time.Duration `yaml:"lastseen_expiration" env:"SNIFFER_LASTSEEN_EXPIRATION"` // 最后一次见到的记录过期时间。
	LastSeenPruneLen   int           `yaml:"lastseen_prunelen" env:"SNIFFER_LASTSEEN_PRUNELEN"`     // 修剪最后一次见到记录的长度。
	LoggerTimeout      time.Duration `yaml:"logger_timeout"`                                        // 日志记录器的超时时间。
	BufferSize         uint          `yaml:"buffer_size" env:"SNIFFER_BUFFER_SIZE"`                 // 缓冲区大小。
}

// SnifferConfig 方法从中央配置中返回组件特定的配置。
func (c *Config) SnifferConfig() *sniffer.Config {
	cfg := sniffer.Config(c.Sniffer) // 将当前配置转换为 Sniffer 配置。
	return &cfg
}

// SnifferDefaults 函数返回组件配置的默认值，基于组件特定的配置。
func SnifferDefaults() Sniffer {
	return Sniffer(*sniffer.DefaultConfig())
}
