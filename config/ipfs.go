package config

import (
	"github.com/c2h5oh/datasize"                                  // 处理数据大小单位解析（如 "100MB" → uint64）
	"github.com/ipfs-search/ipfs-search/components/protocol/ipfs" // 组件内部IPFS协议实现 tocheck: 默认配置来源
)

// IPFS 结构体定义协议层配置（API、网关、分块大小）
// 示例配置
// ipfs:
//
//	api_url: "http://10.0.0.2:5001"     # 自定义远程IPFS节点
//	gateway_url: "https://ipfs.io"      # 使用公共网关
//	partial_size: "2MB"                 # 仅下载前2MB内容
type IPFS struct {
	APIURL      string            `yaml:"api_url" env:"IPFS_API_URL"`         // IPFS API地址（默认：localhost:5001） 用途：连接本地或远程IPFS节点的API端点（如 http://127.0.0.1:5001）。
	GatewayURL  string            `yaml:"gateway_url" env:"IPFS_GATEWAY_URL"` // 网关地址（默认：localhost:8080）用途：访问IPFS网关的URL，用于内容检索（如 http://localhost:8080/ipfs/）。
	PartialSize datasize.ByteSize `yaml:"partial_size"`                       // 部分内容下载大小限制（如仅下载文件头） 用途：定义从IPFS下载时的部分内容大小限制（例如仅下载前1MB用于元数据解析），避免大文件全量下载。
}

// IPFSConfig 将全局Config中的IPFS配置转换为组件所需的ipfs.Config类型
func (c *Config) IPFSConfig() *ipfs.Config {
	cfg := ipfs.Config(c.IPFS) // 类型转换（依赖结构体字段完全匹配）
	return &cfg
}

// IPFSDefaults 返回IPFS协议的默认配置（基于组件内部默认值）
func IPFSDefaults() IPFS {
	return IPFS(*ipfs.DefaultConfig()) // 调用组件自身的DefaultConfig()
}
