package helper

import (
	"context"
	"log/slog"

	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/common/fixsizecache"
	"github.com/autonity/autonity/core/types"

	"Seer/interfaces"
)

var (
	numBuckets     = uint(997)
	entryPerBucket = uint(5)
)

type blockCache struct {
	cache *fixsizecache.Cache[common.Hash, *types.Block]
	cl    interfaces.EthClient
}

func NewBlockCache(cl interfaces.EthClient) interfaces.BlockCache {
	return &blockCache{
		cache: fixsizecache.New[common.Hash, *types.Block](numBuckets, entryPerBucket, fixsizecache.HashKey[common.Hash]),
		cl:    cl,
	}
}

func (bc *blockCache) Get(hash common.Hash) *types.Block {
	blk, ok := bc.cache.Get(hash)
	if ok {
		return blk.(*types.Block)
	}

	block, err := bc.cl.BlockByHash(context.Background(), hash)
	if err != nil {
		slog.Error("unable to fetch block", "error", err)
		//todo: error handling module
		return nil
	}
	bc.cache.Add(hash, block)
	return block
}
