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

//Note: stick to this naming convention for handlers
// EventName + Handler

type InactivityjJaillingEventHandler struct {
	DBHandler interfaces.DatabaseHandler
}

func (ev *InactivityjJaillingEventHandler) Handle(schema model.EventSchema, header *types.Header, core interfaces.Core) {
	con := core.ConnectionProvider().GetWebSocketConnection()
	omissionBindings, err := autonity.NewOmissionAccountability(helper.OmissionAccountabilityContractAddress, con.Client)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
		return
	}

	validator := schema.Fields["validator"]
	inactivityScore, err := omissionBindings.GetInactivityScore(&bind.CallOpts{
		BlockNumber: header.Number,
	}, validator.(common.Address))
	if inactivityScore == nil {
		return
	}
	schema.Fields["InactivityScore"] = inactivityScore.Uint64()
}
