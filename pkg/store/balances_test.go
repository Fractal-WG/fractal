package store_test

import (
	"context"
	"testing"

	"dogecoin.org/fractal-engine/internal/test/support"
	"gotest.tools/assert"
)

var balancesTestCtx = context.Background()

func TestUpsertTokenBalance(t *testing.T) {
	db := support.SetupTestDB()

	// Test inserting a new token balance
	err := db.UpsertTokenBalance(balancesTestCtx, "address1", "mintHash1", 100)
	assert.NilError(t, err)

	// Verify the balance was created
	balances, err := db.GetTokenBalances(balancesTestCtx, "address1", "mintHash1")
	assert.NilError(t, err)
	assert.Equal(t, len(balances), 1)
	assert.Equal(t, balances[0].Address, "address1")
	assert.Equal(t, balances[0].MintHash, "mintHash1")
	assert.Equal(t, balances[0].Quantity, 100)

	// Test inserting another balance for same address/mint (should create another row)
	err = db.UpsertTokenBalance(balancesTestCtx, "address1", "mintHash1", 50)
	assert.NilError(t, err)

	// Verify both balances exist
	balances, err = db.GetTokenBalances(balancesTestCtx, "address1", "mintHash1")
	assert.NilError(t, err)
	assert.Equal(t, len(balances), 2)
}

func TestGetTokenBalances(t *testing.T) {
	db := support.SetupTestDB()

	// Insert multiple balances
	err := db.UpsertTokenBalance(balancesTestCtx, "address1", "mintHash1", 100)
	assert.NilError(t, err)
	err = db.UpsertTokenBalance(balancesTestCtx, "address1", "mintHash1", 50)
	assert.NilError(t, err)
	err = db.UpsertTokenBalance(balancesTestCtx, "address1", "mintHash2", 200)
	assert.NilError(t, err)
	err = db.UpsertTokenBalance(balancesTestCtx, "address2", "mintHash1", 300)
	assert.NilError(t, err)

	// Test getting balances for specific address and mint
	balances, err := db.GetTokenBalances(balancesTestCtx, "address1", "mintHash1")
	assert.NilError(t, err)
	assert.Equal(t, len(balances), 2)
	totalQuantity := 0
	for _, b := range balances {
		totalQuantity += b.Quantity
	}
	assert.Equal(t, totalQuantity, 150)

	// Test getting balances for different mint
	balances, err = db.GetTokenBalances(balancesTestCtx, "address1", "mintHash2")
	assert.NilError(t, err)
	assert.Equal(t, len(balances), 1)
	assert.Equal(t, balances[0].Quantity, 200)

	// Test getting balances for non-existent combination
	balances, err = db.GetTokenBalances(balancesTestCtx, "address3", "mintHash3")
	assert.NilError(t, err)
	assert.Equal(t, len(balances), 0)
}

func TestUpsertPendingTokenBalance(t *testing.T) {
	db := support.SetupTestDB()

	// Test inserting a new pending token balance
	err := db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 100, "onchainTx1", "owner1")
	assert.NilError(t, err)

	// Verify the pending balance exists
	exists, err := db.HasPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", "onchainTx1")
	assert.NilError(t, err)
	assert.Assert(t, exists)

	// Test upserting with same invoice/mint (should update quantity)
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 200, "onchainTx1", "owner1")
	assert.NilError(t, err)

	// Verify the quantity was updated
	pendingBalance, err := db.GetPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", nil)
	assert.NilError(t, err)
	assert.Equal(t, pendingBalance.Quantity, 200)
}

func TestHasPendingTokenBalance(t *testing.T) {
	db := support.SetupTestDB()

	// Test non-existent pending balance
	exists, err := db.HasPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", "onchainTx1")
	assert.NilError(t, err)
	assert.Assert(t, !exists)

	// Insert a pending balance
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 100, "onchainTx1", "owner1")
	assert.NilError(t, err)

	// Test existing pending balance
	exists, err = db.HasPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", "onchainTx1")
	assert.NilError(t, err)
	assert.Assert(t, exists)

	// Test with different onchain transaction ID
	exists, err = db.HasPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", "differentTx")
	assert.NilError(t, err)
	assert.Assert(t, !exists)
}

func TestGetPendingTokenBalance(t *testing.T) {
	db := support.SetupTestDB()

	// Test getting non-existent pending balance
	_, err := db.GetPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", nil)
	assert.ErrorContains(t, err, "no pending token balance found")

	// Insert a pending balance
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 150, "onchainTx1", "owner1")
	assert.NilError(t, err)

	// Get the pending balance
	pendingBalance, err := db.GetPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", nil)
	assert.NilError(t, err)
	assert.Equal(t, pendingBalance.InvoiceHash, "invoice1")
	assert.Equal(t, pendingBalance.MintHash, "mintHash1")
	assert.Equal(t, pendingBalance.Quantity, 150)
	assert.Equal(t, pendingBalance.OwnerAddress, "owner1")
}

