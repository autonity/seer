package handlers

import (
	"github.com/autonity/autonity/core/types"

	"seer/interfaces"
	"seer/model"
	"seer/net"
)

type PenalizedHandler struct {
	DBHandler interfaces.DatabaseHandler
}

func (ev *PenalizedHandler) Handle(schema model.EventSchema, header *types.Header, cp net.ConnectionProvider) {

}
