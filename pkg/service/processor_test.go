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

var ownerAddress = support.GenerateDogecoinAddress(true)

func TestMintMatch(t *testing.T) {
	tokenisationStore := test_support.SetupTestDB(t)
	rpcClient := support.NewTestDogeClient(t)
	ctx := context.Background()

	hash := CreateUnconfirmedMint(t, ctx, support.GenerateRandomHash(), tokenisationStore)

	processor := service.NewFractalEngineProcessor(tokenisationStore, rpcClient)
	processor.Process()

	AssertUnconfirmedMintCreation(t, ctx, hash, tokenisationStore)
}

func TestInvoiceMatch(t *testing.T) {
	tokenisationStore := test_support.SetupTestDB(t)
	rpcClient := support.NewTestDogeClient(t)
	ctx := context.Background()

	hash := CreateUnconfirmedMint(t, ctx, support.GenerateRandomHash(), tokenisationStore)

	processor := service.NewFractalEngineProcessor(tokenisationStore, rpcClient)
	processor.Process()

	AssertUnconfirmedMintCreation(t, ctx, hash, tokenisationStore)

	buyerAddress := support.GenerateDogecoinAddress(true)
	invoiceHash := support.GenerateRandomHash()
	invoiceHash2 := support.GenerateRandomHash()
	txHash2 := support.GenerateDogecoinAddress(true)
	txHash3 := support.GenerateDogecoinAddress(true)
	txHash4 := support.GenerateDogecoinAddress(true)

	CreateOnChainInvoiceMessage(t, ctx, txHash2, 1, 1, ownerAddress, invoiceHash, hash, 50, tokenisationStore)
	processor.Process()

	CreateOnChainInvoiceMessage(t, ctx, txHash3, 1, 1, ownerAddress, invoiceHash2, hash, 90, tokenisationStore)
	processor.Process()

	SaveUnconfirmedInvoice(t, ctx, ownerAddress, buyerAddress, invoiceHash, hash, 50, tokenisationStore)
	processor.Process()

	AssertTokenBalance(t, ctx, ownerAddress, hash, 100, tokenisationStore)
	AssertPendingTokenBalance(t, ctx, invoiceHash, hash, 50, tokenisationStore)

	CreateOnChainPaymentMessage(t, ctx, txHash4, invoiceHash, buyerAddress, ownerAddress, 1, 1, 50*100, tokenisationStore)
	processor.Process()

	AssertTokenBalance(t, ctx, buyerAddress, hash, 50, tokenisationStore)
	AssertTokenBalance(t, ctx, ownerAddress, hash, 50, tokenisationStore)
}

func TestInvoiceMatchEarlierBlockHeightAndTransactionNumber(t *testing.T) {
	tokenisationStore := test_support.SetupTestDB(t)
	rpcClient := support.NewTestDogeClient(t)
	ctx := context.Background()

	hash := CreateUnconfirmedMint(t, ctx, support.GenerateRandomHash(), tokenisationStore)

	processor := service.NewFractalEngineProcessor(tokenisationStore, rpcClient)
	processor.Process()

	AssertUnconfirmedMintCreation(t, ctx, hash, tokenisationStore)

	invoiceHash := support.GenerateRandomHash()
	invoiceHash2 := support.GenerateRandomHash()
	invoiceHash3 := support.GenerateRandomHash()
	txHash2 := support.GenerateDogecoinAddress(true)
	txHash3 := support.GenerateDogecoinAddress(true)
	txHash4 := support.GenerateDogecoinAddress(true)

	CreateOnChainInvoiceMessage(t, ctx, txHash2, 3, 1, ownerAddress, invoiceHash, hash, 66, tokenisationStore)
	CreateOnChainInvoiceMessage(t, ctx, txHash3, 1, 2, ownerAddress, invoiceHash2, hash, 77, tokenisationStore)
	CreateOnChainInvoiceMessage(t, ctx, txHash4, 1, 1, ownerAddress, invoiceHash3, hash, 88, tokenisationStore)

	processor.Process()

	AssertPendingTokenBalance(t, ctx, invoiceHash3, hash, 88, tokenisationStore)
}

