package interfaces

import (
	"github.com/autonity/autonity/core/types"

	"Seer/model"
	"Seer/net"
)

type EventHandler interface {
	Handle(schema model.EventSchema, block *types.Block, tags map[string]string, provider net.ConnectionProvider)
}
