package store

import (
	"context"
	"fmt"
	"log"

	"dogecoin.org/fractal-engine/pkg/protocol"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func (s *TokenisationStore) SaveUnconfirmedTokenBurn(ctx context.Context, burn *UnconfirmedTokenBurn) (string, error) {
	id := uuid.New().String()

	_, err := s.DB.ExecContext(ctx, `
	INSERT INTO unconfirmed_token_burns (id, hash, mint_hash, burn_quantity, burner_address, public_key, signature, created_at)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, burn.Hash, burn.MintHash, burn.BurnQuantity, burn.BurnerAddress, burn.PublicKey, burn.Signature, burn.CreatedAt)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (s *TokenisationStore) ChooseTokenBurn(ctx context.Context) (UnconfirmedTokenBurn, error) {
	row := s.DB.QueryRowContext(ctx, "SELECT id, hash, mint_hash, burn_quantity, burner_address, public_key, signature, created_at FROM unconfirmed_token_burns WHERE id IN (SELECT id FROM unconfirmed_token_burns ORDER BY RANDOM() LIMIT 1)")
	var b UnconfirmedTokenBurn
	if err := row.Scan(&b.Id, &b.Hash, &b.MintHash, &b.BurnQuantity, &b.BurnerAddress, &b.PublicKey, &b.Signature, &b.CreatedAt); err != nil {
		return UnconfirmedTokenBurn{}, err
	}
	return b, nil
}

func (s *TokenisationStore) MatchUnconfirmedTokenBurn(ctx context.Context, onchainTransaction OnChainTransaction) error {
	if onchainTransaction.ActionType != protocol.ACTION_TOKEN_BURN {
		return fmt.Errorf("action type is not token burn: %d", onchainTransaction.ActionType)
	}

	var onchainMessage protocol.OnChainTokenBurnMessage
	if err := proto.Unmarshal(onchainTransaction.ActionData, &onchainMessage); err != nil {
		return err
	}

	burnHash := fmt.Sprintf("%x", onchainMessage.BurnHash)
	mintHash := fmt.Sprintf("%x", onchainMessage.MintHash)

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var burn UnconfirmedTokenBurn
	row := tx.QueryRowContext(ctx, `
		SELECT id, hash, mint_hash, burn_quantity, burner_address, public_key, signature, created_at
		FROM unconfirmed_token_burns WHERE hash = $1
	`, burnHash)
	if err := row.Scan(
		&burn.Id, &burn.Hash, &burn.MintHash, &burn.BurnQuantity,
		&burn.BurnerAddress, &burn.PublicKey, &burn.Signature, &burn.CreatedAt,
	); err != nil {
		return fmt.Errorf("no unconfirmed token burn found for hash %s: %w", burnHash, err)
	}

	if burn.MintHash != mintHash {
		return fmt.Errorf("burn mint_hash mismatch: expected %s got %s", mintHash, burn.MintHash)
	}

	if int32(burn.BurnQuantity) != onchainMessage.BurnQuantity {
		return fmt.Errorf("burn quantity mismatch: expected %d got %d", onchainMessage.BurnQuantity, burn.BurnQuantity)
	}

	// Fetch the mint to validate burn authority
	var mintOwnerAddress string
	var burnAuthority BurnAuthorityType
	mintRow := tx.QueryRowContext(ctx,
		"SELECT owner_address, burn_authority FROM mints WHERE hash = $1",
		burn.MintHash,
	)
	if err := mintRow.Scan(&mintOwnerAddress, &burnAuthority); err != nil {
		return fmt.Errorf("failed to fetch mint for hash %s: %w", burn.MintHash, err)
	}

	isOwner := burn.BurnerAddress == mintOwnerAddress
	switch burnAuthority {
	case BurnAuthorityOwnerOnly:
		if !isOwner {
			return fmt.Errorf("burn authority violation: only the mint owner can burn tokens for this mint")
		}
	case BurnAuthorityHolderOnly:
		if isOwner {
			return fmt.Errorf("burn authority violation: only token holders (not the mint owner) can burn tokens for this mint")
		}
	case BurnAuthorityOwnerOrHolder:
		// either is permitted; balance check below will enforce the holder has tokens
	default:
		return fmt.Errorf("unknown burn authority type: %s", burnAuthority)
	}

	// Compute net available balance
	var tokenBalance int
	balRow := tx.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(quantity), 0) FROM token_balances WHERE address = $1 AND mint_hash = $2",
		burn.BurnerAddress, burn.MintHash,
	)
	if err := balRow.Scan(&tokenBalance); err != nil {
		return fmt.Errorf("failed to get token balance: %w", err)
	}

	var pendingBalance int
	pendRow := tx.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(quantity), 0) FROM pending_token_balances WHERE owner_address = $1 AND mint_hash = $2",
		burn.BurnerAddress, burn.MintHash,
	)
	if err := pendRow.Scan(&pendingBalance); err != nil {
		return fmt.Errorf("failed to get pending token balance: %w", err)
	}

	available := tokenBalance - pendingBalance
	if available < burn.BurnQuantity {
		return fmt.Errorf("insufficient balance: available %d, burn_quantity %d", available, burn.BurnQuantity)
	}

	if err := s.UpsertTokenBalanceWithTransaction(ctx, burn.BurnerAddress, burn.MintHash, -burn.BurnQuantity, tx); err != nil {
		return fmt.Errorf("failed to update token balance: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		"UPDATE mints SET current_supply = current_supply - $1 WHERE hash = $2",
		burn.BurnQuantity, burn.MintHash,
	)
	if err != nil {
		return fmt.Errorf("failed to update current_supply: %w", err)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM unconfirmed_token_burns WHERE id = $1", burn.Id)
	if err != nil {
		return fmt.Errorf("failed to delete unconfirmed token burn: %w", err)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM onchain_transactions WHERE id = $1", onchainTransaction.Id)
	if err != nil {
		return fmt.Errorf("failed to delete onchain transaction: %w", err)
	}

	log.Printf("Token burn matched: burn_hash=%s mint_hash=%s quantity=%d", burnHash, mintHash, burn.BurnQuantity)

	return tx.Commit()
}
