package rpc_test

import (
	"context"
	"fmt"
	"testing"

	connect "connectrpc.com/connect"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"gotest.tools/assert"
)

func TestGetTokenBalance(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	err := tokenisationStore.UpsertTokenBalance(ctx, "address1", "mint1", 10)
	if err != nil {
		t.Fatalf("Failed to upsert token balance: %v", err)
	}

	request := &protocol.GetTokenBalancesRequest{}
	addressProto := &protocol.Address{}
	addressProto.SetValue("address1")
	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue("mint1")
	request.SetAddress(addressProto)
	request.SetMintHash(mintHashProto)

	response, err := feClient.GetTokenBalances(ctx, connect.NewRequest(request))
	if err != nil {
		t.Fatalf("Failed to get token balances: %v", err)
	}

	balances := response.Msg.GetBalances()
	assert.Equal(t, len(balances), 1)
	assert.Equal(t, balances[0].GetAddress().GetValue(), "address1")
	assert.Equal(t, balances[0].GetMintHash().GetValue(), "mint1")
	assert.Equal(t, int(balances[0].GetQuantity()), 10)
}

func TestGetTokenBalanceWithMintDetails(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	_, err := tokenisationStore.SaveMint(ctx, &store.MintWithoutID{
		Title:         "mint1",
		Description:   "description1",
		FractionCount: 10,
		Hash:          "mint1",
	}, "address1")
	if err != nil {
		t.Fatalf("Failed to save mint: %v", err)
	}

	err = tokenisationStore.UpsertTokenBalance(ctx, "address1", "mint1", 10)
	if err != nil {
		t.Fatalf("Failed to upsert token balance: %v", err)
	}

	request := &protocol.GetTokenBalancesRequest{}
	addressProto := &protocol.Address{}
	addressProto.SetValue("address1")
	request.SetAddress(addressProto)
	request.SetIncludeMintDetails(wrapperspb.Bool(true))

	response, err := feClient.GetTokenBalances(ctx, connect.NewRequest(request))
	if err != nil {
		t.Fatalf("Failed to get token balances: %v", err)
	}

	mints := response.Msg.GetMints()
	assert.Equal(t, len(mints), 1)
	assert.Equal(t, mints[0].GetAddress().GetValue(), "address1")
	assert.Equal(t, int(mints[0].GetQuantity()), 10)
	assert.Equal(t, mints[0].GetMint().GetHash().GetValue(), "mint1")
	assert.Equal(t, mints[0].GetMint().GetTitle(), "mint1")
	assert.Equal(t, mints[0].GetMint().GetDescription(), "description1")
	assert.Equal(t, int(mints[0].GetMint().GetFractionCount()), 10)
}

func TestGetTokenBalancesMintDetailsPagination(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	// Seed 5 mints with balances for address1
	for i := 1; i <= 5; i++ {
		mintHash := fmt.Sprintf("mint%d", i)
		_, err := tokenisationStore.SaveMint(ctx, &store.MintWithoutID{
			Title:         mintHash,
			Description:   fmt.Sprintf("description%d", i),
			FractionCount: 100,
			Hash:          mintHash,
		}, "address1")
		if err != nil {
			t.Fatalf("Failed to save mint: %v", err)
		}
		err = tokenisationStore.UpsertTokenBalance(ctx, "address1", mintHash, i*10)
		if err != nil {
			t.Fatalf("Failed to upsert token balance: %v", err)
		}
	}

	addressProto := &protocol.Address{}
	addressProto.SetValue("address1")

	t.Run("first page returns limit records", func(t *testing.T) {
		request := &protocol.GetTokenBalancesRequest{}
		request.SetAddress(addressProto)
		request.SetIncludeMintDetails(wrapperspb.Bool(true))
		request.SetLimit(wrapperspb.Int32(2))

		response, err := feClient.GetTokenBalances(ctx, connect.NewRequest(request))
		if err != nil {
			t.Fatalf("Failed to get token balances: %v", err)
		}

		mints := response.Msg.GetMints()
		assert.Equal(t, len(mints), 2)
		assert.Equal(t, int(response.Msg.GetPage()), 0)
		assert.Equal(t, int(response.Msg.GetLimit()), 2)
	})

	t.Run("second page returns next records", func(t *testing.T) {
		request := &protocol.GetTokenBalancesRequest{}
		request.SetAddress(addressProto)
		request.SetIncludeMintDetails(wrapperspb.Bool(true))
		request.SetLimit(wrapperspb.Int32(2))
		request.SetPage(wrapperspb.Int32(1))

		response, err := feClient.GetTokenBalances(ctx, connect.NewRequest(request))
		if err != nil {
			t.Fatalf("Failed to get token balances: %v", err)
		}

		mints := response.Msg.GetMints()
		assert.Equal(t, len(mints), 2)
		assert.Equal(t, int(response.Msg.GetPage()), 1)
	})

	t.Run("last partial page returns remaining records", func(t *testing.T) {
		request := &protocol.GetTokenBalancesRequest{}
		request.SetAddress(addressProto)
		request.SetIncludeMintDetails(wrapperspb.Bool(true))
		request.SetLimit(wrapperspb.Int32(2))
		request.SetPage(wrapperspb.Int32(2))

		response, err := feClient.GetTokenBalances(ctx, connect.NewRequest(request))
		if err != nil {
			t.Fatalf("Failed to get token balances: %v", err)
		}

		mints := response.Msg.GetMints()
		assert.Equal(t, len(mints), 1)
		assert.Equal(t, int(response.Msg.GetPage()), 2)
	})

	t.Run("page beyond results returns empty", func(t *testing.T) {
		request := &protocol.GetTokenBalancesRequest{}
		request.SetAddress(addressProto)
		request.SetIncludeMintDetails(wrapperspb.Bool(true))
		request.SetLimit(wrapperspb.Int32(2))
		request.SetPage(wrapperspb.Int32(10))

		response, err := feClient.GetTokenBalances(ctx, connect.NewRequest(request))
		if err != nil {
			t.Fatalf("Failed to get token balances: %v", err)
		}

		assert.Equal(t, len(response.Msg.GetMints()), 0)
	})
}
