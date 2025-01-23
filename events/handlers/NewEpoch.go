package handlers

import (
	"log/slog"

	"github.com/autonity/autonity/core/types"

	"seer/model"
	"seer/net"
)

// Note: stick to this naming convention for handlers
// EventName + Handler

type NewEpochHandler struct {
}

func (ev *NewEpochHandler) Handle(schema model.EventSchema, header *types.Header,  cp net.ConnectionProvider) {
	if header.Epoch == nil {
		slog.Error("NewEpoch Handler, committee information is nor present")
		return
	}
	//TODO
	schema.Fields["committee"] = header.Epoch.Committee.String()
}
