package listener

import (
	"context"
	"log/slog"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"Seer/config"
	"Seer/interfaces"
)

type Listener struct {
	nodeConfig config.NodeConfig
	abiParser  interfaces.ABIParser
	dbHandler  interfaces.DatabaseHandler
	newBlocks  chan *types.Block
	newEvents  chan types.Log
	sync.WaitGroup
}

func NewListener(ctx context.Context, cfg config.NodeConfig, parser interfaces.ABIParser, dbHandler interfaces.DatabaseHandler) interfaces.Listener {
	return &Listener{
		nodeConfig: cfg,
		newBlocks:  make(chan *types.Block, 10),
		newEvents:  make(chan types.Log, 100),
		abiParser:  parser,
		dbHandler:  dbHandler,
	}
}

func (l *Listener) Start(ctx context.Context) {
	client, err := ethclient.Dial(l.nodeConfig.RPC)
	if err != nil {
		return
	}
	l.Add(1)
	go l.eventReader(ctx, client)
	l.Add(1)
	go l.blockReader(ctx, client)
}

func (l *Listener) filterQuery(ctx context.Context, cl *ethclient.Client) ethereum.FilterQuery {
	fromBlock := big.NewInt(0)
	switch l.nodeConfig.Sync.From {
	case "last":
		//TODO: pull last processed block
	case "latest":
		number, err := cl.BlockNumber(ctx)
		if err != nil {
			return ethereum.FilterQuery{}
		}
		fromBlock = big.NewInt(int64(number))
	}
	return ethereum.FilterQuery{
		FromBlock: fromBlock,
	}
}

func (l *Listener) blockReader(ctx context.Context, cl *ethclient.Client) {
	defer l.Done()
	headCh := make(chan *types.Header)
	newHeadSub, err := cl.SubscribeNewHead(context.Background(), headCh)
	if err != nil {
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
			if !ok {
				slog.Error("unknown error head ch")
				return
			}
			block, err := cl.BlockByHash(ctx, header.Hash())
			if err != nil {
				slog.Error("Error fetching block by hash",
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
	sub, err := cl.SubscribeFilterLogs(ctx, l.filterQuery(ctx, cl), l.newEvents)
	if err != nil {
		slog.Error("Unable to filter autonity logs", "error", err)
		return
	}
	defer sub.Unsubscribe()

	// start event processor
	l.Add(1)
	go func() {
		ep := NewEventProcessor(ctx, l.newEvents)
		ep.Process()
		defer l.Done()
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
