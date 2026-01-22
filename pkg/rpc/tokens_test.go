package rpc_test

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gotest.tools/assert"
)

func TestGetTokenBalance(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)

	err := tokenisationStore.UpsertTokenBalance("address1", "mint1", 10)
	if err != nil {
		t.Fatalf("Failed to upsert token balance: %v", err)
	}

	request := &protocol.GetTokenBalancesRequest{}
	request.SetAddress("address1")
	request.SetMintHash(wrapperspb.String("mint1"))

	response, err := feClient.GetTokenBalances(context.Background(), connect.NewRequest(request))
	if err != nil {
		t.Fatalf("Failed to get token balances: %v", err)
	}

	data := response.Msg.GetData().AsMap()
	balances, ok := data["balances"].([]interface{})
	assert.Assert(t, ok)
	assert.Equal(t, len(balances), 1)

	balance, ok := balances[0].(map[string]interface{})
	assert.Assert(t, ok)
	assert.Equal(t, balance["address"], "address1")
	assert.Equal(t, balance["mint_hash"], "mint1")
	assert.Equal(t, int(balance["quantity"].(float64)), 10)
}

func TestGetTokenBalanceWithMintDetails(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)

	_, err := tokenisationStore.SaveMint(&store.MintWithoutID{
		Title:         "mint1",
		Description:   "description1",
		FractionCount: 10,
		Hash:          "mint1",
	}, "address1")
	if err != nil {
		t.Fatalf("Failed to save mint: %v", err)
	}

	err = tokenisationStore.UpsertTokenBalance("address1", "mint1", 10)
	if err != nil {
		t.Fatalf("Failed to upsert token balance: %v", err)
	}

	request := &protocol.GetTokenBalancesRequest{}
	request.SetAddress("address1")
	request.SetIncludeMintDetails(wrapperspb.Bool(true))

	response, err := feClient.GetTokenBalances(context.Background(), connect.NewRequest(request))
	if err != nil {
		t.Fatalf("Failed to get token balances: %v", err)
	}

	data := response.Msg.GetData().AsMap()
	mints, ok := data["mints"].([]interface{})
	assert.Assert(t, ok)

	assert.Equal(t, len(mints), 1)

	mint, ok := mints[0].(map[string]interface{})
	assert.Assert(t, ok)
	assert.Equal(t, mint["address"], "address1")
	assert.Equal(t, int(mint["quantity"].(float64)), 10)
	assert.Equal(t, mint["hash"], "mint1")
	assert.Equal(t, mint["title"], "mint1")
	assert.Equal(t, mint["description"], "description1")
	assert.Equal(t, int(mint["fraction_count"].(float64)), 10)
}
