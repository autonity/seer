package interfaces

import "Seer/model"

type DatabaseHandler interface {
	WriteEvent(schema model.EventSchema, tags map[string]string)
	Close()
}
