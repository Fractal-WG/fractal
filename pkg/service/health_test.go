package service_test

import (
	"context"
	"testing"
	"time"

	"dogecoin.org/fractal-engine/internal/test/support"
	test_support "dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/health"
	"gotest.tools/assert"
)

func TestHealth(t *testing.T) {
	tokenisationStore := test_support.SetupTestDB(t)
	ctx := context.Background()

	tokenisationStore.UpsertChainPosition(ctx, 50, "0000000000000000000000000000000000000000000000000000000000000000", false)

	rpcClient := support.NewTestDogeClient(t)

	healthService := health.NewHealthService(rpcClient, tokenisationStore)
	go healthService.Start()

	time.Sleep(1 * time.Second)

	currentBlockHeight, latestBlockHeight, chain, walletsEnabled, updatedAt, err := tokenisationStore.GetHealth(ctx)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, int64(50), currentBlockHeight)
	assert.Equal(t, int64(100), latestBlockHeight)
	assert.Equal(t, updatedAt.IsZero(), false)
	assert.Equal(t, chain, "test")
	assert.Equal(t, walletsEnabled, true)
}
