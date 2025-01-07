package helper

import (
	"errors"
	"testing"
	"time"

	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"Seer/mocks"
)

func TestBlockCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEthClient := mocks.NewMockEthClient(ctrl)

	// Define test variables
	blockHash := common.HexToHash("0x1234")
	mockBlock := &types.Block{ReceivedAt: time.Now()}

	// Create a new block cache with the mocked client
	bc := NewBlockCache(mockEthClient)

	t.Run("Cache miss and fetch from ethclient", func(t *testing.T) {
		// Simulate a block fetch from ethclient
		mockEthClient.EXPECT().BlockByHash(gomock.Any(), blockHash).Return(mockBlock, nil)

		// Call Get and ensure it fetches from the client
		block := bc.Get(blockHash)
		assert.NotNil(t, block, "Expected block to be fetched and returned")
		assert.Equal(t, mockBlock, block, "Fetched block should match the mock block")
	})

	t.Run("Cache hit returns block directly", func(t *testing.T) {
		// No expectation for ethclient as the block should be cached
		block := bc.Get(blockHash)
		assert.NotNil(t, block, "Expected block to be fetched from cache")
		assert.Equal(t, mockBlock, block, "Cached block should match the original block")
	})

	t.Run("Cache miss and fetch error from ethclient", func(t *testing.T) {
		errorHash := common.HexToHash("0x5678")
		mockEthClient.EXPECT().BlockByHash(gomock.Any(), errorHash).Return(nil, errors.New("block not found"))

		// Call Get and ensure it handles the error
		block := bc.Get(errorHash)
		assert.Nil(t, block, "Expected nil block on fetch error")
	})
}
