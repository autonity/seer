package helper

import (
	"context"
	"log/slog"
	"math/big"

	ethereum "github.com/autonity/autonity"
	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/common/fixsizecache"
	"github.com/autonity/autonity/core/types"

	"seer/net"
)

var (
	numBuckets     = uint(8999)
	entryPerBucket = uint(5)
)

type EthClient interface {
	SubscribeNewHead(ctx context.Context, ch chan<- *types.Header) (ethereum.Subscription, error)
	SubscribeFilterLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error)
	BlockByNumber(ctx context.Context, number *big.Int) (*types.Block, error)
	BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error)
	BlockNumber(ctx context.Context) (uint64, error)
	FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error)
}

type blockCache struct {
	cache *fixsizecache.Cache[common.Hash, *types.Block]
	cp    net.ConnectionProvider
}

func NewBlockCache(cp net.ConnectionProvider) BlockCache {
	return &blockCache{
		cache: fixsizecache.New[common.Hash, *types.Block](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		cp:    cp,
	}
}

func (bc *blockCache) Get(number *big.Int) (*types.Block, bool) {
	blk, ok := bc.cache.Get(common.BigToHash(number))
	if ok {
		return blk.(*types.Block), true
	}
	cl := bc.cp.GetWebSocketConnection().Client
	block, err := cl.BlockByNumber(context.Background(), number)
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
	cache             *fixsizecache.Cache[common.Hash, *autonity.AutonityEpochInfo]
	blockToEpochBlock *fixsizecache.Cache[common.Hash, *big.Int]
	cp                net.ConnectionProvider
	autonityBindings  *autonity.Autonity
}

func NewEpochInfoCache(cp net.ConnectionProvider) *EpochCache {
	ec := EpochCache{
		cache:             fixsizecache.New[common.Hash, *autonity.AutonityEpochInfo](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		blockToEpochBlock: fixsizecache.New[common.Hash, *big.Int](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		cp:                cp,
	}
	var err error
	cl := cp.GetWebSocketConnection().Client
	ec.autonityBindings, err = autonity.NewAutonity(AutonityContractAddress, cl)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
		return nil
	}
	return &ec
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

func (ec *EpochCache) Add(header *types.Header) {
	if header.Epoch == nil {
		return
	}
	committee := make([]autonity.AutonityCommitteeMember, 0)
	for _, member := range header.Epoch.Committee.Members {
		committee = append(committee, autonity.AutonityCommitteeMember{
			Addr:         member.Address,
			ConsensusKey: member.ConsensusKeyBytes,
			VotingPower:  member.VotingPower,
		})
	}
	epochInfo := autonity.AutonityEpochInfo{
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

func (ec *EpochCache) retrieveEpochInfo(number *big.Int) *autonity.AutonityEpochInfo {
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
