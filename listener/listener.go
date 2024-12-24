package listener

import (
	"context"
	"log/slog"
	"math/big"
	"sync"

	ethereum "github.com/autonity/autonity"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/ethclient"

	"Seer/config"
	"Seer/interfaces"
)

var (
	maxConcurrency = 100
	batchSize      = uint64(1000)
)

type Listener struct {
	nodeConfig         config.NodeConfig
	abiParser          interfaces.ABIParser
	dbHandler          interfaces.DatabaseHandler
	newBlocks          chan *types.Block
	newEvents          chan types.Log
	lastProcessedBlock *big.Int

	sync.WaitGroup
}

func NewListener(cfg config.NodeConfig, parser interfaces.ABIParser, dbHandler interfaces.DatabaseHandler) interfaces.Listener {
	lastProcessed := dbHandler.LastProcessed()
	return &Listener{
		nodeConfig:         cfg,
		newBlocks:          make(chan *types.Block, 10),
		newEvents:          make(chan types.Log, 100),
		abiParser:          parser,
		dbHandler:          dbHandler,
		lastProcessedBlock: big.NewInt(lastProcessed),
	}
}

func (l *Listener) Start(ctx context.Context) {
	slog.Info("connecting to node using websocket", "url", l.nodeConfig.WS)
	client, err := ethclient.Dial(l.nodeConfig.WS)
	if err != nil {
		slog.Error("dial error", "err", err, "url")
		return
	}
	slog.Info("successfully connected to node", "url", l.nodeConfig.RPC)
	l.Add(1)
	go l.eventReader(ctx, client)
	l.Add(1)
	go l.blockReader(ctx, client)
	if l.nodeConfig.Sync.History {
		l.Add(1)
		// using rpc node endpoint for historical data
		// todo: can only use ws
		client, err := ethclient.Dial(l.nodeConfig.RPC)
		if err != nil {
			slog.Error("dial error", "err", err)
			return
		}
		l.ReadHistoricalData(ctx, client)
	}

}
func (l *Listener) ReadBatch(ctx context.Context, cl *ethclient.Client, workQueue chan [2]uint64) {
	defer l.Done()
	for batch := range workQueue {
		fq := ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(batch[0])),
			ToBlock:   big.NewInt(int64(batch[1])),
		}
		slog.Info("fetching logs from", "startBlock", batch[0], "endBlock", batch[1])
		logs, err := cl.FilterLogs(ctx, fq)
		if err != nil {
			slog.Error("Unable to filter autonity logs", "error", err)
			return
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

func (l *Listener) ReadHistoricalData(ctx context.Context, cl *ethclient.Client) {
	defer l.Done()

	// start event processor
	l.Add(1)
	go func() {
		defer l.Done()
		ep := NewEventProcessor(ctx, l.newEvents)
		ep.Process()
	}()

	startBlock := l.lastProcessedBlock.Uint64()
	endBlock, _ := cl.BlockNumber(ctx)

	workQueue := make(chan [2]uint64, maxConcurrency)

	for i := 0; i <= maxConcurrency; i++ {
		l.Add(1)
		go l.ReadBatch(ctx, cl, workQueue)
	}
	for i := startBlock; i <= endBlock; i += batchSize {
		workQueue <- [2]uint64{i, i + batchSize}
	}

}

func (l *Listener) blockReader(ctx context.Context, cl *ethclient.Client) {
	defer l.Done()
	headCh := make(chan *types.Header)
	slog.Info("subscribing to block events")
	newHeadSub, err := cl.SubscribeNewHead(context.Background(), headCh)
	if err != nil {
		slog.Error("new head subscription failed", "error", err)
		return
	}
	defer newHeadSub.Unsubscribe()

	// start block processor
	l.Add(1)
	go func() {
		bp := NewBlockProcessor(ctx, l.newBlocks)
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
			slog.Info("new head event")
			if !ok {
				slog.Error("unknown error head ch")
				return
			}
			block, err := cl.BlockByNumber(ctx, header.Number)
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

func (l *Listener) eventReader(ctx context.Context, cl *ethclient.Client) {
	defer l.Done()
	number, err := cl.BlockNumber(ctx)
	if err != nil {
		slog.Error("Unable to get the latest block number", "error", err)
		return
	}
	fq := ethereum.FilterQuery{FromBlock: big.NewInt(int64(number))}
	sub, err := cl.SubscribeFilterLogs(ctx, fq, l.newEvents)
	if err != nil {
		slog.Error("Unable to filter autonity logs", "error", err)
		return
	}
	defer sub.Unsubscribe()
	slog.Info("subscribing to log events")

	// start event processor
	l.Add(1)
	go func() {
		defer l.Done()
		ep := NewEventProcessor(ctx, l.newEvents)
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

//func (l *Listener) filterQuery(ctx context.Context, cl *ethclient.Client) ethereum.FilterQuery {
//	//TODO: block number
//	fromBlock := big.NewInt(82200)
//	switch l.nodeConfig.Sync.From {
//	case "last":
//		//TODO: pull last processed block
//	case "latest":
//		number, err := cl.BlockNumber(ctx)
//		if err != nil {
//			return ethereum.FilterQuery{}
//		}
//		fromBlock = big.NewInt(int64(number))
//	}
//	return ethereum.FilterQuery{
//		FromBlock: fromBlock,
//	}
//}
