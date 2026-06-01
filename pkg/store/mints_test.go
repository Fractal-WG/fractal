package store_test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"gotest.tools/assert"
)

var testCtx = context.Background()

func TestSaveMint(t *testing.T) {
	db := support.SetupTestDB(t)

	mint := &store.MintWithoutID{
		Hash:                     "testHash123",
		Title:                    "Test Mint",
		FractionCount:            1000,
		Description:              "Test Description",
		Tags:                     store.StringArray{"tag1", "tag2"},
		Metadata:                 store.StringInterfaceMap{"key": "value"},
		TransactionHash:          "txHash123",
		BlockHeight:              12345,
		Requirements:             store.StringInterfaceMap{"req": "value"},
		LockupOptions:            store.StringInterfaceMap{"lockup": "option"},
		FeedURL:                  "https://example.com/feed",
		PublicKey:                "publicKey123",
		OwnerAddress:             "ownerAddress123",
		Signature:                "signature123",
		ContractOfSale:           "contract of sale",
		SignatureRequirementType: store.SignatureRequirementType_ALL_SIGNATURES,
		AssetManagers:            store.AssetManagers{{Name: "asset manager", PublicKey: "publicKey123", URL: "https://example.com/assetManager"}},
		MinSignatures:            1,
	}

	id, err := db.SaveMint(testCtx, mint, "ownerAddress123")
	assert.NilError(t, err)
	assert.Assert(t, id != "")

	// Verify the mint was saved
	savedMint, err := db.GetMintByHash(testCtx, "testHash123")
	assert.NilError(t, err)
	assert.Equal(t, savedMint.Hash, "testHash123")
	assert.Equal(t, savedMint.Title, "Test Mint")
	assert.Equal(t, savedMint.FractionCount, 1000)
	assert.Equal(t, savedMint.Description, "Test Description")
	assert.Equal(t, savedMint.FeedURL, "https://example.com/feed")
	assert.Equal(t, savedMint.PublicKey, "publicKey123")
	assert.Equal(t, savedMint.OwnerAddress, "ownerAddress123")
}

func TestGetMintByHash(t *testing.T) {
	db := support.SetupTestDB(t)

	// Test non-existent mint
	mint, err := db.GetMintByHash(testCtx, "nonExistent")
	assert.NilError(t, err)
	assert.Equal(t, mint.Hash, "")

	// Save a mint
	mintToSave := &store.MintWithoutID{
		Hash:                     "testHash456",
		Title:                    "Test Mint 2",
		FractionCount:            500,
		Description:              "Another test mint",
		Tags:                     store.StringArray{},
		Metadata:                 store.StringInterfaceMap{},
		Requirements:             store.StringInterfaceMap{},
		LockupOptions:            store.StringInterfaceMap{},
		PublicKey:                "pubKey456",
		SignatureRequirementType: store.SignatureRequirementType_ALL_SIGNATURES,
		AssetManagers:            store.AssetManagers{{Name: "asset manager", PublicKey: "publicKey123", URL: "https://example.com/assetManager"}},
		MinSignatures:            1,
	}

	_, err = db.SaveMint(testCtx, mintToSave, "owner456")
	assert.NilError(t, err)

	// Get the mint
	retrievedMint, err := db.GetMintByHash(testCtx, "testHash456")
	assert.NilError(t, err)
	assert.Equal(t, retrievedMint.Hash, "testHash456")
	assert.Equal(t, retrievedMint.Title, "Test Mint 2")
	assert.Equal(t, retrievedMint.SignatureRequirementType, store.SignatureRequirementType_ALL_SIGNATURES)
	assert.Equal(t, retrievedMint.AssetManagers[0].Name, "asset manager")
	assert.Equal(t, retrievedMint.MinSignatures, 1)
}

