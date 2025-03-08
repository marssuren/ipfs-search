/*
Package config 包提供了组件配置的中央规范表示、读取、解析和验证功能。

配置由两种表示形式组成：

1. 组件特定配置 - 各组件的 Config 结构体和 DefaultConfig() 默认值生成器，
   它们不依赖于也不感知其他组件或其配置。

2. 中央规范配置（本包）- 导入并封装各种组件配置，将它们连接到单个 Config 结构体中。

例如，crawler 包包含 Config 结构体和 DefaultConfig() 函数。config 包包含一个
Crawler 结构体，它封装了来自 crawler 包的 Config，以及 CrawlerDefaults()
函数封装了来自 crawler 包的 DefaultConfig() 函数。封装后的 Crawler 结构体
提供了从 YAML 配置文件和/或操作系统环境变量读取配置值的标签。

Crawler 结构体被包含在 Config 结构体中，后者执行配置文件和操作系统环境变量的
读取、解析和验证。要从 Config 结构体获取 crawler 特定的配置，必须调用
CrawlerConfig() 方法。
*/

package config

import (
	"fmt"
	"io/ioutil" // 文件读写
	"log"       // 日志
	"os"        // 系统交互
	"strings"   // 字符串处理

	env "github.com/ipfs-search/go-env" // 环境变量解析库 tocheck: 具体实现如何映射环境变量
	yaml "gopkg.in/yaml.v3"             // YAML处理库
)

// Config 聚合所有组件配置的顶级结构
type Config struct {
	IPFS       `yaml:"ipfs"`       // IPFS节点配置
	OpenSearch `yaml:"opensearch"` // OpenSearch配置
	Redis      `yaml:"redis"`      // Redis配置
	AMQP       `yaml:"amqp"`       // RabbitMQ配置
	Tika       `yaml:"tika"`       // Tika文本解析服务配置
	NSFW       `yaml:"nsfw"`       // NSFW内容检测配置

	Instr   `yaml:"instrumentation"` // 监控指标配置
	Crawler `yaml:"crawler"`         // 爬虫组件配置
	Sniffer `yaml:"sniffer"`         // 嗅探器配置
	Indexes `yaml:"indexes"`         // 索引定义
	Queues  `yaml:"queues"`          // 消息队列定义
	Workers `yaml:"workers"`         // 工作线程池配置
}

// 将Config序列化为YAML字符串（调试用）
func (c *Config) String() string {
	bs, err := yaml.Marshal(c) // tocheck: 各子结构是否实现yaml序列化
	if err != nil {
		log.Fatalf("YAML序列化失败: %v", err) // 致命错误直接退出
	}
	return string(bs)
}

// ReadFromFile 从YAML文件读取配置（覆盖默认值）
func (c *Config) ReadFromFile(filename string) error {
	yamlFile, err := ioutil.ReadFile(filename) // 读取文件内容
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(yamlFile, c) // 反序列化到Config结构体
	if err != nil {
		return err
	}

	return nil
}

// ReadFromEnv 从环境变量读取配置（覆盖文件/默认值）
func (c *Config) ReadFromEnv() error {
	_, err := env.UnmarshalFromEnviron(c) // tocheck: 环境变量命名规则（如IPFS_ADDRESS）

	if err != nil {
		return err
	}
	return nil
}

// Check 验证必填字段是否已设置
func (c *Config) Check() error {
	zeroElements := findZeroElements(*c) // tocheck: findZeroElements实现（检查零值字段）
	if len(zeroElements) > 0 {
		return fmt.Errorf("缺失配置项: %s", strings.Join(zeroElements, ", "))

	}

	return nil
}

// Marshall 序列化为YAML字节流
func (c *Config) Marshall() ([]byte, error) {
	return yaml.Marshal(c)
}

// 将配置写入文件（生成默认配置时使用）
func (c *Config) Write(configFile string) error {
	bytes, err := c.Marshall()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(configFile, bytes, 0644) // 权限：用户可读写
	if err != nil {
		return err
	}

	return nil
}

// Dump 打印当前配置到标准输出（调试用）
func (c *Config) Dump() error {
	bytes, err := c.Marshall()
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write(bytes)

	return err
}

// Get 整合默认值→文件→环境变量，返回最终配置
func Get(configFile string) (*Config, error) {
	// Start with empty configuration
	cfg := Default()

	if configFile != "" {
		fmt.Printf("Reading configuration file: %s\n", configFile)

		err := cfg.ReadFromFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("配置文件错误: %v", err)
		}
	}

	// Read configuration values from env
	err := cfg.ReadFromEnv()
	if err != nil {
		return nil, fmt.Errorf("环境变量错误: %v", err)
	}

	return cfg, nil
}
