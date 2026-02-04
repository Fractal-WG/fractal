package rpc_test

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"gotest.tools/assert"
)

func TestCreateSellOfferWithInsufficientBalance(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	// Setup: Create a mint and seller with only 100 tokens
	sellerAddress := support.GenerateDogecoinAddress(true)
	mintHash := support.GenerateRandomHash()

	_, err := tokenisationStore.SaveMint(ctx, &store.MintWithoutID{
		Title:         "Test Mint",
		Description:   "Test Description",
		FractionCount: 1000,
		Hash:          mintHash,
	}, "owner")
	assert.NilError(t, err)

	// Give seller 100 tokens
	err = tokenisationStore.UpsertTokenBalance(ctx, sellerAddress, mintHash, 100)
	assert.NilError(t, err)

	// Generate keypair for seller
	privHex, pubHex, _, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	// Attempt to create sell offer for 150 tokens (more than available)
	sellOfferPayload := rpc.CreateSellOfferRequestPayload{
		OffererAddress: sellerAddress,
		MintHash:       mintHash,
		Quantity:       150,
		Price:          50,
	}

	signature, err := doge.SignPayload(sellOfferPayload, privHex, pubHex)
	assert.NilError(t, err)

	// Build protobuf request
	offererAddressProto := &protocol.Address{}
	offererAddressProto.SetValue(sellerAddress)
	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	protoPayload := &protocol.CreateSellOfferRequestPayload{}
	protoPayload.SetOffererAddress(offererAddressProto)
	protoPayload.SetMintHash(mintHashProto)
	protoPayload.SetQuantity(150)
	protoPayload.SetPrice(50)

	sellOfferRequest := &protocol.CreateSellOfferRequest{}
	sellOfferRequest.SetPayload(protoPayload)
	sellOfferRequest.SetPublicKey(pubHex)
	sellOfferRequest.SetSignature(signature)

	// Should fail due to insufficient balance
	_, err = feClient.CreateSellOffer(ctx, connect.NewRequest(sellOfferRequest))
	assert.Assert(t, err != nil, "Should fail with insufficient balance")
	assert.ErrorContains(t, err, "insufficient token balance")
}

func TestCreateSellOfferWithSufficientBalance(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	// Setup: Create a mint and seller with 200 tokens
	sellerAddress := support.GenerateDogecoinAddress(true)
	mintHash := support.GenerateRandomHash()

	_, err := tokenisationStore.SaveMint(ctx, &store.MintWithoutID{
		Title:         "Test Mint",
		Description:   "Test Description",
		FractionCount: 1000,
		Hash:          mintHash,
	}, "owner")
	assert.NilError(t, err)

	// Give seller 200 tokens
	err = tokenisationStore.UpsertTokenBalance(ctx, sellerAddress, mintHash, 200)
	assert.NilError(t, err)

	// Generate keypair for seller
	privHex, pubHex, _, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	// Create sell offer for 100 tokens (less than available)
	sellOfferPayload := rpc.CreateSellOfferRequestPayload{
		OffererAddress: sellerAddress,
		MintHash:       mintHash,
		Quantity:       100,
		Price:          50,
	}

	signature, err := doge.SignPayload(sellOfferPayload, privHex, pubHex)
	assert.NilError(t, err)

	// Build protobuf request
	offererAddressProto := &protocol.Address{}
	offererAddressProto.SetValue(sellerAddress)
	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	protoPayload := &protocol.CreateSellOfferRequestPayload{}
	protoPayload.SetOffererAddress(offererAddressProto)
	protoPayload.SetMintHash(mintHashProto)
	protoPayload.SetQuantity(100)
	protoPayload.SetPrice(50)

	sellOfferRequest := &protocol.CreateSellOfferRequest{}
	sellOfferRequest.SetPayload(protoPayload)
	sellOfferRequest.SetPublicKey(pubHex)
	sellOfferRequest.SetSignature(signature)

	// Should succeed
	response, err := feClient.CreateSellOffer(ctx, connect.NewRequest(sellOfferRequest))
	assert.NilError(t, err)
	assert.Assert(t, response.Msg.GetId() != "", "Should return offer ID")

	// Verify offer was created
	offers, err := tokenisationStore.GetSellOffers(ctx, 0, 10, mintHash, sellerAddress)
	assert.NilError(t, err)
	assert.Equal(t, len(offers), 1)
	assert.Equal(t, offers[0].Quantity, 100)
}

