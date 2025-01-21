package interfaces

import (
	"time"

	"seer/model"
)

type DatabaseHandler interface {
	LastProcessed() uint64
	WriteEventBlocking(schema model.EventSchema, timeStamp time.Time) error
	WriteEvent(schema model.EventSchema, timeStamp time.Time)
	WritePoint(measurement string, tags map[string]string, fields map[string]interface{}, timeStamp time.Time)
	Close()
	Flush()
	SaveLastProcessed(uint64)
}
