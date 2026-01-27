package service

import (
	"context"
	"encoding/hex"
	"log"
	"strings"

	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"dogecoin.org/fractal-engine/pkg/validation"
	"google.golang.org/protobuf/proto"
)

type InvoiceProcessor struct {
	store *store.TokenisationStore
}

func NewInvoiceProcessor(store *store.TokenisationStore) *InvoiceProcessor {
	return &InvoiceProcessor{store: store}
}

/*
* Check if invoice already taken pending availability
* If so, then attempt to match to existing invoice or unconfirmed invoice

* If not, check if theres enough availability on token balances
* If so, create a pending token balance
* If not, remove onchain transaction (discard invoice)
 */
func (p *InvoiceProcessor) Process(tx store.OnChainTransaction) error {
	ctx := context.Background()
	invoice := protocol.OnChainInvoiceMessage{}
	err := proto.Unmarshal(tx.ActionData, &invoice)
	if err != nil {
		log.Println("Error unmarshalling invoice:", err)
		return err
	}

	// Validate protobuf content
	if err := validation.ValidateProtobufQuantity(invoice.Quantity); err != nil {
		log.Printf("Invalid quantity in protobuf: %v", err)
		return err
	}

	hasPendingTokenBalance, err := p.EnsurePendingTokenBalance(tx)
	if err != nil {
		return err
	}

	if !hasPendingTokenBalance {
		log.Println("Invoice discarded, not enough availability")
		return nil
	}

	mint, err := p.store.GetMintByHash(ctx, hex.EncodeToString(invoice.MintHash))
	if err != nil {
		log.Println("Error getting mint:", err)
		return err
	}

	// Check if signatures are required and if the number of signatures is correct
	if mint.SignatureRequired() {
		signatures, err := p.store.GetApprovedInvoiceSignatures(ctx, hex.EncodeToString(invoice.InvoiceHash))
		if err != nil {
			log.Println("Error getting invoice signatures:", err)
			return err
		}

		if !mint.HasRequiredSignatures(signatures) {
			log.Println("Invalid number of signatures")
			return err
		}
	}

	// Try to match confirmed invoice first
	if p.store.MatchInvoice(ctx, tx) {
		return nil
	}

	// Try to match unconfirmed invoice (already transaction-safe)
	err = p.store.MatchUnconfirmedInvoice(ctx, tx)
	if err == nil {
		log.Println("Matched invoice:", tx.TxHash)
	} else {
		log.Println("Error matching unconfirmed invoice:", err)
		// If no unconfirmed invoice found, this is not necessarily an error
		// The pending balance is already created, so processing succeeded
		if strings.Contains(err.Error(), "no unconfirmed invoice found for hash:") {
			return nil
		}
	}

	return err
}

func (p *InvoiceProcessor) EnsurePendingTokenBalance(tx store.OnChainTransaction) (bool, error) {
	ctx := context.Background()
	invoice := protocol.OnChainInvoiceMessage{}
	err := proto.Unmarshal(tx.ActionData, &invoice)
	if err != nil {
		log.Println("Error unmarshalling invoice:", err)
		return false, err
	}

	// Start transaction for atomic operations
	dbTx, err := p.store.DB.Begin()
	if err != nil {
		return false, err
	}
	defer dbTx.Rollback()

	// Check if pending token balance already exists with lock
	pendingTokenBalance, _ := p.store.GetPendingTokenBalance(ctx, hex.EncodeToString(invoice.InvoiceHash), hex.EncodeToString(invoice.MintHash), dbTx)
	if pendingTokenBalance.InvoiceHash != "" {
		log.Println("Pending token balance already exists")
		dbTx.Commit()
		return true, nil
	}

	tokenBalances, err := p.store.GetTokenBalances(ctx, tx.Address, hex.EncodeToString(invoice.MintHash))
	if err != nil {
		log.Println("Error getting token balance:", err)
		return false, err
	}

	pendingTokenBalanceTotal, err := p.store.GetPendingTokenBalanceTotalForMintAndOwner(ctx, hex.EncodeToString(invoice.MintHash), tx.Address)
	if err != nil {
		log.Println("Error getting pending token balance total:", err)
		return false, err
	}

	totalTokenBalance := 0
	for _, tokenBalance := range tokenBalances {
		totalTokenBalance += tokenBalance.Quantity
	}

	tokenBalanceAvailable := totalTokenBalance - pendingTokenBalanceTotal

	if tokenBalanceAvailable >= int(invoice.Quantity) {
		log.Println("Token balance is enough")

		// Use transaction-aware UpsertPendingTokenBalance
		err = p.store.UpsertPendingTokenBalanceWithTx(ctx, hex.EncodeToString(invoice.InvoiceHash), hex.EncodeToString(invoice.MintHash), int(invoice.Quantity), tx.Id, tx.Address, dbTx)
		if err != nil {
			log.Println("Error inserting pending token balance:", err)
			return false, err
		}

		// Commit transaction
		err = dbTx.Commit()
		if err != nil {
			return false, err
		}

		return true, nil
	} else {
		log.Println("Token balance is not enough")

		// Remove onchain transaction within same transaction
		_, err = dbTx.Exec("DELETE FROM onchain_transactions WHERE id = $1", tx.Id)
		if err != nil {
			log.Println("Error removing onchain transaction:", err)
			return false, err
		}

		// Commit the removal
		err = dbTx.Commit()
		if err != nil {
			return false, err
		}

		return false, nil
	}
}
