package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"
)

func (s *TokenisationStore) UpsertTokenBalance(ctx context.Context, address, mintHash string, quantity int) error {
	log.Println("Upserting token balance:", address, mintHash, quantity)

	_, err := s.DB.Exec(`
	INSERT INTO token_balances (address, mint_hash, quantity, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5)
	`, address, mintHash, quantity, time.Now(), time.Now())

	if err != nil {
		return err
	}

	return nil
}

func (s *TokenisationStore) UpsertPendingTokenBalance(ctx context.Context, invoiceHash, mintHash string, quantity int, onchainTransactionId string, ownerAddress string) error {
	return s.UpsertPendingTokenBalanceWithTx(ctx, invoiceHash, mintHash, quantity, onchainTransactionId, ownerAddress, nil)
}

func (s *TokenisationStore) UpsertPendingTokenBalanceWithTx(ctx context.Context, invoiceHash, mintHash string, quantity int, onchainTransactionId string, ownerAddress string, tx *sql.Tx) error {
	log.Println("Upserting pending token balance:", invoiceHash, mintHash, quantity, onchainTransactionId, ownerAddress)

	query := `
	INSERT INTO pending_token_balances (invoice_hash, mint_hash, quantity, onchain_transaction_id, created_at, owner_address)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (invoice_hash, mint_hash)
	DO UPDATE SET quantity = $3
	`

	var err error
	if tx != nil {
		_, err = tx.Exec(query, invoiceHash, mintHash, quantity, onchainTransactionId, time.Now(), ownerAddress)
	} else {
		_, err = s.DB.Exec(query, invoiceHash, mintHash, quantity, onchainTransactionId, time.Now(), ownerAddress)
	}

	return err
}