func TestGetMints(t *testing.T) {
	db := support.SetupTestDB(t)

	// Save multiple mints
	for i := 0; i < 5; i++ {
		mint := &store.MintWithoutID{
			Hash:          string(rune(i+65)) + "hash",
			Title:         string(rune(i+65)) + " Mint",
			FractionCount: (i + 1) * 100,
			Description:   "Test mint " + string(rune(i+65)),
			Tags:          store.StringArray{},
			Metadata:      store.StringInterfaceMap{},
			Requirements:  store.StringInterfaceMap{},
			LockupOptions: store.StringInterfaceMap{},
			PublicKey:     "pubKey" + string(rune(i+65)),
		}
		_, err := db.SaveMint(testCtx, mint, "owner"+string(rune(i+65)))
		assert.NilError(t, err)
	}

	// Test pagination
	mints, err := db.GetMints(testCtx, 0, 3)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 3)

	// Test offset
	mints, err = db.GetMints(testCtx, 2, 3)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 3)

	// Test getting all
	mints, err = db.GetMints(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 5)
}

func TestGetMintsByPublicKey(t *testing.T) {
	db := support.SetupTestDB(t)

	// Save mints with different public keys
	mint1 := &store.MintWithoutID{
		Hash:            "hash1",
		Title:           "Mint 1",
		FractionCount:   100,
		Description:     "Test mint 1",
		Tags:            store.StringArray{},
		Metadata:        store.StringInterfaceMap{},
		Requirements:    store.StringInterfaceMap{},
		LockupOptions:   store.StringInterfaceMap{},
		PublicKey:       "pubKey1",
		TransactionHash: "tx1",
	}
	_, err := db.SaveMint(testCtx, mint1, "owner1")
	assert.NilError(t, err)

	mint2 := &store.MintWithoutID{
		Hash:            "hash2",
		Title:           "Mint 2",
		FractionCount:   200,
		Description:     "Test mint 2",
		Tags:            store.StringArray{},
		Metadata:        store.StringInterfaceMap{},
		Requirements:    store.StringInterfaceMap{},
		LockupOptions:   store.StringInterfaceMap{},
		PublicKey:       "pubKey1",
		TransactionHash: "tx2",
	}
	_, err = db.SaveMint(testCtx, mint2, "owner2")
	assert.NilError(t, err)

	mint3 := &store.MintWithoutID{
		Hash:            "hash3",
		Title:           "Mint 3",
		FractionCount:   300,
		Description:     "Test mint 3",
		Tags:            store.StringArray{},
		Metadata:        store.StringInterfaceMap{},
		Requirements:    store.StringInterfaceMap{},
		LockupOptions:   store.StringInterfaceMap{},
		PublicKey:       "pubKey2",
		TransactionHash: "tx3",
	}
	_, err = db.SaveMint(testCtx, mint3, "owner3")
	assert.NilError(t, err)

	// Get mints by public key
	mints, err := db.GetMintsByPublicKey(testCtx, 0, 10, "pubKey1", false)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 2)
	assert.Equal(t, mints[0].PublicKey, "pubKey1")
	assert.Equal(t, mints[1].PublicKey, "pubKey1")

	// Test with different public key
	mints, err = db.GetMintsByPublicKey(testCtx, 0, 10, "pubKey2", false)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 1)
	assert.Equal(t, mints[0].PublicKey, "pubKey2")

	// Test with non-existent public key
	mints, err = db.GetMintsByPublicKey(testCtx, 0, 10, "nonExistent", false)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 0)
}

