package interfaces

import (
	"math/big"

	"github.com/autonity/autonity/core/types"

	"seer/model"
)

type ABIParser interface {
	Start() error
	Parse(filepath string) error
	Decode(log types.Log) (model.EventSchema, error)
	Stop() error
}

type BlockCache interface {
	Get(number *big.Int) (*types.Block, bool)
	Add(block *types.Block)
}
