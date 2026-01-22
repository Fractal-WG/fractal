package rpc_test

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"gotest.tools/assert"
)

func TestGetHealth(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)

	_, err := feClient.GetHealth(context.Background(), connect.NewRequest(&protocol.GetHealthRequest{}))
	assert.Equal(t, connect.CodeOf(err), connect.CodeNotFound)

	tokenisationStore.UpsertHealth(100, 200, "test", true)

	healthResponse, err := feClient.GetHealth(context.Background(), connect.NewRequest(&protocol.GetHealthRequest{}))
	assert.NilError(t, err)
	assert.Equal(t, healthResponse.Msg.GetCurrentBlockHeight(), int32(100))
	assert.Equal(t, healthResponse.Msg.GetLatestBlockHeight(), int32(200))
	assert.Equal(t, healthResponse.Msg.GetUpdatedAt() != "", true)
	assert.Equal(t, healthResponse.Msg.GetChain(), "test")
	assert.Equal(t, healthResponse.Msg.GetWalletsEnabled(), true)
}
