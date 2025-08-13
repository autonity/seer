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
	ctx     context.Context
	core    *core
	eventCh chan types.Log
	isLive  bool
}

func NewEventProcessor(ctx context.Context, core *core, evCh chan types.Log, isLive bool) interfaces.Processor {
	return &eventProcessor{
		ctx:     ctx,
		core:    core,
		eventCh: evCh,
		isLive:  isLive,
	}
}

func (ep *eventProcessor) Process() {
	go func() {
		for {
			select {
			case <-ep.ctx.Done():
				return
			case event, ok := <-ep.eventCh:
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

				/* todo: block cache fetch can be avoided if we push these events
				to be processed later when block is synced */
				block, _ := ep.core.blockCache.Get(big.NewInt(int64(event.BlockNumber)))
				if block == nil {
					slog.Error("couldn't fetch block", "hash", event.BlockHash)
					continue
				}
				ep.recordEvent(block.Header(), evSchema)
			}
		}
	}()
}

func (ep *eventProcessor) recordEvent(header *types.Header, schema model.EventSchema) {

	handler := registry.GetEventHandler(schema.Measurement)
	//custom event handling only if registered
	if handler != nil {
		handler.Handle(schema, header, ep.core)
	}
	slog.Debug("new log event received", "name", schema.Measurement, "block", header.Number.Uint64())
	schema.Fields["block"] = header.Number.Uint64()
	ts := time.Unix(int64(header.Time), 0)
	ep.core.dbHandler.WriteEvent(schema, ts)
}
