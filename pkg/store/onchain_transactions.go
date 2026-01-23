package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

func getOnChainTransactionsCount(ctx context.Context, s *TokenisationStore) (int, error) {
	if s.backend == "postgres" {
		return approximateTableCountPostgres(ctx, s.DB, "onchain_transactions")
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT COUNT(*) FROM onchain_transactions")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	if !rows.Next() {
		return 0, nil
	}

	var count int
	err = rows.Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *TokenisationStore) SaveOnChainTransaction(ctx context.Context, tx_hash string, height int64, blockHash string, transaction_number int, action_type uint8, action_version uint8, action_data []byte, address string, values StringInterfaceMap) (string, error) {
	id := uuid.New().String()

	jsonValues, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	_, err = s.DB.ExecContext(ctx, `
	INSERT INTO onchain_transactions (id, tx_hash, block_height, block_hash, transaction_number, action_type, action_version, action_data, address, "values")
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, id, tx_hash, height, blockHash, transaction_number, action_type, action_version, action_data, address, jsonValues)
	return id, err
}

func (s *TokenisationStore) GetOldOnchainTransactions(ctx context.Context, blockHeight int) ([]OnChainTransaction, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, tx_hash, block_height, block_hash, transaction_number, action_type, action_version, action_data, address, "values" FROM onchain_transactions WHERE block_height < $1`, blockHeight)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []OnChainTransaction
	for rows.Next() {
		var transaction OnChainTransaction
		if err := rows.Scan(&transaction.Id, &transaction.TxHash, &transaction.Height, &transaction.BlockHash, &transaction.TransactionNumber, &transaction.ActionType, &transaction.ActionVersion, &transaction.ActionData, &transaction.Address, &transaction.Values); err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return transactions, nil
}

func (s *TokenisationStore) TrimOldOnChainTransactions(ctx context.Context, blockHeightToKeep int) error {
	sqlQuery := fmt.Sprintf("DELETE FROM onchain_transactions WHERE block_height < %d", blockHeightToKeep)
	_, err := s.DB.ExecContext(ctx, sqlQuery)
	if err != nil {
		return err
	}
	return nil
}

func (s *TokenisationStore) RemoveOnChainTransaction(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, "DELETE FROM onchain_transactions WHERE id = $1", id)
	if err != nil {
		return err
	}
	return nil
}

func (s *TokenisationStore) CountOnChainTransactions(ctx context.Context, blockHeight int64) (int, error) {
	var count int
	err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM onchain_transactions WHERE block_height = $1", blockHeight).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (s *TokenisationStore) GetOnChainTransactions(ctx context.Context, offset int, limit int) ([]OnChainTransaction, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, tx_hash, block_height, block_hash, transaction_number, action_type, action_version, action_data, address, "values" FROM onchain_transactions ORDER BY block_height ASC, transaction_number ASC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []OnChainTransaction
	for rows.Next() {
		var transaction OnChainTransaction
		if err := rows.Scan(&transaction.Id, &transaction.TxHash, &transaction.Height, &transaction.BlockHash, &transaction.TransactionNumber, &transaction.ActionType, &transaction.ActionVersion, &transaction.ActionData, &transaction.Address, &transaction.Values); err != nil {
			return nil, err
		}
		transactions = append(transactions, transaction)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return transactions, nil
}
