package config

// Redis 结构体保存了 Redis 的配置。
type Redis struct {
	Addresses []string `yaml:"addresses" env:"REDIS_ADDRESSES"` // Redis 的地址列表，从 YAML 文件或环境变量读取。
}

// RedisDefaults 函数返回 Redis 的默认配置。
func RedisDefaults() Redis {
	return Redis{
		Addresses: []string{"localhost:6379"}, // 默认的 Redis 地址为 "localhost:6379"。
	}
}
