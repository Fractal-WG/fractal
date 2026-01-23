package service

import (
	"context"
	"fmt"
	"log"

	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/store"
)

const MIN_CONFIRMATIONS_REQUIRED = 6

type PaymentProcessor struct {
	store      *store.TokenisationStore
	dogeClient *doge.RpcClient
}

func NewPaymentProcessor(store *store.TokenisationStore, dogeClient *doge.RpcClient) *PaymentProcessor {
	return &PaymentProcessor{store: store, dogeClient: dogeClient}
}

func (p *PaymentProcessor) Process(tx store.OnChainTransaction) error {
	ctx := context.Background()
	invoice, err := p.store.MatchPayment(ctx, tx)
	if err != nil {
		log.Println("Match Payment", err)
		return err
	}

	if tx.BlockHash == "" {
		blockHash, err := p.dogeClient.GetBlockHash(int(tx.Height))
		if err != nil {
			log.Println("GetBlockHash", err)
			return err
		}

		tx.BlockHash = blockHash
	}

	blockHeader, err := p.dogeClient.GetBlockHeader(tx.BlockHash)
	if err != nil {
		log.Println("GetBlockHeader", err)
		return err
	}

	if blockHeader.Confirmations < MIN_CONFIRMATIONS_REQUIRED {
		log.Println("Minimum confirmations not met:", err)
		return fmt.Errorf("Minimum confirmations not met: %d < %d", blockHeader.Confirmations, MIN_CONFIRMATIONS_REQUIRED)
	}

	err = p.store.ProcessPayment(ctx, tx, invoice)
	if err != nil {
		log.Println("ProcessPayment:", err)
		return err
	}

	log.Println("Matched payment:", tx.TxHash)
	return nil
}
