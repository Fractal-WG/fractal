package dogenet

import (
	"context"
	"log"

	"code.dogecoin.org/gossip/dnet"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (c *DogeNetClient) GossipInvoiceSignature(record store.InvoiceSignature) error {
	invoiceSignatureMessage := protocol.InvoiceSignatureMessage{
		InvoiceHash: record.InvoiceHash,
		Signature:   record.Signature,
		PublicKey:   record.PublicKey,
		CreatedAt:   timestamppb.New(record.CreatedAt),
	}

	envelope := protocol.InvoiceSignatureMessageEnvelope{
		Type:    protocol.ACTION_INVOICE_SIGNATURE,
		Version: protocol.DEFAULT_VERSION,
		Payload: &invoiceSignatureMessage,
	}

	data, err := proto.Marshal(&envelope)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	encodedMsg := dnet.EncodeMessageRaw(ChanFE, TagInvoiceSignature, c.feKey, data)

	err = encodedMsg.Send(c.sock)
	if err != nil {
		return err
	}

	return nil
}

func (c *DogeNetClient) recvInvoiceSignature(msg dnet.Message) {
	log.Printf("[FE] received invoice signature message")
	ctx := context.Background()

	envelope := protocol.InvoiceSignatureMessageEnvelope{}
	err := proto.Unmarshal(msg.Payload, &envelope)
	if err != nil {
		log.Println("Error deserializing message envelope:", err)
		return
	}

	if envelope.Type != protocol.ACTION_INVOICE_SIGNATURE {
		log.Printf("[FE] unexpected action: [%s][%s][%d]", msg.Chan, msg.Tag, envelope.Type)
		return
	}

	invoiceSignature := envelope.Payload

	invoiceSignatureWithoutID := store.InvoiceSignature{
		InvoiceHash: invoiceSignature.InvoiceHash,
		Signature:   invoiceSignature.Signature,
		PublicKey:   invoiceSignature.PublicKey,
		CreatedAt:   invoiceSignature.CreatedAt.AsTime(),
	}

	id, err := c.store.SaveApprovedInvoiceSignature(ctx, &invoiceSignatureWithoutID)
	if err != nil {
		log.Println("Error saving unconfirmed invoice:", err)
		return
	}

	log.Printf("[FE] unconfirmed invoice saved: %v", id)
}
