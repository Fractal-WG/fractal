package service_test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"dogecoin.org/fractal-engine/internal/test/support"
	test_support "dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/service"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/proto"
	"gotest.tools/assert"
)

// Helper function to find a transaction by ID
func findInvoiceTransactionById(txs []store.OnChainTransaction, id string) *store.OnChainTransaction {
	for i := range txs {
		if txs[i].Id == id {
			return &txs[i]
		}
	}
	return nil
}

func TestNewInvoiceProcessor(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	processor := service.NewInvoiceProcessor(tokenStore)

	assert.Assert(t, processor != nil, "Processor should be created")
}

func TestInvoiceProcessorProcessSuccess(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()
	processor := service.NewInvoiceProcessor(tokenStore)

	// Setup: Create mint and token balance for seller
	mintHash := support.GenerateRandomHash()
	sellerAddress := support.GenerateDogecoinAddress(true)
	invoiceHash := support.GenerateRandomHash()
	quantity := int32(50)

	// Create and match mint to establish token balance
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Test Mint",
		Description:     "Test Description",
		FractionCount:   100,
		BlockHeight:     1,
		TransactionHash: "mintTx",
	})
	assert.NilError(t, err)

	mintMsg := &protocol.OnChainMintMessage{Hash: mintHash}
	encodedMintMsg, _ := proto.Marshal(mintMsg)
	mintTxId, err := tokenStore.SaveOnChainTransaction(ctx, "mintTx", 1, "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMintMsg, sellerAddress, map[string]interface{}{
		sellerAddress: 100,
	})
	assert.NilError(t, err)

	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	mintTx := findInvoiceTransactionById(txs, mintTxId)
	assert.Assert(t, mintTx != nil)

	err = tokenStore.MatchUnconfirmedMint(ctx, *mintTx)
	assert.NilError(t, err)

	// Create unconfirmed invoice
	_, err = tokenStore.SaveUnconfirmedInvoice(ctx, &store.UnconfirmedInvoice{
		Hash:           invoiceHash,
		PaymentAddress: sellerAddress,
		BuyerAddress:   support.GenerateDogecoinAddress(true),
		MintHash:       mintHash,
		Quantity:       int(quantity),
		Price:          100,
		CreatedAt:      time.Now(),
		SellerAddress:  sellerAddress,
	})
	assert.NilError(t, err)

	// Create invoice transaction
	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoiceMsg := &protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    quantity,
	}
	encodedInvoiceMsg, _ := proto.Marshal(invoiceMsg)
	invoiceTxId, err := tokenStore.SaveOnChainTransaction(ctx, "invoiceTx", 2, "blockHash", 1, protocol.ACTION_INVOICE, protocol.DEFAULT_VERSION, encodedInvoiceMsg, sellerAddress, map[string]interface{}{
		sellerAddress: quantity,
	})
	assert.NilError(t, err)

	txs, err = tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	invoiceTx := findInvoiceTransactionById(txs, invoiceTxId)
	assert.Assert(t, invoiceTx != nil)

	// Test Process
	err = processor.Process(*invoiceTx)
	assert.NilError(t, err)

	// Verify pending token balance was created
	tx, _ := tokenStore.DB.Begin()
	defer tx.Rollback()

	pendingBalance, err := tokenStore.GetPendingTokenBalance(ctx, invoiceHash, mintHash, tx)
	assert.NilError(t, err)
	assert.Equal(t, int(quantity), pendingBalance.Quantity)
	assert.Equal(t, invoiceHash, pendingBalance.InvoiceHash)
	assert.Equal(t, mintHash, pendingBalance.MintHash)
}

func TestInvoiceProcessorProcessInvalidProtobuf(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	processor := service.NewInvoiceProcessor(tokenStore)

	// Create transaction with invalid protobuf data
	invalidTx := store.OnChainTransaction{
		Id:         "invalidTx",
		TxHash:     "invalidTxHash",
		ActionType: protocol.ACTION_INVOICE,
		ActionData: []byte("invalid protobuf data"),
		Address:    support.GenerateDogecoinAddress(true),
		Values: map[string]interface{}{
			"address": 50,
		},
	}

	// Process should handle invalid protobuf gracefully
	// The address check will fail since we can't unmarshal the data
	err := processor.Process(invalidTx)
	assert.Assert(t, err != nil, "Should fail with invalid protobuf")
}

