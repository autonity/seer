package listener

import (
	"context"
	"log/slog"

	"github.com/autonity/autonity/core/types"

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
		case event, ok := <-ep.eventCh:
			slog.Info("new log event received")
			if !ok {
				return
			}
			ep.Decode(event)
		}
	}
}

func (ep *eventProcessor) Decode(ev types.Log) {

}
