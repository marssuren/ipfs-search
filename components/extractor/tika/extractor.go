package tika

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"

	"github.com/ipfs-search/ipfs-search/components/extractor"
	"github.com/ipfs-search/ipfs-search/components/protocol"
	"github.com/ipfs-search/ipfs-search/instr"
	t "github.com/ipfs-search/ipfs-search/types"
	"github.com/ipfs-search/ipfs-search/utils"
)

// Extractor 使用ipfs-tika服务器提取元数据的提取器结构体。
type Extractor struct {
	config   *Config
	getter   utils.HTTPBodyGetter
	protocol protocol.Protocol

	*instr.Instrumentation
}

// getExtractURL 生成用于提取元数据的URL。
func (e *Extractor) getExtractURL(r *t.AnnotatedResource) string {
	gwURL := e.protocol.GatewayURL(r) // 获取资源的网关URL
	return fmt.Sprintf("%s/extract?url=%s", e.config.TikaExtractorURL, url.QueryEscape(gwURL))
}

// Extract 从（可能是）引用的资源中提取元数据，更新元数据或返回错误。
func (e *Extractor) Extract(ctx context.Context, r *t.AnnotatedResource, m interface{}) error {
	ctx, span := e.Tracer.Start(ctx, "extractor.tika.Extract")
	defer span.End()

	if err := extractor.ValidateMaxSize(ctx, r, e.config.MaxFileSize); err != nil { // 验证资源大小是否超过最大限制
		return err
	}

	// 如果提取在指定时间内未完成，则超时。
	ctx, cancel := context.WithTimeout(ctx, e.config.RequestTimeout)
	defer cancel()

	body, err := e.getter.GetBody(ctx, e.getExtractURL(r), 200) // 获取提取URL的响应体
	if err != nil {
		return err
	}
	defer body.Close()

	// 解析JSON结果
	if err := json.NewDecoder(body).Decode(m); err != nil {
		err := fmt.Errorf("%w: %v", t.ErrUnexpectedResponse, err)
		span.RecordError(err)
		return err
	}

	log.Printf("Got tika metadata metadata for '%v'", r) // 打印获取到的元数据信息

	return nil
}

// New 返回一个新的Tika提取器实例。
func New(config *Config, getter utils.HTTPBodyGetter, protocol protocol.Protocol, instr *instr.Instrumentation) extractor.Extractor {
	return &Extractor{
		config,
		getter,
		protocol,
		instr,
	}
}

// 编译时保证实现满足接口要求。
var _ extractor.Extractor = &Extractor{}
