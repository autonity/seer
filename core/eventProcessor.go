package core

import (
	"context"
	"log/slog"
	"math/big"
	"time"

	"Seer/events/registry"
	"Seer/helper"
	"Seer/interfaces"
)

type eventProcessor struct {
	ctx  context.Context
	core *core
}

func NewEventProcessor(ctx context.Context, core *core) interfaces.Processor {
	return &eventProcessor{
		ctx:  ctx,
		core: core,
	}
}

func (ep *eventProcessor) Process() {
	for {
		select {
		case <-ep.ctx.Done():
			return
		case event, ok := <-ep.core.newEvents:
			if !ok {
				return
			}
			evSchema, err := ep.core.abiParser.Decode(event)
			if err != nil {
				slog.Error("new event decode", "error", err)
				continue
			}
			if evSchema.Measurement == "" {
				slog.Warn("Unknown event received", "event", event, "schema", evSchema, "Contract", helper.AddressToContractName(event.Address))
				continue
			}

			block, _ := ep.core.blockCache.Get(big.NewInt(int64(event.BlockNumber)))
			if block == nil {
				slog.Error("couldn't fetch block", "hash", event.BlockHash)
				continue
			}

			ts := time.Unix(int64(block.NumberU64()), 0)
			tags := map[string]string{}
			tags["event_type"] = "protocol"
			handler := registry.GetHandler(evSchema.Measurement)
			//custom event handling only if registered
			if handler != nil {
				handler.Handle(evSchema, block, tags, ep.core.cp)
			}

			slog.Debug("new log event received", "name", evSchema.Measurement, "block", block.NumberU64())
			go ep.core.dbHandler.WriteEvent(evSchema, tags, ts)
		}
	}
}
