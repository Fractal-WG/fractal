package store_test

import (
	"context"
	"testing"
	"time"

	"dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/store"
	"gotest.tools/assert"
)

func TestSaveAndGetInvoices(t *testing.T) {
	tokenisationStore := support.SetupTestDB()
	ctx := context.Background()

	paymentAddress := support.GenerateDogecoinAddress(true)
	offererAddress := support.GenerateDogecoinAddress(true)
	sellOfferAddress := support.GenerateDogecoinAddress(true)

	invoice := store.Invoice{
		Id:             "myId",
		Hash:           "myHash",
		PaymentAddress: paymentAddress,
		BuyerAddress:   offererAddress,
		MintHash:       "myMintHash",
		Quantity:       10,
		Price:          25,
		CreatedAt:      time.Now(),
		PublicKey:      "myPublicKey",
		SellerAddress:  sellOfferAddress,
		Signature:      "mySignature",
	}

	id, err := tokenisationStore.SaveInvoice(ctx, &invoice)
	if err != nil {
		t.Fatalf("Failed to save invoice: %v", err)
	}

	invoices, err := tokenisationStore.GetInvoices(ctx, 0, 10, "myMintHash", offererAddress)
	if err != nil {
		t.Fatalf("Failed to get offer: %v", err)
	}

	assert.Equal(t, len(invoices), 1)
	assert.Equal(t, invoices[0].Id, id, "failed to match invoice id")
	assert.Equal(t, invoices[0].Hash, invoice.Hash, "failed to match invoice hash")
	assert.Equal(t, invoices[0].PaymentAddress, invoice.PaymentAddress, "failed to match invoice payment address")
	assert.Equal(t, invoices[0].BuyerAddress, invoice.BuyerAddress, "failed to match invoice offerer address")
	assert.Equal(t, invoices[0].MintHash, invoice.MintHash, "failed to match invoice buy offer mint hash")
	assert.Equal(t, invoices[0].Quantity, invoice.Quantity, "failed to match invoice buy offer quantity")
	assert.Equal(t, invoices[0].Price, invoice.Price, "failed to match invoice buy offer price")
	assert.Equal(t, invoices[0].PublicKey, invoice.PublicKey, "failed to match invoice public key")
}
