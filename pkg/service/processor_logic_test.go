package service_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	"dogecoin.org/fractal-engine/internal/test/support"
	test_support "dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/service"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/proto"
	"gotest.tools/assert"
)

func TestProcessEmptyDatabase(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	rpcClient := support.NewTestDogeClient(t)

	processor := service.NewFractalEngineProcessor(tokenStore, rpcClient)

	// Process with empty database should complete without error
	err := processor.Process()
	if err != nil {
		t.Fatalf("Expected no error processing empty database, got: %v", err)
	}
}

func TestProcessMintTransactionMatched(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	rpcClient := support.NewTestDogeClient(t)

	processor := service.NewFractalEngineProcessor(tokenStore, rpcClient)

	// Create a mint that will be matched
	mintHash := support.GenerateRandomHash()
	mintMsg := &protocol.OnChainMintMessage{
		Hash: mintHash,
	}
	encodedMsg, _ := proto.Marshal(mintMsg)

	// First create an unconfirmed mint
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Test Mint",
		Description:     "Test Description",
		FractionCount:   100,
		BlockHeight:     1,
		TransactionHash: "txHash001",
	})
	if err != nil {
		t.Fatalf("Failed to save unconfirmed mint: %v", err)
	}

	// Create the on-chain transaction
	_, err = tokenStore.SaveOnChainTransaction(ctx, "txHash001", 1, "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMsg, "ownerAddress", map[string]interface{}{
		"ownerAddress": 100,
	})
	if err != nil {
		t.Fatalf("Failed to save on-chain transaction: %v", err)
	}

	// Process should match the mint
	err = processor.Process()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify mint was matched by checking token balances
	balances, err := tokenStore.GetTokenBalances(ctx, "ownerAddress", mintHash)
	if err != nil {
		t.Fatalf("Failed to get token balances: %v", err)
	}

	if len(balances) == 0 {
		t.Error("Expected token balance to be created")
	}

	totalBalance := 0
	for _, balance := range balances {
		totalBalance += balance.Quantity
	}
	if totalBalance != 100 {
		t.Errorf("Expected total balance of 100, got %d", totalBalance)
	}
}

func TestProcessMintTransactionNotMatched(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	rpcClient := support.NewTestDogeClient(t)

	processor := service.NewFractalEngineProcessor(tokenStore, rpcClient)

	// Create an on-chain mint transaction without unconfirmed mint
	mintHash := support.GenerateRandomHash()
	mintMsg := &protocol.OnChainMintMessage{
		Hash: mintHash,
	}
	encodedMsg, _ := proto.Marshal(mintMsg)

	_, err := tokenStore.SaveOnChainTransaction(ctx, "txHash002", 1, "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMsg, "ownerAddress", map[string]interface{}{
		"ownerAddress": 100,
	})
	if err != nil {
		t.Fatalf("Failed to save on-chain transaction: %v", err)
	}

	// Process should not match (no unconfirmed mint exists)
	err = processor.Process()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify no token balance was created
	balances, err := tokenStore.GetTokenBalances(ctx, "ownerAddress", mintHash)
	if err != nil {
		t.Fatalf("Failed to get token balances: %v", err)
	}

	if len(balances) != 0 {
		t.Error("Expected no token balance to be created")
	}
}

func TestProcessPaymentTransaction(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	rpcClient := support.NewTestDogeClient(t)

	processor := service.NewFractalEngineProcessor(tokenStore, rpcClient)
	buyerAddress := support.GenerateDogecoinAddress(true)
	sellerAddress := support.GenerateDogecoinAddress(true)

	// Setup: Create a mint and invoice first
	mintHash := support.GenerateRandomHash()
	invoiceHash := support.GenerateRandomHash()

	// Create unconfirmed mint
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Test Mint",
		Description:     "Test Description",
		FractionCount:   100,
		BlockHeight:     1,
		TransactionHash: "txMint",
	})
	if err != nil {
		t.Fatalf("Failed to save unconfirmed mint: %v", err)
	}

	// Create and process mint transaction
	mintMsg := &protocol.OnChainMintMessage{Hash: mintHash}
	encodedMintMsg, _ := proto.Marshal(mintMsg)
	_, err = tokenStore.SaveOnChainTransaction(ctx, "txMint", 1, "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMintMsg, sellerAddress, map[string]interface{}{
		sellerAddress: 100,
	})
	if err != nil {
		t.Fatalf("Failed to save mint transaction: %v", err)
	}
	processor.Process()

	// Create unconfirmed invoice
	_, err = tokenStore.SaveUnconfirmedInvoice(ctx, &store.UnconfirmedInvoice{
		Hash:           invoiceHash,
		PaymentAddress: sellerAddress,
		BuyerAddress:   buyerAddress,
		MintHash:       mintHash,
		Quantity:       50,
		Price:          100,
		SellerAddress:  sellerAddress,
	})
	if err != nil {
		t.Fatalf("Failed to save unconfirmed invoice: %v", err)
	}

	// Create and process invoice transaction
	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoiceMsg := &protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    50,
	}

	encodedInvoiceMsg, _ := proto.Marshal(invoiceMsg)
	_, err = tokenStore.SaveOnChainTransaction(ctx, "txInvoice", 2, "blockHash", 1, protocol.ACTION_INVOICE, protocol.DEFAULT_VERSION, encodedInvoiceMsg, sellerAddress, map[string]interface{}{
		sellerAddress: 50,
	})
	if err != nil {
		t.Fatalf("Failed to save invoice transaction: %v", err)
	}
	processor.Process()

	// Create payment transaction
	paymentMsg := &protocol.OnChainPaymentMessage{
		Hash: invoiceHash,
	}
	encodedPaymentMsg, _ := proto.Marshal(paymentMsg)
	_, err = tokenStore.SaveOnChainTransaction(ctx, "txPayment", 3, "blockHash", 1, protocol.ACTION_PAYMENT, protocol.DEFAULT_VERSION, encodedPaymentMsg, buyerAddress, map[string]interface{}{
		sellerAddress: 5000,
	})
	if err != nil {
		t.Fatalf("Failed to save payment transaction: %v", err)
	}

	// Process payment
	err = processor.Process()
	if err != nil {
		t.Fatalf("Expected no error processing payment, got: %v", err)
	}

	// Verify buyer received tokens
	buyerBalances, err := tokenStore.GetTokenBalances(ctx, buyerAddress, mintHash)
	if err != nil {
		t.Fatalf("Failed to get buyer token balances: %v", err)
	}

	buyerTotal := 0
	for _, balance := range buyerBalances {
		buyerTotal += balance.Quantity
	}
	if buyerTotal != 50 {
		t.Errorf("Expected buyer to have 50 tokens, got %d", buyerTotal)
	}
}

