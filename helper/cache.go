package helper

import (
	"context"
	"log/slog"
	"math/big"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/common/fixsizecache"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/ethclient"

	"Seer/interfaces"
)

var (
	numBuckets     = uint(997)
	entryPerBucket = uint(5)
)

type blockCache struct {
	cache *fixsizecache.Cache[common.Hash, *types.Block]

	cl interfaces.EthClient
}

func NewBlockCache(cl interfaces.EthClient) interfaces.BlockCache {
	return &blockCache{
		cache: fixsizecache.New[common.Hash, *types.Block](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		cl:    cl,
	}
}

func (bc *blockCache) Get(number *big.Int) *types.Block {
	blk, ok := bc.cache.Get(common.BigToHash(number))
	if ok {
		return blk.(*types.Block)
	}

	block, err := bc.cl.BlockByNumber(context.Background(), number)
	if err != nil {
		slog.Error("unable to fetch block", "error", err)
		//todo: error handling module
		return nil
	}
	bc.cache.Add(common.BigToHash(number), block)
	return block
}

type EpochCache struct {
	cache        *fixsizecache.Cache[common.Hash, *autonity.AutonityEpochInfo]
	blockToEpoch *fixsizecache.Cache[common.Hash, *big.Int]
	cl           *ethclient.Client
}

func NewEpochInfoCache(cl *ethclient.Client) *EpochCache {
	return &EpochCache{
		cache:        fixsizecache.New[common.Hash, *autonity.AutonityEpochInfo](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		blockToEpoch: fixsizecache.New[common.Hash, *big.Int](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		cl:           cl,
	}
}

func (bc *EpochCache) Get(number *big.Int) *autonity.AutonityEpochInfo {
	epochBlockNum, ok := bc.blockToEpoch.Get(common.BigToHash(number))
	if !ok {
		return bc.retrieveEpochInfo(number)
	}
	epochBlock, ok := bc.cache.Get(common.BigToHash(epochBlockNum.(*big.Int)))
	if !ok {
		return bc.retrieveEpochInfo(number)
	}
	return epochBlock.(*autonity.AutonityEpochInfo)
}

func (bc *EpochCache) Add(block *types.Block) {
	if block.Header().Epoch == nil {
		return
	}
	epoch := block.Header().Epoch
	committee := make([]autonity.AutonityCommitteeMember, 0)
	for _, member := range epoch.Committee.Members {
		committee = append(committee, autonity.AutonityCommitteeMember{
			Addr:         member.Address,
			ConsensusKey: member.ConsensusKeyBytes,
			VotingPower:  member.VotingPower,
		})
	}
	epochInfo := &autonity.AutonityEpochInfo{
		PreviousEpochBlock: epoch.PreviousEpochBlock,
		NextEpochBlock:     epoch.NextEpochBlock,
		EpochBlock:         block.Number(),
		Committee:          committee,
	}
	bc.cache.Add(common.BigToHash(block.Number()), epochInfo)

	previousEpoch := block.Header().Epoch.PreviousEpochBlock
	currentEpoch := block.Number()
	//TODO: hash based look up for range search
	i := previousEpoch.Add(previousEpoch, big.NewInt(1))
	for i.Cmp(currentEpoch) < 0 {
		bc.blockToEpoch.Add(common.BigToHash(i), block.Number())
		i.Add(i, big.NewInt(1))
	}
}

func (bc *EpochCache) retrieveEpochInfo(number *big.Int) *autonity.AutonityEpochInfo {
	autonityBindings, err := autonity.NewAutonity(AutonityContractAddress, bc.cl)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
		return nil
	}
	epochInfo, err := autonityBindings.GetEpochByHeight(&bind.CallOpts{}, number)
	if err != nil {
		return nil
	}
	previousEpoch := epochInfo.PreviousEpochBlock
	currentEpoch := epochInfo.EpochBlock
	bc.cache.Add(common.BigToHash(currentEpoch), &epochInfo)
	//TODO: hash based look up for range search
	i := previousEpoch.Add(previousEpoch, big.NewInt(1))
	//TODO - check whther to do epoch block or nextEpochBlock
	for i.Cmp(currentEpoch) < 0 {
		bc.blockToEpoch.Add(common.BigToHash(i), currentEpoch)
		i.Add(i, big.NewInt(1))
	}
	return &epochInfo
}
