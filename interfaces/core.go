package interfaces

import (
	"context"

	"seer/helper"
	"seer/net"
)

type Core interface {
	Start(ctx context.Context)
	Stop()
	ProcessRange(ctx context.Context, start, end uint64)
	EpochCache() *helper.EpochCache
	ConnectionProvider() net.ConnectionProvider
}

type Processor interface {
	Process()
}
