package core

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"time"

	ethereum "github.com/autonity/autonity"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/rpc"

	"seer/config"
	"seer/helper"
	"seer/interfaces"
	"seer/net"
)

var (
	maxConcurrency = 100
	batchSize      = uint64(100)

	newBlockProcessors = 10
	newEventProcessors = 10
)

type blockTracker struct {
	sync.Mutex
	processed     map[uint64]bool
	lastProcessed uint64
}

type core struct {
	cancel     context.CancelFunc
	nodeConfig config.NodeConfig
	abiParser  interfaces.ABIParser
	dbHandler  interfaces.DatabaseHandler
	cp         net.ConnectionProvider

	newBlocks chan *types.Header
	newEvents chan types.Log

	blockCache     interfaces.BlockCache
	epochInfoCache *helper.EpochCache

	bt *blockTracker
	sync.WaitGroup
}

func New(cfg config.NodeConfig, parser interfaces.ABIParser, dbHandler interfaces.DatabaseHandler, cp net.ConnectionProvider) interfaces.Core {
	return &core{
		nodeConfig:     cfg,
		newBlocks:      make(chan *types.Header, newBlockProcessors),
		newEvents:      make(chan types.Log, newEventProcessors),
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

func (c *core) handleHistory(ctx context.Context, start, end uint64) {
	go func() {
		// Expose the pprof endpoints on a specific port
		port := 6060
		slog.Info("Starting pprof server on", "port", port)
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			slog.Error("Error starting pprof server: %v", err)
		}
	}()
	c.runInGoroutine(ctx, func(ctx context.Context) {
		c.ReadHistoricalData(ctx, start, end, c.ReadEventHistory)
	})
	c.runInGoroutine(ctx, func(ctx context.Context) {
		c.ReadHistoricalData(ctx, start, end, c.ReadBlockHistory)
	})
}

func (c *core) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	helper.PrintContractAddresses()

	for i := 0; i < newEventProcessors; i++ {
		NewEventProcessor(ctx, c, c.newEvents).Process()
	}
	for i := 0; i < newBlockProcessors; i++ {
		NewBlockProcessor(ctx, c, c.newBlocks).Process()
	}
	// read current/future events and blocks
	c.runInGoroutine(ctx, c.eventReader)
	c.runInGoroutine(ctx, c.blockReader)
	if c.nodeConfig.Sync.History {
		start, end := c.getHistoricalRange(ctx)
		c.handleHistory(ctx, start, end)
	}
	c.Wait()
}

func (c *core) ProcessRange(ctx context.Context, start, end uint64) {
	helper.PrintContractAddresses()
	ctx, c.cancel = context.WithCancel(ctx)
	c.handleHistory(ctx, start, end)
	c.Wait()
}

func (c *core) Stop() {
	c.cancel()
}

func (c *core) runInGoroutine(ctx context.Context, fn func(ctx context.Context)) {
	c.Add(1)
	go func() {
		defer c.Done()
		fn(ctx)
	}()
}

func (c *core) markProcessed(start, end uint64) {
	c.bt.Lock()
	defer c.bt.Unlock()

	// mark all processed
	for i := start; i <= end; i++ {
		c.bt.processed[i] = true
	}

	update := false
	for c.bt.processed[c.bt.lastProcessed+1] {
		delete(c.bt.processed, c.bt.lastProcessed+1)
		c.bt.lastProcessed++
		update = true
	}

	if update {
		slog.Info("saving Last processed", "number", c.bt.lastProcessed)
		c.dbHandler.SaveLastProcessed(c.bt.lastProcessed)
	}
}

func (c *core) getHistoricalRange(ctx context.Context) (uint64, uint64) {
	var startBlock uint64
	lastProcessed := c.dbHandler.LastProcessed()
	if lastProcessed < batchSize {
		startBlock = 1
	} else {
		//ensure we don't miss any blocks, duplicate processing is acceptable
		startBlock = lastProcessed - batchSize
	}
	c.bt.lastProcessed = startBlock
	con := c.cp.GetWebSocketConnection()
	endBlock, _ := con.Client.BlockNumber(ctx)
	slog.Info("Reading Historical Data", "lastProcessed", startBlock, "current block", endBlock)
	return startBlock, endBlock
}

