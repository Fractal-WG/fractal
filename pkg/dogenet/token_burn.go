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

// tokenBurnSignaturePayload mirrors rpc.BurnTokensRequestPayload for signature validation.
type tokenBurnSignaturePayload struct {
	MintHash     string `json:"mint_hash"`
	BurnQuantity int    `json:"burn_quantity"`
}

func (c *DogeNetClient) GossipTokenBurn(record store.UnconfirmedTokenBurn) error {
	burnMessage := protocol.TokenBurnMessage{
		Hash:          record.Hash,
		MintHash:      record.MintHash,
		BurnQuantity:  int32(record.BurnQuantity),
		BurnerAddress: record.BurnerAddress,
		CreatedAt:     timestamppb.New(record.CreatedAt),
	}

	envelope := protocol.TokenBurnMessageEnvelope{
		Type:      protocol.ACTION_TOKEN_BURN,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   &burnMessage,
		PublicKey: record.PublicKey,
		Signature: record.Signature,
	}

	data, err := proto.Marshal(&envelope)
	if err != nil {
		log.Fatalf("Failed to marshal token burn envelope: %v", err)
	}

	encodedMsg := dnet.EncodeMessageRaw(ChanFE, TagTokenBurn, c.feKey, data)

	return encodedMsg.Send(c.sock)
}

func (c *DogeNetClient) recvTokenBurn(msg dnet.Message) {
	log.Printf("[FE] received token burn message")
	ctx := context.Background()

	envelope := protocol.TokenBurnMessageEnvelope{}
	err := proto.Unmarshal(msg.Payload, &envelope)
	if err != nil {
		log.Println("Error deserializing token burn envelope:", err)
		return
	}

	if envelope.Type != protocol.ACTION_TOKEN_BURN {
		log.Printf("[FE] unexpected action in token burn: [%s][%s][%d]", msg.Chan, msg.Tag, envelope.Type)
		return
	}

	burn := envelope.Payload

	sigPayload := tokenBurnSignaturePayload{
		MintHash:     burn.MintHash,
		BurnQuantity: int(burn.BurnQuantity),
	}

	err = doge.ValidateSignature(sigPayload, envelope.PublicKey, envelope.Signature)
	if err != nil {
		log.Println("Error validating token burn signature:", err)
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

	if address != burn.BurnerAddress {
		log.Println("Token burn burner address does not match public key")
		return
	}

	record := &store.UnconfirmedTokenBurn{
		Hash:          burn.Hash,
		MintHash:      burn.MintHash,
		BurnQuantity:  int(burn.BurnQuantity),
		BurnerAddress: burn.BurnerAddress,
		PublicKey:     envelope.PublicKey,
		Signature:     envelope.Signature,
		CreatedAt:     burn.CreatedAt.AsTime(),
	}

	id, err := c.store.SaveUnconfirmedTokenBurn(ctx, record)
	if err != nil {
		log.Println("Error saving unconfirmed token burn:", err)
		return
	}

	log.Printf("[FE] unconfirmed token burn saved: %v", id)
}