func TestInvoiceProcessorProcessInsufficientTokenBalance(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()
	processor := service.NewInvoiceProcessor(tokenStore)

	// Setup: Create mint with small token balance
	mintHash := support.GenerateRandomHash()
	sellerAddress := support.GenerateDogecoinAddress(true)
	invoiceHash := support.GenerateRandomHash()
	quantity := int32(150) // More than available balance

	// Create and match mint to establish small token balance (100 tokens)
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Test Mint",
		Description:     "Test Description",
		FractionCount:   100,
		BlockHeight:     1,
		TransactionHash: "mintTx",
	})
	assert.NilError(t, err)

	mintMsg := &protocol.OnChainMintMessage{Hash: mintHash}
	encodedMintMsg, _ := proto.Marshal(mintMsg)
	mintTxId, err := tokenStore.SaveOnChainTransaction(ctx, "mintTx", 1, "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMintMsg, sellerAddress, map[string]interface{}{
		sellerAddress: 100,
	})
	assert.NilError(t, err)

	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	mintTx := findInvoiceTransactionById(txs, mintTxId)
	assert.Assert(t, mintTx != nil)

	err = tokenStore.MatchUnconfirmedMint(ctx, *mintTx)
	assert.NilError(t, err)

	// Create invoice transaction requesting more tokens than available
	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoiceMsg := &protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    quantity,
	}
	encodedInvoiceMsg, _ := proto.Marshal(invoiceMsg)
	invoiceTxId, err := tokenStore.SaveOnChainTransaction(ctx, "invoiceTx", 2, "blockHash", 1, protocol.ACTION_INVOICE, protocol.DEFAULT_VERSION, encodedInvoiceMsg, sellerAddress, map[string]interface{}{
		sellerAddress: quantity,
	})
	assert.NilError(t, err)

	txs, err = tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	invoiceTx := findInvoiceTransactionById(txs, invoiceTxId)
	assert.Assert(t, invoiceTx != nil)

	// Test Process - should succeed but not create pending balance
	err = processor.Process(*invoiceTx)
	assert.NilError(t, err)

	// Verify no pending token balance was created
	tx, _ := tokenStore.DB.Begin()
	defer tx.Rollback()

	_, err = tokenStore.GetPendingTokenBalance(ctx, invoiceHash, mintHash, tx)
	assert.Assert(t, err != nil, "No pending balance should be created")

	// Verify transaction was removed due to insufficient balance
	txsAfter, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	removedTx := findInvoiceTransactionById(txsAfter, invoiceTxId)
	assert.Assert(t, removedTx == nil, "Transaction should be removed due to insufficient balance")
}

func TestInvoiceProcessorProcessExistingPendingBalance(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()
	processor := service.NewInvoiceProcessor(tokenStore)

	// Setup: Create mint and token balance
	mintHash := support.GenerateRandomHash()
	sellerAddress := support.GenerateDogecoinAddress(true)
	invoiceHash := support.GenerateRandomHash()
	quantity := int32(50)

	// Create and match mint
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Test Mint",
		Description:     "Test Description",
		FractionCount:   100,
		BlockHeight:     1,
		TransactionHash: "mintTx",
	})
	assert.NilError(t, err)

	mintMsg := &protocol.OnChainMintMessage{Hash: mintHash}
	encodedMintMsg, _ := proto.Marshal(mintMsg)
	mintTxId, err := tokenStore.SaveOnChainTransaction(ctx, "mintTx", 1, "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMintMsg, sellerAddress, map[string]interface{}{
		sellerAddress: 100,
	})
	assert.NilError(t, err)

	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	mintTx := findInvoiceTransactionById(txs, mintTxId)
	assert.Assert(t, mintTx != nil)

	err = tokenStore.MatchUnconfirmedMint(ctx, *mintTx)
	assert.NilError(t, err)

	// Create existing pending token balance
	err = tokenStore.UpsertPendingTokenBalance(ctx, invoiceHash, mintHash, int(quantity), "existingTxId", sellerAddress)
	assert.NilError(t, err)

	// Create invoice transaction
	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoiceMsg := &protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    quantity,
	}
	encodedInvoiceMsg, _ := proto.Marshal(invoiceMsg)
	invoiceTxId, err := tokenStore.SaveOnChainTransaction(ctx, "invoiceTx", 2, "blockHash", 1, protocol.ACTION_INVOICE, protocol.DEFAULT_VERSION, encodedInvoiceMsg, sellerAddress, map[string]interface{}{
		sellerAddress: quantity,
	})
	assert.NilError(t, err)

	txs, err = tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	invoiceTx := findInvoiceTransactionById(txs, invoiceTxId)
	assert.Assert(t, invoiceTx != nil)

	// Test Process - should succeed and not create duplicate pending balance
	err = processor.Process(*invoiceTx)
	assert.NilError(t, err)

	// Verify pending token balance still exists with original values
	tx, _ := tokenStore.DB.Begin()
	defer tx.Rollback()

	pendingBalance, err := tokenStore.GetPendingTokenBalance(ctx, invoiceHash, mintHash, tx)
	assert.NilError(t, err)
	assert.Equal(t, int(quantity), pendingBalance.Quantity)
	// Just verify the pending balance exists with correct values
	assert.Equal(t, sellerAddress, pendingBalance.OwnerAddress)
}

