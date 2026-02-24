package store_test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/proto"
	"gotest.tools/assert"
)

// saveBurnableMint saves a confirmed mint with Burnable=true and seeds an initial token balance.
func saveBurnableMint(t *testing.T, db *store.TokenisationStore, mintHash string, fractionCount int, ownerAddress string) {
	t.Helper()
	mint := &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Burn Test Mint",
		FractionCount:   fractionCount,
		Description:     "Mint for burn tests",
		Tags:            store.StringArray{},
		Metadata:        store.StringInterfaceMap{},
		Requirements:    store.StringInterfaceMap{},
		LockupOptions:   store.StringInterfaceMap{},
		PublicKey:       "pubKey",
		TransactionHash: "txHash",
		Burnable:        true,
		BurnAuthority:   store.BurnAuthorityHolderOnly,
	}
	_, err := db.SaveMint(context.Background(), mint, ownerAddress)
	assert.NilError(t, err)
}

// saveBurnOnChainTx builds the OnChainTokenBurnMessage, saves it as an on-chain transaction,
// and returns the OnChainTransaction struct.
func saveBurnOnChainTx(t *testing.T, db *store.TokenisationStore, mintHashHex, burnHashHex string, burnQuantity int32) store.OnChainTransaction {
	t.Helper()
	mintHashBytes, err := hex.DecodeString(mintHashHex)
	assert.NilError(t, err)
	burnHashBytes, err := hex.DecodeString(burnHashHex)
	assert.NilError(t, err)

	msg := &protocol.OnChainTokenBurnMessage{
		MintHash:     mintHashBytes,
		BurnQuantity: burnQuantity,
		BurnHash:     burnHashBytes,
	}
	actionData, err := proto.Marshal(msg)
	assert.NilError(t, err)

	txId, err := db.SaveOnChainTransaction(context.Background(), "burnTxHash", 5000, "blockHash", 1, protocol.ACTION_TOKEN_BURN, 1, actionData, "burnerAddr", map[string]interface{}{})
	assert.NilError(t, err)

	return store.OnChainTransaction{
		Id:            txId,
		TxHash:        "burnTxHash",
		Height:        5000,
		ActionType:    protocol.ACTION_TOKEN_BURN,
		ActionVersion: 1,
		ActionData:    actionData,
		Address:       "burnerAddr",
	}
}

func TestSaveUnconfirmedTokenBurn(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	burnHash := support.GenerateRandomHash()
	burnerAddress := support.GenerateDogecoinAddress(true)

	burn := &store.UnconfirmedTokenBurn{
		Hash:          burnHash,
		MintHash:      mintHash,
		BurnQuantity:  100,
		BurnerAddress: burnerAddress,
		PublicKey:     "pubKey123",
		Signature:     "sig123",
		CreatedAt:     time.Now(),
	}

	id, err := db.SaveUnconfirmedTokenBurn(testCtx, burn)
	assert.NilError(t, err)
	assert.Assert(t, id != "")
}

func TestMatchUnconfirmedTokenBurn(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	burnHash := support.GenerateRandomHash()
	burnerAddress := support.GenerateDogecoinAddress(true)
	const fractionCount = 1000
	const burnQuantity = 200

	// Save confirmed mint
	saveBurnableMint(t, db, mintHash, fractionCount, burnerAddress)

	// Seed token balance for the burner
	err := db.UpsertTokenBalance(testCtx, burnerAddress, mintHash, fractionCount)
	assert.NilError(t, err)

	// Verify initial current_supply
	mintBefore, err := db.GetMintByHash(testCtx, mintHash)
	assert.NilError(t, err)
	assert.Equal(t, mintBefore.CurrentSupply, fractionCount)

	// Save unconfirmed burn
	burn := &store.UnconfirmedTokenBurn{
		Hash:          burnHash,
		MintHash:      mintHash,
		BurnQuantity:  burnQuantity,
		BurnerAddress: burnerAddress,
		PublicKey:     "pubKey",
		Signature:     "sig",
		CreatedAt:     time.Now(),
	}
	_, err = db.SaveUnconfirmedTokenBurn(testCtx, burn)
	assert.NilError(t, err)

	// Build and save matching on-chain transaction
	onchainTx := saveBurnOnChainTx(t, db, mintHash, burnHash, burnQuantity)

	// Perform the match
	err = db.MatchUnconfirmedTokenBurn(testCtx, onchainTx)
	assert.NilError(t, err)

	// current_supply should be reduced by burn_quantity
	mintAfter, err := db.GetMintByHash(testCtx, mintHash)
	assert.NilError(t, err)
	assert.Equal(t, mintAfter.CurrentSupply, fractionCount-burnQuantity)

	// token_balances for the burner should reflect the deduction
	balances, err := db.GetTokenBalances(testCtx, burnerAddress, mintHash)
	assert.NilError(t, err)
	var burnerBalance int
	for _, b := range balances {
		burnerBalance += b.Quantity
	}
	assert.Equal(t, burnerBalance, fractionCount-burnQuantity)

	// unconfirmed burn should be deleted — a second match should fail
	err = db.MatchUnconfirmedTokenBurn(testCtx, onchainTx)
	assert.Assert(t, err != nil, "second match should fail — burn already consumed")

	// on-chain transaction should be deleted
	transactions, err := db.GetOnChainTransactions(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(transactions), 0)
}

