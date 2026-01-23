package store

import (
	"context"
	"database/sql"
)

func (s *TokenisationStore) GetChainPosition(ctx context.Context) (int64, string, bool, error) {
	var blockHeight int64
	var blockHash string
	var waitingForNextHash bool

	err := s.DB.QueryRow("SELECT block_height, block_hash, waiting_for_next_hash FROM chain_position").Scan(&blockHeight, &blockHash, &waitingForNextHash)
	if err == sql.ErrNoRows {
		return 0, "", false, nil
	}

	if err != nil {
		return 0, "", false, err
	}

	return blockHeight, blockHash, waitingForNextHash, nil
}

func (s *TokenisationStore) UpsertChainPosition(ctx context.Context, blockHeight int64, blockHash string, waitingForNextHash bool) error {

	_, err := s.DB.Exec(`
	INSERT INTO chain_position (id, block_height, block_hash, waiting_for_next_hash)
	VALUES (1, $1, $2, $3)
	ON CONFLICT (id)
	DO UPDATE SET block_height = EXCLUDED.block_height,
				  block_hash = EXCLUDED.block_hash,
				  waiting_for_next_hash = EXCLUDED.waiting_for_next_hash
`, blockHeight, blockHash, waitingForNextHash)

	return err
}