func TestSaveUnconfirmedMint(t *testing.T) {
	db := support.SetupTestDB(t)

	mint := &store.MintWithoutID{
		Hash:                     "unconfirmedHash123",
		Title:                    "Unconfirmed Mint",
		FractionCount:            1500,
		Description:              "Unconfirmed test mint",
		Tags:                     store.StringArray{"unconfirmed"},
		Metadata:                 store.StringInterfaceMap{"status": "unconfirmed"},
		Requirements:             store.StringInterfaceMap{},
		LockupOptions:            store.StringInterfaceMap{},
		FeedURL:                  "https://example.com/unconfirmed",
		PublicKey:                "unconfirmedPubKey",
		OwnerAddress:             "unconfirmedOwner",
		TransactionHash:          "",
		SignatureRequirementType: store.SignatureRequirementType_ALL_SIGNATURES,
		AssetManagers:            store.AssetManagers{{Name: "asset manager", PublicKey: "publicKey123", URL: "https://example.com/assetManager"}},
		MinSignatures:            1,
	}

	id, err := db.SaveUnconfirmedMint(testCtx, mint)
	assert.NilError(t, err)
	assert.Assert(t, id != "")

	// Verify the unconfirmed mint was saved
	unconfirmedMints, err := db.GetUnconfirmedMints(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(unconfirmedMints), 1)
	assert.Equal(t, unconfirmedMints[0].Hash, "unconfirmedHash123")
	assert.Equal(t, unconfirmedMints[0].Title, "Unconfirmed Mint")
	assert.Equal(t, unconfirmedMints[0].SignatureRequirementType, store.SignatureRequirementType_ALL_SIGNATURES)
	assert.Equal(t, unconfirmedMints[0].AssetManagers[0].Name, "asset manager")
	assert.Equal(t, unconfirmedMints[0].MinSignatures, 1)
}

func TestGetUnconfirmedMints(t *testing.T) {
	db := support.SetupTestDB(t)

	// Save multiple unconfirmed mints
	for i := 0; i < 3; i++ {
		mint := &store.MintWithoutID{
			Hash:          "unconfHash" + string(rune(i+65)),
			Title:         "Unconfirmed " + string(rune(i+65)),
			FractionCount: (i + 1) * 100,
			Description:   "Unconfirmed mint " + string(rune(i+65)),
			Tags:          store.StringArray{},
			Metadata:      store.StringInterfaceMap{},
			Requirements:  store.StringInterfaceMap{},
			LockupOptions: store.StringInterfaceMap{},
			PublicKey:     "pubKey" + string(rune(i+65)),
		}
		_, err := db.SaveUnconfirmedMint(testCtx, mint)
		assert.NilError(t, err)
	}

	// Test pagination
	mints, err := db.GetUnconfirmedMints(testCtx, 0, 2)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 2)

	// Test getting all
	mints, err = db.GetUnconfirmedMints(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 3)
}

func TestTrimOldUnconfirmedMints(t *testing.T) {
	db := support.SetupTestDB(t)

	// Save 5 unconfirmed mints
	for i := 0; i < 5; i++ {
		mint := &store.MintWithoutID{
			Hash:          "trimHash" + string(rune(i+65)),
			Title:         "Trim Mint " + string(rune(i+65)),
			FractionCount: 100,
			Description:   "Mint to trim",
			Tags:          store.StringArray{},
			Metadata:      store.StringInterfaceMap{},
			Requirements:  store.StringInterfaceMap{},
			LockupOptions: store.StringInterfaceMap{},
			PublicKey:     "pubKey",
		}
		_, err := db.SaveUnconfirmedMint(testCtx, mint)
		assert.NilError(t, err)
	}

	// Trim to keep only 3 most recent
	err := db.TrimOldUnconfirmedMints(testCtx, 3)
	assert.NilError(t, err)

	// Verify only 3 remain
	mints, err := db.GetUnconfirmedMints(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 3)
}

