package handlers

import (
	"log/slog"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"

	"Seer/helper"
	"Seer/model"
	"Seer/net"
)

//Note: stick to this naming convention for handlers
// EventName + Handler

type InactivityjJaillingEventHandler struct{}

func (ev *InactivityjJaillingEventHandler) Handle(schema model.EventSchema, block *types.Block, tags map[string]string, cp net.ConnectionProvider) {
	slog.Debug("Handling Inactivity Jailing event", "")
	con := cp.GetWebSocketConnection()
	defer cp.PutWebSocketConnection(con)
	omissionBindings, err := autonity.NewOmissionAccountability(helper.OmissionAccountabilityContractAddress, con.Client)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
	}

	validator := schema.Fields["validator"]
	inactivityScore, err := omissionBindings.GetInactivityScore(&bind.CallOpts{
		BlockNumber: block.Number(),
	}, validator.(common.Address))
	schema.Fields["InactivityScore"] = inactivityScore
}