func TestInvoiceProcessorProcessPartialTokenBalance(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()
	processor := service.NewInvoiceProcessor(tokenStore)

	// Setup: Create mint and use some tokens in existing pending balance
	mintHash := support.GenerateRandomHash()
	sellerAddress := support.GenerateDogecoinAddress(true)
	invoiceHash := support.GenerateRandomHash()
	existingInvoiceHash := "existingInvoice123"
	quantity := int32(60) // Will exceed available balance after existing pending

	// Create and match mint (100 tokens)
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Test Mint",
		Description:     "Test Description",
		FractionCount:   100,
		BlockHeight:     1,
		TransactionHash: "mintTx",
	})
	assert.NilError(t, err)

	mintMsg := &protocol.OnChainMintMessage{Hash: mintHash}
	encodedMintMsg, _ := proto.Marshal(mintMsg)
	mintTxId, err := tokenStore.SaveOnChainTransaction(ctx, "mintTx", 1, "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMintMsg, sellerAddress, map[string]interface{}{
		sellerAddress: 100,
	})
	assert.NilError(t, err)

	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	mintTx := findInvoiceTransactionById(txs, mintTxId)
	assert.Assert(t, mintTx != nil)

	err = tokenStore.MatchUnconfirmedMint(ctx, *mintTx)
	assert.NilError(t, err)

	// Create existing pending balance (50 tokens)
	err = tokenStore.UpsertPendingTokenBalance(ctx, existingInvoiceHash, mintHash, 50, "existingTxId", sellerAddress)
	assert.NilError(t, err)

	// Create invoice transaction requesting 60 tokens (50 available after existing pending)
	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoiceMsg := &protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    quantity,
	}
	encodedInvoiceMsg, _ := proto.Marshal(invoiceMsg)
	invoiceTxId, err := tokenStore.SaveOnChainTransaction(ctx, "invoiceTx", 2, "blockHash", 1, protocol.ACTION_INVOICE, protocol.DEFAULT_VERSION, encodedInvoiceMsg, sellerAddress, map[string]interface{}{
		sellerAddress: quantity,
	})
	assert.NilError(t, err)

	txs, err = tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	invoiceTx := findInvoiceTransactionById(txs, invoiceTxId)
	assert.Assert(t, invoiceTx != nil)

	// Test Process - should succeed but not create pending balance due to insufficient available tokens
	err = processor.Process(*invoiceTx)
	assert.NilError(t, err)

	// Verify no new pending token balance was created
	tx, _ := tokenStore.DB.Begin()
	defer tx.Rollback()

	_, err = tokenStore.GetPendingTokenBalance(ctx, invoiceHash, mintHash, tx)
	assert.Assert(t, err != nil, "No new pending balance should be created")

	// Verify original pending balance still exists
	existingBalance, err := tokenStore.GetPendingTokenBalance(ctx, existingInvoiceHash, mintHash, tx)
	assert.NilError(t, err)
	assert.Equal(t, 50, existingBalance.Quantity)

	// Verify transaction was removed
	txsAfter, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	removedTx := findInvoiceTransactionById(txsAfter, invoiceTxId)
	assert.Assert(t, removedTx == nil, "Transaction should be removed")
}

