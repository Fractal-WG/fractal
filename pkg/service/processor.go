package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
)

type FractalEngineProcessor struct {
	store      *store.TokenisationStore
	dogeClient *doge.RpcClient
	Running    bool
}

func NewFractalEngineProcessor(store *store.TokenisationStore, dogeClient *doge.RpcClient) *FractalEngineProcessor {
	return &FractalEngineProcessor{store: store, dogeClient: dogeClient}
}

func (p *FractalEngineProcessor) Process() error {
	ctx := context.Background()
	offset := 0
	limit := 100

	for {
		txs, err := p.store.GetOnChainTransactions(ctx, offset, limit)
		if err != nil {
			return err
		}

		if len(txs) == 0 {
			break
		}

		for _, tx := range txs {
			fmt.Println("Processing transaction:", tx.TxHash)

			if tx.ActionType == protocol.ACTION_MINT {
				if p.store.MatchMint(ctx, tx) {
					continue
				}
				err = p.store.MatchUnconfirmedMint(ctx, tx)
				if err == nil {
					log.Println("Matched mint:", tx.TxHash)
				}
			} else if tx.ActionType == protocol.ACTION_PAYMENT {
				paymentProcessor := NewPaymentProcessor(p.store, p.dogeClient)
				err = paymentProcessor.Process(tx)
				if err != nil {
					log.Println("Error processing payment:", err)
				}
			} else if tx.ActionType == protocol.ACTION_INVOICE {
				invoiceProcessor := NewInvoiceProcessor(p.store)
				err = invoiceProcessor.Process(tx)
				if err != nil {
					log.Println("Error processing invoice:", err)
				}

			}
		}

		offset += limit

		time.Sleep(5 * time.Second)
	}

	return nil
}

func (p *FractalEngineProcessor) Start() {
	p.Running = true

	for {
		if !p.Running {
			break
		}

		err := p.Process()
		if err != nil {
			log.Println("Error processing:", err)
		}

		time.Sleep(3 * time.Second)
	}
}

func (p *FractalEngineProcessor) Stop() {
	fmt.Println("Stopping processor")
	p.Running = false
}
