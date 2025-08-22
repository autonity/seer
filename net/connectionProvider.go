package net

import (
	"seer/interfaces"

	"github.com/autonity/autonity/ethclient"
	"github.com/autonity/autonity/rpc"
)

// EthClientAdapter embeds the concrete ethclient.Client.
type EthClientAdapter struct {
	*ethclient.Client
}

// RPCClientAdapter embeds the concrete rpc.Client.
type RPCClientAdapter struct {
	*rpc.Client
}

type connectionProvider struct {
	WSPool  *ConnectionPool[*ethclient.Client]
	RPCPool *ConnectionPool[*rpc.Client]
}

func NewConnectionProvider(wsp *ConnectionPool[*ethclient.Client], rpcPool *ConnectionPool[*rpc.Client]) interfaces.ConnectionProvider {
	return &connectionProvider{WSPool: wsp, RPCPool: rpcPool}
}

// GetWebSocketConnection now wraps the concrete client in our adapter before returning.
func (cp *connectionProvider) GetWebSocketConnection() interfaces.EthClient {
	client := cp.WSPool.Get().Client
	return &EthClientAdapter{client} // Return the adapter
}

// GetRPCConnection does the same for the RPC client.
func (cp *connectionProvider) GetRPCConnection() interfaces.RPCClient {
	client := cp.RPCPool.Get().Client
	return &RPCClientAdapter{client} // Return the adapter
}
