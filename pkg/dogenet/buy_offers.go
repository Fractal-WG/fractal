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

func (c *DogeNetClient) GossipBuyOffer(record store.BuyOffer) error {
	offerMessage := protocol.BuyOfferMessage{
		Id:        record.Id,
		Hash:      record.Hash,
		CreatedAt: timestamppb.New(record.CreatedAt),
		Payload: &protocol.BuyOfferPayload{
			OffererAddress: record.OffererAddress,
			SellerAddress:  record.SellerAddress,
			MintHash:       record.MintHash,
			Quantity:       int32(record.Quantity),
			Price:          int32(record.Price),
		},
	}

	envelope := protocol.BuyOfferMessageEnvelope{
		Type:      protocol.ACTION_BUY_OFFER,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   &offerMessage,
		PublicKey: record.PublicKey,
		Signature: record.Signature,
	}

	data, err := proto.Marshal(&envelope)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	encodedMsg := dnet.EncodeMessageRaw(ChanFE, TagBuyOffer, c.feKey, data)

	err = encodedMsg.Send(c.sock)
	if err != nil {
		return err
	}

	return nil
}

func (c *DogeNetClient) GossipDeleteBuyOffer(hash string, publicKey string, signature string) error {
	message := protocol.DeleteBuyOfferMessage{
		Hash: hash,
	}

	envelope := protocol.DeleteBuyOfferMessageEnvelope{
		Type:      protocol.ACTION_DELETE_BUY_OFFER,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   &message,
		PublicKey: publicKey,
		Signature: signature,
	}

	data, err := proto.Marshal(&envelope)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	encodedMsg := dnet.EncodeMessageRaw(ChanFE, TagDeleteBuyOffer, c.feKey, data)

	err = encodedMsg.Send(c.sock)
	if err != nil {
		return err
	}

	return nil
}

func (c *DogeNetClient) recvBuyOffer(msg dnet.Message) {
	log.Printf("[FE] received buy offer message")
	ctx := context.Background()

	envelope := protocol.BuyOfferMessageEnvelope{}
	err := proto.Unmarshal(msg.Payload, &envelope)
	if err != nil {
		log.Println("Error deserializing message envelope:", err)
		return
	}

	if envelope.Type != protocol.ACTION_BUY_OFFER {
		log.Printf("[FE] unexpected action: [%s][%s][%d]", msg.Chan, msg.Tag, envelope.Type)
		return
	}

	offer := envelope.Payload

	buyOfferPayload := protocol.BuyOfferPayload{
		OffererAddress: offer.Payload.OffererAddress,
		SellerAddress:  offer.Payload.SellerAddress,
		MintHash:       offer.Payload.MintHash,
		Quantity:       offer.Payload.Quantity,
		Price:          offer.Payload.Price,
	}

	offerPayload, err := protojson.Marshal(&buyOfferPayload)
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

	if address != offer.Payload.OffererAddress {
		log.Println("Offerer address does not match public key")
		return
	}

	offerWithoutID := store.BuyOfferWithoutID{
		OffererAddress: offer.Payload.OffererAddress,
		SellerAddress:  offer.Payload.SellerAddress,
		Hash:           offer.Hash,
		MintHash:       offer.Payload.MintHash,
		Quantity:       int(offer.Payload.Quantity),
		Price:          int(offer.Payload.Price),
		CreatedAt:      offer.CreatedAt.AsTime(),
		PublicKey:      envelope.PublicKey,
		Signature:      envelope.Signature,
	}

	id, err := c.store.SaveBuyOffer(ctx, &offerWithoutID)
	if err != nil {
		log.Println("Error saving buy offer:", err)
		return
	}

	log.Printf("[FE] buy offer saved: %v", id)
}

func (c *DogeNetClient) recvDeleteBuyOffer(msg dnet.Message) {
	log.Printf("[FE] received delete buy offer message")
	ctx := context.Background()

	envelope := protocol.DeleteBuyOfferMessageEnvelope{}
	err := proto.Unmarshal(msg.Payload, &envelope)
	if err != nil {
		log.Println("Error deserializing message envelope:", err)
		return
	}

	if envelope.Type != protocol.ACTION_DELETE_BUY_OFFER {
		log.Printf("[FE] unexpected action: [%s][%s][%d]", msg.Chan, msg.Tag, envelope.Type)
		return
	}

	message := envelope.Payload

	err = doge.ValidateSignature([]byte(message.Hash), envelope.PublicKey, envelope.Signature)
	if err != nil {
		log.Println("Error validating signature:", err)
		return
	}

	err = c.store.DeleteBuyOffer(ctx, message.Hash, envelope.PublicKey)
	if err != nil {
		log.Println("Error deleting buy offer:", err)
		return
	}

	log.Printf("[FE] buy offer deleted: %v", message.Hash)
}
