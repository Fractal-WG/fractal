package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"dogecoin.org/fractal-engine/pkg/protocol"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func (s *TokenisationStore) GetMintByHash(ctx context.Context, hash string) (Mint, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, created_at, title, description, fraction_count, tags, metadata, hash, transaction_hash, requirements, lockup_options, feed_url, owner_address, public_key, contract_of_sale, signature_requirement_type, asset_managers, min_signatures FROM mints WHERE hash = $1", hash)
	if err != nil {
		return Mint{}, err
	}

	var m Mint
	if rows.Next() {
		if err := rows.Scan(&m.Id, &m.CreatedAt, &m.Title, &m.Description, &m.FractionCount, &m.Tags, &m.Metadata, &m.Hash, &m.TransactionHash, &m.Requirements, &m.LockupOptions, &m.FeedURL, &m.OwnerAddress, &m.PublicKey, &m.ContractOfSale, &m.SignatureRequirementType, &m.AssetManagers, &m.MinSignatures); err != nil {
			return Mint{}, err
		}
	}

	rows.Close()

	return m, nil
}

func (s *TokenisationStore) GetMintsByPublicKey(ctx context.Context, offset int, limit int, publicKey string, includeUnconfirmed bool) ([]Mint, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, created_at, title, description, fraction_count, tags, metadata, hash, transaction_hash, requirements, lockup_options, feed_url, owner_address, public_key, contract_of_sale, signature_requirement_type, asset_managers, min_signatures FROM mints WHERE public_key = $1 and transaction_hash is not null LIMIT $2 OFFSET $3", publicKey, limit, offset)
	if err != nil {
		return nil, err
	}

	var mints []Mint
	for rows.Next() {
		var m Mint
		if err := rows.Scan(&m.Id, &m.CreatedAt, &m.Title, &m.Description, &m.FractionCount, &m.Tags, &m.Metadata, &m.Hash, &m.TransactionHash, &m.Requirements, &m.LockupOptions, &m.FeedURL, &m.OwnerAddress, &m.PublicKey, &m.ContractOfSale, &m.SignatureRequirementType, &m.AssetManagers, &m.MinSignatures); err != nil {
			return nil, err
		}
		mints = append(mints, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows.Close()

	if includeUnconfirmed {
		rows, err = s.DB.QueryContext(ctx, "SELECT id, created_at, title, description, fraction_count, tags, metadata, hash, transaction_hash, requirements, lockup_options, feed_url, owner_address, public_key, contract_of_sale, signature_requirement_type, asset_managers, min_signatures FROM unconfirmed_mints WHERE public_key = $1 LIMIT $2 OFFSET $3", publicKey, limit, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var m Mint
			if err := rows.Scan(&m.Id, &m.CreatedAt, &m.Title, &m.Description, &m.FractionCount, &m.Tags, &m.Metadata, &m.Hash, &m.TransactionHash, &m.Requirements, &m.LockupOptions, &m.FeedURL, &m.OwnerAddress, &m.PublicKey, &m.ContractOfSale, &m.SignatureRequirementType, &m.AssetManagers, &m.MinSignatures); err != nil {
				return nil, err
			}
			mints = append(mints, m)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		rows.Close()
	}

	return mints, nil
}

func (s *TokenisationStore) GetMintsByAddress(ctx context.Context, offset int, limit int, address string, includeUnconfirmed bool) ([]Mint, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, created_at, title, description, fraction_count, tags, metadata, hash, transaction_hash, requirements, lockup_options, feed_url, owner_address, public_key, contract_of_sale, signature_requirement_type, asset_managers, min_signatures FROM mints WHERE owner_address = $1 LIMIT $2 OFFSET $3", address, limit, offset)
	if err != nil {
		return nil, err
	}

	var mints []Mint
	for rows.Next() {
		var m Mint
		if err := rows.Scan(&m.Id, &m.CreatedAt, &m.Title, &m.Description, &m.FractionCount, &m.Tags, &m.Metadata, &m.Hash, &m.TransactionHash, &m.Requirements, &m.LockupOptions, &m.FeedURL, &m.OwnerAddress, &m.PublicKey, &m.ContractOfSale, &m.SignatureRequirementType, &m.AssetManagers, &m.MinSignatures); err != nil {
			return nil, err
		}
		mints = append(mints, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows.Close()

	if includeUnconfirmed {
		rows, err = s.DB.QueryContext(ctx, "SELECT id, created_at, title, description, fraction_count, tags, metadata, hash, transaction_hash, requirements, lockup_options, feed_url, owner_address, public_key, contract_of_sale, signature_requirement_type, asset_managers, min_signatures FROM unconfirmed_mints WHERE owner_address = $1 LIMIT $2 OFFSET $3", address, limit, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var m Mint
			if err := rows.Scan(&m.Id, &m.CreatedAt, &m.Title, &m.Description, &m.FractionCount, &m.Tags, &m.Metadata, &m.Hash, &m.TransactionHash, &m.Requirements, &m.LockupOptions, &m.FeedURL, &m.OwnerAddress, &m.PublicKey, &m.ContractOfSale, &m.SignatureRequirementType, &m.AssetManagers, &m.MinSignatures); err != nil {
				return nil, err
			}
			mints = append(mints, m)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		rows.Close()
	}

	return mints, nil
}

func (s *TokenisationStore) ChooseMint(ctx context.Context) (Mint, error) {
	row := s.DB.QueryRowContext(ctx, "SELECT id, created_at, title, description, fraction_count, tags, metadata, hash, transaction_hash, requirements, lockup_options, feed_url, owner_address, public_key, contract_of_sale, signature_requirement_type, asset_managers, min_signatures FROM mints WHERE hash IN (SELECT hash FROM mints ORDER BY RANDOM() LIMIT 1)")
	var m Mint
	if err := row.Scan(&m.Id, &m.CreatedAt, &m.Title, &m.Description, &m.FractionCount, &m.Tags, &m.Metadata, &m.Hash, &m.TransactionHash, &m.Requirements, &m.LockupOptions, &m.FeedURL, &m.OwnerAddress, &m.PublicKey, &m.ContractOfSale, &m.SignatureRequirementType, &m.AssetManagers, &m.MinSignatures); err != nil {
		return Mint{}, err
	}
	return m, nil
}

func (s *TokenisationStore) GetMints(ctx context.Context, offset int, limit int) ([]Mint, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, created_at, title, description, fraction_count, tags, metadata, hash, transaction_hash, requirements, lockup_options, feed_url, owner_address, public_key, contract_of_sale, signature_requirement_type, asset_managers, min_signatures FROM mints LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mints []Mint
	for rows.Next() {
		var m Mint
		if err := rows.Scan(&m.Id, &m.CreatedAt, &m.Title, &m.Description, &m.FractionCount, &m.Tags, &m.Metadata, &m.Hash, &m.TransactionHash, &m.Requirements, &m.LockupOptions, &m.FeedURL, &m.OwnerAddress, &m.PublicKey, &m.ContractOfSale, &m.SignatureRequirementType, &m.AssetManagers, &m.MinSignatures); err != nil {
			return nil, err
		}
		mints = append(mints, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return mints, nil
}

func (s *TokenisationStore) GetUnconfirmedMints(ctx context.Context, offset int, limit int) ([]Mint, error) {
	rows, err := s.DB.QueryContext(ctx, "SELECT id, created_at, title, description, fraction_count, tags, metadata, hash, transaction_hash, requirements, lockup_options, feed_url, public_key, contract_of_sale, signature_requirement_type, asset_managers, min_signatures FROM unconfirmed_mints LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mints []Mint
	for rows.Next() {
		var m Mint
		if err := rows.Scan(&m.Id, &m.CreatedAt, &m.Title, &m.Description, &m.FractionCount, &m.Tags, &m.Metadata, &m.Hash, &m.TransactionHash, &m.Requirements, &m.LockupOptions, &m.FeedURL, &m.PublicKey, &m.ContractOfSale, &m.SignatureRequirementType, &m.AssetManagers, &m.MinSignatures); err != nil {
			return nil, err
		}
		mints = append(mints, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return mints, nil
}

func (s *TokenisationStore) SaveMint(ctx context.Context, mint *MintWithoutID, ownerAddress string) (string, error) {
	return s.SaveMintWithTx(ctx, mint, ownerAddress, nil)
}

func (s *TokenisationStore) SaveMintWithTx(ctx context.Context, mint *MintWithoutID, ownerAddress string, tx *sql.Tx) (string, error) {
	id := uuid.New().String()

	metadata, err := json.Marshal(mint.Metadata)
	if err != nil {
		return "", err
	}

	requirements, err := json.Marshal(mint.Requirements)
	if err != nil {
		return "", err
	}

	lockupOptions, err := json.Marshal(mint.LockupOptions)
	if err != nil {
		return "", err
	}

	contractOfSale, err := json.Marshal(mint.ContractOfSale)
	if err != nil {
		return "", err
	}

	tags, err := json.Marshal(mint.Tags)
	if err != nil {
		return "", err
	}

	query := `
	INSERT INTO mints (id, title, description, fraction_count, tags, metadata, hash, requirements, lockup_options, feed_url, owner_address, public_key, block_height, transaction_hash, contract_of_sale, signature_requirement_type, asset_managers, min_signatures)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`

	if tx != nil {
		_, err = tx.ExecContext(ctx, query, id, mint.Title, mint.Description, mint.FractionCount, string(tags), string(metadata), mint.Hash, string(requirements), string(lockupOptions), mint.FeedURL, ownerAddress, mint.PublicKey, mint.BlockHeight, mint.TransactionHash, string(contractOfSale), mint.SignatureRequirementType, mint.AssetManagers, mint.MinSignatures)
	} else {
		_, err = s.DB.ExecContext(ctx, query, id, mint.Title, mint.Description, mint.FractionCount, string(tags), string(metadata), mint.Hash, string(requirements), string(lockupOptions), mint.FeedURL, ownerAddress, mint.PublicKey, mint.BlockHeight, mint.TransactionHash, string(contractOfSale), mint.SignatureRequirementType, mint.AssetManagers, mint.MinSignatures)
	}

	return id, err
}

func (s *TokenisationStore) TrimOldUnconfirmedMints(ctx context.Context, limit int) error {
	sqlQuery := fmt.Sprintf("DELETE FROM unconfirmed_mints WHERE id NOT IN (SELECT id FROM unconfirmed_mints ORDER BY id DESC LIMIT %d)", limit)
	_, err := s.DB.ExecContext(ctx, sqlQuery)
	if err != nil {
		return err
	}
	return nil
}

func (s *TokenisationStore) SaveUnconfirmedMint(ctx context.Context, mint *MintWithoutID) (string, error) {
	fmt.Println("Saving unconfirmed mint:", mint.Hash)

	id := uuid.New().String()

	metadata, err := json.Marshal(mint.Metadata)
	if err != nil {
		return "", err
	}

	requirements, err := json.Marshal(mint.Requirements)
	if err != nil {
		return "", err
	}

	lockupOptions, err := json.Marshal(mint.LockupOptions)
	if err != nil {
		return "", err
	}

	contractOfSale, err := json.Marshal(mint.ContractOfSale)
	if err != nil {
		return "", err
	}

	tags, err := json.Marshal(mint.Tags)
	if err != nil {
		return "", err
	}

	_, err = s.DB.ExecContext(ctx, `
	INSERT INTO unconfirmed_mints (id, title, description, fraction_count, tags, metadata, hash, requirements, lockup_options, feed_url, public_key, owner_address, transaction_hash, contract_of_sale, signature_requirement_type, asset_managers, min_signatures)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`, id, mint.Title, mint.Description, mint.FractionCount, string(tags), string(metadata), mint.Hash, string(requirements), string(lockupOptions), mint.FeedURL, mint.PublicKey, mint.OwnerAddress, mint.TransactionHash, string(contractOfSale), mint.SignatureRequirementType, mint.AssetManagers, mint.MinSignatures)
	log.Println("err:", err)

	return id, err
}

func (s *TokenisationStore) MatchMint(ctx context.Context, onchainTransaction OnChainTransaction) bool {
	if onchainTransaction.ActionType != protocol.ACTION_MINT {
		return false
	}

	var onchainMessage protocol.OnChainMintMessage
	err := proto.Unmarshal(onchainTransaction.ActionData, &onchainMessage)
	if err != nil {
		return false
	}

	if onchainMessage.Hash != onchainTransaction.TxHash {
		return false
	}

	rows, err := s.DB.QueryContext(ctx, "SELECT hash, transaction_hash FROM mints WHERE transaction_hash = $1 and block_height = $2 and hash = $3", onchainTransaction.TxHash, onchainTransaction.Height, onchainMessage.Hash)
	if err != nil {
		return false
	}
	defer rows.Close()

	exists := rows.Next()

	if exists {
		_, err = s.DB.ExecContext(ctx, "DELETE FROM onchain_transactions WHERE id = $1", onchainTransaction.Id)
		if err != nil {
			return false
		}
	}

	return exists
}

func (s *TokenisationStore) MatchUnconfirmedMint(ctx context.Context, onchainTransaction OnChainTransaction) error {

	if onchainTransaction.ActionType != protocol.ACTION_MINT {
		return fmt.Errorf("action type is not mint: %d", onchainTransaction.ActionType)
	}

	var onchainMessage protocol.OnChainMintMessage
	err := proto.Unmarshal(onchainTransaction.ActionData, &onchainMessage)
	if err != nil {
		return err
	}

	// Start transaction for atomic operations
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, "SELECT id, title, description, fraction_count, tags, metadata, hash, transaction_hash, requirements, lockup_options, feed_url, public_key, contract_of_sale, signature_requirement_type, asset_managers, min_signatures FROM unconfirmed_mints WHERE hash = $1", onchainMessage.Hash)
	if err != nil {
		return err
	}

	defer rows.Close()

	var unconfirmedMint Mint
	if rows.Next() {
		if err := rows.Scan(
			&unconfirmedMint.Id, &unconfirmedMint.Title, &unconfirmedMint.Description,
			&unconfirmedMint.FractionCount, &unconfirmedMint.Tags, &unconfirmedMint.Metadata,
			&unconfirmedMint.Hash, &unconfirmedMint.TransactionHash, &unconfirmedMint.Requirements,
			&unconfirmedMint.LockupOptions, &unconfirmedMint.FeedURL, &unconfirmedMint.PublicKey, &unconfirmedMint.ContractOfSale, &unconfirmedMint.SignatureRequirementType, &unconfirmedMint.AssetManagers, &unconfirmedMint.MinSignatures); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("no unconfirmed mint found for hash: %s", onchainMessage.Hash)
	}

	rows.Close()

	// Use transaction-aware SaveMint
	id, err := s.SaveMintWithTx(ctx, &MintWithoutID{
		Hash:                     unconfirmedMint.Hash,
		Title:                    unconfirmedMint.Title,
		FractionCount:            unconfirmedMint.FractionCount,
		Description:              unconfirmedMint.Description,
		Tags:                     unconfirmedMint.Tags,
		Metadata:                 unconfirmedMint.Metadata,
		TransactionHash:          onchainTransaction.TxHash,
		BlockHeight:              onchainTransaction.Height,
		CreatedAt:                unconfirmedMint.CreatedAt,
		Requirements:             unconfirmedMint.Requirements,
		LockupOptions:            unconfirmedMint.LockupOptions,
		FeedURL:                  unconfirmedMint.FeedURL,
		PublicKey:                unconfirmedMint.PublicKey,
		OwnerAddress:             onchainTransaction.Address,
		ContractOfSale:           unconfirmedMint.ContractOfSale,
		SignatureRequirementType: unconfirmedMint.SignatureRequirementType,
		AssetManagers:            unconfirmedMint.AssetManagers,
		MinSignatures:            unconfirmedMint.MinSignatures,
	}, onchainTransaction.Address, tx)

	if err != nil {
		return err
	}

	log.Println("Saved mint:", id)

	// Use transaction-aware UpsertTokenBalance
	err = s.UpsertTokenBalanceWithTransaction(ctx, onchainTransaction.Address, unconfirmedMint.Hash, unconfirmedMint.FractionCount, tx)
	if err != nil {
		log.Println("error upserting token balance", err)
		return err
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM unconfirmed_mints WHERE id = $1", unconfirmedMint.Id)
	if err != nil {
		log.Println("error deleting unconfirmed mint", err)
		return err
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM onchain_transactions WHERE id = $1", onchainTransaction.Id)
	if err != nil {
		log.Println("error deleting onchain transaction", err)
		return err
	}

	// Commit the transaction
	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func getMintsCount(ctx context.Context, s *TokenisationStore) (int, error) {
	if s.backend == "postgres" {
		return approximateTableCountPostgres(ctx, s.DB, "mints")
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT COUNT(*) FROM mints")
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

func getUnconfirmedMintsCount(ctx context.Context, s *TokenisationStore) (int, error) {
	if s.backend == "postgres" {
		return approximateTableCountPostgres(ctx, s.DB, "unconfirmed_mints")
	}
	rows, err := s.DB.QueryContext(ctx, "SELECT COUNT(*) FROM unconfirmed_mints")
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

func (s *TokenisationStore) ClearMints(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, "DELETE FROM mints")
	if err != nil {
		return err
	}
	return nil
}
