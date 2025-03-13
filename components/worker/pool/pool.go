package pool

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	samqp "github.com/rabbitmq/amqp091-go"

	"github.com/ipfs-search/ipfs-search/components/crawler"
	"github.com/ipfs-search/ipfs-search/components/worker"
	"github.com/ipfs-search/ipfs-search/config"
	"github.com/ipfs-search/ipfs-search/instr"
	"github.com/ipfs-search/ipfs-search/utils"
)

// consumeChans 定义了一个结构体，包含三个只读的 RabbitMQ 消息通道
type consumeChans struct {
	Files       <-chan samqp.Delivery
	Directories <-chan samqp.Delivery
	Hashes      <-chan samqp.Delivery
}

// Pool 表示一个池的集合。
type Pool struct {
	config  *config.Config
	dialer  *utils.RetryingDialer
	crawler *crawler.Crawler

	*consumeChans
	*instr.Instrumentation
}

// startWorkers 启动指定数量的 worker 来处理消息
func (p *Pool) startWorkers(ctx context.Context, deliveries <-chan samqp.Delivery, workers int, poolName string) {
	ctx, span := p.Tracer.Start(ctx, "crawler.pool.start")
	defer span.End()

	log.Printf("Starting %d workers for %s", workers, poolName)

	for i := 0; i < workers; i++ {
		name := fmt.Sprintf("%s-%d", poolName, i)
		worker := worker.New(name, p.crawler, p.Instrumentation)
		go worker.Start(ctx, deliveries)
	}
}

// Start 方法启动整个池。
func (p *Pool) Start(ctx context.Context) {
	ctx, span := p.Tracer.Start(ctx, "crawler.pool.Start")
	defer span.End()

	p.startWorkers(ctx, p.consumeChans.Files, p.config.Workers.FileWorkers, "files")
	p.startWorkers(ctx, p.consumeChans.Hashes, p.config.Workers.HashWorkers, "hashes")
	p.startWorkers(ctx, p.consumeChans.Directories, p.config.Workers.DirectoryWorkers, "directories")
}

// init 初始化 Pool 对象
func (p *Pool) init(ctx context.Context) error {
	var err error

	p.dialer = &utils.RetryingDialer{
		Dialer: net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: false,
		},
		Context: ctx,
	}

	log.Println("Initializing crawler.")
	if p.crawler, err = p.getCrawler(ctx); err != nil {
		return err
	}

	log.Println("Initializing consuming channels.")
	if p.consumeChans, err = p.getConsumeChans(ctx); err != nil {
		return err
	}

	return nil
}

// New 函数初始化并返回一个新的 Pool 对象。
func New(ctx context.Context, c *config.Config, i *instr.Instrumentation) (*Pool, error) {
	if i == nil {
		panic("Instrumentation cannot be null.")
	}

	if c == nil {
		panic("Config cannot be nil.")
	}

	p := &Pool{
		config:          c,
		Instrumentation: i,
	}

	err := p.init(ctx)

	return p, err
}
