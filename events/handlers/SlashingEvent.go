package handlers

import (
	"log/slog"
	"math/big"
	"time"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/core/types"

	"seer/helper"
	"seer/interfaces"
	"seer/model"
)

var (
	AccountabilityEvent = "AccountabilityEvent"
)

type SlashingEventHandler struct {
	DBHandler interfaces.DatabaseHandler
}

func (handler *SlashingEventHandler) Handle(schema model.EventSchema, header *types.Header, core interfaces.Core) {
	con := core.ConnectionProvider().GetWebSocketConnection()
	accountabilityBindings, err := autonity.NewAccountability(helper.AccountabilityContractAddress, con.Client)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
		return
	}
	evID := schema.Fields["id"]
	event, err := accountabilityBindings.Events(&bind.CallOpts{
		BlockNumber: header.Number,
	}, evID.(*big.Int))
	if err != nil {
		slog.Error("unable to fetch accountability event", "error", err)
		return
	}
	RecordAccountabilityEvent(handler.DBHandler, event, header)
}

func RecordAccountabilityEvent(dbhandler interfaces.DatabaseHandler, event autonity.AccountabilityEvent, header *types.Header) {
	tags := make(map[string]string)
	tags["reporter"] = event.Reporter.String()

	fields := make(map[string]interface{})
	fields["block"] = header.Number.Uint64()
	fields["type"] = event.EventType
	fields["eventBlock"] = event.Block.Uint64()
	fields["epoch"] = event.Epoch.Uint64()
	fields["offender"] = event.Offender
	fields["rule"] = event.Rule
	fields["messageHash"] = event.MessageHash
	fields["rawProof"] = event.RawProof
	fields["id"] = event.Id

	ts := time.Unix(int64(header.Time), 0)
	dbhandler.WritePoint(AccountabilityEvent, tags, fields, ts)
}
