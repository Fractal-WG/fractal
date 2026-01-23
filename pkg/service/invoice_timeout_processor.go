package service

import (
	"context"
	"encoding/hex"
	"log"

	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/proto"
)

type InvoiceTimeoutProcessor struct {
	store *store.TokenisationStore
}

func NewInvoiceTimeoutProcessor(store *store.TokenisationStore) *InvoiceTimeoutProcessor {
	return &InvoiceTimeoutProcessor{store: store}
}

func (p *InvoiceTimeoutProcessor) Process(oldestBlockHeight int) error {
	ctx := context.Background()
	unconfirmedInvoices, err := p.store.GetOldOnchainTransactions(ctx, oldestBlockHeight)
	if err != nil {
		log.Println("Error getting old onchain transactions:", err)
		return err
	}

	for _, invoice := range unconfirmedInvoices {
		if invoice.ActionType == protocol.ACTION_INVOICE {
			invoiceMessage := protocol.OnChainInvoiceMessage{}

			err := proto.Unmarshal(invoice.ActionData, &invoiceMessage)
			if err != nil {
				log.Println("Error unmarshalling invoice message:", err)
				continue
			}

			pendingTokenBalance, err := p.store.GetPendingTokenBalance(ctx, hex.EncodeToString(invoiceMessage.InvoiceHash), hex.EncodeToString(invoiceMessage.MintHash), nil)
			if err != nil {
				log.Println("Error getting pending token balance:", err)
				continue
			}

			if pendingTokenBalance.InvoiceHash != "" {
				err = p.store.RemovePendingTokenBalance(ctx, hex.EncodeToString(invoiceMessage.InvoiceHash), hex.EncodeToString(invoiceMessage.MintHash))
				if err != nil {
					log.Println("Error removing pending token balance:", err)
					continue
				}
			}
		}
	}

	return nil
}
