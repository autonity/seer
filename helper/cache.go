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

	"seer/interfaces"
	"seer/net"
)

var (
	numBuckets     = uint(9997)
	entryPerBucket = uint(10)
)

type blockCache struct {
	cache *fixsizecache.Cache[common.Hash, *types.Block]
	cl    interfaces.EthClient
}

func NewBlockCache(cp net.ConnectionProvider) interfaces.BlockCache {
	return &blockCache{
		cache: fixsizecache.New[common.Hash, *types.Block](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		cl:    cp.GetWebSocketConnection().Client,
	}
}

func (bc *blockCache) Get(number *big.Int) (*types.Block, bool) {
	blk, ok := bc.cache.Get(common.BigToHash(number))
	if ok {
		return blk.(*types.Block), true
	}
	block, err := bc.cl.BlockByNumber(context.Background(), number)
	if err != nil {
		slog.Error("unable to fetch block", "error", err)
		//todo: error handling module
		return nil, false
	}
	bc.cache.Add(common.BigToHash(number), block)
	return block, false
}

func (bc *blockCache) Add(block *types.Block) {
	bc.cache.Add(common.BigToHash(block.Number()), block)
}

type EpochCache struct {
	cache             *fixsizecache.Cache[common.Hash, *autonity.AutonityEpochInfo]
	blockToEpochBlock *fixsizecache.Cache[common.Hash, *big.Int]
	cl                interfaces.EthClient
}

func NewEpochInfoCache(cp net.ConnectionProvider) *EpochCache {
	return &EpochCache{
		cache:             fixsizecache.New[common.Hash, *autonity.AutonityEpochInfo](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		blockToEpochBlock: fixsizecache.New[common.Hash, *big.Int](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		cl:                cp.GetWebSocketConnection().Client,
	}
}

func (ec *EpochCache) Get(number *big.Int) *autonity.AutonityEpochInfo {
	epochBlockNum, ok := ec.blockToEpochBlock.Get(common.BigToHash(number))
	if !ok {
		return ec.retrieveEpochInfo(number)
	}
	epochBlock, ok := ec.cache.Get(common.BigToHash(epochBlockNum.(*big.Int)))
	if !ok {
		return ec.retrieveEpochInfo(number)
	}
	return epochBlock.(*autonity.AutonityEpochInfo)
}

func (ec *EpochCache) Add(block *types.Block) {
	if block.Header().Epoch == nil {
		return
	}
	//TODO : epoch cache info for the epoch block is wrong
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
	nextEpochBlock := block.Header().Epoch.NextEpochBlock
	currentEpochBlock := block.Number()
	i := big.NewInt(currentEpochBlock.Int64())
	i = i.Add(i, big.NewInt(1))

	// committee in epochInfo is applicable from epochBlock + 1 onwards
	ec.cache.Add(common.BigToHash(i), epochInfo)
	for i.Cmp(nextEpochBlock) == -1 {
		ec.blockToEpochBlock.Add(common.BigToHash(i), block.Number())
		i.Add(i, big.NewInt(1))
	}
}

func (ec *EpochCache) retrieveEpochInfo(number *big.Int) *autonity.AutonityEpochInfo {
	autonityBindings, err := autonity.NewAutonity(AutonityContractAddress, ec.cl.(*ethclient.Client))
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
		return nil
	}
	epochInfo, err := autonityBindings.GetEpochByHeight(&bind.CallOpts{}, number)
	if err != nil {
		return nil
	}

	nextEpochBlock := epochInfo.NextEpochBlock
	currentEpochBlock := epochInfo.EpochBlock
	//TODO: hash based look up for range search
	i := big.NewInt(currentEpochBlock.Int64())
	i.Add(i, big.NewInt(1))
	// committee in epochInfo is applicable from epochBlock + 1 onwards
	ec.cache.Add(common.BigToHash(i), &epochInfo)
	for i.Cmp(nextEpochBlock) == -1 {
		ec.blockToEpochBlock.Add(common.BigToHash(i), currentEpochBlock)
		i.Add(i, big.NewInt(1))
	}
	return &epochInfo
}
