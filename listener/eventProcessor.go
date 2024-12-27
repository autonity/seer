package listener

import (
	"context"
	"log/slog"

	"github.com/autonity/autonity/core/types"

	"Seer/interfaces"
)

type eventProcessor struct {
	eventCh   <-chan types.Log
	ctx       context.Context
	parser    interfaces.ABIParser
	dbHandler interfaces.DatabaseHandler
}

func NewEventProcessor(ctx context.Context, eventCh <-chan types.Log, parser interfaces.ABIParser, dbHandler interfaces.DatabaseHandler) interfaces.Processor {
	return &eventProcessor{
		eventCh:   eventCh,
		ctx:       ctx,
		parser:    parser,
		dbHandler: dbHandler,
	}
}

func (ep *eventProcessor) Process() {
	for {
		select {
		case <-ep.ctx.Done():
			return
		case event, ok := <-ep.eventCh:
			if !ok {
				return
			}
			evSchema, err := ep.parser.Decode(event)
			if err != nil {
				slog.Error("new event decode", "error", err)
				continue
			}
			if evSchema.Name == "" {
				slog.Info("Unknown event received")
				continue
			}
			slog.Info("new log event received", "name", evSchema.Name)
			//todo: tags
			ep.dbHandler.WriteEvent(evSchema, nil)
		}
	}
}