func TestProcessInvoiceTransaction(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	rpcClient := support.NewTestDogeClient(t)

	processor := service.NewFractalEngineProcessor(tokenStore, rpcClient)

	// Setup: Create a mint first
	mintHash := support.GenerateRandomHash()
	ownerAddress := support.GenerateDogecoinAddress(true)

	// Create unconfirmed mint
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Test Mint",
		Description:     "Test Description",
		FractionCount:   100,
		BlockHeight:     1,
		TransactionHash: "txMint",
	})
	if err != nil {
		t.Fatalf("Failed to save unconfirmed mint: %v", err)
	}

	// Create and process mint transaction
	mintMsg := &protocol.OnChainMintMessage{Hash: mintHash}
	encodedMintMsg, _ := proto.Marshal(mintMsg)
	_, err = tokenStore.SaveOnChainTransaction(ctx, "txMint", 1, "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMintMsg, ownerAddress, map[string]interface{}{
		ownerAddress: 100,
	})
	if err != nil {
		t.Fatalf("Failed to save mint transaction: %v", err)
	}
	processor.Process()

	// Create invoice transaction
	invoiceHash := support.GenerateRandomHash()
	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoiceMsg := &protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    30,
	}
	encodedInvoiceMsg, _ := proto.Marshal(invoiceMsg)
	_, err = tokenStore.SaveOnChainTransaction(ctx, "txInvoice", 2, "blockHash", 1, protocol.ACTION_INVOICE, protocol.DEFAULT_VERSION, encodedInvoiceMsg, ownerAddress, map[string]interface{}{
		ownerAddress: 30,
	})
	if err != nil {
		t.Fatalf("Failed to save invoice transaction: %v", err)
	}

	// Process invoice
	err = processor.Process()
	if err != nil {
		t.Fatalf("Expected no error processing invoice, got: %v", err)
	}

	// Verify pending token balance was created
	tx, _ := tokenStore.DB.Begin()
	defer tx.Rollback()

	pendingBalance, err := tokenStore.GetPendingTokenBalance(ctx, invoiceHash, mintHash, tx)
	if err != nil {
		t.Fatalf("Failed to get pending token balance: %v", err)
	}

	if pendingBalance.Quantity != 30 {
		t.Errorf("Expected pending balance of 30, got %d", pendingBalance.Quantity)
	}
}

func TestProcessPagination(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	rpcClient := support.NewTestDogeClient(t)

	processor := service.NewFractalEngineProcessor(tokenStore, rpcClient)

	// Create 150 transactions to test pagination (limit is 100)
	for i := 0; i < 150; i++ {
		mintHash := fmt.Sprintf("mint%d", i)
		mintMsg := &protocol.OnChainMintMessage{Hash: mintHash}
		encodedMsg, _ := proto.Marshal(mintMsg)

		txHash := fmt.Sprintf("tx%d", i)
		_, err := tokenStore.SaveOnChainTransaction(ctx, txHash, int64(i+1), "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMsg, "ownerAddress", map[string]interface{}{
			"ownerAddress": 1,
		})
		if err != nil {
			t.Fatalf("Failed to save transaction %d: %v", i, err)
		}
	}

	// Process should handle pagination correctly
	err := processor.Process()
	if err != nil {
		t.Fatalf("Expected no error with pagination, got: %v", err)
	}

	// Verify all transactions were processed
	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 200)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}

	if len(txs) != 150 {
		t.Errorf("Expected 150 transactions, got %d", len(txs))
	}
}

func TestProcessUnknownActionType(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	rpcClient := support.NewTestDogeClient(t)

	processor := service.NewFractalEngineProcessor(tokenStore, rpcClient)

	// Create transaction with unknown action type (use a valid uint8 value)
	_, err := tokenStore.SaveOnChainTransaction(ctx, "txUnknown", 1, "blockHash", 1, 99, protocol.DEFAULT_VERSION, []byte{}, "ownerAddress", map[string]interface{}{
		"ownerAddress": 0,
	})
	if err != nil {
		t.Fatalf("Failed to save transaction: %v", err)
	}

	// Process should skip unknown action types without error
	err = processor.Process()
	if err != nil {
		t.Fatalf("Expected no error with unknown action type, got: %v", err)
	}
}
