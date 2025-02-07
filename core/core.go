package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"sync/atomic"
	"time"

	ethereum "github.com/autonity/autonity"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/common/math"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/rpc"
	"github.com/autonity/autonity/trie"

	"seer/config"
	"seer/db"
	"seer/helper"
	"seer/interfaces"
	"seer/model"
	"seer/net"
)

var (
	eventMaxConcurrency = 20
	eventBatchSize      = uint64(100)
	liveEventProcessors = 10

	blockMaxConcurrency = 50
	blockBatchSize      = uint64(50)
	liveBlockProcessors = 10
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

	newBlocks chan *types.Block
	newEvents chan types.Log

	blockCache     interfaces.BlockCache
	epochInfoCache *helper.EpochCache

	blockTracker *syncTracker
	eventTracker *syncTracker
	sync.WaitGroup
	pendingMu     sync.Mutex
	pendingEvents map[uint64][]model.EventSchema
	chainID       *big.Int
}

type rpcBlock struct {
	Hash         common.Hash      `json:"hash"`
	Transactions []rpcTransaction `json:"transactions"`
}

type rpcTransaction struct {
	tx *types.Transaction
	txExtraInfo
}

type txExtraInfo struct {
	BlockNumber *string         `json:"blockNumber,omitempty"`
	BlockHash   *common.Hash    `json:"blockHash,omitempty"`
	To          *common.Address `json:"to,omitempty"`
}

func (tx *rpcTransaction) UnmarshalJSON(msg []byte) error {
	if err := json.Unmarshal(msg, &tx.tx); err != nil {
		slog.Error("error unmarshalling rpc transaction", "error", err)
		return err
	}
	return json.Unmarshal(msg, &tx.txExtraInfo)
}

func New(cfg config.NodeConfig, parser interfaces.ABIParser, dbHandler interfaces.DatabaseHandler, cp net.ConnectionProvider) interfaces.Core {
	c := &core{
		nodeConfig:     cfg,
		newBlocks:      make(chan *types.Block, liveBlockProcessors),
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
	c.chainID, _ = cp.GetWebSocketConnection().Client.NetworkID(context.Background())
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
		c.ReadHistoricalData(ctx, start, end, c.ReadBlockHistory, blockMaxConcurrency, blockBatchSize)
	})
}

func (c *core) handleEventHistory(ctx context.Context, start, end uint64) {
	slog.Info("Reading event history ", "from", start, "to", end)
	c.runInGoroutine(ctx, func(ctx context.Context) {
		c.ReadHistoricalData(ctx, start, end, c.ReadEventHistory, eventMaxConcurrency, eventBatchSize)
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
		NewBlockProcessor(ctx, c, c.newBlocks, true, c.chainID).Process()
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
		slog.Debug("saving Last processed Block", "number", lastProcessed)
		c.dbHandler.SaveLastProcessed(db.BlockNumKey, lastProcessed)
	}
}

func (c *core) markProcessedEvent(start, end uint64) {
	update, lastProcessed := c.eventTracker.updateProcessed(start, end)
	if update {
		slog.Debug("saving Last processed Event", "number", lastProcessed)
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
	switch fieldKey {
	case db.EventNumKey:
		if lastProcessed < eventBatchSize {
			startBlock = 1
		} else {

			startBlock = lastProcessed - eventBatchSize
		}
		c.eventTracker.lastProcessed = startBlock
	case db.BlockNumKey:
		if lastProcessed < blockBatchSize {
			startBlock = 0
		} else {
			//ensure we don't miss any blocks, duplicate processing is acceptable
			startBlock = lastProcessed - blockBatchSize
		}
		c.blockTracker.lastProcessed = startBlock
	}
	con := c.cp.GetWebSocketConnection()
	endBlock, _ := con.Client.BlockNumber(ctx)
	return startBlock, endBlock
}

func (c *core) ReadHistoricalData(ctx context.Context, start, end uint64, batchReader func(ctx context.Context, wq chan [2]uint64), maxConcurrency int, batchSize uint64) {
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
	processorConcurrency := eventMaxConcurrency * 4
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
	processorConcurrency := int(blockBatchSize) * 10
	blockChs := make([]chan *types.Block, processorConcurrency)
	for i := 0; i < processorConcurrency; i++ {
		blockChs[i] = make(chan *types.Block)
		NewBlockProcessor(ctx, c, blockChs[i], false, c.chainID).Process()
	}
	counter := 0
	con := c.cp.GetRPCConnection()
	requests := make([]rpc.BatchElem, blockBatchSize)
	raws := make([]json.RawMessage, blockBatchSize)
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
			now := time.Now()
			slog.Info("Starting block batch from", "startBlock", batch[0], "endBlock", batch[1])
			fetchTime := time.Now()
			index := 0
			for i := batch[0]; i < batch[1]; i++ {
				requests[index] = rpc.BatchElem{
					Method: "eth_getBlockByNumber",
					Args:   []interface{}{fmt.Sprintf("0x%x", i), true}, // false to exclude full transaction details
					Result: &raws[index],
				}
				index++
			}

			err := con.Client.BatchCallContext(ctx, requests)
			if err != nil {
				slog.Error("failed to run batch call", "error", err)
				c.markProcessedBlock(batch[0], batch[1])
				continue
			}
			for _, call := range requests {
				if call.Error != nil {
					slog.Error("Error in batch call", "error", err)
					continue
				}
				var head *types.Header
				var body rpcBlock
				if err := json.Unmarshal(*call.Result.(*json.RawMessage), &head); err != nil {
					slog.Error("Error json unmarshalling head", "error", err)
					continue
				}
				if err := json.Unmarshal(*call.Result.(*json.RawMessage), &body); err != nil {
					slog.Error("Error json unmarshalling, body", "error", err)
					continue
				}
				txs := make([]*types.Transaction, 0)
				for _, rpcTx := range body.Transactions {
					if rpcTx.To != nil && *rpcTx.To == helper.OracleContractAddress {
						txs = append(txs, rpcTx.tx)
					}
				}
				blockChs[counter%processorConcurrency] <- types.NewBlock(head, txs, nil, nil, new(trie.Trie))
				counter++
			}
			// get Block receipts
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
			c.newBlocks <- block
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
