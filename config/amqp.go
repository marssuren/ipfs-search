package config

import (
	"github.com/ipfs-search/ipfs-search/components/queue/amqp"
	"time"
)

// AMQP 结构体包含了有关 AMQP 的配置。
type AMQP struct {
	URL           string        `yaml:"url" env:"AMQP_URL"`                 // AMQP 服务器的 URL
	MaxReconnect  int           `yaml:"max_reconnect"`                      // 服务器连接丢失后重连尝试的最大次数，使用 yaml:"max_reconnect" 标签来指定 YAML 配置文件中的名称。
	ReconnectTime time.Duration `yaml:"reconnect_time"`                     // 重连尝试之间的等待时间，使用 yaml:"reconnect_time" 标签来指定 YAML 配置文件中的名称。
	MessageTTL    time.Duration `yaml:"message_ttl" env:"AMQP_MESSAGE_TTL"` // 队列中消息的过期时间，使用 yaml:"message_ttl" 和 env:"AMQP_MESSAGE_TTL" 标签来指定 YAML 配置文件和环境变量中的名称。
}

// AMQPConfig 函数从规范配置中返回特定组件的配置。
func (c *Config) AMQPConfig() *amqp.Config {
	cfg := amqp.Config(c.AMQP) // 将 c.AMQP 转换为 amqp.Config 类型，并赋值给 cfg。
	return &cfg
}

// AMQPDefaults 函数基于特定组件的配置返回默认配置。
func AMQPDefaults() AMQP {
	return AMQP(*amqp.DefaultConfig()) // 调用 amqp.DefaultConfig() 获取默认配置，并将其转换为 AMQP 类型后返回。
}
