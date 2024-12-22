package listener

import (
	"context"
	"log/slog"

	"github.com/ethereum/go-ethereum/core/types"

	"Seer/interfaces"
)

type blockProcessor struct {
	blockCh <-chan *types.Block
	ctx     context.Context
}

func NewBlockProcessor(ctx context.Context, blockCh <-chan *types.Block) interfaces.Processor {
	return &blockProcessor{
		blockCh: blockCh,
		ctx:     ctx,
	}
}

func (bp *blockProcessor) Process() {
	for {
		select {
		case <-bp.ctx.Done():
			return
		case block, ok := <-bp.blockCh:
			if !ok {
				return
			}
			slog.Info("new block received", "number", block.Number().Uint64())
		}
	}

}