func TestMatchMint(t *testing.T) {
	db := support.SetupTestDB(t)

	// Save a mint where hash matches the transaction hash (as required by MatchMint logic)
	mint := &store.MintWithoutID{
		Hash:            "matchTxHash", // This must match the transaction hash
		Title:           "Match Mint",
		FractionCount:   100,
		Description:     "Mint to match",
		Tags:            store.StringArray{},
		Metadata:        store.StringInterfaceMap{},
		Requirements:    store.StringInterfaceMap{},
		LockupOptions:   store.StringInterfaceMap{},
		PublicKey:       "pubKey",
		TransactionHash: "matchTxHash",
		BlockHeight:     1000,
	}
	_, err := db.SaveMint(testCtx, mint, "owner")
	assert.NilError(t, err)

	// Create matching onchain message with same hash
	onchainMsg := &protocol.OnChainMintMessage{
		Hash: "matchTxHash", // Must match the transaction hash
	}
	actionData, err := proto.Marshal(onchainMsg)
	assert.NilError(t, err)

	// Save onchain transaction
	txId, err := db.SaveOnChainTransaction(testCtx, "matchTxHash", 1000, "blockHash", 1, protocol.ACTION_MINT, 1, actionData, "addr", map[string]interface{}{
		"addr": 0,
	})
	assert.NilError(t, err)

	// Create OnChainTransaction
	onchainTx := store.OnChainTransaction{
		Id:            txId,
		TxHash:        "matchTxHash",
		Height:        1000,
		ActionType:    protocol.ACTION_MINT,
		ActionVersion: 1,
		ActionData:    actionData,
		Address:       "addr",
		Values: map[string]interface{}{
			"addr": 0,
		},
		TransactionNumber: 1,
	}

	// Test matching
	matched := db.MatchMint(testCtx, onchainTx)
	assert.Assert(t, matched)

	// Verify the onchain transaction was deleted
	transactions, err := db.GetOnChainTransactions(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(transactions), 0)
}

func TestMatchUnconfirmedMint(t *testing.T) {
	db := support.SetupTestDB(t)

	// Save an unconfirmed mint
	unconfirmedMint := &store.MintWithoutID{
		Hash:          "unconfMatchHash",
		Title:         "Unconfirmed Match Mint",
		FractionCount: 500,
		Description:   "Unconfirmed mint to match",
		Tags:          store.StringArray{"test"},
		Metadata:      store.StringInterfaceMap{"key": "value"},
		Requirements:  store.StringInterfaceMap{"req": "test"},
		LockupOptions: store.StringInterfaceMap{"lockup": "test"},
		FeedURL:       "https://example.com",
		PublicKey:     "pubKeyMatch",
	}
	_, err := db.SaveUnconfirmedMint(testCtx, unconfirmedMint)
	assert.NilError(t, err)

	// Create matching onchain message
	onchainMsg := &protocol.OnChainMintMessage{
		Hash: "unconfMatchHash",
	}
	actionData, err := proto.Marshal(onchainMsg)
	assert.NilError(t, err)

	// Save onchain transaction
	txId, err := db.SaveOnChainTransaction(testCtx, "confirmTxHash", 2000, "blockHash", 1, protocol.ACTION_MINT, 1, actionData, "confirmedAddr", map[string]interface{}{
		"addr": 0,
	})
	assert.NilError(t, err)

	// Create OnChainTransaction
	onchainTx := store.OnChainTransaction{
		Id:            txId,
		TxHash:        "confirmTxHash",
		Height:        2000,
		ActionType:    protocol.ACTION_MINT,
		ActionVersion: 1,
		ActionData:    actionData,
		Address:       "confirmedAddr",
		Values: map[string]interface{}{
			"addr": 0,
		},
		TransactionNumber: 1,
	}

	// Match the unconfirmed mint
	err = db.MatchUnconfirmedMint(testCtx, onchainTx)
	assert.NilError(t, err)

	// Verify the mint is now confirmed
	confirmedMint, err := db.GetMintByHash(testCtx, "unconfMatchHash")
	assert.NilError(t, err)
	assert.Equal(t, confirmedMint.Hash, "unconfMatchHash")
	assert.Equal(t, confirmedMint.TransactionHash, "confirmTxHash")
	// Note: BlockHeight is not returned by GetMintByHash query
	assert.Equal(t, confirmedMint.OwnerAddress, "confirmedAddr")

	// Verify the unconfirmed mint was deleted
	unconfirmedMints, err := db.GetUnconfirmedMints(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(unconfirmedMints), 0)

	// Verify the onchain transaction was deleted
	transactions, err := db.GetOnChainTransactions(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(transactions), 0)
}

