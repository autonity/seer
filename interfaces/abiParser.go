package interfaces

import (
	"github.com/autonity/autonity/core/types"

	"seer/model"
)

type ABIParser interface {
	Start() error
	Parse(filepath string) error
	Decode(log types.Log) (model.EventSchema, error)
	Stop() error
}
