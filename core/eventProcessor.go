package core

import (
	"context"
	"log/slog"
	"math/big"
	"time"

	"github.com/autonity/autonity/core/types"

	"seer/events/registry"
	"seer/helper"
	"seer/interfaces"
	"seer/model"
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
	go func() {
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

				if event.BlockNumber == 1800 {
					slog.Info("Received event", "event", evSchema.Measurement)
				}
				block, _ := ep.core.blockCache.Get(big.NewInt(int64(event.BlockNumber)))
				if block == nil {
					slog.Error("couldn't fetch block", "hash", event.BlockHash)
					continue
				}
				ep.recordEvent(block, evSchema, event)
			}
		}

	}()
}

func (ep *eventProcessor) recordEvent(block *types.Block, schema model.EventSchema, log types.Log) {

	handler := registry.GetHandler(schema.Measurement)
	//custom event handling only if registered
	if handler != nil {
		handler.Handle(schema, block, ep.core.cp)
	}
	slog.Debug("new log event received", "name", schema.Measurement, "block", block.NumberU64())
	schema.Fields["block"] = block.NumberU64()
	ts := time.Unix(int64(block.Time()), 0)
	go ep.core.dbHandler.WriteEvent(schema, ts)
}
