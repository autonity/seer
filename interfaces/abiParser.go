package interfaces

import "Seer/model"

type ABIParser interface {
	Parse(filepath string) error
	EventSchema(eventName string) model.EventSchema
}