func TestMatchUnconfirmedTokenBurn_NotFound(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	unknownBurnHash := support.GenerateRandomHash()

	onchainTx := saveBurnOnChainTx(t, db, mintHash, unknownBurnHash, 100)

	err := db.MatchUnconfirmedTokenBurn(testCtx, onchainTx)
	assert.Assert(t, err != nil, "should fail when no matching unconfirmed burn exists")
}

func TestMatchUnconfirmedTokenBurn_MintHashMismatch(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	differentMintHash := support.GenerateRandomHash()
	burnHash := support.GenerateRandomHash()
	burnerAddress := support.GenerateDogecoinAddress(true)

	// Save burn against mintHash
	burn := &store.UnconfirmedTokenBurn{
		Hash:          burnHash,
		MintHash:      mintHash,
		BurnQuantity:  100,
		BurnerAddress: burnerAddress,
		PublicKey:     "pubKey",
		Signature:     "sig",
		CreatedAt:     time.Now(),
	}
	_, err := db.SaveUnconfirmedTokenBurn(testCtx, burn)
	assert.NilError(t, err)

	// On-chain message references a different mint_hash
	onchainTx := saveBurnOnChainTx(t, db, differentMintHash, burnHash, 100)

	err = db.MatchUnconfirmedTokenBurn(testCtx, onchainTx)
	assert.Assert(t, err != nil, "should fail when mint_hash in on-chain message does not match stored record")
}

func TestMatchUnconfirmedTokenBurn_QuantityMismatch(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	burnHash := support.GenerateRandomHash()
	burnerAddress := support.GenerateDogecoinAddress(true)

	burn := &store.UnconfirmedTokenBurn{
		Hash:          burnHash,
		MintHash:      mintHash,
		BurnQuantity:  100,
		BurnerAddress: burnerAddress,
		PublicKey:     "pubKey",
		Signature:     "sig",
		CreatedAt:     time.Now(),
	}
	_, err := db.SaveUnconfirmedTokenBurn(testCtx, burn)
	assert.NilError(t, err)

	// On-chain message claims a different burn_quantity
	onchainTx := saveBurnOnChainTx(t, db, mintHash, burnHash, 999)

	err = db.MatchUnconfirmedTokenBurn(testCtx, onchainTx)
	assert.Assert(t, err != nil, "should fail when burn_quantity in on-chain message does not match stored record")
}

func TestMatchUnconfirmedTokenBurn_InsufficientBalance(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	burnHash := support.GenerateRandomHash()
	burnerAddress := support.GenerateDogecoinAddress(true)
	const fractionCount = 50
	const burnQuantity = 200 // more than available

	saveBurnableMint(t, db, mintHash, fractionCount, burnerAddress)

	// Seed a small token balance
	err := db.UpsertTokenBalance(testCtx, burnerAddress, mintHash, fractionCount)
	assert.NilError(t, err)

	burn := &store.UnconfirmedTokenBurn{
		Hash:          burnHash,
		MintHash:      mintHash,
		BurnQuantity:  burnQuantity,
		BurnerAddress: burnerAddress,
		PublicKey:     "pubKey",
		Signature:     "sig",
		CreatedAt:     time.Now(),
	}
	_, err = db.SaveUnconfirmedTokenBurn(testCtx, burn)
	assert.NilError(t, err)

	onchainTx := saveBurnOnChainTx(t, db, mintHash, burnHash, burnQuantity)

	err = db.MatchUnconfirmedTokenBurn(testCtx, onchainTx)
	assert.Assert(t, err != nil, "should fail when burner has insufficient balance")
}
