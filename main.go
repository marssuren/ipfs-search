// Search engine for IPFS using OpenSearch, RabbitMQ and Tika.
// 声明包名为 main（可执行程序入口）
package main

// 导入依赖库
import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ipfs-search/ipfs-search/commands" // tocheck: 核心命令实现
	"github.com/ipfs-search/ipfs-search/config"
	"gopkg.in/urfave/cli.v1" // CLI框架
)

// 主函数入口
func main() {
	// 配置日志格式（注释掉的代码是带文件名和行号的调试模式）
	// Prefix logging with filename and line number: "d.go:23"
	// log.SetFlags(log.Lshortfile)

	// Logging w/o prefix
	log.SetFlags(0) // 禁用日志前缀，仅输出日志内容

	// 初始化CLI应用
	app := cli.NewApp()
	app.Name = "ipfs-search"          // 应用名称
	app.Usage = "IPFS search engine." // 帮助信息

	// 定义所有支持的CLI命令
	app.Commands = []cli.Command{
		{
			Name:    "add",                         // 添加哈希命令
			Aliases: []string{"a"},                 // 别名
			Usage:   "add `HASH` to crawler queue", // 用法提示
			Action:  add,                           // 执行函数（下方定义的add函数）
		},
		{
			Name:    "crawl", // 启动爬虫命令
			Aliases: []string{"c"},
			Usage:   "start crawler",
			Action:  crawl, // 执行函数（下方定义的crawl函数）
		},
		{
			Name:    "config", // 配置管理命令组
			Aliases: []string{},
			Usage:   "configuration",
			Subcommands: []cli.Command{ // 子命令
				{
					Name:   "generate", // 生成默认配置
					Usage:  "generate default configuration",
					Action: generateConfig,
				},
				{
					Name:   "check", // 验证配置有效性
					Usage:  "check configuration",
					Action: checkConfig,
				},
				{
					Name:   "dump", // 导出配置到控制台
					Usage:  "dump current configuration to stdout",
					Action: dumpConfig,
				},
			},
		},
	}

	// 定义全局标志（所有命令可用）
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c", // 配置文件路径
			Usage: "Load configuration from `FILE`",
		},
	}

	// 启动CLI应用，处理命令行参数
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// 加载配置的公共方法（被多个命令调用）
func getConfig(c *cli.Context) (*config.Config, error) {
	configFile := c.GlobalString("config") // 获取全局--config参数值

	cfg, err := config.Get(configFile)
	if err != nil {
		return nil, err
	}

	// 验证配置必填项
	err = cfg.Check()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// config check子命令实现
func checkConfig(c *cli.Context) error {
	// 复用getConfig的配置检查逻辑
	_, err := getConfig(c)

	if err != nil {
		// 包装错误为CLI退出错误（状态码1）
		return cli.NewExitError(err.Error(), 1)
	}

	fmt.Println("Configuration checked.")

	return nil
}

// config generate子命令实现
func generateConfig(c *cli.Context) error {
	// tocheck: 获取默认配置结构
	cfg := config.Default()

	configFile := c.GlobalString("config")
	if configFile == "" {
		return cli.NewExitError("需通过-c指定配置文件路径", 1)
	}

	fmt.Printf("正在生成默认配置到: %s\n", configFile)
	// tocheck: 将默认配置写入文件
	return cfg.Write(configFile)
}

// config dump子命令实现
func dumpConfig(c *cli.Context) error {
	cfg, err := getConfig(c)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	// tocheck: 将配置序列化输出到控制台
	return cfg.Dump()
}

// add命令的具体实现
func add(c *cli.Context) error {
	// 创建可取消的上下文（用于信号处理）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // 确保资源释放

	// 注册信号处理（Control-C触发cancel）
	onSigTerm(cancel)

	// 验证参数数量
	if c.NArg() != 1 {
		return cli.NewExitError("请提供一个哈希参数", 1)
	}
	hash := c.Args().Get(0) // 获取第一个参数作为哈希

	// 加载配置
	cfg, err := getConfig(c)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	fmt.Printf("正在添加哈希 '%s' 到队列\n", hash)

	// tocheck: 调用commands包的哈希添加逻辑
	err = commands.AddHash(ctx, cfg, hash)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	return nil
}

// 信号处理函数（监听SIGTERM和Control-C）
func onSigTerm(f func()) {
	sigChan := make(chan os.Signal, 2)
	// 注册要监听的信号
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 第二次信号处理：强制退出
	var fail = func() {
		<-sigChan
		os.Exit(1) // 强制终止
	}

	// 第一次信号处理：优雅退出
	var quit = func() {
		// 阻塞直到收到信号
		<-sigChan

		go fail() // 处理第二次信号

		fmt.Println("收到SIGTERM，正在退出... 再次发送将强制终止！")
		f()
	}

	go quit() // 启动信号监听协程
}

// crawl命令的具体实现
func crawl(c *cli.Context) error {
	fmt.Println("启动爬虫工作进程")

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 注册信号处理
	onSigTerm(cancel)

	// 加载配置
	cfg, err := getConfig(c)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	// tocheck: 调用commands包的爬虫主逻辑
	err = commands.Crawl(ctx, cfg)
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	return nil
}