func (c *core) ReadHistoricalData(ctx context.Context, start, end uint64, batchReader func(ctx context.Context, wq chan [2]uint64)) {
	workQueue := make(chan [2]uint64, maxConcurrency)
	for i := 0; i <= maxConcurrency; i++ {
		c.runInGoroutine(ctx, func(ctx context.Context) {
			batchReader(ctx, workQueue)
		})
	}
	for i := start; i <= end; i += batchSize {
		workQueue <- [2]uint64{i, i + batchSize}
	}
}

func (c *core) ReadEventHistory(ctx context.Context, workQueue chan [2]uint64) {
	processorConcurrency := 2
	eventChs := make([]chan types.Log, processorConcurrency)
	for i := 0; i < processorConcurrency; i++ {
		eventChs[i] = make(chan types.Log, 10)
		NewEventProcessor(ctx, c, eventChs[i]).Process()
	}
	con := c.cp.GetWebSocketConnection()
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-workQueue:
			fq := ethereum.FilterQuery{
				FromBlock: big.NewInt(int64(batch[0])),
				ToBlock:   big.NewInt(int64(batch[1])),
				Addresses: helper.ContractAddresses,
			}
			slog.Debug("Starting event batch from", "startBlock", batch[0], "endBlock", batch[1])
			logs, err := con.Client.FilterLogs(ctx, fq)
			if err != nil {
				slog.Error("Unable to filter autonity logs", "error", err, "batch start", batch[0], "batch end", batch[1])
			}
			for i, log := range logs {
				select {
				case <-ctx.Done():
					return
				case eventChs[i%processorConcurrency] <- log:
				}
			}
			slog.Debug("event batch complete", "startBlock", batch[0], "endBlock", batch[1])
		}
	}
}

func (c *core) ReadBlockHistory(ctx context.Context, workQueue chan [2]uint64) {
	processorConcurrency := int(batchSize) * 10
	blockChs := make([]chan *types.Header, processorConcurrency)
	for i := 0; i < processorConcurrency; i++ {
		blockChs[i] = make(chan *types.Header, 10)
		NewBlockProcessor(ctx, c, blockChs[i]).Process()
	}
	counter := 0
	con := c.cp.GetRPCConnection()
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-workQueue:
			counter++
			now := time.Now()
			slog.Debug("Starting block batch from", "startBlock", batch[0], "endBlock", batch[1])
			var requests []rpc.BatchElem
			for i := batch[0]; i <= batch[1]; i++ {
				header := types.Header{}
				requests = append(requests, rpc.BatchElem{
					Method: "eth_getBlockByNumber",
					Args:   []interface{}{fmt.Sprintf("0x%x", i), false}, // false to exclude full transaction details
					Result: &header,
				})
			}
			err := con.Client.BatchCallContext(ctx, requests)
			if err != nil {
				slog.Error("failed to run batch call", "error", err)
				c.markProcessed(batch[0], batch[1])
				continue
			}
			fetchTime := time.Now()

			for _, call := range requests {
				if call.Error != nil {
					slog.Error("Error in batch call", "error", err)
					continue
				}
				blockChs[counter%processorConcurrency] <- call.Result.(*types.Header)
			}
			c.markProcessed(batch[0], batch[1])
			slog.Info("block batch complete", "startBlock", batch[0], "endBlock", batch[1],
				"fetch time", time.Since(fetchTime).Seconds(),
				"time taken", time.Since(now).Seconds())
		}
	}
}

func (c *core) blockReader(ctx context.Context) {
	slog.Info("subscribing to block events")

	headCh := make(chan *types.Header)
	con := c.cp.GetWebSocketConnection()
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
			c.newBlocks <- block.Header()
		}
	}
}

func (c *core) eventReader(ctx context.Context) {
	con := c.cp.GetWebSocketConnection()
	number, err := con.Client.BlockNumber(ctx)
	if err != nil {
		slog.Error("Unable to get the latest block number", "error", err)
		return
	}
	fq := ethereum.FilterQuery{FromBlock: big.NewInt(int64(number)), Addresses: helper.ContractAddresses}
	sub, err := con.Client.SubscribeFilterLogs(ctx, fq, c.newEvents)
	if err != nil {
		slog.Error("Unable to filter autonity logs", "error", err)
		return
	}
	defer sub.Unsubscribe()
	slog.Debug("subscribing to log events")

	for {
		select {
		case <-sub.Err():
			return
		case <-ctx.Done():
			return
		}
	}
}
