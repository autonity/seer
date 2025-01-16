package interfaces

import (
	"github.com/autonity/autonity/core/types"

	"seer/model"
	"seer/net"
)

type EventHandler interface {
	Handle(schema model.EventSchema, block *types.Block, tags map[string]string, provider net.ConnectionProvider)
}
