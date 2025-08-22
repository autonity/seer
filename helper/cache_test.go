package helper

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/autonity/autonity/core/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"seer/mocks"
)

func TestBlockCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEthClient := mocks.NewMockEthClient(ctrl)

	// Define test variables
	blockNum := big.NewInt(0)
	mockBlock := &types.Block{ReceivedAt: time.Now()}

	// Create a new block cache with the mocked client
	bc := NewBlockCache(func() BlockFetcher {
		return mockEthClient
	})

	t.Run("Cache miss and fetch from ethclient", func(t *testing.T) {
		// Simulate a block fetch from ethclient
		mockEthClient.EXPECT().BlockByNumber(gomock.Any(), blockNum).Return(mockBlock, nil)

		// Call Get and ensure it fetches from the client
		block, _ := bc.Get(blockNum)
		assert.NotNil(t, block, "Expected block to be fetched and returned")
		assert.Equal(t, mockBlock, block, "Fetched block should match the mock block")
	})

	t.Run("Cache hit returns block directly", func(t *testing.T) {
		// No expectation for ethclient as the block should be cached
		block, _ := bc.Get(blockNum)
		assert.NotNil(t, block, "Expected block to be fetched from cache")
		assert.Equal(t, mockBlock, block, "Cached block should match the original block")
	})

	t.Run("Cache miss and fetch error from ethclient", func(t *testing.T) {
		errorNum := big.NewInt(1)
		mockEthClient.EXPECT().BlockByNumber(gomock.Any(), errorNum).Return(nil, errors.New("block not found"))

		// Call Get and ensure it handles the error
		block, _ := bc.Get(errorNum)
		assert.Nil(t, block, "Expected nil block on fetch error")
	})
}
