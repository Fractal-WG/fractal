package store

import (
	"context"
	"fmt"
	"log"
	"time"

	"dogecoin.org/fractal-engine/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

func (s *TokenisationStore) ProcessPayment(ctx context.Context, onchainTransaction OnChainTransaction, invoice Invoice) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec("UPDATE invoices SET paid_at = $1 WHERE id = $2", time.Now().UTC(), invoice.Id)
	if err != nil {
		log.Println("Error updating invoice:", err)
		return err
	}

	_, err = tx.Exec("DELETE FROM onchain_transactions WHERE id = $1", onchainTransaction.Id)
	if err != nil {
		log.Println("Error deleting onchain transaction:", err)
		return err
	}

	pendingTokenBalance, err := s.GetPendingTokenBalanceForQuantity(ctx, invoice.Hash, invoice.MintHash, invoice.Quantity, tx)
	if err != nil {
		log.Println("Error getting pending token balance:", err)
		return err
	}

	err = s.MovePendingToTokenBalance(ctx, pendingTokenBalance, invoice.BuyerAddress, tx)
	if err != nil {
		log.Println("Error moving pending to token balance:", err)
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (s *TokenisationStore) MatchPayment(ctx context.Context, onchainTransaction OnChainTransaction) (Invoice, error) {
	if onchainTransaction.ActionType != protocol.ACTION_PAYMENT {
		return Invoice{}, fmt.Errorf("action type is not payment: %d", onchainTransaction.ActionType)
	}

	var onchainMessage protocol.OnChainPaymentMessage
	err := proto.Unmarshal(onchainTransaction.ActionData, &onchainMessage)
	if err != nil {
		return Invoice{}, err
	}

	rows, err := s.DB.Query("SELECT id, hash, payment_address, buyer_address, mint_hash, quantity, price, created_at, seller_address FROM invoices WHERE hash = $1", onchainMessage.Hash)
	if err != nil {
		log.Println("Error querying invoices:", err)
		return Invoice{}, err
	}

	var invoice Invoice

	if rows.Next() {
		err := rows.Scan(&invoice.Id, &invoice.Hash, &invoice.PaymentAddress, &invoice.BuyerAddress, &invoice.MintHash, &invoice.Quantity, &invoice.Price, &invoice.CreatedAt, &invoice.SellerAddress)
		if err != nil {
			log.Println("Error scanning invoice:", err)
			return Invoice{}, err
		}
	}

	rows.Close()

	if invoice.Id == "" {
		return Invoice{}, fmt.Errorf("invoice not found")
	}

	value := float64(invoice.Quantity * invoice.Price)

	paymentValue := onchainTransaction.Values[invoice.SellerAddress]

	if paymentValue != value {
		return Invoice{}, fmt.Errorf("payment value is not equal to buy offer value: %f != %f", paymentValue, value)
	}

	return invoice, nil
}
