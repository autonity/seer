package core

import (
	"context"
	"log/slog"
	"testing"

	"github.com/autonity/autonity/core/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"seer/config"
	"seer/mocks"
)

func defaultListener() *core {
	cfg := config.NodeConfig{}
	return &core{
		nodeConfig: cfg,
		newBlocks:  make(chan *types.Block, 10),
		newEvents:  make(chan types.Log, 100),
		abiParser:  nil,
		dbHandler:  nil,
		bt: &blockTracker{
			processed:     make(map[uint64]bool),
			lastProcessed: 0,
		},
	}
}

func TestListener_markProcessed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock dependencies
	mockDBHandler := mocks.NewMockDatabaseHandler(ctrl)

	// Setup core
	l := defaultListener()
	l.dbHandler = mockDBHandler

	// Mock DatabaseHandler behavior
	mockDBHandler.EXPECT().SaveLastProcessed(uint64(10)).Times(1)

	// Mark a range as processed
	l.markProcessed(0, 10)

	// Verify lastProcessed was updated
	assert.Equal(t, uint64(10), l.bt.lastProcessed)
	l.markProcessed(10, 20)
	assert.Equal(t, uint64(20), l.bt.lastProcessed)
	l.markProcessed(21, 21)
	assert.Equal(t, uint64(21), l.bt.lastProcessed)
	slog.Info("markProcessed test passed.")
}

func TestListener_ReadBatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock dependencies
	mockDBHandler := mocks.NewMockDatabaseHandler(ctrl)
	mockABIParser := mocks.NewMockABIParser(ctrl)

	// Setup core
	listener := defaultListener()
	listener.dbHandler = mockDBHandler
	listener.abiParser = mockABIParser

	// Mock Ethereum client
	mockClient := mocks.NewMockEthClient(ctrl)

	// Simulate log fetching
	mockClient.EXPECT().FilterLogs(gomock.Any(), gomock.Any()).Return([]types.Log{}, nil).Times(2)
	mockDBHandler.EXPECT().SaveLastProcessed(gomock.Any()).AnyTimes()

	// Test ReadBatch
	workQueue := make(chan [2]uint64, 1)
	listener.Add(1)
	go listener.ReadEventBatch(context.Background(), mockClient, workQueue)

	workQueue <- [2]uint64{1, 10}
	workQueue <- [2]uint64{10, 20}
	close(workQueue)
	listener.Wait()
	assert.Equal(t, uint64(20), listener.bt.lastProcessed)
}

func TestListener_ReadHistoricalData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock dependencies
	mockDBHandler := mocks.NewMockDatabaseHandler(ctrl)
	mockABIParser := mocks.NewMockABIParser(ctrl)

	// Setup core
	listener := defaultListener()
	listener.dbHandler = mockDBHandler
	listener.abiParser = mockABIParser

	// Mock Ethereum client
	mockClient := mocks.NewMockEthClient(ctrl)

	// Mock DatabaseHandler behavior
	mockDBHandler.EXPECT().LastProcessed().Return(uint64(0)).Times(1)
	mockDBHandler.EXPECT().SaveLastProcessed(gomock.Any()).AnyTimes()

	logs := make([]types.Log, 0)
	logs = append(logs, types.Log{})

	// Mock Ethereum client behavior
	mockClient.EXPECT().BlockNumber(gomock.Any()).Return(uint64(100), nil).Times(1)
	mockClient.EXPECT().FilterLogs(gomock.Any(), gomock.Any()).Return(logs, nil)

	// Test ReadHistoricalData
	ctx, cancel := context.WithCancel(context.Background())

	listener.Add(1)
	go listener.ReadHistoricalData(ctx, mockClient)
	cancel()
	listener.Wait()
}
