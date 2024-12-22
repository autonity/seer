package listener

import (
	"context"
	"log/slog"

	"github.com/ethereum/go-ethereum/core/types"

	"Seer/interfaces"
)

type eventProcessor struct {
	eventCh <-chan types.Log
	ctx     context.Context
}

func NewEventProcessor(ctx context.Context, eventCh <-chan types.Log) interfaces.Processor {
	return &eventProcessor{
		eventCh: eventCh,
		ctx:     ctx,
	}
}

func (ep *eventProcessor) Process() {
	for {
		select {
		case <-ep.ctx.Done():
			return
		case _, ok := <-ep.eventCh:
			if !ok {
				return
			}
			slog.Info("new event received")
		}
	}

}
