package interfaces

import (
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"

	"Seer/model"
)

type ABIParser interface {
	Start() error
	Parse(filepath string) error
	Decode(log types.Log) (model.EventSchema,error)
	Stop() error
}

type BlockCache interface {
	Get(hash common.Hash) *types.Block
}
