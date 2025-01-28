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
	cache *fixsizecache.Cache[common.Hash, *types.Header]
	cl    interfaces.EthClient
}

func NewBlockCache(cp net.ConnectionProvider) interfaces.BlockCache {
	return &blockCache{
		cache: fixsizecache.New[common.Hash, *types.Header](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		cl:    cp.GetWebSocketConnection().Client,
	}
}

func (bc *blockCache) Get(number *big.Int) (*types.Header, bool) {
	blk, ok := bc.cache.Get(common.BigToHash(number))
	if ok {
		return blk.(*types.Header), true
	}
	block, err := bc.cl.BlockByNumber(context.Background(), number)
	if err != nil {
		slog.Error("unable to fetch block", "error", err)
		//todo: error handling module
		return nil, false
	}
	header := block.Header()
	bc.cache.Add(common.BigToHash(number), header)
	return header, true
}

func (bc *blockCache) Add(header *types.Header) {
	bc.cache.Add(common.BigToHash(header.Number), header)
}

type EpochCache struct {
	cache             *fixsizecache.Cache[common.Hash, *autonity.AutonityEpochInfo]
	blockToEpochBlock *fixsizecache.Cache[common.Hash, *big.Int]
	cl                interfaces.EthClient
	autonityBindings  *autonity.Autonity
}

func NewEpochInfoCache(cp net.ConnectionProvider) *EpochCache {
	ec := EpochCache{
		cache:             fixsizecache.New[common.Hash, *autonity.AutonityEpochInfo](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		blockToEpochBlock: fixsizecache.New[common.Hash, *big.Int](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		cl:                cp.GetWebSocketConnection().Client,
	}
	var err error
	ec.autonityBindings, err = autonity.NewAutonity(AutonityContractAddress, ec.cl.(*ethclient.Client))
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
	//TODO : epoch cache info for the epoch block is wrong

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
		return nil
	}

	nextEpochBlock := epochInfo.NextEpochBlock
	currentEpochBlock := epochInfo.EpochBlock
	//TODO: hash based look up for range search
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
