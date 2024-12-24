package interfaces

import "Seer/model"

type ABIParser interface {
	Start() error
	Parse(filepath string) error
	EventSchema(eventName string) model.EventSchema
	Stop() error
}
