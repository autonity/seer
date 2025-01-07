package listener

import (
	"context"
	"log/slog"
	"time"

	"Seer/helper"
	"Seer/interfaces"
)

type eventProcessor struct {
	ctx      context.Context
	listener *Listener
}

func NewEventProcessor(ctx context.Context, listener *Listener) interfaces.Processor {
	return &eventProcessor{
		ctx:      ctx,
		listener: listener,
	}
}

func (ep *eventProcessor) Process() {
	for {
		select {
		case <-ep.ctx.Done():
			return
		case event, ok := <-ep.listener.newEvents:
			if !ok {
				return
			}
			evSchema, err := ep.listener.abiParser.Decode(event)
			if err != nil {
				slog.Error("new event decode", "error", err)
				continue
			}
			if evSchema.Measurement == "" {
				slog.Warn("Unknown event received", "event", event, "schema", evSchema, "Contract", helper.AddressToContractName(event.Address))
				continue
			}

			block := ep.listener.blockCache.Get(event.BlockHash)
			if block == nil {
				slog.Error("couldn't fetch block", "hash", event.BlockHash)
				continue
			}

			//todo: tags
			ts := time.Unix(int64(block.Time()), 0)
			evSchema.Fields["block"] = block.Number().Uint64()
			slog.Debug("new log event received", "name", evSchema.Measurement, "block", block.NumberU64())
			err = ep.listener.dbHandler.WriteEvent(evSchema, nil, ts)
			if err != nil {
				slog.Error("Error writing point to db", "error", err)
			}
		}
	}
}
