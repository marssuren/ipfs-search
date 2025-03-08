package config

import (
	"github.com/ipfs-search/ipfs-search/instr"
)

// Instr specifies the configuration for instrumentation.
type Instr struct {
	SamplingRatio  float64 `yaml:"sampling_ratio" env:"OTEL_TRACE_SAMPLER_ARG"`         // 采样比例（被跟踪的哈希的比例）。默认为 `0.01`（1%）。由于某些原因，设置这个环境变量选项会失败。
	JaegerEndpoint string  `yaml:"jaeger_endpoint" env:"OTEL_EXPORTER_JAEGER_ENDPOINT"` // 发送 span 到 Jaeger 的 HTTP 端点，例如 `http://jaeger:14268/api/traces`。
}

// InstrConfig 方法从中央配置中返回组件特定的配置。
func (c *Config) InstrConfig() *instr.Config {
	cfg := instr.Config(c.Instr)
	return &cfg
}

// InstrDefaults 函数返回组件配置的默认值，基于组件特定的配置。
func InstrDefaults() Instr {
	return Instr(*instr.DefaultConfig())
}
