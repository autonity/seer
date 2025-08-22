package helper

import (
	"context"
	"log/slog"
	"math/big"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity/bindings"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/common/fixsizecache"
	"github.com/autonity/autonity/core/types"
)

var (
	numBuckets     = uint(8999)
	entryPerBucket = uint(5)
)

type BlockFetcher interface {
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
}

// BlockFetcherFactory defines a function type that can provide a BlockFetcher on demand.
type BlockFetcherFactory func() BlockFetcher

type blockCache struct {
	cache   *fixsizecache.Cache[common.Hash, *types.Block]
	factory BlockFetcherFactory
}

func NewBlockCache(bf BlockFetcherFactory) BlockCache {
	return &blockCache{
		cache:   fixsizecache.New[common.Hash, *types.Block](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		factory: bf,
	}
}

func (bc *blockCache) Get(number *big.Int) (*types.Block, bool) {
	blk, ok := bc.cache.Get(common.BigToHash(number))
	if ok {
		return blk.(*types.Block), true
	}
	bf := bc.factory()
	block, err := bf.BlockByNumber(context.Background(), number)
	if err != nil {
		slog.Error("unable to fetch block", "error", err)
		//todo: error handling module
		return nil, false
	}
	bc.cache.Add(common.BigToHash(number), block)
	return block, true
}

func (bc *blockCache) Add(block *types.Block) {
	bc.cache.Add(common.BigToHash(block.Number()), block)
}

type EpochCache struct {
	cache             *fixsizecache.Cache[common.Hash, *bindings.IAutonityEpochInfo]
	blockToEpochBlock *fixsizecache.Cache[common.Hash, *big.Int]
	autonityBindings  *bindings.Autonity
}

func NewEpochInfoCache(backend bind.ContractBackend) *EpochCache {
	ec := EpochCache{
		cache:             fixsizecache.New[common.Hash, *bindings.IAutonityEpochInfo](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		blockToEpochBlock: fixsizecache.New[common.Hash, *big.Int](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
	}
	var err error
	ec.autonityBindings, err = bindings.NewAutonity(AutonityContractAddress, backend)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
		return nil
	}
	return &ec
}

func (ec *EpochCache) Get(number *big.Int) *bindings.IAutonityEpochInfo {
	epochBlockNum, ok := ec.blockToEpochBlock.Get(common.BigToHash(number))
	if !ok {
		return ec.retrieveEpochInfo(number)
	}
	epochBlock, ok := ec.cache.Get(common.BigToHash(epochBlockNum.(*big.Int)))
	if !ok {
		return ec.retrieveEpochInfo(number)
	}
	return epochBlock.(*bindings.IAutonityEpochInfo)
}

func (ec *EpochCache) Add(header *types.Header) {
	if header.Epoch == nil {
		return
	}
	committee := make([]bindings.IAutonityCommitteeMember, 0)
	for _, member := range header.Epoch.Committee.Members {
		committee = append(committee, bindings.IAutonityCommitteeMember{
			Addr:         member.Address,
			ConsensusKey: member.ConsensusKeyBytes,
			VotingPower:  member.VotingPower,
		})
	}
	epochInfo := bindings.IAutonityEpochInfo{
		PreviousEpochBlock: header.Epoch.PreviousEpochBlock,
		NextEpochBlock:     header.Epoch.NextEpochBlock,
		EpochBlock:         header.Number,
		Committee:          committee,
	}
	nextEpochBlock := header.Epoch.NextEpochBlock
	currentEpochBlock := header.Number

	nextBlock := big.NewInt(currentEpochBlock.Int64()).Add(currentEpochBlock, big.NewInt(1))
	// committee in epochInfo is applicable from epochBlock + 1 onwards
	ec.cache.Add(common.BigToHash(nextBlock), &epochInfo)

	i := big.NewInt(currentEpochBlock.Int64())
	i = i.Add(i, big.NewInt(1))
	for i.Cmp(nextEpochBlock) == -1 {
		ec.blockToEpochBlock.Add(common.BigToHash(i), nextBlock)
		i.Add(i, big.NewInt(1))
	}
}

func (ec *EpochCache) retrieveEpochInfo(number *big.Int) *bindings.IAutonityEpochInfo {
	epochInfo, err := ec.autonityBindings.GetEpochByHeight(&bind.CallOpts{}, number)
	if err != nil {
		slog.Error("unable to fetch epoch by height", "error", err)
		return nil
	}

	nextEpochBlock := epochInfo.NextEpochBlock
	currentEpochBlock := epochInfo.EpochBlock
	nextBlock := big.NewInt(currentEpochBlock.Int64()).Add(currentEpochBlock, big.NewInt(1))
	// committee in epochInfo is applicable from epochBlock + 1 onwards
	ec.cache.Add(common.BigToHash(nextBlock), &epochInfo)

	i := big.NewInt(currentEpochBlock.Int64())
	i.Add(i, big.NewInt(1))
	for i.Cmp(nextEpochBlock) == -1 {
		ec.blockToEpochBlock.Add(common.BigToHash(i), nextBlock)
		i.Add(i, big.NewInt(1))
	}
	return &epochInfo
}

type BlockCache interface {
	Get(number *big.Int) (*types.Block, bool)
	Add(block *types.Block)
}
