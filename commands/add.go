package commands

import (
	"context" // 上下文控制
	"net"     // 网络操作
	"time"    // 时间处理

	samqp "github.com/rabbitmq/amqp091-go" // RabbitMQ客户端，别名为samqp

	"github.com/ipfs-search/ipfs-search/components/queue/amqp" // 队列组件
	"github.com/ipfs-search/ipfs-search/config"                // 配置管理
	"github.com/ipfs-search/ipfs-search/instr"                 // 监控工具 （tocheck: 具体实现？）
	t "github.com/ipfs-search/ipfs-search/types"               // 类型定义，别名为t
	"github.com/ipfs-search/ipfs-search/utils"                 // 工具函数
)

// AddHash 将单个IPFS哈希添加到索引队列中，接收上下文、配置对象和哈希字符串
func AddHash(ctx context.Context, cfg *config.Config, hash string) error {
	// 初始化监控组件，命名空间为"ipfs-crawler add"
	instFlusher, err := instr.Install(cfg.InstrConfig(), "ipfs-crawler add")
	if err != nil {
		return err
	}
	defer instFlusher(ctx) // 确保退出前刷新监控数据（如指标上报）

	i := instr.New() // 创建监控实例（tocheck: 是否与Install的实例关联？）

	// 配置带重试的拨号器（TCP连接）
	dialer := &utils.RetryingDialer{
		Dialer: net.Dialer{
			Timeout:   30 * time.Second, // 连接超时
			KeepAlive: 30 * time.Second, // 保持连接存活
			DualStack: false,            // 禁用IPv6回退（强制IPv4）
		},
		Context: ctx, // 绑定上下文以支持取消
	}

	// AMQP配置（使用自定义拨号器）
	amqpConfig := &samqp.Config{
		Dial: dialer.Dial, // 注入带重试的拨号逻辑
	}

	// 创建AMQP发布者工厂
	f := amqp.PublisherFactory{
		Config:          cfg.AMQPConfig(), // 从配置获取AMQP参数（tocheck: 是否包含必要字段？）
		Queue:           "hashes",         // 目标队列名
		AMQPConfig:      amqpConfig,       // 自定义AMQP配置
		Instrumentation: i,                // 注入监控
	}

	// 创建队列发布者实例
	queue, err := f.NewPublisher(ctx)
	if err != nil {
		return err // 连接失败（如认证错误、网络不可达）
	}

	// 构建资源对象（IPFS协议 + 用户输入哈希）
	resource := &t.Resource{
		Protocol: t.IPFSProtocol, // 协议类型（tocheck: 是否支持IPNS？）
		ID:       hash,           // 用户提供的CID
	}

	// 添加元数据（来源标记为手动）
	r := t.AnnotatedResource{
		Resource: resource,
		Source:   t.ManualSource, // 区分任务来源（如爬虫发现 vs 用户添加）
	}

	// TODO: 使用provider字段（当前未实现，可能影响内容路由效率）

	// 发布消息到队列，优先级9（最高）
	return queue.Publish(ctx, &r, 9) // tocheck: 队列是否启用优先级支持？
}