func TestRemovePendingTokenBalance(t *testing.T) {
	db := support.SetupTestDB()

	// Insert a pending balance
	err := db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 100, "onchainTx1", "owner1")
	assert.NilError(t, err)

	// Verify it exists
	exists, err := db.HasPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", "onchainTx1")
	assert.NilError(t, err)
	assert.Assert(t, exists)

	// Remove the pending balance
	err = db.RemovePendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1")
	assert.NilError(t, err)

	// Verify it no longer exists
	exists, err = db.HasPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", "onchainTx1")
	assert.NilError(t, err)
	assert.Assert(t, !exists)

	// Test removing non-existent balance (should not error)
	err = db.RemovePendingTokenBalance(balancesTestCtx, "nonExistent", "nonExistent")
	assert.NilError(t, err)
}

func TestGetPendingTokenBalanceTotalForMintAndOwner(t *testing.T) {
	db := support.SetupTestDB()

	// Test with no pending balances
	total, err := db.GetPendingTokenBalanceTotalForMintAndOwner(balancesTestCtx, "mintHash1", "owner1")
	assert.NilError(t, err)
	assert.Equal(t, total, 0)

	// Insert multiple pending balances for same mint and owner
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 100, "onchainTx1", "owner1")
	assert.NilError(t, err)
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice2", "mintHash1", 150, "onchainTx2", "owner1")
	assert.NilError(t, err)
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice3", "mintHash1", 50, "onchainTx3", "owner1")
	assert.NilError(t, err)

	// Insert pending balance for different owner
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice4", "mintHash1", 200, "onchainTx4", "owner2")
	assert.NilError(t, err)

	// Insert pending balance for different mint
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice5", "mintHash2", 300, "onchainTx5", "owner1")
	assert.NilError(t, err)

	// Test getting total for mint1 and owner1
	total, err = db.GetPendingTokenBalanceTotalForMintAndOwner(balancesTestCtx, "mintHash1", "owner1")
	assert.NilError(t, err)
	assert.Equal(t, total, 300) // 100 + 150 + 50

	// Test getting total for mint1 and owner2
	total, err = db.GetPendingTokenBalanceTotalForMintAndOwner(balancesTestCtx, "mintHash1", "owner2")
	assert.NilError(t, err)
	assert.Equal(t, total, 200)

	// Test getting total for mint2 and owner1
	total, err = db.GetPendingTokenBalanceTotalForMintAndOwner(balancesTestCtx, "mintHash2", "owner1")
	assert.NilError(t, err)
	assert.Equal(t, total, 300)
}

func TestUpsertTokenBalanceWithTransaction(t *testing.T) {
	db := support.SetupTestDB()

	// Begin a transaction
	tx, err := db.DB.Begin()
	assert.NilError(t, err)
	defer tx.Rollback()

	// Insert a token balance within transaction
	err = db.UpsertTokenBalanceWithTransaction(balancesTestCtx, "address1", "mintHash1", 100, tx)
	assert.NilError(t, err)

	// Commit the transaction
	err = tx.Commit()
	assert.NilError(t, err)

	// Verify the balance was created
	balances, err := db.GetTokenBalances(balancesTestCtx, "address1", "mintHash1")
	assert.NilError(t, err)
	assert.Equal(t, len(balances), 1)
	assert.Equal(t, balances[0].Quantity, 100)
}

func TestMovePendingToTokenBalance(t *testing.T) {
	db := support.SetupTestDB()

	// Setup: Create initial token balance for the owner
	err := db.UpsertTokenBalance(balancesTestCtx, "owner1", "mintHash1", 500)
	assert.NilError(t, err)

	// Create a pending token balance
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 100, "onchainTx1", "owner1")
	assert.NilError(t, err)

	// Get the pending balance
	pendingBalance, err := db.GetPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", nil)
	assert.NilError(t, err)

	// Begin a transaction
	tx, err := db.DB.Begin()
	assert.NilError(t, err)

	// Move pending to token balance
	err = db.MovePendingToTokenBalance(balancesTestCtx, pendingBalance, "buyer1", tx)
	assert.NilError(t, err)

	// Commit the transaction
	err = tx.Commit()
	assert.NilError(t, err)

	// Verify buyer received the tokens
	buyerBalances, err := db.GetTokenBalances(balancesTestCtx, "buyer1", "mintHash1")
	assert.NilError(t, err)
	assert.Equal(t, len(buyerBalances), 1)
	assert.Equal(t, buyerBalances[0].Quantity, 100)

	// Verify owner has a negative balance entry (deduction)
	ownerBalances, err := db.GetTokenBalances(balancesTestCtx, "owner1", "mintHash1")
	assert.NilError(t, err)
	assert.Equal(t, len(ownerBalances), 2) // Original + deduction
	totalOwnerBalance := 0
	for _, b := range ownerBalances {
		totalOwnerBalance += b.Quantity
	}
	assert.Equal(t, totalOwnerBalance, 400) // 500 - 100

	// Verify pending balance was removed
	_, err = db.GetPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", nil)
	assert.ErrorContains(t, err, "no pending token balance found")
}

