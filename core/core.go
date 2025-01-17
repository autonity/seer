package core

import (
	"context"
	"log/slog"
	"math/big"
	"sync"
	"time"

	ethereum "github.com/autonity/autonity"
	"github.com/autonity/autonity/core/types"

	"seer/config"
	"seer/helper"
	"seer/interfaces"
	"seer/net"
)

var (
	maxConcurrency = 100
	batchSize      = uint64(50)
)

type blockTracker struct {
	sync.Mutex
	processed     map[uint64]bool
	lastProcessed uint64
}

type core struct {
	nodeConfig config.NodeConfig
	abiParser  interfaces.ABIParser
	dbHandler  interfaces.DatabaseHandler
	cp         net.ConnectionProvider

	newBlocks chan *types.Block
	newEvents chan types.Log

	blockCache     interfaces.BlockCache
	epochInfoCache *helper.EpochCache

	bt *blockTracker
	sync.WaitGroup
}

func New(cfg config.NodeConfig, parser interfaces.ABIParser, dbHandler interfaces.DatabaseHandler, cp net.ConnectionProvider) interfaces.Core {
	return &core{
		nodeConfig:     cfg,
		newBlocks:      make(chan *types.Block, 10),
		newEvents:      make(chan types.Log, 100),
		abiParser:      parser,
		dbHandler:      dbHandler,
		cp:             cp,
		blockCache:     helper.NewBlockCache(cp),
		epochInfoCache: helper.NewEpochInfoCache(cp),
		bt: &blockTracker{
			processed:     make(map[uint64]bool),
			lastProcessed: 0,
		},
	}
}

func (l *core) Start(ctx context.Context) {
	helper.PrintContractAddresses()

	l.runInGoroutine(ctx, l.eventReader)
	l.runInGoroutine(ctx, l.blockReader)

	NewEventProcessor(ctx, l).Process()
	NewBlockProcessor(ctx, l).Process()

	if l.nodeConfig.Sync.History {
		start, end := l.getHistoricalRange(ctx)
		l.runInGoroutine(ctx, func(ctx context.Context) {
			l.ReadHistoricalData(ctx, start, end, l.ReadEventHistory)
		})
		l.runInGoroutine(ctx, func(ctx context.Context) {
			l.ReadHistoricalData(ctx, start, end, l.ReadBlockHistory)
		})
	}
}

func (l *core) Stop() {
	l.Wait()
}

func (c *core) runInGoroutine(ctx context.Context, fn func(ctx context.Context)) {
	c.Add(1)
	go func() {
		defer c.Done()
		fn(ctx)
	}()
}

func (l *core) ProcessRange(ctx context.Context, start, end uint64) {
	helper.PrintContractAddresses()

	NewEventProcessor(ctx, l).Process()
	NewBlockProcessor(ctx, l).Process()

	l.runInGoroutine(ctx, func(ctx context.Context) {
		l.ReadHistoricalData(ctx, start, end, l.ReadEventHistory)
	})
	l.runInGoroutine(ctx, func(ctx context.Context) {
		l.ReadHistoricalData(ctx, start, end, l.ReadBlockHistory)
	})
}

func (l *core) markProcessed(start, end uint64) {
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

func (l *core) getHistoricalRange(ctx context.Context) (uint64, uint64) {
	var startBlock uint64
	lastProcessed := l.dbHandler.LastProcessed()
	if lastProcessed < batchSize {
		startBlock = 1
	} else {
		//ensure we don't miss any blocks, duplicate processing is acceptable
		startBlock = lastProcessed - batchSize
	}
	l.bt.lastProcessed = startBlock
	con := l.cp.GetWebSocketConnection()
	endBlock, _ := con.Client.BlockNumber(ctx)
	slog.Info("Reading Historical Data", "lastProcessed", startBlock, "current block", endBlock)
	return startBlock, endBlock
}

func (l *core) ReadHistoricalData(ctx context.Context, start, end uint64, batchReader func(ctx context.Context, wq chan [2]uint64)) {
	workQueue := make(chan [2]uint64, maxConcurrency)
	for i := 0; i <= maxConcurrency; i++ {
		l.runInGoroutine(ctx, func(ctx context.Context) {
			batchReader(ctx, workQueue)
		},
		)
	}
	for i := start; i <= end; i += batchSize {
		workQueue <- [2]uint64{i, i + batchSize}
	}

}

func (l *core) ReadEventHistory(ctx context.Context, workQueue chan [2]uint64) {
	con := l.cp.GetWebSocketConnection()
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
			logs, err := con.Client.FilterLogs(ctx, fq)
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
		}
	}
}

func (l *core) ReadBlockHistory(ctx context.Context, workQueue chan [2]uint64) {
	con := l.cp.GetWebSocketConnection()
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-workQueue:
			now := time.Now()

			////TODO: batch requests for blocks
			//for i := batch[0]; i <= batch[1]; i++ {
			//	requests = append(requests, JSONRPCRequest{
			//		Jsonrpc: "2.0",
			//		Method:  "eth_getBlockByNumber",
			//		Params:  []interface{}{fmt.Sprintf("0x%x", i), false}, // true to include full transaction details
			//		ID:      i,
			//	})
			//}
			for num := batch[0]; num < batch[1]; num++ {
				block, err := con.Client.BlockByNumber(ctx, big.NewInt(int64(num)))
				if err != nil {
					slog.Error("Unable to fetch block", "error", err, "batch start", batch[0], "batch end", batch[1])
					continue
				}
				l.newBlocks <- block
			}

			slog.Info("Batch Complete", "startBlock", batch[0], "endBlock", batch[1], "time taken", time.Since(now).Seconds())
			l.markProcessed(batch[0], batch[1])
			slog.Info("Batch Complete and mark processed", "startBlock", batch[0], "endBlock", batch[1], "time taken", time.Since(now).Seconds())
		}
	}
}

func (l *core) blockReader(ctx context.Context) {
	slog.Info("subscribing to block events")

	headCh := make(chan *types.Header)
	con := l.cp.GetWebSocketConnection()
	newHeadSub, err := con.Client.SubscribeNewHead(context.Background(), headCh)
	if err != nil {
		slog.Error("new head subscription failed", "error", err)
		return
	}
	defer newHeadSub.Unsubscribe()

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

			block, err := con.Client.BlockByNumber(ctx, header.Number)
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

func (l *core) eventReader(ctx context.Context) {
	defer l.Done()
	con := l.cp.GetWebSocketConnection()
	number, err := con.Client.BlockNumber(ctx)
	if err != nil {
		slog.Error("Unable to get the latest block number", "error", err)
		return
	}
	fq := ethereum.FilterQuery{FromBlock: big.NewInt(int64(number)), Addresses: helper.ContractAddresses}
	sub, err := con.Client.SubscribeFilterLogs(ctx, fq, l.newEvents)
	if err != nil {
		slog.Error("Unable to filter autonity logs", "error", err)
		return
	}
	defer sub.Unsubscribe()
	slog.Debug("subscribing to log events")

	// start event processor
	NewEventProcessor(ctx, l).Process()

	for {
		select {
		case <-sub.Err():
			return
		case <-ctx.Done():
			return
		}
	}
}
