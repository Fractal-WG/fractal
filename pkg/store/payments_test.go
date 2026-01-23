package store_test

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	test_support "dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/proto"
	"gotest.tools/assert"
)

// Helper function to find a transaction by ID
func findTransactionById(txs []store.OnChainTransaction, id string) *store.OnChainTransaction {
	for i := range txs {
		if txs[i].Id == id {
			return &txs[i]
		}
	}
	return nil
}

func TestMatchPaymentSuccess(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()

	// This test demonstrates the full payment flow:
	// 1. Create mint and match it to establish token balance
	// 2. Create invoice and match it to establish pending token balance
	// 3. Create payment and match it to transfer tokens

	mintHash := test_support.GenerateRandomHash()
	invoiceHash := test_support.GenerateRandomHash()
	sellerAddress := test_support.GenerateDogecoinAddress(true)
	buyerAddress := test_support.GenerateDogecoinAddress(true)
	quantity := 50
	value := 50.0 * 100

	// Step 1: Create and match mint
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
	mintTx := findTransactionById(txs, mintTxId)
	assert.Assert(t, mintTx != nil)

	err = tokenStore.MatchUnconfirmedMint(ctx, *mintTx)
	assert.NilError(t, err)

	// Step 2: Create and match invoice
	_, err = tokenStore.SaveUnconfirmedInvoice(ctx, &store.UnconfirmedInvoice{
		Hash:           invoiceHash,
		PaymentAddress: sellerAddress,
		BuyerAddress:   buyerAddress,
		MintHash:       mintHash,
		Quantity:       quantity,
		Price:          100,
		CreatedAt:      time.Now(),
		SellerAddress:  sellerAddress,
	})
	assert.NilError(t, err)

	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	invoiceMsg := &protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    int32(quantity),
	}
	encodedInvoiceMsg, _ := proto.Marshal(invoiceMsg)
	invoiceTxId, err := tokenStore.SaveOnChainTransaction(ctx, "invoiceTx", 2, "blockHash", 1, protocol.ACTION_INVOICE, protocol.DEFAULT_VERSION, encodedInvoiceMsg, sellerAddress, map[string]interface{}{
		sellerAddress: quantity,
	})
	assert.NilError(t, err)

	txs, err = tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	invoiceTx := findTransactionById(txs, invoiceTxId)
	assert.Assert(t, invoiceTx != nil)

	// The invoice processing normally creates pending token balance
	// For this test, we need to create it manually
	err = tokenStore.UpsertPendingTokenBalance(ctx, invoiceHash, mintHash, quantity, invoiceTx.Id, sellerAddress)
	assert.NilError(t, err)

	err = tokenStore.MatchUnconfirmedInvoice(ctx, *invoiceTx)
	assert.NilError(t, err)

	// Step 3: Create and match payment
	paymentMsg := &protocol.OnChainPaymentMessage{
		Hash: invoiceHash,
	}
	encodedPaymentMsg, _ := proto.Marshal(paymentMsg)
	paymentTxId, err := tokenStore.SaveOnChainTransaction(ctx, "paymentTx", 3, "blockHash", 1, protocol.ACTION_PAYMENT, protocol.DEFAULT_VERSION, encodedPaymentMsg, buyerAddress, map[string]interface{}{
		sellerAddress: value,
	})
	assert.NilError(t, err)

	txs, err = tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	paymentTx := findTransactionById(txs, paymentTxId)
	assert.Assert(t, paymentTx != nil)

	// Test MatchPayment
	invoice, err := tokenStore.MatchPayment(ctx, *paymentTx)
	assert.NilError(t, err)

	err = tokenStore.ProcessPayment(ctx, *paymentTx, invoice)
	assert.NilError(t, err)

	row := tokenStore.DB.QueryRow("SELECT paid_at FROM invoices WHERE hash = $1", invoiceHash)
	var paidAt sql.NullTime
	if err := row.Scan(&paidAt); err != nil {
		fmt.Println(err)
	}

	assert.NilError(t, err)
	assert.Assert(t, paidAt.Valid, "Invoice should be marked as paid")

	// Check payment transaction was deleted
	var count int
	err = tokenStore.DB.QueryRow("SELECT COUNT(*) FROM onchain_transactions WHERE id = $1", paymentTx.Id).Scan(&count)
	assert.NilError(t, err)
	assert.Equal(t, 0, count, "Payment transaction should be deleted")

	// Check pending balance was removed
	_, err = tokenStore.GetPendingTokenBalance(ctx, invoiceHash, mintHash, nil)
	assert.Assert(t, err != nil, "Pending balance should be removed")

	// Check buyer received tokens
	buyerBalances, err := tokenStore.GetTokenBalances(ctx, buyerAddress, mintHash)
	assert.NilError(t, err)
	assert.Equal(t, 1, len(buyerBalances))
	assert.Equal(t, quantity, buyerBalances[0].Quantity)

	// Check seller's balance was reduced
	sellerBalances, err := tokenStore.GetTokenBalances(ctx, sellerAddress, mintHash)
	assert.NilError(t, err)
	totalSellerBalance := 0
	for _, balance := range sellerBalances {
		totalSellerBalance += balance.Quantity
	}
	assert.Equal(t, 50, totalSellerBalance) // 100 - 50
}

