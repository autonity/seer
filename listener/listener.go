package listener

import (
	"Seer/config"
	"Seer/interfaces"
)

type Listener struct {
	nodeConfig config.NodeConfig
	abiParser  interfaces.ABIParser
	dbHandler  interfaces.DatabaseHandler
}

func (l *Listener) Start() {
	//TODO

}

func (l *Listener) Stop() {
	//TODO
}

func NewListener(cfg config.NodeConfig, parser interfaces.ABIParser, dbHandler interfaces.DatabaseHandler) interfaces.Listener {
	return &Listener{nodeConfig: cfg, abiParser: parser, dbHandler: dbHandler}
}
