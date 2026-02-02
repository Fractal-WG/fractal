package service_test

import (
	"context"
	"encoding/hex"
	"testing"

	"dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/service"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/proto"
	"gotest.tools/assert"
)

// TestInvoiceProcessorBypassesSignatureRequirement tests if invoices can be confirmed
// without required asset manager signatures
func TestInvoiceProcessorBypassesSignatureRequirement(t *testing.T) {
	tokenStore := support.SetupTestDB(t)
	ctx := context.Background()

	// Setup: Create a mint that requires ALL_SIGNATURES from 2 asset managers
	mintHash := support.GenerateRandomHash()
	sellerAddress := support.GenerateDogecoinAddress(true)
	buyerAddress := support.GenerateDogecoinAddress(true)

	mint := &store.MintWithoutID{
		Title:                    "Signature Required Mint",
		Description:              "Test mint requiring signatures",
		FractionCount:            1000,
		Hash:                     mintHash,
		OwnerAddress:             sellerAddress,
		SignatureRequirementType: store.SignatureRequirementType_ALL_SIGNATURES,
		AssetManagers: store.AssetManagers{
			{Name: "Manager 1", PublicKey: "02b4632d08485ff1df2db55b9dafd23347d1c47a457072a1e87be26896549a8737", URL: "https://example.com"},
			{Name: "Manager 2", PublicKey: "03c2abfa93eae5ec68d3c36e99aa2c35f0a0d3e1c2b6e8e4a8f2e3d1c0b1a2b3c4", URL: "https://example.com"},
		},
	}

	_, err := tokenStore.SaveMint(ctx, mint, sellerAddress)
	assert.NilError(t, err)

	// Give seller 500 tokens
	err = tokenStore.UpsertTokenBalance(ctx, sellerAddress, mintHash, 500)
	assert.NilError(t, err)

	// Create an invoice
	invoiceHash := support.GenerateRandomHash()

	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoice := protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    100,
	}

	invoiceData, err := proto.Marshal(&invoice)
	assert.NilError(t, err)

	// Create onchain transaction for the invoice
	txId, err := tokenStore.SaveOnChainTransaction(ctx, "txhash1", 1000, "blockhash1", 0, protocol.ACTION_INVOICE, 1, invoiceData, sellerAddress, store.StringInterfaceMap{})
	assert.NilError(t, err)

	// Get the transaction we just created
	transactions, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	assert.Assert(t, len(transactions) > 0, "Should have created transaction")

	tx := transactions[0]
	assert.Equal(t, txId, tx.Id)

	// Create unconfirmed invoice (simulating what CreateInvoice RPC does)
	unconfirmedInvoice := &store.UnconfirmedInvoice{
		Hash:          invoiceHash,
		MintHash:      mintHash,
		Quantity:      100,
		Price:         50,
		BuyerAddress:  buyerAddress,
		SellerAddress: sellerAddress,
		Status:        "pending_signatures",
	}
	_, err = tokenStore.SaveUnconfirmedInvoice(ctx, unconfirmedInvoice)
	assert.NilError(t, err)

	// Process the invoice WITHOUT any asset manager signatures
	processor := service.NewInvoiceProcessor(tokenStore)
	err = processor.Process(tx)

	// EXPECTED: Should return error because signatures are missing
	// ACTUAL: Check if it allows the invoice to be confirmed
	if err != nil {
		t.Logf("Good! Invoice processing failed with error: %v", err)
	} else {
		t.Error("CRITICAL SECURITY BUG: Invoice was processed without required signatures!")
	}

	// Verify that the invoice was NOT moved to confirmed state
	confirmedInvoice, err := tokenStore.GetInvoiceByHash(ctx, invoiceHash)
	if err != nil || confirmedInvoice.Id == "" {
		t.Log("Good! Invoice is not in confirmed state")
	} else {
		t.Error("CRITICAL SECURITY BUG: Invoice was confirmed without required signatures!")
		t.Logf("Confirmed invoice ID: %s", confirmedInvoice.Id)
	}

	// Verify pending balance was created (this is okay - it reserves tokens)
	pendingBalance, err := tokenStore.GetPendingTokenBalanceTotalForMintAndOwner(ctx, mintHash, sellerAddress)
	assert.NilError(t, err)

	if pendingBalance == 100 {
		t.Log("Pending balance was created (expected - tokens are reserved)")
	}
}

