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

// mintExpansionSignaturePayload mirrors rpc.ExpandMintRequestPayload for signature validation.
type mintExpansionSignaturePayload struct {
	MintHash         string `json:"mint_hash"`
	AdditionalSupply int    `json:"additional_supply"`
}

func (c *DogeNetClient) GossipMintExpansion(record store.UnconfirmedMintExpansion) error {
	expansionMessage := protocol.MintExpansionMessage{
		Hash:             record.Hash,
		MintHash:         record.MintHash,
		AdditionalSupply: int32(record.AdditionalSupply),
		OwnerAddress:     record.OwnerAddress,
		CreatedAt:        timestamppb.New(record.CreatedAt),
		Nonce:            record.Nonce,
	}

	envelope := protocol.MintExpansionMessageEnvelope{
		Type:      protocol.ACTION_MINT_EXPANSION,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   &expansionMessage,
		PublicKey: record.PublicKey,
		Signature: record.Signature,
	}

	data, err := proto.Marshal(&envelope)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	encodedMsg := dnet.EncodeMessageRaw(ChanFE, TagMintExpansion, c.feKey, data)

	err = encodedMsg.Send(c.sock)
	if err != nil {
		return err
	}

	return nil
}

func (c *DogeNetClient) recvMintExpansion(msg dnet.Message) {
	log.Printf("[FE] received mint expansion message")
	ctx := context.Background()

	envelope := protocol.MintExpansionMessageEnvelope{}
	err := proto.Unmarshal(msg.Payload, &envelope)
	if err != nil {
		log.Println("Error deserializing message envelope:", err)
		return
	}

	if envelope.Type != protocol.ACTION_MINT_EXPANSION {
		log.Printf("[FE] unexpected action: [%s][%s][%d]", msg.Chan, msg.Tag, envelope.Type)
		return
	}

	expansion := envelope.Payload

	if expansion == nil {
		log.Println("Error nil payload:", err)
		return
	}

	sigPayload := mintExpansionSignaturePayload{
		MintHash:         expansion.MintHash,
		AdditionalSupply: int(expansion.AdditionalSupply),
	}

	err = doge.ValidateSignature(sigPayload, envelope.PublicKey, envelope.Signature)
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

	if address != expansion.OwnerAddress {
		log.Println("Mint expansion owner address does not match public key")
		return
	}

	mint, err := c.store.GetMintByHash(ctx, expansion.MintHash)
	if err != nil || mint.Hash == "" {
		log.Println("Mint not found for expansion:", expansion.MintHash)
		return
	}

	if !mint.AllowExpansion {
		log.Println("Mint does not allow expansion:", expansion.MintHash)
		return
	}

	if mint.OwnerAddress != expansion.OwnerAddress {
		log.Println("Mint expansion owner address does not match mint owner")
		return
	}

	record := &store.UnconfirmedMintExpansion{
		Hash:             expansion.Hash,
		MintHash:         expansion.MintHash,
		AdditionalSupply: int(expansion.AdditionalSupply),
		OwnerAddress:     expansion.OwnerAddress,
		PublicKey:        envelope.PublicKey,
		Signature:        envelope.Signature,
		Nonce:            expansion.Nonce,
		CreatedAt:        expansion.CreatedAt.AsTime(),
	}

	expectedHash, err := record.GenerateHash()
	if err != nil {
		log.Println("Error generating mint expansion hash:", err)
		return
	}
	if record.Hash != expectedHash {
		log.Println("Mint expansion hash mismatch")
		return
	}

	id, err := c.store.SaveUnconfirmedMintExpansion(ctx, record)
	if err != nil {
		log.Println("Error saving unconfirmed mint expansion:", err)
		return
	}

	log.Printf("[FE] unconfirmed mint expansion saved: %v", id)
}
