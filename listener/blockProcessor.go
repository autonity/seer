package listener

import (
	"context"
	"log/slog"

	"Seer/interfaces"
)

type blockProcessor struct {
	listener *Listener
	ctx      context.Context
}

func NewBlockProcessor(ctx context.Context, listener *Listener) interfaces.Processor {
	return &blockProcessor{
		listener: listener,
		ctx:      ctx,
	}
}

func (bp *blockProcessor) Process() {
	for {
		select {
		case <-bp.ctx.Done():
			return
		case block, ok := <-bp.listener.newBlocks:
			if !ok {
				return
			}
			slog.Debug("new block received", "number", block.Number().Uint64())
			bp.listener.markProcessed(block.NumberU64(), block.NumberU64())
		}
	}

}
