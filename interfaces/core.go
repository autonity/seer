package interfaces

import (
	"context"
)

type Core interface {
	Start(ctx context.Context)
	Stop()
	ProcessRange(ctx context.Context, start, end uint64)
	ConnectionProvider() ConnectionProvider
}

type Processor interface {
	Process()
}