func TestCreateSellOfferAccountsForPendingBalances(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	// Setup: Create mint and seller with 200 tokens
	sellerAddress := support.GenerateDogecoinAddress(true)
	mintHash := support.GenerateRandomHash()

	_, err := tokenisationStore.SaveMint(ctx, &store.MintWithoutID{
		Title:         "Test Mint",
		Description:   "Test Description",
		FractionCount: 1000,
		Hash:          mintHash,
	}, "owner")
	assert.NilError(t, err)

	// Give seller 200 tokens
	err = tokenisationStore.UpsertTokenBalance(ctx, sellerAddress, mintHash, 200)
	assert.NilError(t, err)

	// Create a pending balance (simulating an invoice that reserved 80 tokens)
	invoiceHash := support.GenerateRandomHash()
	err = tokenisationStore.UpsertPendingTokenBalance(ctx, invoiceHash, mintHash, 80, "txId", sellerAddress)
	assert.NilError(t, err)

	// Generate keypair for seller
	privHex, pubHex, _, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	// Try to create sell offer for 150 tokens
	// Available = 200 (total) - 80 (pending) = 120
	// So 150 should fail
	sellOfferPayload := rpc.CreateSellOfferRequestPayload{
		OffererAddress: sellerAddress,
		MintHash:       mintHash,
		Quantity:       150,
		Price:          50,
	}

	signature, err := doge.SignPayload(sellOfferPayload, privHex, pubHex)
	assert.NilError(t, err)

	offererAddressProto := &protocol.Address{}
	offererAddressProto.SetValue(sellerAddress)
	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	protoPayload := &protocol.CreateSellOfferRequestPayload{}
	protoPayload.SetOffererAddress(offererAddressProto)
	protoPayload.SetMintHash(mintHashProto)
	protoPayload.SetQuantity(150)
	protoPayload.SetPrice(50)

	sellOfferRequest := &protocol.CreateSellOfferRequest{}
	sellOfferRequest.SetPayload(protoPayload)
	sellOfferRequest.SetPublicKey(pubHex)
	sellOfferRequest.SetSignature(signature)

	// Should fail - would exceed available balance
	_, err = feClient.CreateSellOffer(ctx, connect.NewRequest(sellOfferRequest))
	assert.Assert(t, err != nil, "Should fail when pending balance reduces available tokens")
	assert.ErrorContains(t, err, "insufficient token balance")

	// Now try with 100 tokens (within available = 120)
	sellOfferPayload.Quantity = 100
	signature, err = doge.SignPayload(sellOfferPayload, privHex, pubHex)
	assert.NilError(t, err)

	protoPayload.SetQuantity(100)
	sellOfferRequest.SetSignature(signature)

	// Should succeed
	response, err := feClient.CreateSellOffer(ctx, connect.NewRequest(sellOfferRequest))
	assert.NilError(t, err)
	assert.Assert(t, response.Msg.GetId() != "")
}

func TestCreateSellOfferAccountsForExistingOffers(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	// Setup: Create mint and seller with 200 tokens
	sellerAddress := support.GenerateDogecoinAddress(true)
	mintHash := support.GenerateRandomHash()

	_, err := tokenisationStore.SaveMint(ctx, &store.MintWithoutID{
		Title:         "Test Mint",
		Description:   "Test Description",
		FractionCount: 1000,
		Hash:          mintHash,
	}, "owner")
	assert.NilError(t, err)

	// Give seller 200 tokens
	err = tokenisationStore.UpsertTokenBalance(ctx, sellerAddress, mintHash, 200)
	assert.NilError(t, err)

	// Generate keypair for seller
	privHex, pubHex, _, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	// Create first sell offer for 120 tokens
	sellOfferPayload := rpc.CreateSellOfferRequestPayload{
		OffererAddress: sellerAddress,
		MintHash:       mintHash,
		Quantity:       120,
		Price:          50,
	}

	signature, err := doge.SignPayload(sellOfferPayload, privHex, pubHex)
	assert.NilError(t, err)

	offererAddressProto := &protocol.Address{}
	offererAddressProto.SetValue(sellerAddress)
	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	protoPayload := &protocol.CreateSellOfferRequestPayload{}
	protoPayload.SetOffererAddress(offererAddressProto)
	protoPayload.SetMintHash(mintHashProto)
	protoPayload.SetQuantity(120)
	protoPayload.SetPrice(50)

	sellOfferRequest := &protocol.CreateSellOfferRequest{}
	sellOfferRequest.SetPayload(protoPayload)
	sellOfferRequest.SetPublicKey(pubHex)
	sellOfferRequest.SetSignature(signature)

	// First offer should succeed
	_, err = feClient.CreateSellOffer(ctx, connect.NewRequest(sellOfferRequest))
	assert.NilError(t, err)

	// Try to create second offer for 100 tokens
	// Available = 200 (total) - 120 (existing offer) = 80
	// So 100 should fail
	sellOfferPayload.Quantity = 100
	signature, err = doge.SignPayload(sellOfferPayload, privHex, pubHex)
	assert.NilError(t, err)

	protoPayload.SetQuantity(100)
	sellOfferRequest.SetSignature(signature)

	_, err = feClient.CreateSellOffer(ctx, connect.NewRequest(sellOfferRequest))
	assert.Assert(t, err != nil, "Second offer should fail - exceeds available balance")
	assert.ErrorContains(t, err, "insufficient token balance")

	// Try with 70 tokens (within available = 80)
	sellOfferPayload.Quantity = 70
	signature, err = doge.SignPayload(sellOfferPayload, privHex, pubHex)
	assert.NilError(t, err)

	protoPayload.SetQuantity(70)
	sellOfferRequest.SetSignature(signature)

	// Should succeed
	response, err := feClient.CreateSellOffer(ctx, connect.NewRequest(sellOfferRequest))
	assert.NilError(t, err)
	assert.Assert(t, response.Msg.GetId() != "")

	// Verify both offers exist
	offers, err := tokenisationStore.GetSellOffers(ctx, 0, 10, mintHash, sellerAddress)
	assert.NilError(t, err)
	assert.Equal(t, len(offers), 2)
}
