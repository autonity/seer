package net

import (
	"github.com/autonity/autonity/ethclient"
	"github.com/autonity/autonity/rpc"
)

type ConnectionProvider interface {
	GetWebSocketConnection() *Connection[*ethclient.Client]
	GetRPCConnection() *Connection[*rpc.Client]
}

type connectionProvider struct {
	WSPool  *ConnectionPool[*ethclient.Client]
	RPCPool *ConnectionPool[*rpc.Client]
}

func NewConnectionProvider(wsp *ConnectionPool[*ethclient.Client], rpcPool *ConnectionPool[*rpc.Client]) ConnectionProvider {
	return &connectionProvider{WSPool: wsp, RPCPool: rpcPool}
}

func (cp *connectionProvider) GetWebSocketConnection() *Connection[*ethclient.Client] {
	return cp.WSPool.Get()
}

func (cp *connectionProvider) GetRPCConnection() *Connection[*rpc.Client] {
	return cp.RPCPool.Get()
}