func TestClearMints(t *testing.T) {
	db := support.SetupTestDB(t)

	// Save some mints
	for i := 0; i < 3; i++ {
		mint := &store.MintWithoutID{
			Hash:          "clearHash" + string(rune(i+65)),
			Title:         "Clear Mint " + string(rune(i+65)),
			FractionCount: 100,
			Description:   "Mint to clear",
			Tags:          store.StringArray{},
			Metadata:      store.StringInterfaceMap{},
			Requirements:  store.StringInterfaceMap{},
			LockupOptions: store.StringInterfaceMap{},
			PublicKey:     "pubKey",
		}
		_, err := db.SaveMint(testCtx, mint, "owner")
		assert.NilError(t, err)
	}

	// Verify mints exist
	mints, err := db.GetMints(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 3)

	// Clear all mints
	err = db.ClearMints(testCtx)
	assert.NilError(t, err)

	// Verify all mints are gone
	mints, err = db.GetMints(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 0)
}

func TestMintWithComplexMetadata(t *testing.T) {
	db := support.SetupTestDB(t)

	// Test with complex nested metadata
	complexMetadata := store.StringInterfaceMap{
		"string":  "value",
		"number":  float64(123),
		"boolean": true,
		"array":   []interface{}{"item1", "item2", float64(3)},
		"nested": map[string]interface{}{
			"key1": "value1",
			"key2": float64(456),
		},
	}

	mint := &store.MintWithoutID{
		Hash:          "complexHash",
		Title:         "Complex Mint",
		FractionCount: 100,
		Description:   "Mint with complex metadata",
		Tags:          store.StringArray{"tag1", "tag2", "tag3"},
		Metadata:      complexMetadata,
		Requirements: store.StringInterfaceMap{
			"minBalance": float64(1000),
			"verified":   true,
		},
		LockupOptions: store.StringInterfaceMap{
			"duration": float64(86400),
			"penalty":  float64(10),
		},
		FeedURL:   "https://example.com/complex",
		PublicKey: "complexPubKey",
	}

	id, err := db.SaveMint(testCtx, mint, "complexOwner")
	assert.NilError(t, err)
	assert.Assert(t, id != "")

	// Retrieve and verify
	retrievedMint, err := db.GetMintByHash(testCtx, "complexHash")
	assert.NilError(t, err)
	assert.Equal(t, retrievedMint.Hash, "complexHash")
	assert.Equal(t, len(retrievedMint.Tags), 3)
	assert.Equal(t, retrievedMint.Metadata["string"], "value")
	assert.Equal(t, retrievedMint.Metadata["number"], float64(123))
	assert.Equal(t, retrievedMint.Metadata["boolean"], true)
}

func TestGetMintsByPublicKeyWithUnconfirmed(t *testing.T) {
	db := support.SetupTestDB(t)

	// Save a confirmed mint
	confirmedMint := &store.MintWithoutID{
		Hash:            "confHash",
		Title:           "Confirmed Mint",
		FractionCount:   100,
		Description:     "Confirmed mint",
		Tags:            store.StringArray{},
		Metadata:        store.StringInterfaceMap{},
		Requirements:    store.StringInterfaceMap{},
		LockupOptions:   store.StringInterfaceMap{},
		PublicKey:       "testPubKey",
		TransactionHash: "txHash",
	}
	_, err := db.SaveMint(testCtx, confirmedMint, "owner")
	assert.NilError(t, err)

	// Save an unconfirmed mint with same public key
	unconfirmedMint := &store.MintWithoutID{
		Hash:          "unconfHash",
		Title:         "Unconfirmed Mint",
		FractionCount: 200,
		Description:   "Unconfirmed mint",
		Tags:          store.StringArray{},
		Metadata:      store.StringInterfaceMap{},
		Requirements:  store.StringInterfaceMap{},
		LockupOptions: store.StringInterfaceMap{},
		PublicKey:     "testPubKey",
	}
	_, err = db.SaveUnconfirmedMint(testCtx, unconfirmedMint)
	assert.NilError(t, err)

	// Get mints without unconfirmed
	mints, err := db.GetMintsByPublicKey(testCtx, 0, 10, "testPubKey", false)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 1)
	assert.Equal(t, mints[0].Title, "Confirmed Mint")

	// Get mints with unconfirmed
	mints, err = db.GetMintsByPublicKey(testCtx, 0, 10, "testPubKey", true)
	assert.NilError(t, err)
	assert.Equal(t, len(mints), 2)
}

