package store_test

import (
	"context"
	"testing"

	"dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/store"
	"gotest.tools/assert"
)

var sellOffersTestCtx = context.Background()

func TestGetSellOffersTotalQuantity(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	offererAddress := support.GenerateDogecoinAddress(true)
	publicKey := "02b4632d08485ff1df2db55b9dafd23347d1c47a457072a1e87be26896549a8737"

	// Test with no existing offers
	total, err := db.GetSellOffersTotalQuantity(sellOffersTestCtx, mintHash, offererAddress)
	assert.NilError(t, err)
	assert.Equal(t, total, 0)

	// Create first sell offer
	offer1 := &store.SellOfferWithoutID{
		OffererAddress: offererAddress,
		MintHash:       mintHash,
		Quantity:       100,
		Price:          50,
		PublicKey:      publicKey,
		Signature:      "signature1",
	}
	var hash string
	hash, err = offer1.GenerateHash()
	assert.NilError(t, err)
	offer1.Hash = hash

	_, err = db.SaveSellOffer(sellOffersTestCtx, offer1)
	assert.NilError(t, err)

	// Verify total quantity
	total, err = db.GetSellOffersTotalQuantity(sellOffersTestCtx, mintHash, offererAddress)
	assert.NilError(t, err)
	assert.Equal(t, total, 100)

	// Create second sell offer
	offer2 := &store.SellOfferWithoutID{
		OffererAddress: offererAddress,
		MintHash:       mintHash,
		Quantity:       75,
		Price:          60,
		PublicKey:      publicKey,
		Signature:      "signature2",
	}
	hash, err = offer2.GenerateHash()
	assert.NilError(t, err)
	offer2.Hash = hash

	_, err = db.SaveSellOffer(sellOffersTestCtx, offer2)
	assert.NilError(t, err)

	// Verify total quantity is sum of both offers
	total, err = db.GetSellOffersTotalQuantity(sellOffersTestCtx, mintHash, offererAddress)
	assert.NilError(t, err)
	assert.Equal(t, total, 175)

	// Create offer for different address - should not affect total
	otherAddress := support.GenerateDogecoinAddress(true)
	otherPublicKey := "03c2abfa93eae5ec68d3c36e99aa2c35f0a0d3e1c2b6e8e4a8f2e3d1c0b1a2b3c4"
	offer3 := &store.SellOfferWithoutID{
		OffererAddress: otherAddress,
		MintHash:       mintHash,
		Quantity:       50,
		Price:          60,
		PublicKey:      otherPublicKey,
		Signature:      "signature3",
	}
	hash, err = offer3.GenerateHash()
	assert.NilError(t, err)
	offer3.Hash = hash

	_, err = db.SaveSellOffer(sellOffersTestCtx, offer3)
	assert.NilError(t, err)

	// Verify original offerer's total is unchanged
	total, err = db.GetSellOffersTotalQuantity(sellOffersTestCtx, mintHash, offererAddress)
	assert.NilError(t, err)
	assert.Equal(t, total, 175)

	// Verify other offerer's total
	total, err = db.GetSellOffersTotalQuantity(sellOffersTestCtx, mintHash, otherAddress)
	assert.NilError(t, err)
	assert.Equal(t, total, 50)
}

func TestGetSellOffersTotalQuantityDifferentMints(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash1 := support.GenerateRandomHash()
	mintHash2 := support.GenerateRandomHash()
	offererAddress := support.GenerateDogecoinAddress(true)
	publicKey := "02b4632d08485ff1df2db55b9dafd23347d1c47a457072a1e87be26896549a8737"

	// Create offer for mint1
	offer1 := &store.SellOfferWithoutID{
		OffererAddress: offererAddress,
		MintHash:       mintHash1,
		Quantity:       100,
		Price:          50,
		PublicKey:      publicKey,
		Signature:      "signature1",
	}
	hash, err := offer1.GenerateHash()
	assert.NilError(t, err)
	offer1.Hash = hash

	_, err = db.SaveSellOffer(sellOffersTestCtx, offer1)
	assert.NilError(t, err)

	// Create offer for mint2
	offer2 := &store.SellOfferWithoutID{
		OffererAddress: offererAddress,
		MintHash:       mintHash2,
		Quantity:       75,
		Price:          60,
		PublicKey:      publicKey,
		Signature:      "signature2",
	}
	hash, err = offer2.GenerateHash()
	assert.NilError(t, err)
	offer2.Hash = hash

	_, err = db.SaveSellOffer(sellOffersTestCtx, offer2)
	assert.NilError(t, err)

	// Verify quantities are separate by mint
	total1, err := db.GetSellOffersTotalQuantity(sellOffersTestCtx, mintHash1, offererAddress)
	assert.NilError(t, err)
	assert.Equal(t, total1, 100)

	total2, err := db.GetSellOffersTotalQuantity(sellOffersTestCtx, mintHash2, offererAddress)
	assert.NilError(t, err)
	assert.Equal(t, total2, 75)
}
