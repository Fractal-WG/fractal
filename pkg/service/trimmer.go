package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/store"
)

type TrimmerService struct {
	blocksToKeep            int
	unconfirmedMintsToKeep  int
	store                   *store.TokenisationStore
	dogeClient              *doge.RpcClient
	running                 bool
	invoiceTimeoutProcessor *InvoiceTimeoutProcessor
}

func NewTrimmerService(blocksToKeep int, unconfirmedMintsToKeep int, store *store.TokenisationStore, dogeClient *doge.RpcClient) *TrimmerService {
	return &TrimmerService{blocksToKeep: blocksToKeep, unconfirmedMintsToKeep: unconfirmedMintsToKeep, store: store, dogeClient: dogeClient, running: false, invoiceTimeoutProcessor: NewInvoiceTimeoutProcessor(store)}
}

func (t *TrimmerService) Start() {
	t.running = true
	ctx := context.Background()

	for {
		bestBlockHash, err := t.dogeClient.GetBestBlockHash(ctx)
		if err != nil {
			log.Println("Error getting best block hash:", err)
			time.Sleep(10 * time.Second)
			continue
		}

		blockHeader, err := t.dogeClient.GetBlockHeader(ctx, bestBlockHash)
		if err != nil {
			log.Println("Error getting block header:", err)
			time.Sleep(10 * time.Second)
			continue
		}

		latestBlockHeight := int(blockHeader.Height)
		oldestBlockHeight := latestBlockHeight - t.blocksToKeep

		err = t.invoiceTimeoutProcessor.Process(oldestBlockHeight)
		if err != nil {
			log.Println("Error processing invoice timeout:", err)
		}

		err = t.store.TrimOldUnconfirmedMints(ctx, t.unconfirmedMintsToKeep)
		if err != nil {
			log.Println("Error trimming unconfirmed mints:", err)
		}

		err = t.store.TrimOldOnChainTransactions(ctx, oldestBlockHeight)
		if err != nil {
			log.Println("Error trimming on chain transactions:", err)
		}

		time.Sleep(10 * time.Second)

		if !t.running {
			break
		}
	}
}

func (t *TrimmerService) Stop() {
	fmt.Println("Stopping trimmer service")
	t.running = false
}
