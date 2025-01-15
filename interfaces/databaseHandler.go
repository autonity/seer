package interfaces

import (
	"time"

	"Seer/model"
)

type DatabaseHandler interface {
	LastProcessed() uint64
	WriteEventBlocking(schema model.EventSchema, tags map[string]string, timeStamp time.Time) error
	WriteEvent(schema model.EventSchema, tags map[string]string, timeStamp time.Time)
	WritePoint(measurement string, tags map[string]string, fields map[string]interface{}, ts time.Time)
	Close()
	Flush()
	SaveLastProcessed(uint64)
}
