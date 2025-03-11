package handlers

import (
	"github.com/autonity/autonity/core/types"

	"seer/interfaces"
	"seer/model"
)

type EventHandler interface {
	Handle(schema model.EventSchema, block *types.Header, core interfaces.Core)
}
