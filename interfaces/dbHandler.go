package interfaces

import "Seer/model"

type DatabaseHandler interface {
	LastProcessed() int64
	WriteEvent(schema model.EventSchema, tags map[string]string)
	Close()
}