func TestMatchPaymentWrongActionType(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()

	paymentTx := store.OnChainTransaction{
		Id:         "payment123",
		TxHash:     "paymentTx",
		ActionType: protocol.ACTION_MINT, // Wrong type
	}

	_, err := tokenStore.MatchPayment(ctx, paymentTx)
	assert.ErrorContains(t, err, "action type is not payment")
}

func TestMatchPaymentInvalidProtobuf(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()

	paymentTx := store.OnChainTransaction{
		Id:         "payment123",
		TxHash:     "paymentTx",
		ActionType: protocol.ACTION_PAYMENT,
		ActionData: []byte("invalid protobuf data"),
	}

	_, err := tokenStore.MatchPayment(ctx, paymentTx)
	assert.Assert(t, err != nil, "Should fail with invalid protobuf")
}

func TestMatchPaymentInvoiceNotFound(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()

	buyerAddress := test_support.GenerateDogecoinAddress(true)

	paymentMsg := &protocol.OnChainPaymentMessage{
		Hash: "nonexistentInvoice",
	}
	encodedPaymentMsg, _ := proto.Marshal(paymentMsg)

	paymentTxId, err := tokenStore.SaveOnChainTransaction(ctx, "paymentTx", 1, "blockHash", 1, protocol.ACTION_PAYMENT, protocol.DEFAULT_VERSION, encodedPaymentMsg, buyerAddress, map[string]interface{}{
		buyerAddress: 50,
	})
	assert.NilError(t, err)

	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	paymentTx := findTransactionById(txs, paymentTxId)
	assert.Assert(t, paymentTx != nil)

	_, err = tokenStore.MatchPayment(ctx, *paymentTx)
	assert.ErrorContains(t, err, "invoice not found")
}

func TestMatchPaymentValueMismatch(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()

	invoiceHash := "testInvoice123"
	buyerAddress := test_support.GenerateDogecoinAddress(true)
	sellerAddress := test_support.GenerateDogecoinAddress(true)
	mintHash := test_support.GenerateRandomHash()
	// Create invoice with specific value
	actualInvoice := &store.Invoice{
		Hash:           invoiceHash,
		PaymentAddress: sellerAddress,
		MintHash:       mintHash,
		BuyerAddress:   buyerAddress,
		Quantity:       50,
		Price:          100,
		CreatedAt:      time.Now(),
		SellerAddress:  sellerAddress,
	}
	_, err := tokenStore.SaveInvoice(ctx, actualInvoice)
	assert.NilError(t, err)

	// Create payment with wrong value
	paymentMsg := &protocol.OnChainPaymentMessage{
		Hash: invoiceHash,
	}
	encodedPaymentMsg, _ := proto.Marshal(paymentMsg)

	paymentTxId, err := tokenStore.SaveOnChainTransaction(ctx, "paymentTx", 1, "blockHash", 1, protocol.ACTION_PAYMENT, protocol.DEFAULT_VERSION, encodedPaymentMsg, buyerAddress, map[string]interface{}{
		buyerAddress: 25,
	}) // Wrong value
	assert.NilError(t, err)

	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	paymentTx := findTransactionById(txs, paymentTxId)
	assert.Assert(t, paymentTx != nil)

	_, err = tokenStore.MatchPayment(ctx, *paymentTx)
	assert.ErrorContains(t, err, "payment value is not equal to buy offer value")
}

