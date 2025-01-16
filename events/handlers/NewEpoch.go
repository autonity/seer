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

func (ev *NewEpochHandler) Handle(schema model.EventSchema, block *types.Block, tags map[string]string, cp net.ConnectionProvider) {
	if !block.Header().HasCommitInformation() {
		slog.Error("NewEpoch Handler, committee information is nor present")
		return
	}
	schema.Fields["committee"] = block.Header().Epoch.Committee.String()
}