func TestGetPendingTokenBalanceWithTransaction(t *testing.T) {
	db := support.SetupTestDB()

	// Insert a pending balance
	err := db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 100, "onchainTx1", "owner1")
	assert.NilError(t, err)

	// Begin a transaction
	tx, err := db.DB.Begin()
	assert.NilError(t, err)
	defer tx.Rollback()

	// Get the pending balance within transaction
	pendingBalance, err := db.GetPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", tx)
	assert.NilError(t, err)
	assert.Equal(t, pendingBalance.InvoiceHash, "invoice1")
	assert.Equal(t, pendingBalance.MintHash, "mintHash1")
	assert.Equal(t, pendingBalance.Quantity, 100)
}

func TestMultipleTokenBalancesForSameAddressMint(t *testing.T) {
	db := support.SetupTestDB()

	// Create multiple token balance entries for same address/mint
	// This simulates multiple transactions adding to the balance
	err := db.UpsertTokenBalance(balancesTestCtx, "address1", "mintHash1", 100)
	assert.NilError(t, err)
	err = db.UpsertTokenBalance(balancesTestCtx, "address1", "mintHash1", 200)
	assert.NilError(t, err)
	err = db.UpsertTokenBalance(balancesTestCtx, "address1", "mintHash1", -50) // Deduction
	assert.NilError(t, err)

	// Get all balances
	balances, err := db.GetTokenBalances(balancesTestCtx, "address1", "mintHash1")
	assert.NilError(t, err)
	assert.Equal(t, len(balances), 3)

	// Calculate total balance
	totalBalance := 0
	for _, b := range balances {
		totalBalance += b.Quantity
	}
	assert.Equal(t, totalBalance, 250) // 100 + 200 - 50
}

func TestPendingTokenBalanceConflictResolution(t *testing.T) {
	db := support.SetupTestDB()

	// Insert initial pending balance
	err := db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 100, "onchainTx1", "owner1")
	assert.NilError(t, err)

	// Verify initial quantity
	pendingBalance, err := db.GetPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", nil)
	assert.NilError(t, err)
	assert.Equal(t, pendingBalance.Quantity, 100)

	// Update with new quantity (ON CONFLICT should update)
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 250, "onchainTx2", "owner1")
	assert.NilError(t, err)

	// Verify quantity was updated
	pendingBalance, err = db.GetPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", nil)
	assert.NilError(t, err)
	assert.Equal(t, pendingBalance.Quantity, 250)

	// The owner address should remain the same
	assert.Equal(t, pendingBalance.OwnerAddress, "owner1")
}

func TestTransactionRollback(t *testing.T) {
	db := support.SetupTestDB()

	// Create initial state
	err := db.UpsertTokenBalance(balancesTestCtx, "owner1", "mintHash1", 500)
	assert.NilError(t, err)
	err = db.UpsertPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", 100, "onchainTx1", "owner1")
	assert.NilError(t, err)

	// Get pending balance
	pendingBalance, err := db.GetPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", nil)
	assert.NilError(t, err)

	// Begin transaction
	tx, err := db.DB.Begin()
	assert.NilError(t, err)

	// Perform operations within transaction
	err = db.MovePendingToTokenBalance(balancesTestCtx, pendingBalance, "buyer1", tx)
	assert.NilError(t, err)

	// Rollback the transaction
	err = tx.Rollback()
	assert.NilError(t, err)

	// Verify nothing changed - pending balance still exists
	pendingBalance, err = db.GetPendingTokenBalance(balancesTestCtx, "invoice1", "mintHash1", nil)
	assert.NilError(t, err)
	assert.Equal(t, pendingBalance.Quantity, 100)

	// Verify buyer has no balance
	buyerBalances, err := db.GetTokenBalances(balancesTestCtx, "buyer1", "mintHash1")
	assert.NilError(t, err)
	assert.Equal(t, len(buyerBalances), 0)

	// Verify owner balance unchanged
	ownerBalances, err := db.GetTokenBalances(balancesTestCtx, "owner1", "mintHash1")
	assert.NilError(t, err)
	assert.Equal(t, len(ownerBalances), 1)
	assert.Equal(t, ownerBalances[0].Quantity, 500)
}
