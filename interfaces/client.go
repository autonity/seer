package interfaces

import (
	"context"
	"math/big"

	ethereum "github.com/autonity/autonity"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/rpc"
)

// EthClient defines the methods needed from an ethclient.Client.
type EthClient interface {
	BlockNumber(ctx context.Context) (uint64, error)
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	NetworkID(ctx context.Context) (*big.Int, error)
	SubscribeFilterLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error)
	TransactionReceipt(ctx context.Context, hash common.Hash) (*types.Receipt, error)
}

// RPCClient defines the methods needed from a rpc.Client.
type RPCClient interface {
	BatchCallContext(ctx context.Context, b []rpc.BatchElem) error
	CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error
}

type ConnectionProvider interface {
	GetWebSocketConnection() EthClient
	GetRPCConnection() RPCClient
}