// helpers shared by expansion tests

func saveConfirmedMintForExpansion(t *testing.T, db *store.TokenisationStore, mintHash string, fractionCount int, allowExpansion bool, ownerAddress string) {
	t.Helper()
	mint := &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Expansion Mint",
		FractionCount:   fractionCount,
		Description:     "Mint for expansion tests",
		Tags:            store.StringArray{},
		Metadata:        store.StringInterfaceMap{},
		Requirements:    store.StringInterfaceMap{},
		LockupOptions:   store.StringInterfaceMap{},
		PublicKey:       "pubKey",
		TransactionHash: "txHash",
		AllowExpansion:  allowExpansion,
	}
	_, err := db.SaveMint(context.Background(), mint, ownerAddress)
	assert.NilError(t, err)
}

func saveExpansionOnChainTx(t *testing.T, db *store.TokenisationStore, mintHashHex, expansionHashHex string, additionalSupply int32) store.OnChainTransaction {
	t.Helper()
	mintHashBytes, err := hex.DecodeString(mintHashHex)
	assert.NilError(t, err)
	expansionHashBytes, err := hex.DecodeString(expansionHashHex)
	assert.NilError(t, err)

	msg := &protocol.OnChainMintExpansionMessage{
		MintHash:         mintHashBytes,
		AdditionalSupply: additionalSupply,
		ExpansionHash:    expansionHashBytes,
	}
	actionData, err := proto.Marshal(msg)
	assert.NilError(t, err)

	txId, err := db.SaveOnChainTransaction(context.Background(), "expansionTxHash", 3000, "blockHash", 1, protocol.ACTION_MINT_EXPANSION, 1, actionData, "ownerAddr", map[string]interface{}{})
	assert.NilError(t, err)

	return store.OnChainTransaction{
		Id:            txId,
		TxHash:        "expansionTxHash",
		Height:        3000,
		ActionType:    protocol.ACTION_MINT_EXPANSION,
		ActionVersion: 1,
		ActionData:    actionData,
		Address:       "ownerAddr",
	}
}

func TestMintExpansionGenerateHashIsUniquePerRequest(t *testing.T) {
	mintHash := support.GenerateRandomHash()
	ownerAddress := "nTestOwnerAddress1234567890123456"
	pubKey := "pubKey123"
	signature := "sameSig"

	e1 := &store.UnconfirmedMintExpansion{
		MintHash:         mintHash,
		AdditionalSupply: 10,
		OwnerAddress:     ownerAddress,
		PublicKey:        pubKey,
		Signature:        signature,
		Nonce:            uuid.New().String(),
		CreatedAt:        time.Now(),
	}
	e2 := &store.UnconfirmedMintExpansion{
		MintHash:         mintHash,
		AdditionalSupply: 10,
		OwnerAddress:     ownerAddress,
		PublicKey:        pubKey,
		Signature:        signature,
		Nonce:            uuid.New().String(),
		CreatedAt:        time.Now(),
	}

	h1, err := e1.GenerateHash()
	assert.NilError(t, err)
	h2, err := e2.GenerateHash()
	assert.NilError(t, err)

	assert.Assert(t, h1 != h2, "identical expansion params with different nonces must produce different hashes")
}