func TestPaymentIsLessThanExpected(t *testing.T) {
	tokenisationStore := test_support.SetupTestDB(t)
	rpcClient := support.NewTestDogeClient(t)
	ctx := context.Background()

	hash := CreateUnconfirmedMint(t, ctx, support.GenerateRandomHash(), tokenisationStore)

	processor := service.NewFractalEngineProcessor(tokenisationStore, rpcClient)
	processor.Process()

	AssertUnconfirmedMintCreation(t, ctx, hash, tokenisationStore)

	buyerAddress := support.GenerateDogecoinAddress(true)
	invoiceHash := support.GenerateRandomHash()
	txHash2 := support.GenerateDogecoinAddress(true)
	txHash3 := support.GenerateDogecoinAddress(true)

	CreateOnChainInvoiceMessage(t, ctx, txHash2, 3, 1, ownerAddress, invoiceHash, hash, 50, tokenisationStore)
	SaveUnconfirmedInvoice(t, ctx, ownerAddress, buyerAddress, invoiceHash, hash, 50, tokenisationStore)

	processor.Process()

	AssertPendingTokenBalance(t, ctx, invoiceHash, hash, 50, tokenisationStore)

	CreateOnChainPaymentMessage(t, ctx, txHash3, invoiceHash, buyerAddress, ownerAddress, 1, 1, 49, tokenisationStore)
	processor.Process()

	AssertTokenBalance(t, ctx, buyerAddress, hash, 0, tokenisationStore)
	AssertTokenBalance(t, ctx, ownerAddress, hash, 100, tokenisationStore)
}

func TestInvoiceTimesOutAfter14BlockDays(t *testing.T) {
	tokenisationStore := test_support.SetupTestDB(t)
	rpcClient := support.NewTestDogeClient(t)
	ctx := context.Background()

	hash := CreateUnconfirmedMint(t, ctx, support.GenerateRandomHash(), tokenisationStore)

	invoiceTimeoutProcessor := service.NewInvoiceTimeoutProcessor(tokenisationStore)
	processor := service.NewFractalEngineProcessor(tokenisationStore, rpcClient)
	processor.Process()

	AssertUnconfirmedMintCreation(t, ctx, hash, tokenisationStore)

	invoiceHash := support.GenerateRandomHash()
	txHash2 := support.GenerateDogecoinAddress(true)

	CreateOnChainInvoiceMessage(t, ctx, txHash2, 3, 1, ownerAddress, invoiceHash, hash, 50, tokenisationStore)
	processor.Process()

	AssertPendingTokenBalance(t, ctx, invoiceHash, hash, 50, tokenisationStore)

	invoiceTimeoutProcessor.Process(4)

	AssertNoPendingTokenBalance(t, ctx, invoiceHash, hash, tokenisationStore)
}

func AssertNoPendingTokenBalance(t *testing.T, ctx context.Context, invoiceHash string, mintHash string, tokenisationStore *store.TokenisationStore) {
	tx, err := tokenisationStore.DB.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	_, err = tokenisationStore.GetPendingTokenBalance(ctx, invoiceHash, mintHash, tx)
	if err == nil {
		t.Fatalf("Expected error getting pending token balance")
	}
}

func AssertPendingTokenBalance(t *testing.T, ctx context.Context, invoiceHash string, mintHash string, quantity int, tokenisationStore *store.TokenisationStore) {
	tx, err := tokenisationStore.DB.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	pendingTokenBalance, err := tokenisationStore.GetPendingTokenBalance(ctx, invoiceHash, mintHash, tx)
	if err != nil {
		t.Fatalf("Failed to get pending token balance: %v", err)
	}

	assert.Equal(t, quantity, pendingTokenBalance.Quantity)
}

func AssertTokenBalance(t *testing.T, ctx context.Context, s, hash string, i int, tokenisationStore *store.TokenisationStore) {
	tokenBalance, err := tokenisationStore.GetTokenBalances(ctx, s, hash)
	if err != nil {
		t.Fatalf("Failed to get token balance: %v", err)
	}

	totalQuantity := 0
	for _, balance := range tokenBalance {
		totalQuantity += balance.Quantity
	}

	assert.Equal(t, i, totalQuantity)
}

