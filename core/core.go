package core

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"sync/atomic"
	"time"

	ethereum "github.com/autonity/autonity"
	"github.com/autonity/autonity/common/math"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/rpc"

	"seer/config"
	"seer/db"
	"seer/helper"
	"seer/interfaces"
	"seer/model"
	"seer/net"
)

var (
	maxConcurrency = 100
	batchSize      = uint64(100)

	liveBlockProcessors = 10
	liveEventProcessors = 10
)

type syncTracker struct {
	sync.Mutex
	processed     map[uint64]bool
	lastProcessed uint64
}

func (s *syncTracker) updateProcessed(start, end uint64) (bool, uint64) {
	s.Lock()
	defer s.Unlock()

	// mark all processed
	for i := start; i <= end; i++ {
		s.processed[i] = true
	}

	update := false
	for s.processed[s.lastProcessed+1] {
		delete(s.processed, s.lastProcessed+1)
		s.lastProcessed++
		update = true
	}
	return update, s.lastProcessed
}

func (s *syncTracker) updateProcessedUpto(end uint64) (bool, uint64) {
	s.Lock()
	defer s.Unlock()

	// if already updated
	if s.processed[end] {
		return false, s.lastProcessed
	}

	// mark all processed upto this block
	for i := s.lastProcessed; i <= end; i++ {
		s.processed[i] = true
	}

	update := false
	for s.processed[s.lastProcessed+1] {
		delete(s.processed, s.lastProcessed+1)
		s.lastProcessed++
		update = true
	}
	return update, s.lastProcessed
}

type core struct {
	cancel        context.CancelFunc
	nodeConfig    config.NodeConfig
	abiParser     interfaces.ABIParser
	dbHandler     interfaces.DatabaseHandler
	cp            net.ConnectionProvider
	historySynced atomic.Bool

	newBlocks chan *types.Header
	newEvents chan types.Log

	blockCache     interfaces.BlockCache
	epochInfoCache *helper.EpochCache

	blockTracker *syncTracker
	eventTracker *syncTracker
	sync.WaitGroup
	pendingMu     sync.Mutex
	pendingEvents map[uint64][]model.EventSchema
}

func New(cfg config.NodeConfig, parser interfaces.ABIParser, dbHandler interfaces.DatabaseHandler, cp net.ConnectionProvider) interfaces.Core {
	c := &core{
		nodeConfig:     cfg,
		newBlocks:      make(chan *types.Header, liveBlockProcessors),
		newEvents:      make(chan types.Log, liveEventProcessors),
		abiParser:      parser,
		dbHandler:      dbHandler,
		cp:             cp,
		blockCache:     helper.NewBlockCache(cp),
		epochInfoCache: helper.NewEpochInfoCache(cp),
		blockTracker: &syncTracker{
			processed:     make(map[uint64]bool),
			lastProcessed: 0,
		},
		eventTracker: &syncTracker{
			processed:     make(map[uint64]bool),
			lastProcessed: 0,
		},
	}
	return c
}

func (c *core) addPendingEvent(number uint64, ev model.EventSchema) {

	evs, ok := c.pendingEvents[number]
	if ok {
		evs = append(evs, ev)
	}
	c.pendingEvents[number] = evs
}

func (c *core) removePendingEvent(number uint64) {
	delete(c.pendingEvents, number)
}

func (c *core) getPendingEvent(number uint64) []model.EventSchema {
	return c.pendingEvents[number]
}

func (c *core) handleBlockHistory(ctx context.Context, start, end uint64) {
	slog.Info("Reading block history ", "from", start, "to", end)
	c.runInGoroutine(ctx, func(ctx context.Context) {
		c.ReadHistoricalData(ctx, start, end, c.ReadBlockHistory)
	})
}

func (c *core) handleEventHistory(ctx context.Context, start, end uint64) {
	slog.Info("Reading event history ", "from", start, "to", end)
	c.runInGoroutine(ctx, func(ctx context.Context) {
		c.ReadHistoricalData(ctx, start, end, c.ReadEventHistory)
	})
}

func (c *core) Start(ctx context.Context) {
	go func() {
		// Expose the pprof endpoints on a specific port
		port := 6060
		slog.Info("Starting pprof server on", "port", port)
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			slog.Error("Error starting pprof server: %v", err)
		}
	}()
	ctx, c.cancel = context.WithCancel(ctx)
	helper.PrintContractAddresses()

	for i := 0; i < liveBlockProcessors; i++ {
		NewBlockProcessor(ctx, c, c.newBlocks, true).Process()
	}
	for i := 0; i < liveEventProcessors; i++ {
		NewEventProcessor(ctx, c, c.newEvents, true).Process()
	}
	// read current/future events and blocks
	c.runInGoroutine(ctx, c.blockReader)
	c.runInGoroutine(ctx, c.eventReader)
	if c.nodeConfig.Sync.History {
		start, end := c.getHistoricalRange(ctx, db.BlockNumKey)
		c.handleBlockHistory(ctx, start, end)

		start, end = c.getHistoricalRange(ctx, db.EventNumKey)
		c.handleEventHistory(ctx, start, end)
	}
	c.Wait()
}

