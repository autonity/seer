package interfaces

import (
	"time"

	"Seer/model"
)

type DatabaseHandler interface {
	LastProcessed() uint64
	WriteEvent(schema model.EventSchema, tags map[string]string, timeStamp time.Time) error
	Close()
	SaveLastProcessed(uint64)
}
