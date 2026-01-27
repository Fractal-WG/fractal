package dogenet

import (
	"context"
	"log"

	"code.dogecoin.org/gossip/dnet"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (c *DogeNetClient) GossipSellOffer(record store.SellOffer) error {
	offerMessage := protocol.SellOfferMessage{
		Id:        record.Id,
		Hash:      record.Hash,
		CreatedAt: timestamppb.New(record.CreatedAt),
		Payload: &protocol.SellOfferPayload{
			OffererAddress: record.OffererAddress,
			MintHash:       record.MintHash,
			Quantity:       int32(record.Quantity),
			Price:          int32(record.Price),
		},
	}

	envelope := protocol.SellOfferMessageEnvelope{
		Type:      protocol.ACTION_SELL_OFFER,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   &offerMessage,
		PublicKey: record.PublicKey,
		Signature: record.Signature,
	}

	data, err := proto.Marshal(&envelope)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	encodedMsg := dnet.EncodeMessageRaw(ChanFE, TagSellOffer, c.feKey, data)

	err = encodedMsg.Send(c.sock)
	if err != nil {
		return err
	}

	return nil
}

func (c *DogeNetClient) GossipDeleteSellOffer(hash string, publicKey string, signature string) error {
	message := protocol.DeleteSellOfferMessage{
		Hash: hash,
	}

	envelope := protocol.DeleteSellOfferMessageEnvelope{
		Type:      protocol.ACTION_DELETE_SELL_OFFER,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   &message,
		PublicKey: publicKey,
		Signature: signature,
	}

	data, err := proto.Marshal(&envelope)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	encodedMsg := dnet.EncodeMessageRaw(ChanFE, TagDeleteSellOffer, c.feKey, data)

	err = encodedMsg.Send(c.sock)
	if err != nil {
		return err
	}

	return nil
}

func (c *DogeNetClient) recvSellOffer(msg dnet.Message) {
	log.Printf("[FE] received sell offer message")
	ctx := context.Background()

	envelope := protocol.SellOfferMessageEnvelope{}
	err := proto.Unmarshal(msg.Payload, &envelope)
	if err != nil {
		log.Println("Error deserializing message envelope:", err)
		return
	}

	if envelope.Type != protocol.ACTION_SELL_OFFER {
		log.Printf("[FE] unexpected action: [%s][%s][%d]", msg.Chan, msg.Tag, envelope.Type)
		return
	}

	offer := envelope.Payload

	signaturePayload := protocol.SellOfferPayload{
		OffererAddress: offer.Payload.OffererAddress,
		MintHash:       offer.Payload.MintHash,
		Quantity:       offer.Payload.Quantity,
		Price:          offer.Payload.Price,
	}

	offerPayload, err := protojson.Marshal(&signaturePayload)
	if err != nil {
		log.Println("Error marshalling offer:", err)
		return
	}

	err = doge.ValidateSignature(offerPayload, envelope.PublicKey, envelope.Signature)
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

	log.Printf("[FE] address: %s", address)
	log.Printf("[FE] offerer address: %s", offer.Payload.OffererAddress)

	if address != offer.Payload.OffererAddress {
		log.Println("Offerer address does not match public key")
		return
	}

	offerWithoutID := store.SellOfferWithoutID{
		OffererAddress: offer.Payload.OffererAddress,
		MintHash:       offer.Payload.MintHash,
		Hash:           offer.Hash,
		Quantity:       int(offer.Payload.Quantity),
		Price:          int(offer.Payload.Price),
		CreatedAt:      offer.CreatedAt.AsTime(),
		PublicKey:      envelope.PublicKey,
		Signature:      envelope.Signature,
	}

	id, err := c.store.SaveSellOffer(ctx, &offerWithoutID)
	if err != nil {
		log.Println("Error saving sell offer:", err)
		return
	}

	log.Printf("[FE] sell offer saved: %v", id)
}

func (c *DogeNetClient) recvDeleteSellOffer(msg dnet.Message) {
	log.Printf("[FE] received delete sell offer message")
	ctx := context.Background()

	envelope := protocol.DeleteSellOfferMessageEnvelope{}
	err := proto.Unmarshal(msg.Payload, &envelope)
	if err != nil {
		log.Println("Error deserializing message envelope:", err)
		return
	}

	if envelope.Type != protocol.ACTION_DELETE_SELL_OFFER {
		log.Printf("[FE] unexpected action: [%s][%s][%d]", msg.Chan, msg.Tag, envelope.Type)
		return
	}

	message := envelope.Payload

	err = doge.ValidateSignature([]byte(message.Hash), envelope.PublicKey, envelope.Signature)
	if err != nil {
		log.Println("Error validating signature:", err)
		return
	}

	err = c.store.DeleteSellOffer(ctx, message.Hash, envelope.PublicKey)
	if err != nil {
		log.Println("Error deleting sell offer:", err)
		return
	}

	log.Printf("[FE] sell offer deleted: %v", message.Hash)
}
