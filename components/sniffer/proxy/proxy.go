package proxy

import (
	hook "github.com/alanshaw/ipfs-hookds"
	"github.com/alanshaw/ipfs-hookds/batch"
	"github.com/ipfs/go-datastore"
)

// New 将一个数据存储包装在一个代理中，并在每次 Put() 操作之后调用 afterPut 函数。
func New(ds datastore.Batching, afterPut hook.AfterPutFunc) datastore.Batching {
	// 定义一个 afterBatch 函数，它在每次批处理操作之后调用。
	afterBatch := func(b datastore.Batch, err error) (datastore.Batch, error) {
		// 创建一个新的批处理对象，并在每次 Put 操作之后调用 afterPut 函数。
		return batch.NewBatch(b, batch.WithAfterPut(batch.AfterPutFunc(afterPut))), err
	}

	// 返回一个新的代理数据存储，该数据存储在每次 Put 操作和批处理操作之后调用相应的回调函数。
	return hook.NewBatching(ds, hook.WithAfterPut(afterPut), hook.WithAfterBatch(afterBatch))
}
