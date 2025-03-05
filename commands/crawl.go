package commands

import (
	"context"

	"github.com/ipfs-search/ipfs-search/components/worker/pool" // 工作池组件
	"github.com/ipfs-search/ipfs-search/config"                 // 配置管理
	"github.com/ipfs-search/ipfs-search/instr"                  // 监控工具

	"log" // 标准日志库
)

// Crawl 配置并启动爬虫
func Crawl(ctx context.Context, cfg *config.Config) error {
	// 初始化监控，命名空间为"ipfs-crawler"
	instFlusher, err := instr.Install(cfg.InstrConfig(), "ipfs-crawler")
	if err != nil {
		log.Fatal(err) // 直接终止（tocheck: 是否应返回错误？与函数签名不一致）
	}
	defer instFlusher(ctx) // 确保监控数据刷新

	i := instr.New() // 创建监控实例（tocheck: 与Install的关系？）

	// 启动分布式追踪Span
	ctx, span := i.Tracer.Start(ctx, "commands.Crawl")
	defer span.End() // 结束Span（记录执行时间）

	// 创建工作池（协程管理）
	pool, err := pool.New(ctx, cfg, i) // tocheck: 如何配置worker数量？
	if err != nil {
		return err // 初始化失败（如配置错误）
	}

	pool.Start(ctx) // 启动所有worker协程

	// 阻塞等待上下文取消信号（如SIGTERM）
	<-ctx.Done()

	// 返回错误原因（如context.Canceled）
	return ctx.Err() // tocheck: 是否处理工作池的优雅关闭？
}
