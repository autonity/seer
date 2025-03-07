package handlers

import (
	"log/slog"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"

	"seer/helper"
	"seer/interfaces"
	"seer/model"
)

type NewAccusationHandler struct {
	DBHandler interfaces.DatabaseHandler
}

func (handler *NewAccusationHandler) Handle(schema model.EventSchema, header *types.Header, core interfaces.Core) {
	con := core.ConnectionProvider().GetWebSocketConnection()
	accBindings, err := autonity.NewAccountability(helper.AccountabilityContractAddress, con.Client)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
		return
	}

	validator := schema.Fields["offender"]
	event, err := accBindings.GetValidatorAccusation(&bind.CallOpts{
		BlockNumber: header.Number,
	}, validator.(common.Address))
	if err != nil {
		slog.Error("unable to fetch validator accusation", "error", err)
		return
	}
	RecordAccountabilityEvent(handler.DBHandler, event, header)
}
