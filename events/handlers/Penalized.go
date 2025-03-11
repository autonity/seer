package handlers

import (
	"github.com/autonity/autonity/core/types"

	"seer/interfaces"
	"seer/model"
)

type PenalizedHandler struct {
	DBHandler interfaces.DatabaseHandler
}

func (ev *PenalizedHandler) Handle(schema model.EventSchema, header *types.Header, core interfaces.Core) {

}
