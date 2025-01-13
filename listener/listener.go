package listener

import (
	"context"
	"log/slog"
	"math/big"
	"sync"
	"time"

	ethereum "github.com/autonity/autonity"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/ethclient"
	"github.com/autonity/autonity/rpc"

	"Seer/config"
	"Seer/helper"
	"Seer/interfaces"
)

var (
	maxConcurrency = 100
	batchSize      = uint64(100)
)

type blockTracker struct {
	sync.Mutex
	processed     map[uint64]bool
	lastProcessed uint64
}

type Listener struct {
	nodeConfig config.NodeConfig
	abiParser  interfaces.ABIParser
	dbHandler  interfaces.DatabaseHandler
	newBlocks  chan *types.Block
	newEvents  chan types.Log
	blockCache interfaces.BlockCache
	epochInfoCache *helper.EpochCache
	bt         *blockTracker
	client     *ethclient.Client
	rpc        *rpc.Client
	sync.WaitGroup
}

func NewListener(cfg config.NodeConfig, parser interfaces.ABIParser, dbHandler interfaces.DatabaseHandler) interfaces.Listener {
	return &Listener{
		nodeConfig: cfg,
		newBlocks:  make(chan *types.Block, 10),
		newEvents:  make(chan types.Log, 100),
		abiParser:  parser,
		dbHandler:  dbHandler,
		bt: &blockTracker{
			processed:     make(map[uint64]bool),
			lastProcessed: 0,
		},
	}
}

func (l *Listener) Start(ctx context.Context) {
	var err error
	l.client, err = ethclient.Dial(l.nodeConfig.WS)
	if err != nil {
		slog.Error("dial error", "err", err, "url", l.nodeConfig.WS)
		return
	}
	l.rpc, err = rpc.Dial(l.nodeConfig.RPC)
	if err != nil {
		slog.Error("rpc dial error", "err", err, "url", l.nodeConfig.RPC)
		return
	}
	l.blockCache = helper.NewBlockCache(l.client)
	l.epochInfoCache = helper.NewEpochInfoCache(l.client)
	slog.Info("successfully connected to node", "url", l.nodeConfig.WS)
	helper.PrintContractAddresses()
	l.Add(1)
	go l.eventReader(ctx)
	l.Add(1)
	go l.blockReader(ctx)
	if l.nodeConfig.Sync.History {
		l.Add(1)
		l.ReadHistoricalData(ctx)
	}
}

func (l *Listener) markProcessed(start, end uint64) {
	l.bt.Lock()
	defer l.bt.Unlock()

	// mark all processed
	for i := start; i <= end; i++ {
		l.bt.processed[i] = true
	}

	update := false
	for l.bt.processed[l.bt.lastProcessed+1] {
		delete(l.bt.processed, l.bt.lastProcessed+1)
		l.bt.lastProcessed++
		update = true
	}

	if update {
		slog.Info("saving Last processed", "number", l.bt.lastProcessed)
		l.dbHandler.SaveLastProcessed(l.bt.lastProcessed)
	}
}

func (l *Listener) ReadHistoricalData(ctx context.Context) {
	defer l.Done()

	// start event processor
	l.Add(1)
	go func() {
		defer l.Done()
		ep := NewEventProcessor(ctx, l)
		ep.Process()
	}()
	var startBlock uint64
	lastProcessed := l.dbHandler.LastProcessed()
	if lastProcessed < batchSize {
		startBlock = 1
	} else {
		//ensure we don't miss any blocks, duplicate processing is acceptable
		startBlock = lastProcessed - batchSize
	}
	l.bt.lastProcessed = startBlock
	endBlock, _ := l.client.BlockNumber(ctx)
	slog.Debug("Reading Historical Data", "lastProcessed", startBlock, "current block", endBlock)

	workQueue := make(chan [2]uint64, maxConcurrency)

	for i := 0; i <= maxConcurrency; i++ {
		l.Add(1)
		go l.ReadBatch(ctx, workQueue)
	}

	for i := startBlock; i <= endBlock; i += batchSize {
		workQueue <- [2]uint64{i, i + batchSize}
	}
}

func (l *Listener) ReadBatch(ctx context.Context, workQueue chan [2]uint64) {
	defer l.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-workQueue:
		retry:
			fq := ethereum.FilterQuery{
				FromBlock: big.NewInt(int64(batch[0])),
				ToBlock:   big.NewInt(int64(batch[1])),
				Addresses: helper.ContractAddresses,
			}
			slog.Info("Starting Batch from", "startBlock", batch[0], "endBlock", batch[1])
			now := time.Now()
			logs, err := l.client.FilterLogs(ctx, fq)
			if err != nil {
				slog.Error("Unable to filter autonity logs", "error", err, "batch start", batch[0], "batch end", batch[1])
				time.Sleep(time.Second)
				goto retry
			}
			for _, log := range logs {
				select {
				case <-ctx.Done():
					return
				case l.newEvents <- log:
				}
			}
			// batch complete
			slog.Info("Batch Complete", "startBlock", batch[0], "endBlock", batch[1], "time taken", time.Since(now).Seconds())
			l.markProcessed(batch[0], batch[1])
			slog.Info("Batch Complete and mark processed", "startBlock", batch[0], "endBlock", batch[1], "time taken", time.Since(now).Seconds())
		}
	}

}

func (l *Listener) blockReader(ctx context.Context) {
	defer l.Done()
	headCh := make(chan *types.Header)
	slog.Info("subscribing to block events")

	newHeadSub, err := l.client.SubscribeNewHead(context.Background(), headCh)
	if err != nil {
		slog.Error("new head subscription failed", "error", err)
		return
	}
	defer newHeadSub.Unsubscribe()

	// start block processor
	l.Add(1)
	go func() {
		bp := NewBlockProcessor(ctx, l)
		bp.Process()
		defer l.Done()
	}()

	for {
		select {
		case <-newHeadSub.Err():
			return
		case <-ctx.Done():
			return
		case header, ok := <-headCh:
			slog.Debug("new head event")
			if !ok {
				slog.Error("unknown error head ch")
				return
			}
			block, err := l.client.BlockByNumber(ctx, header.Number)
			if err != nil {
				slog.Error("Error fetching block by number",
					"hash", header.Hash(),
					"number", header.Number.Uint64(),
					"error", err)
				continue
			}
			l.newBlocks <- block
		}
	}
}

func (l *Listener) eventReader(ctx context.Context) {
	defer l.Done()
	number, err := l.client.BlockNumber(ctx)
	if err != nil {
		slog.Error("Unable to get the latest block number", "error", err)
		return
	}
	fq := ethereum.FilterQuery{FromBlock: big.NewInt(int64(number)), Addresses: helper.ContractAddresses}
	sub, err := l.client.SubscribeFilterLogs(ctx, fq, l.newEvents)
	if err != nil {
		slog.Error("Unable to filter autonity logs", "error", err)
		return
	}
	defer sub.Unsubscribe()
	slog.Debug("subscribing to log events")

	// start event processor
	l.Add(1)
	go func() {
		defer l.Done()
		ep := NewEventProcessor(ctx, l)
		ep.Process()
	}()

	for {
		select {
		case <-sub.Err():
			return
		case <-ctx.Done():
			return
		}
	}
}

func (l *Listener) Stop() {
	l.Wait()
}
