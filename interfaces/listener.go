package interfaces

import "context"

type Listener interface {
	Start(ctx context.Context)
	Stop()
}

type Processor interface {
	Process()
}