func TestSaveUnconfirmedMintExpansion(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	expansionHash := support.GenerateRandomHash()

	expansion := &store.UnconfirmedMintExpansion{
		Hash:             expansionHash,
		MintHash:         mintHash,
		AdditionalSupply: 250,
		OwnerAddress:     "nTestOwnerAddress1234567890123456",
		PublicKey:        "pubKey123",
		Signature:        "sig123",
		Nonce:            uuid.New().String(),
		CreatedAt:        time.Now(),
	}

	id, err := db.SaveUnconfirmedMintExpansion(testCtx, expansion)
	assert.NilError(t, err)
	assert.Assert(t, id != "")
}

func TestMatchUnconfirmedMintExpansion(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	expansionHash := support.GenerateRandomHash()
	ownerAddress := support.GenerateDogecoinAddress(true)
	const fractionCount = 1000
	const additionalSupply = 250

	// Save the confirmed mint with an initial token balance for the owner
	saveConfirmedMintForExpansion(t, db, mintHash, fractionCount, true, ownerAddress)
	err := db.UpsertTokenBalance(testCtx, ownerAddress, mintHash, fractionCount)
	assert.NilError(t, err)

	// Save the unconfirmed expansion
	expansion := &store.UnconfirmedMintExpansion{
		Hash:             expansionHash,
		MintHash:         mintHash,
		AdditionalSupply: additionalSupply,
		OwnerAddress:     ownerAddress,
		PublicKey:        "pubKey",
		Signature:        "sig",
		Nonce:            uuid.New().String(),
		CreatedAt:        time.Now(),
	}
	_, err = db.SaveUnconfirmedMintExpansion(testCtx, expansion)
	assert.NilError(t, err)

	// Build and save the matching on-chain transaction
	onchainTx := saveExpansionOnChainTx(t, db, mintHash, expansionHash, additionalSupply)

	// Perform the match
	err = db.MatchUnconfirmedMintExpansion(testCtx, onchainTx)
	assert.NilError(t, err)

	// current_supply on the mint should now be fraction_count + additional_supply
	confirmedMint, err := db.GetMintByHash(testCtx, mintHash)
	assert.NilError(t, err)
	assert.Equal(t, confirmedMint.CurrentSupply, fractionCount+additionalSupply)

	// token_balances for the owner should reflect the additional supply
	balances, err := db.GetTokenBalances(testCtx, ownerAddress, mintHash)
	assert.NilError(t, err)
	assert.Assert(t, len(balances) > 0)
	var ownerBalance int
	for _, b := range balances {
		ownerBalance += b.Quantity
	}
	assert.Equal(t, ownerBalance, fractionCount+additionalSupply)

	// unconfirmed expansion should be deleted
	// (we verify indirectly — a second match attempt should fail to find the record)
	err = db.MatchUnconfirmedMintExpansion(testCtx, onchainTx)
	assert.Assert(t, err != nil, "second match should fail — expansion already consumed")

	// on-chain transaction should be deleted
	transactions, err := db.GetOnChainTransactions(testCtx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, len(transactions), 0)
}

func TestMatchUnconfirmedMintExpansion_NotFound(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	unknownExpansionHash := support.GenerateRandomHash()

	onchainTx := saveExpansionOnChainTx(t, db, mintHash, unknownExpansionHash, 100)

	err := db.MatchUnconfirmedMintExpansion(testCtx, onchainTx)
	assert.Assert(t, err != nil, "should fail when no matching unconfirmed expansion exists")
}

func TestMatchUnconfirmedMintExpansion_MintHashMismatch(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	differentMintHash := support.GenerateRandomHash()
	expansionHash := support.GenerateRandomHash()

	// Store the expansion against mintHash
	expansion := &store.UnconfirmedMintExpansion{
		Hash:             expansionHash,
		MintHash:         mintHash,
		AdditionalSupply: 100,
		OwnerAddress:     "nOwner",
		PublicKey:        "pubKey",
		Signature:        "sig",
		Nonce:            uuid.New().String(),
		CreatedAt:        time.Now(),
	}
	_, err := db.SaveUnconfirmedMintExpansion(testCtx, expansion)
	assert.NilError(t, err)

	// On-chain message references a different mint_hash
	onchainTx := saveExpansionOnChainTx(t, db, differentMintHash, expansionHash, 100)

	err = db.MatchUnconfirmedMintExpansion(testCtx, onchainTx)
	assert.Assert(t, err != nil, "should fail when mint_hash in on-chain message does not match stored record")
}

