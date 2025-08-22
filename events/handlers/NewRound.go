package handlers

import (
	"math/big"
	"time"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity/bindings"
	"github.com/autonity/autonity/core/types"

	"seer/helper"
	"seer/interfaces"
	"seer/model"
	"seer/net"
)

var (
	OracleVoters = "OracleVoters"
)

type NewRoundHandler struct {
	DBHandler interfaces.DatabaseHandler
}

func (ev *NewRoundHandler) Handle(schema model.EventSchema, header *types.Header, core interfaces.Core) {

	tags := make(map[string]string)
	fields := make(map[string]interface{})
	fields["round"] = schema.Fields["_round"].(*big.Int).Uint64()
	fields["block"] = header.Number.Uint64()

	con := core.ConnectionProvider().GetWebSocketConnection()
	oracleBindings, _ := bindings.NewOracle(helper.OracleContractAddress, con.(*net.EthClientAdapter))
	voters, _ := oracleBindings.GetNewVoters(&bind.CallOpts{
		BlockNumber: header.Number,
	})
	fields["numVoters"] = len(voters)

	ts := time.Unix(int64(header.Time), 0)
	for _, m := range voters {
		tags["voter"] = m.String()
		ev.DBHandler.WritePoint(OracleVoters, tags, fields, ts)
	}
}
