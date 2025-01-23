package handlers

import (
	"log/slog"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"

	"seer/helper"
	"seer/model"
	"seer/net"
)

//Note: stick to this naming convention for handlers
// EventName + Handler

type InactivityjJaillingEventHandler struct{}

func (ev *InactivityjJaillingEventHandler) Handle(schema model.EventSchema, header *types.Header, cp net.ConnectionProvider) {
	slog.Debug("Handling Inactivity Jailing event", "")
	con := cp.GetWebSocketConnection()
	omissionBindings, err := autonity.NewOmissionAccountability(helper.OmissionAccountabilityContractAddress, con.Client)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
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