func TestInvoiceProcessorEnsurePendingTokenBalanceSuccess(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()
	processor := service.NewInvoiceProcessor(tokenStore)

	// Setup: Create mint and token balance
	mintHash := support.GenerateRandomHash()
	sellerAddress := support.GenerateDogecoinAddress(true)
	invoiceHash := support.GenerateRandomHash()
	quantity := int32(50)

	// Create and match mint
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Test Mint",
		Description:     "Test Description",
		FractionCount:   100,
		BlockHeight:     1,
		TransactionHash: "mintTx",
	})
	assert.NilError(t, err)

	mintMsg := &protocol.OnChainMintMessage{Hash: mintHash}
	encodedMintMsg, _ := proto.Marshal(mintMsg)
	mintTxId, err := tokenStore.SaveOnChainTransaction(ctx, "mintTx", 1, "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMintMsg, sellerAddress, map[string]interface{}{
		sellerAddress: 100,
	})
	assert.NilError(t, err)

	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	mintTx := findInvoiceTransactionById(txs, mintTxId)
	assert.Assert(t, mintTx != nil)

	err = tokenStore.MatchUnconfirmedMint(ctx, *mintTx)
	assert.NilError(t, err)

	// Create invoice transaction
	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoiceMsg := &protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    quantity,
	}
	encodedInvoiceMsg, _ := proto.Marshal(invoiceMsg)
	invoiceTx := store.OnChainTransaction{
		Id:         "invoiceTxId",
		TxHash:     "invoiceTxHash",
		ActionType: protocol.ACTION_INVOICE,
		ActionData: encodedInvoiceMsg,
		Address:    sellerAddress,
		Values:     map[string]interface{}{sellerAddress: quantity},
	}

	// Test EnsurePendingTokenBalance
	hasPending, err := processor.EnsurePendingTokenBalance(invoiceTx)
	assert.NilError(t, err)
	assert.Assert(t, hasPending, "Should have pending token balance")

	// Verify pending token balance was created
	tx, _ := tokenStore.DB.Begin()
	defer tx.Rollback()

	pendingBalance, err := tokenStore.GetPendingTokenBalance(ctx, invoiceHash, mintHash, tx)
	assert.NilError(t, err)
	assert.Equal(t, int(quantity), pendingBalance.Quantity)
}

func TestInvoiceProcessorEnsurePendingTokenBalanceInsufficientBalance(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()
	processor := service.NewInvoiceProcessor(tokenStore)

	mintHash := support.GenerateRandomHash()
	sellerAddress := support.GenerateDogecoinAddress(true)
	invoiceHash := support.GenerateRandomHash()
	quantity := int32(150) // More than any existing balance

	// Create invoice transaction (no token balance setup)
	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoiceMsg := &protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    quantity,
	}
	encodedInvoiceMsg, _ := proto.Marshal(invoiceMsg)
	invoiceTxId, err := tokenStore.SaveOnChainTransaction(ctx, "invoiceTx", 2, "blockHash", 1, protocol.ACTION_INVOICE, protocol.DEFAULT_VERSION, encodedInvoiceMsg, sellerAddress, map[string]interface{}{
		sellerAddress: quantity,
	})
	assert.NilError(t, err)

	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	invoiceTx := findInvoiceTransactionById(txs, invoiceTxId)
	assert.Assert(t, invoiceTx != nil)

	// Test EnsurePendingTokenBalance
	hasPending, err := processor.EnsurePendingTokenBalance(*invoiceTx)
	assert.NilError(t, err)
	assert.Assert(t, !hasPending, "Should not have pending token balance")

	// Verify transaction was removed
	txsAfter, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	removedTx := findInvoiceTransactionById(txsAfter, invoiceTxId)
	assert.Assert(t, removedTx == nil, "Transaction should be removed")
}
