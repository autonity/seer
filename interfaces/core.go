package interfaces

import (
	"context"

	"seer/helper"
)

type Core interface {
	Start(ctx context.Context)
	Stop()
	ProcessRange(ctx context.Context, start, end uint64)
	EpochCache() *helper.EpochCache
	ConnectionProvider() ConnectionProvider
}

type Processor interface {
	Process()
}
