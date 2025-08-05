package handlers

import (
	"log/slog"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity/bindings"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"

	"seer/helper"
	"seer/interfaces"
	"seer/model"
)

type NewFaultProofHandler struct {
	DBHandler interfaces.DatabaseHandler
}

func (handler *NewFaultProofHandler) Handle(schema model.EventSchema, header *types.Header, core interfaces.Core) {

	con := core.ConnectionProvider().GetWebSocketConnection()
	accBindings, err := bindings.NewAccountability(helper.AccountabilityContractAddress, con.Client)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
		return
	}

	validator := schema.Fields["offender"]
	events, err := accBindings.GetValidatorFaults(&bind.CallOpts{
		BlockNumber: header.Number,
	}, validator.(common.Address))
	for _, ev := range events {
		RecordAccountabilityEvent(handler.DBHandler, ev, header)
	}
}