func (c *core) ProcessRange(ctx context.Context, start, end uint64) {
	go func() {
		// Expose the pprof endpoints on a specific port
		port := 6060
		slog.Info("Starting pprof server on", "port", port)
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			slog.Error("Error starting pprof server: %v", err)
		}
	}()
	helper.PrintContractAddresses()
	ctx, c.cancel = context.WithCancel(ctx)
	c.handleBlockHistory(ctx, start, end)
	c.handleEventHistory(ctx, start, end)
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

func (c *core) markProcessedBlock(start, end uint64) {
	update, lastProcessed := c.blockTracker.updateProcessed(start, end)
	if update {
		slog.Info("saving Last processed Block", "number", lastProcessed)
		c.dbHandler.SaveLastProcessed(db.BlockNumKey, lastProcessed)
	}
}

func (c *core) markProcessedEvent(start, end uint64) {
	update, lastProcessed := c.eventTracker.updateProcessed(start, end)
	if update {
		slog.Info("saving Last processed Event", "number", lastProcessed)
		c.dbHandler.SaveLastProcessed(db.EventNumKey, lastProcessed)
	}
}

func (c *core) markProcessedEventUpto(end uint64) {
	update, lastProcessed := c.eventTracker.updateProcessedUpto(end)
	if update {
		slog.Info("saving Last processed Event", "number", lastProcessed)
		c.dbHandler.SaveLastProcessed(db.EventNumKey, lastProcessed)
	}
}

func (c *core) getHistoricalRange(ctx context.Context, fieldKey string) (uint64, uint64) {
	var startBlock uint64
	lastProcessed := c.dbHandler.LastProcessed(fieldKey)
	if lastProcessed < batchSize {
		if fieldKey == db.EventNumKey { // for blocks start with 0, for events 1 because genesis event fetch fails
			startBlock = 1
		}
	} else {
		//ensure we don't miss any blocks, duplicate processing is acceptable
		startBlock = lastProcessed - batchSize
	}
	c.blockTracker.lastProcessed = startBlock
	con := c.cp.GetWebSocketConnection()
	endBlock, _ := con.Client.BlockNumber(ctx)
	return startBlock, endBlock
}

func (c *core) ReadHistoricalData(ctx context.Context, start, end uint64, batchReader func(ctx context.Context, wq chan [2]uint64)) {
	workQueues := make([]chan [2]uint64, maxConcurrency)
	for i := 0; i < maxConcurrency; i++ {
		workQueues[i] = make(chan [2]uint64)
		c.runInGoroutine(ctx, func(ctx context.Context) {
			batchReader(ctx, workQueues[i])
		})
	}
	i := start
	for i <= end {
		for j := 0; j < maxConcurrency; j++ {
			workQueues[j] <- [2]uint64{i, i + batchSize}
			i += batchSize
		}
	}

	// end marker to each goroutine
	for i := 0; i < maxConcurrency; i++ {
		workQueues[i] <- [2]uint64{math.MaxUint64, math.MaxUint64} // endmarker for bath readers
	}
	slog.Info("finished reading historical data")
	c.historySynced.Store(true)
}

func (c *core) ReadEventHistory(ctx context.Context, workQueue chan [2]uint64) {
	processorConcurrency := 2
	eventChs := make([]chan types.Log, processorConcurrency)
	for i := 0; i < processorConcurrency; i++ {
		eventChs[i] = make(chan types.Log)
		NewEventProcessor(ctx, c, eventChs[i], false).Process()
	}
	con := c.cp.GetWebSocketConnection()
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-workQueue:
			if batch[0] == math.MaxUint64 {
				for i := 0; i < processorConcurrency; i++ {
					close(eventChs[i])
				}
				return
			}
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
			slog.Info("event batch complete", "startBlock", batch[0], "endBlock", batch[1])
			c.markProcessedEvent(batch[0], batch[1])
		}
	}
}

func (c *core) ReadBlockHistory(ctx context.Context, workQueue chan [2]uint64) {
	processorConcurrency := int(batchSize) * 10
	blockChs := make([]chan *types.Header, processorConcurrency)
	for i := 0; i < processorConcurrency; i++ {
		blockChs[i] = make(chan *types.Header)
		NewBlockProcessor(ctx, c, blockChs[i], false).Process()
	}
	counter := 0
	con := c.cp.GetRPCConnection()
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-workQueue:
			if batch[0] == math.MaxUint64 {
				for i := 0; i < processorConcurrency; i++ {
					close(blockChs[i])
				}
				return
			}
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
				c.markProcessedBlock(batch[0], batch[1])
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
			c.markProcessedBlock(batch[0], batch[1])
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
