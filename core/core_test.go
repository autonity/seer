package core

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/autonity/autonity/core/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"seer/config"
	"seer/db"
	"seer/helper"
	"seer/mocks"
	"seer/model"
)

// A helper function to create a default listener with mocked dependencies
func setupTestCore(t *testing.T) (*core, *mocks.MockDatabaseHandler, *mocks.MockConnectionProvider, *mocks.MockEthClient, *mocks.MockRPCClient, *mocks.MockABIParser) {
	ctrl := gomock.NewController(t)

	mockDB := mocks.NewMockDatabaseHandler(ctrl)
	mockCP := mocks.NewMockConnectionProvider(ctrl)
	mockEthClient := mocks.NewMockEthClient(ctrl)
	mockRPCClient := mocks.NewMockRPCClient(ctrl)

	mockCP.EXPECT().GetWebSocketConnection().Return(mockEthClient).AnyTimes()
	mockCP.EXPECT().GetRPCConnection().Return(mockRPCClient).AnyTimes()

	mockEthClient.EXPECT().NetworkID(gomock.Any()).Return(big.NewInt(1), nil).AnyTimes()
	mockABIParser := mocks.NewMockABIParser(ctrl)

	cfg := config.NodeConfig{}
	c := &core{
		nodeConfig: cfg,
		newBlocks:  make(chan *SeerBlock, 10),
		newEvents:  make(chan types.Log, 100),
		dbHandler:  mockDB,
		cp:         mockCP,
		blockCache: helper.NewBlockCache(func() helper.BlockFetcher {
			return mockEthClient
		}),
		abiParser: mockABIParser,
		blockTracker: &syncTracker{
			processed:     make(map[uint64]bool),
			lastProcessed: 0,
		},
	}

	return c, mockDB, mockCP, mockEthClient, mockRPCClient, mockABIParser
}

func TestCore_markProcessed(t *testing.T) {
	c, mockDB, _, _, _, _ := setupTestCore(t)

	// Expect SaveLastProcessed to be called when the contiguous block number increases
	mockDB.EXPECT().SaveLastProcessed(db.BlockNumKey, uint64(10)).Times(1)
	c.markProcessedBlock(0, 10)
	assert.Equal(t, uint64(10), c.blockTracker.lastProcessed)

	mockDB.EXPECT().SaveLastProcessed(db.BlockNumKey, uint64(20)).Times(1)
	c.markProcessedBlock(10, 20)
	assert.Equal(t, uint64(20), c.blockTracker.lastProcessed)

	mockDB.EXPECT().SaveLastProcessed(db.BlockNumKey, uint64(21)).Times(1)
	c.markProcessedBlock(21, 21)
	assert.Equal(t, uint64(21), c.blockTracker.lastProcessed)
}

func TestCore_ReadEventHistory(t *testing.T) {
	c, _, _, mockEthClient, _, _ := setupTestCore(t)

	// Mock the behavior of FilterLogs for two separate batches
	mockEthClient.EXPECT().FilterLogs(gomock.Any(), gomock.Any()).Return([]types.Log{}, nil).Times(2)

	workQueue := make(chan [2]uint64, 2)
	c.Add(1)
	go func() {
		defer c.Done()
		c.ReadEventHistory(context.Background(), workQueue)
	}()

	workQueue <- [2]uint64{1, 10}
	workQueue <- [2]uint64{10, 20}
	workQueue <- [2]uint64{math.MaxUint64, math.MaxUint64} // End marker

	c.Wait() // Wait for the goroutine to finish
}

func TestCore_ReadHistoricalData(t *testing.T) {
	// The function signature now includes the mockABIParser
	c, mockDB, _, mockEthClient, _, mockABIParser := setupTestCore(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Mock the initial state and dependencies
	mockDB.EXPECT().LastProcessed(db.BlockNumKey).Return(uint64(0)).Times(1)
	mockEthClient.EXPECT().BlockNumber(gomock.Any()).Return(uint64(100), nil).Times(1)

	// Mock the log filtering to return one sample log
	logs := []types.Log{{BlockNumber: 50}}
	mockEthClient.EXPECT().FilterLogs(gomock.Any(), gomock.Any()).Return(logs, nil).MinTimes(1)

	decodedEvent := model.EventSchema{Measurement: "TestEvent", Fields: make(map[string]interface{})}
	mockABIParser.EXPECT().Decode(gomock.Any()).Return(decodedEvent, nil).AnyTimes()

	// Mock the database write call that will be triggered by the event processor
	mockDB.EXPECT().WriteEvent(gomock.Any(), gomock.Any()).AnyTimes()

	// Mock the block cache to return a valid block for the event
	header := &types.Header{Number: big.NewInt(50), Time: uint64(time.Now().Unix())}
	block := types.NewBlockWithHeader(header)
	c.blockCache.Add(block) // Pre-populate the cache

	go func() {
		start, end := c.getHistoricalRange(ctx, db.BlockNumKey)
		c.handleEventHistory(ctx, start, end)
	}()

	// Allow the test to run for a moment and then stop it
	time.Sleep(100 * time.Millisecond)
	c.Wait()
}

func TestCore_ReadBlockHistory_BatchCallError(t *testing.T) {
	c, mockDB, _, _, mockRPCClient, _ := setupTestCore(t)

	// Simulate an error from BatchCallContext
	mockRPCClient.EXPECT().BatchCallContext(gomock.Any(), gomock.Any()).Return(errors.New("RPC error")).Times(1)

	// Expect that we still mark the blocks as processed to avoid getting stuck
	mockDB.EXPECT().SaveLastProcessed(db.BlockNumKey, gomock.Any()).AnyTimes()

	workQueue := make(chan [2]uint64, 1)
	c.Add(1)
	go func() {
		defer c.Done()
		c.ReadBlockHistory(context.Background(), workQueue)
	}()

	workQueue <- [2]uint64{1, 10}
	workQueue <- [2]uint64{math.MaxUint64, math.MaxUint64}

	c.Wait()
}