func CreateOnChainPaymentMessage(t *testing.T, ctx context.Context, trxnHash string, invoiceHash string, buyerAddress string, sellerAddress string, blockHeight int64, trxnNo int, value float64, tokenisationStore *store.TokenisationStore) {
	message3 := protocol.OnChainPaymentMessage{
		Hash: invoiceHash,
	}

	encodedMessage3, err := proto.Marshal(&message3)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	_, err = tokenisationStore.SaveOnChainTransaction(ctx, trxnHash, blockHeight, "blockHash", trxnNo, protocol.ACTION_PAYMENT, protocol.DEFAULT_VERSION, encodedMessage3, buyerAddress, map[string]interface{}{
		sellerAddress: value,
	})
	if err != nil {
		t.Fatalf("Failed to save on chain transaction: %v", err)
	}
}

func SaveUnconfirmedInvoice(t *testing.T, ctx context.Context, ownerAddress string, buyerAddress string, invoiceHash string, mintHash string, quantity int, tokenisationStore *store.TokenisationStore) {
	_, err := tokenisationStore.SaveUnconfirmedInvoice(ctx, &store.UnconfirmedInvoice{
		Hash:           invoiceHash,
		PaymentAddress: ownerAddress,
		BuyerAddress:   buyerAddress,
		MintHash:       mintHash,
		Quantity:       quantity,
		Price:          100,
		CreatedAt:      time.Now(),
		SellerAddress:  ownerAddress,
	})

	if err != nil {
		t.Fatalf("Failed to save invoice: %v", err)
	}

}

func CreateOnChainInvoiceMessage(t *testing.T, ctx context.Context, trxnHash string, blockHeight int64, trxnNo int, ownerAddress string, invoiceHash string, mintHash string, quantity int, tokenisationStore *store.TokenisationStore) {
	invoiceHashBytes, err := hex.DecodeString(invoiceHash)
	assert.NilError(t, err)
	mintHashBytes, err := hex.DecodeString(mintHash)
	assert.NilError(t, err)

	message := &protocol.OnChainInvoiceMessage{
		InvoiceHash: invoiceHashBytes,
		MintHash:    mintHashBytes,
		Quantity:    int32(quantity),
	}
	encodedMessage, err := proto.Marshal(message)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	_, err = tokenisationStore.SaveOnChainTransaction(ctx, trxnHash, blockHeight, "blockHash", trxnNo, protocol.ACTION_INVOICE, protocol.DEFAULT_VERSION, encodedMessage, ownerAddress, map[string]interface{}{
		ownerAddress: quantity,
	})
	if err != nil {
		t.Fatalf("Failed to save on chain transaction: %v", err)
	}
}

func AssertUnconfirmedMintCreation(t *testing.T, ctx context.Context, hash string, tokenisationStore *store.TokenisationStore) {
	mints, err := tokenisationStore.GetMints(ctx, 0, 100)
	if err != nil {
		t.Fatalf("Failed to get mints: %v", err)
	}

	assert.Equal(t, 1, len(mints))
	assert.Equal(t, "Test Mint", mints[0].Title)

	tokenBalance, err := tokenisationStore.GetTokenBalances(ctx, ownerAddress, hash)
	if err != nil {
		t.Fatalf("Failed to get token balance: %v", err)
	}

	assert.Equal(t, 100, tokenBalance[0].Quantity)
}

func CreateUnconfirmedMint(t *testing.T, ctx context.Context, trxnHash string, tokenisationStore *store.TokenisationStore) string {

	mintWithoutId := store.MintWithoutID{
		Title:           "Test Mint",
		Description:     "Test Description",
		Tags:            []string{"test", "test2"},
		FeedURL:         "https://test.com",
		FractionCount:   100,
		BlockHeight:     20,
		TransactionHash: trxnHash,
	}
	hash, err := mintWithoutId.GenerateHash()
	if err != nil {
		t.Fatalf("Failed to generate hash: %v", err)
	}
	mintWithoutId.Hash = hash

	_, err = tokenisationStore.SaveUnconfirmedMint(ctx, &mintWithoutId)
	if err != nil {
		t.Fatalf("Failed to save unconfirmed mint: %v", err)
	}

	message := protocol.OnChainMintMessage{
		Hash: hash,
	}

	encodedMessage, err := proto.Marshal(&message)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	_, err = tokenisationStore.SaveOnChainTransaction(ctx, trxnHash, 1, "blockHash", 1, protocol.ACTION_MINT, protocol.DEFAULT_VERSION, encodedMessage, ownerAddress, map[string]interface{}{
		ownerAddress: 100,
	})
	if err != nil {
		t.Fatalf("Failed to save on chain transaction: %v", err)
	}

	return hash
}