// TestInvoiceProcessorAllowsWithValidSignatures tests that invoices ARE confirmed
// when all required signatures are present
func TestInvoiceProcessorAllowsWithValidSignatures(t *testing.T) {
	tokenStore := support.SetupTestDB(t)
	ctx := context.Background()

	// Setup: Create a mint that requires ONE_SIGNATURE
	mintHash := support.GenerateRandomHash()
	sellerAddress := support.GenerateDogecoinAddress(true)
	buyerAddress := support.GenerateDogecoinAddress(true)

	mint := &store.MintWithoutID{
		Title:                    "Signature Required Mint",
		Description:              "Test mint requiring one signature",
		FractionCount:            1000,
		Hash:                     mintHash,
		OwnerAddress:             sellerAddress,
		SignatureRequirementType: store.SignatureRequirementType_ONE_SIGNATURE,
		AssetManagers: store.AssetManagers{
			{Name: "Manager 1", PublicKey: "02b4632d08485ff1df2db55b9dafd23347d1c47a457072a1e87be26896549a8737", URL: "https://example.com"},
		},
	}

	_, err := tokenStore.SaveMint(ctx, mint, sellerAddress)
	assert.NilError(t, err)

	// Give seller 500 tokens
	err = tokenStore.UpsertTokenBalance(ctx, sellerAddress, mintHash, 500)
	assert.NilError(t, err)

	// Create an invoice
	invoiceHash := support.GenerateRandomHash()

	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoice := protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    100,
	}

	invoiceData, err := proto.Marshal(&invoice)
	assert.NilError(t, err)

	// Create onchain transaction for the invoice
	txId, err := tokenStore.SaveOnChainTransaction(ctx, "txhash2", 1000, "blockhash2", 0, protocol.ACTION_INVOICE, 1, invoiceData, sellerAddress, store.StringInterfaceMap{})
	assert.NilError(t, err)

	// Get the transaction we just created
	transactions, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	assert.Assert(t, len(transactions) > 0, "Should have created transaction")

	tx := transactions[0]
	assert.Equal(t, txId, tx.Id)

	// Create unconfirmed invoice
	unconfirmedInvoice := &store.UnconfirmedInvoice{
		Hash:          invoiceHash,
		MintHash:      mintHash,
		Quantity:      100,
		Price:         50,
		BuyerAddress:  buyerAddress,
		SellerAddress: sellerAddress,
		Status:        "pending_signatures",
	}
	_, err = tokenStore.SaveUnconfirmedInvoice(ctx, unconfirmedInvoice)
	assert.NilError(t, err)

	// Add ONE valid asset manager signature
	signature := &store.InvoiceSignature{
		InvoiceHash: invoiceHash,
		PublicKey:   "02b4632d08485ff1df2db55b9dafd23347d1c47a457072a1e87be26896549a8737",
		Signature:   "valid_signature_here",
	}
	_, err = tokenStore.SaveApprovedInvoiceSignature(ctx, signature)
	assert.NilError(t, err)

	// Process the invoice WITH the required signature
	processor := service.NewInvoiceProcessor(tokenStore)
	err = processor.Process(tx)
	assert.NilError(t, err, "Invoice processing should succeed with valid signature")

	// Verify that the invoice WAS moved to confirmed state
	confirmedInvoice, err := tokenStore.GetInvoiceByHash(ctx, invoiceHash)
	assert.NilError(t, err)
	assert.Assert(t, confirmedInvoice.Id != "", "Invoice should be confirmed with valid signature")
}