func (s *TokenisationStore) HasPendingTokenBalance(ctx context.Context, invoiceHash, mintHash string, onChainTransactionId string) (bool, error) {
	rows, err := s.DB.Query(`
		SELECT COUNT(*) FROM pending_token_balances WHERE invoice_hash = $1 AND mint_hash = $2 AND onchain_transaction_id = $3
	`, invoiceHash, mintHash, onChainTransactionId)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	if !rows.Next() {
		return false, nil
	}

	var count int
	err = rows.Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (s *TokenisationStore) RemovePendingTokenBalance(ctx context.Context, invoiceHash, mintHash string) error {
	_, err := s.DB.Exec(`
		DELETE FROM pending_token_balances WHERE invoice_hash = $1 AND mint_hash = $2
	`, invoiceHash, mintHash)
	return err
}

func (s *TokenisationStore) GetPendingTokenBalance(ctx context.Context, invoiceHash, mintHash string, tx *sql.Tx) (PendingTokenBalance, error) {
	var rows *sql.Rows
	var err error

	if tx == nil {
		rows, err = s.DB.Query(`
		SELECT quantity, invoice_hash, mint_hash, owner_address FROM pending_token_balances WHERE invoice_hash = $1 AND mint_hash = $2
	`, invoiceHash, mintHash)
	} else {
		rows, err = tx.Query(`
			SELECT quantity, invoice_hash, mint_hash, owner_address FROM pending_token_balances WHERE invoice_hash = $1 AND mint_hash = $2
		`, invoiceHash, mintHash)
	}

	defer rows.Close()

	if err != nil {
		return PendingTokenBalance{}, err
	}

	if rows.Next() {
		var pendingTokenBalance PendingTokenBalance
		err := rows.Scan(&pendingTokenBalance.Quantity, &pendingTokenBalance.InvoiceHash, &pendingTokenBalance.MintHash, &pendingTokenBalance.OwnerAddress)
		if err != nil {
			return PendingTokenBalance{}, err
		}

		return pendingTokenBalance, nil
	}

	return PendingTokenBalance{}, errors.New("no pending token balance found")
}

func (s *TokenisationStore) GetPendingTokenBalanceForQuantity(ctx context.Context, invoiceHash, mintHash string, quantity int, tx *sql.Tx) (PendingTokenBalance, error) {
	var rows *sql.Rows
	var err error

	fmt.Println("Getting token balance", invoiceHash, mintHash, quantity)

	if tx == nil {
		rows, err = s.DB.Query(`
		SELECT quantity, invoice_hash, mint_hash, owner_address FROM pending_token_balances WHERE invoice_hash = $1 AND mint_hash = $2 and quantity = $3
	`, invoiceHash, mintHash, quantity)
	} else {
		rows, err = tx.Query(`
			SELECT quantity, invoice_hash, mint_hash, owner_address FROM pending_token_balances WHERE invoice_hash = $1 AND mint_hash = $2 and quantity = $3
		`, invoiceHash, mintHash, quantity)
	}

	defer rows.Close()

	if err != nil {
		return PendingTokenBalance{}, err
	}

	if rows.Next() {
		var pendingTokenBalance PendingTokenBalance
		err := rows.Scan(&pendingTokenBalance.Quantity, &pendingTokenBalance.InvoiceHash, &pendingTokenBalance.MintHash, &pendingTokenBalance.OwnerAddress)
		if err != nil {
			return PendingTokenBalance{}, err
		}

		return pendingTokenBalance, nil
	}

	return PendingTokenBalance{}, errors.New("no pending token balance found")
}

func (s *TokenisationStore) GetPendingTokenBalanceTotalForMintAndOwner(ctx context.Context, mintHash string, ownerAddress string) (int, error) {
	rows, err := s.DB.Query(`
		SELECT COALESCE(SUM(quantity), 0) FROM pending_token_balances WHERE mint_hash = $1 AND owner_address = $2
	`, mintHash, ownerAddress)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	if rows.Next() {
		var quantity int
		err := rows.Scan(&quantity)
		if err != nil {
			return 0, err
		}
		return quantity, nil
	}

	return 0, nil
}

func (s *TokenisationStore) GetMyMintTokenBalances(ctx context.Context, address string, offset int, limit int) ([]TokenBalanceWithMint, error) {
	rows, err := s.DB.Query(`
		SELECT
  m.id,
  m.created_at,
  m.title,
  m.description,
  m.fraction_count,
  m.tags,
  m.metadata,
  m.hash,
  m.transaction_hash,
  m.requirements,
  m.lockup_options,
  m.feed_url,
  m.owner_address,
  m.public_key,
  m.contract_of_sale,
  COALESCE(tb.balance_quantity, 0) AS balance_quantity,
  tb.address AS token_owner_address
FROM mints m
INNER JOIN (
  SELECT
    mint_hash,
    address,
    SUM(quantity) AS balance_quantity
  FROM token_balances
  WHERE address = $1
  GROUP BY mint_hash, address
) tb
  ON m.hash = tb.mint_hash
LIMIT $2 OFFSET $3
	`, address, limit, offset)

	if err != nil {
		return []TokenBalanceWithMint{}, err
	}

	defer rows.Close()

	tokenBalances := []TokenBalanceWithMint{}

	for rows.Next() {
		var mint TokenBalanceWithMint

		err := rows.Scan(
			&mint.Id,
			&mint.CreatedAt,
			&mint.Title,
			&mint.Description,
			&mint.FractionCount,
			&mint.Tags,
			&mint.Metadata,
			&mint.Hash,
			&mint.TransactionHash,
			&mint.Requirements,
			&mint.LockupOptions,
			&mint.FeedURL,
			&mint.OwnerAddress,
			&mint.PublicKey,
			&mint.ContractOfSale,
			&mint.Quantity,
			&mint.Address,
		)
		if err != nil {
			return []TokenBalanceWithMint{}, err
		}

		tokenBalances = append(tokenBalances, mint)
	}

	return tokenBalances, nil
}

func (s *TokenisationStore) GetTokenBalances(ctx context.Context, address string, mintHash string) ([]TokenBalance, error) {
	rows, err := s.DB.Query(`
		SELECT quantity FROM token_balances WHERE address = $1 AND mint_hash = $2
	`, address, mintHash)

	if err != nil {
		return []TokenBalance{}, err
	}

	defer rows.Close()

	tokenBalances := []TokenBalance{}

	for rows.Next() {

		var quantity int
		err := rows.Scan(&quantity)
		if err != nil {
			return []TokenBalance{}, err
		}
		tokenBalances = append(tokenBalances, TokenBalance{
			Address:  address,
			MintHash: mintHash,
			Quantity: quantity,
		})
	}

	return tokenBalances, nil
}

func (s *TokenisationStore) GetPendingTokenBalances(ctx context.Context, address string, mintHash string) ([]TokenBalance, error) {
	log.Println("Getting token balance: ADDRESS", address, "MINT HASH", mintHash)

	rows, err := s.DB.Query(`
		SELECT quantity FROM pending_token_balances WHERE owner_address = $1 AND mint_hash = $2
	`, address, mintHash)

	if err != nil {
		return []TokenBalance{}, err
	}

	defer rows.Close()

	tokenBalances := []TokenBalance{}

	for rows.Next() {

		var quantity int
		err := rows.Scan(&quantity)
		if err != nil {
			return []TokenBalance{}, err
		}
		tokenBalances = append(tokenBalances, TokenBalance{
			Address:  address,
			MintHash: mintHash,
			Quantity: quantity,
		})
	}

	return tokenBalances, nil
}

func (s *TokenisationStore) UpsertTokenBalanceWithTransaction(ctx context.Context, address, mintHash string, quantity int, tx *sql.Tx) error {
	log.Println("Upserting token balance with transaction:", address, mintHash, quantity)

	_, err := tx.Exec(`
	INSERT INTO token_balances (address, mint_hash, quantity, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5)
	`, address, mintHash, quantity, time.Now(), time.Now())

	return err
}

func (s *TokenisationStore) MovePendingToTokenBalance(ctx context.Context, pendingTokenBalance PendingTokenBalance, buyerAddress string, tx *sql.Tx) error {
	_, err := tx.Exec(`
	INSERT INTO token_balances (address, mint_hash, quantity, created_at, updated_at)
	VALUES ($1, $2, $3, $4, $5)
	`, buyerAddress, pendingTokenBalance.MintHash, pendingTokenBalance.Quantity, time.Now(), time.Now())

	if err != nil {
		return err
	}

	err = s.UpsertTokenBalanceWithTransaction(ctx, pendingTokenBalance.OwnerAddress, pendingTokenBalance.MintHash, -pendingTokenBalance.Quantity, tx)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
	DELETE FROM pending_token_balances WHERE invoice_hash = $1 AND mint_hash = $2
	`, pendingTokenBalance.InvoiceHash, pendingTokenBalance.MintHash)
	if err != nil {
		return err
	}

	return err
}