func TestMatchPaymentPendingBalanceMismatch(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()

	invoiceHash := "testInvoice123"
	mintHash := "mint123"
	buyerAddress := test_support.GenerateDogecoinAddress(true)
	sellerAddress := test_support.GenerateDogecoinAddress(true)

	// Create invoice
	actualInvoice := &store.Invoice{
		Hash:           invoiceHash,
		PaymentAddress: sellerAddress,
		BuyerAddress:   buyerAddress,
		MintHash:       mintHash,
		Quantity:       50, // Expected quantity
		Price:          100,
		CreatedAt:      time.Now(),
		SellerAddress:  sellerAddress,
	}
	_, err := tokenStore.SaveInvoice(ctx, actualInvoice)
	assert.NilError(t, err)

	// Create pending balance with wrong quantity
	err = tokenStore.UpsertPendingTokenBalance(ctx, invoiceHash, mintHash, 30, "invoiceTxId", sellerAddress) // Wrong quantity
	assert.NilError(t, err)

	// Create payment
	paymentMsg := &protocol.OnChainPaymentMessage{
		Hash: invoiceHash,
	}
	encodedPaymentMsg, _ := proto.Marshal(paymentMsg)

	paymentTxId, err := tokenStore.SaveOnChainTransaction(ctx, "paymentTx", 1, "blockHash", 1, protocol.ACTION_PAYMENT, protocol.DEFAULT_VERSION, encodedPaymentMsg, buyerAddress, map[string]interface{}{
		sellerAddress: 5000,
	})
	assert.NilError(t, err)

	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	paymentTx := findTransactionById(txs, paymentTxId)
	assert.Assert(t, paymentTx != nil)

	invoice, err := tokenStore.MatchPayment(ctx, *paymentTx)
	assert.NilError(t, err)

	err = tokenStore.ProcessPayment(ctx, *paymentTx, invoice)
	assert.ErrorContains(t, err, "no pending token balance found")
}

func TestMatchPaymentNoPendingBalance(t *testing.T) {
	tokenStore := test_support.SetupTestDB()
	ctx := context.Background()

	invoiceHash := "testInvoice123"
	buyerAddress := test_support.GenerateDogecoinAddress(true)
	sellerAddress := test_support.GenerateDogecoinAddress(true)

	// Create invoice without pending balance
	actualInvoice := &store.Invoice{
		Hash:           invoiceHash,
		PaymentAddress: sellerAddress,
		BuyerAddress:   buyerAddress,
		MintHash:       "mint123",
		Quantity:       50,
		Price:          100,
		CreatedAt:      time.Now(),
		SellerAddress:  sellerAddress,
	}
	_, err := tokenStore.SaveInvoice(ctx, actualInvoice)
	assert.NilError(t, err)

	// Create payment
	paymentMsg := &protocol.OnChainPaymentMessage{
		Hash: invoiceHash,
	}
	encodedPaymentMsg, _ := proto.Marshal(paymentMsg)

	paymentTxId, err := tokenStore.SaveOnChainTransaction(ctx, "paymentTx", 1, "blockHash", 1, protocol.ACTION_PAYMENT, protocol.DEFAULT_VERSION, encodedPaymentMsg, buyerAddress, map[string]interface{}{
		buyerAddress: 50,
	})
	assert.NilError(t, err)

	txs, err := tokenStore.GetOnChainTransactions(ctx, 0, 10)
	assert.NilError(t, err)
	paymentTx := findTransactionById(txs, paymentTxId)
	assert.Assert(t, paymentTx != nil)

	// Should fail due to missing pending balance
	inv, err := tokenStore.MatchPayment(ctx, *paymentTx)
	assert.Assert(t, err != nil, "Should fail without pending balance")

	err = tokenStore.ProcessPayment(ctx, *paymentTx, inv)
	assert.Assert(t, err != nil, "Should fail without pending balance")

	var paidAt sql.NullTime
	err = tokenStore.DB.QueryRow("SELECT paid_at FROM invoices WHERE hash = $1", invoiceHash).Scan(&paidAt)
	fmt.Println(paidAt)
	assert.NilError(t, err)
	assert.Assert(t, !paidAt.Valid, "Invoice should NOT be paid due to rollback")
}
