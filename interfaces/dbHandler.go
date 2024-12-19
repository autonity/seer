package interfaces

import "Seer/model"

type DatabaseHandler interface {
	WriteEvent(schema model.EventSchema)
	Close()
}
