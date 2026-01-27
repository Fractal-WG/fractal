package dogenet

import (
	"context"
	"log"

	"code.dogecoin.org/gossip/dnet"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (c *DogeNetClient) GossipUnconfirmedInvoice(record store.UnconfirmedInvoice) error {
	invoiceMessage := protocol.InvoiceMessage{
		Id: record.Id,
		Payload: &protocol.InvoicePayload{
			PaymentAddress: record.PaymentAddress,
			MintHash:       record.MintHash,
			BuyerAddress:   record.BuyerAddress,
			Quantity:       int32(record.Quantity),
			Price:          int32(record.Price),
			SellerAddress:  record.SellerAddress,
		},
		Hash:      record.Hash,
		CreatedAt: timestamppb.New(record.CreatedAt),
	}

	envelope := protocol.InvoiceMessageEnvelope{
		Type:      protocol.ACTION_INVOICE,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   &invoiceMessage,
		PublicKey: record.PublicKey,
		Signature: record.Signature,
	}

	data, err := proto.Marshal(&envelope)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	encodedMsg := dnet.EncodeMessageRaw(ChanFE, TagInvoice, c.feKey, data)

	err = encodedMsg.Send(c.sock)
	if err != nil {
		return err
	}

	return nil
}

func (c *DogeNetClient) recvInvoice(msg dnet.Message) {
	log.Printf("[FE] received invoice message")
	ctx := context.Background()

	envelope := protocol.InvoiceMessageEnvelope{}
	err := proto.Unmarshal(msg.Payload, &envelope)
	if err != nil {
		log.Println("Error deserializing message envelope:", err)
		return
	}

	if envelope.Type != protocol.ACTION_INVOICE {
		log.Printf("[FE] unexpected action: [%s][%s][%d]", msg.Chan, msg.Tag, envelope.Type)
		return
	}

	invoice := envelope.Payload

	invoiceSignaturePayload := &protocol.InvoicePayload{
		PaymentAddress: invoice.Payload.PaymentAddress,
		BuyerAddress:   invoice.Payload.BuyerAddress,
		MintHash:       invoice.Payload.MintHash,
		Quantity:       invoice.Payload.Quantity,
		Price:          invoice.Payload.Price,
		SellerAddress:  invoice.Payload.SellerAddress,
	}

	err = doge.ValidateSignature(invoiceSignaturePayload, envelope.PublicKey, envelope.Signature)
	if err != nil {
		log.Println("Error validating signature:", err)
		return
	}

	prefix, err := doge.GetPrefix(c.cfg.DogeNetChain)
	if err != nil {
		log.Println("Error getting prefix:", err)
		return
	}

	address, err := doge.PublicKeyToDogeAddress(envelope.PublicKey, prefix)
	if err != nil {
		log.Println("Error converting public key to doge address:", err)
		return
	}

	if address != invoice.Payload.SellerAddress {
		log.Println("Sell offer address does not match public key")
		return
	}

	invoiceWithoutID := store.UnconfirmedInvoice{
		PaymentAddress: invoice.Payload.PaymentAddress,
		MintHash:       invoice.Payload.MintHash,
		BuyerAddress:   invoice.Payload.BuyerAddress,
		Quantity:       int(invoice.Payload.Quantity),
		Price:          int(invoice.Payload.Price),
		CreatedAt:      invoice.CreatedAt.AsTime(),
		Hash:           invoice.Hash,
		Id:             invoice.Id,
		PublicKey:      envelope.PublicKey,
		SellerAddress:  invoice.Payload.SellerAddress,
		Signature:      envelope.Signature,
	}

	id, err := c.store.SaveUnconfirmedInvoice(ctx, &invoiceWithoutID)
	if err != nil {
		log.Println("Error saving unconfirmed invoice:", err)
		return
	}

	log.Printf("[FE] unconfirmed invoice saved: %v", id)
}
