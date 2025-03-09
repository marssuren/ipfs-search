package factory

import (
	"context"
	"fmt"
	"time"

	"net"

	"github.com/ipfs-search/ipfs-search/components/queue/amqp"
	"github.com/ipfs-search/ipfs-search/components/sniffer"
	"github.com/ipfs-search/ipfs-search/config"
	"github.com/ipfs-search/ipfs-search/instr"
	"github.com/ipfs-search/ipfs-search/utils"

	"github.com/ipfs/go-datastore"
	samqp "github.com/rabbitmq/amqp091-go"
)

// getConfig 获取并检查配置。
func getConfig() (*config.Config, error) {
	cfg, err := config.Get("")
	if err != nil {
		return nil, err
	}

	if err = cfg.Check(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getInstr 初始化仪表化并返回它以及一个刷新函数。
func getInstr(cfg *instr.Config) (*instr.Instrumentation, func(context.Context), error) {
	instFlusher, err := instr.Install(cfg, "ipfs-sniffer")
	if err != nil {
		return nil, nil, err
	}
	return instr.New(), instFlusher, nil
}

// getQueue 使用重试拨号器初始化 AMQP 发布者工厂。
func getQueue(ctx context.Context, cfg *amqp.Config, i *instr.Instrumentation) amqp.PublisherFactory {
	// 用于连接的重试拨号器
	dialer := &utils.RetryingDialer{
		Dialer: net.Dialer{
			Timeout:   30 * time.Second, // 设置拨号超时时间。
			KeepAlive: 30 * time.Second, // 设置保持连接时间。
			DualStack: false,
		},
		Context: ctx,
	}
	samqpConfig := &samqp.Config{
		Dial: dialer.Dial,
	}

	return amqp.PublisherFactory{
		Config:          cfg,
		AMQPConfig:      samqpConfig,
		Queue:           "hashes", // 设置队列名称。
		Instrumentation: i,
	}
}

// getSniffer 使用提供的配置、数据存储、队列和仪表化初始化一个 Sniffer 实例。
func getSniffer(cfg *sniffer.Config, ds datastore.Batching, q amqp.PublisherFactory, i *instr.Instrumentation) (*sniffer.Sniffer, error) {
	return sniffer.New(cfg, ds, q, i)
}

// Start initialises a sniffer and all its dependencies and launches it in a goroutine, returning a wrapped context
// Start 初始化一个 sniffer 及其所有依赖项，并在一个 goroutine 中启动它，返回一个包装的上下文和数据存储，应该替换原始的上下文和数据存储，或者返回初始化错误。
func Start(ctx context.Context, ds datastore.Batching) (context.Context, datastore.Batching, error) {
	cfg, err := getConfig()
	if err != nil {
		return nil, nil, err
	}

	i, instFlusher, err := getInstr(cfg.InstrConfig())
	if err != nil {
		return nil, nil, err
	}

	// 创建一个可以被 sniffer 取消的上下文，以便从 sniffer goroutine 传播失败。
	ctx, cancel := context.WithCancel(ctx)

	q := getQueue(ctx, cfg.AMQPConfig(), i)

	s, err := getSniffer(cfg.SnifferConfig(), ds, q, i)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	// 使用批处理数据存储。
	ds = s.Batching()

	// 启动 sniffer。
	go func() {
		// 完成时取消父上下文。
		defer cancel()
		defer instFlusher(ctx)

		err = s.Sniff(ctx)
		fmt.Printf("Sniffer exited: %s\n", err)
	}()

	return ctx, ds, nil
}