func TestMatchUnconfirmedMintExpansion_SupplyMismatch(t *testing.T) {
	db := support.SetupTestDB(t)

	mintHash := support.GenerateRandomHash()
	expansionHash := support.GenerateRandomHash()

	expansion := &store.UnconfirmedMintExpansion{
		Hash:             expansionHash,
		MintHash:         mintHash,
		AdditionalSupply: 100,
		OwnerAddress:     "nOwner",
		PublicKey:        "pubKey",
		Signature:        "sig",
		Nonce:            uuid.New().String(),
		CreatedAt:        time.Now(),
	}
	_, err := db.SaveUnconfirmedMintExpansion(testCtx, expansion)
	assert.NilError(t, err)

	// On-chain message claims a different additional_supply
	onchainTx := saveExpansionOnChainTx(t, db, mintHash, expansionHash, 999)

	err = db.MatchUnconfirmedMintExpansion(testCtx, onchainTx)
	assert.Assert(t, err != nil, "should fail when additional_supply in on-chain message does not match stored record")
}

func TestCurrentSupplyInitialisedOnMintConfirmation(t *testing.T) {
	db := support.SetupTestDB(t)

	const fractionCount = 750

	unconfirmedMint := &store.MintWithoutID{
		Hash:          "currentSupplyTestHash",
		Title:         "Current Supply Test",
		FractionCount: fractionCount,
		Description:   "Checks current_supply is set on confirmation",
		Tags:          store.StringArray{},
		Metadata:      store.StringInterfaceMap{},
		Requirements:  store.StringInterfaceMap{},
		LockupOptions: store.StringInterfaceMap{},
		PublicKey:     "pubKey",
	}
	_, err := db.SaveUnconfirmedMint(testCtx, unconfirmedMint)
	assert.NilError(t, err)

	onchainMsg := &protocol.OnChainMintMessage{Hash: "currentSupplyTestHash"}
	actionData, err := proto.Marshal(onchainMsg)
	assert.NilError(t, err)

	txId, err := db.SaveOnChainTransaction(testCtx, "csTxHash", 4000, "blockHash", 1, protocol.ACTION_MINT, 1, actionData, "csOwner", map[string]interface{}{})
	assert.NilError(t, err)

	onchainTx := store.OnChainTransaction{
		Id:            txId,
		TxHash:        "csTxHash",
		Height:        4000,
		ActionType:    protocol.ACTION_MINT,
		ActionVersion: 1,
		ActionData:    actionData,
		Address:       "csOwner",
	}

	err = db.MatchUnconfirmedMint(testCtx, onchainTx)
	assert.NilError(t, err)

	confirmed, err := db.GetMintByHash(testCtx, "currentSupplyTestHash")
	assert.NilError(t, err)
	assert.Equal(t, confirmed.CurrentSupply, fractionCount)
}

func TestAllowExpansionDefaultsFalse(t *testing.T) {
	db := support.SetupTestDB(t)

	mint := &store.MintWithoutID{
		Hash:          "allowExpDefaultHash",
		Title:         "Allow Expansion Default",
		FractionCount: 100,
		Description:   "Checks allow_expansion defaults to false",
		Tags:          store.StringArray{},
		Metadata:      store.StringInterfaceMap{},
		Requirements:  store.StringInterfaceMap{},
		LockupOptions: store.StringInterfaceMap{},
		PublicKey:     "pubKey",
	}
	_, err := db.SaveMint(testCtx, mint, "owner")
	assert.NilError(t, err)

	retrieved, err := db.GetMintByHash(testCtx, "allowExpDefaultHash")
	assert.NilError(t, err)
	assert.Equal(t, retrieved.AllowExpansion, false)
}
