package dogenet

import (
	"context"
	"log"

	"code.dogecoin.org/gossip/dnet"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (c *DogeNetClient) GossipMint(record store.Mint) error {
	var assetManagers []*protocol.AssetManager
	for _, assetManager := range record.AssetManagers {
		assetManagers = append(assetManagers, &protocol.AssetManager{
			Name:      assetManager.Name,
			PublicKey: assetManager.PublicKey,
			Url:       assetManager.URL,
		})
	}
	mintMessage := protocol.MintMessage{
		Title:                    record.Title,
		Description:              record.Description,
		FractionCount:            int32(record.FractionCount),
		Tags:                     record.Tags,
		TransactionHash:          record.TransactionHash,
		Metadata:                 &structpb.Struct{Fields: convertToStructPBMap(record.Metadata)},
		Hash:                     record.Hash,
		Requirements:             &structpb.Struct{Fields: convertToStructPBMap(record.Requirements)},
		LockupOptions:            &structpb.Struct{Fields: convertToStructPBMap(record.LockupOptions)},
		FeedUrl:                  record.FeedURL,
		CreatedAt:                timestamppb.New(record.CreatedAt),
		ContractOfSale:           record.ContractOfSale,
		OwnerAddress:             record.OwnerAddress,
		SignatureRequirementType: string(record.SignatureRequirementType),
		AssetManagers:            assetManagers,
		MinSignatures:            int32(record.MinSignatures),
	}

	envelope := protocol.MintMessageEnvelope{
		Type:      protocol.ACTION_MINT,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   &mintMessage,
		PublicKey: record.PublicKey,
		Signature: record.Signature,
	}

	data, err := proto.Marshal(&envelope)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	encodedMsg := dnet.EncodeMessageRaw(ChanFE, TagMint, c.feKey, data)

	err = encodedMsg.Send(c.sock)
	if err != nil {
		return err
	}

	return nil
}

func (c *DogeNetClient) recvMint(msg dnet.Message) {
	log.Printf("[FE] received mint message")
	ctx := context.Background()

	envelope := protocol.MintMessageEnvelope{}
	err := proto.Unmarshal(msg.Payload, &envelope)
	if err != nil {
		log.Println("Error deserializing message envelope:", err)
		return
	}

	if envelope.Type != protocol.ACTION_MINT {
		log.Printf("[FE] unexpected action: [%s][%s][%d]", msg.Chan, msg.Tag, envelope.Type)
		return
	}

	mintMessage := envelope.Payload

	var assetManagers store.AssetManagers
	for _, assetManager := range mintMessage.AssetManagers {
		assetManagers = append(assetManagers, store.AssetManager{
			Name:      assetManager.Name,
			PublicKey: assetManager.PublicKey,
			URL:       assetManager.Url,
		})
	}

	mintRecord := &store.MintWithoutID{
		Hash:                     mintMessage.Hash,
		Title:                    mintMessage.Title,
		FractionCount:            int(mintMessage.FractionCount),
		Description:              mintMessage.Description,
		Tags:                     mintMessage.Tags,
		Metadata:                 mintMessage.Metadata.AsMap(),
		TransactionHash:          mintMessage.TransactionHash,
		CreatedAt:                mintMessage.CreatedAt.AsTime(),
		Requirements:             mintMessage.Requirements.AsMap(),
		LockupOptions:            mintMessage.LockupOptions.AsMap(),
		PublicKey:                envelope.PublicKey,
		Signature:                envelope.Signature,
		FeedURL:                  mintMessage.FeedUrl,
		ContractOfSale:           mintMessage.ContractOfSale,
		OwnerAddress:             mintMessage.OwnerAddress,
		SignatureRequirementType: store.SignatureRequirementType(mintMessage.SignatureRequirementType),
		AssetManagers:            assetManagers,
		MinSignatures:            int(mintMessage.MinSignatures),
	}

	mintSignaturePayload := protocol.MintMessage{
		Title:                    mintRecord.Title,
		Description:              mintRecord.Description,
		FractionCount:            int32(mintRecord.FractionCount),
		Tags:                     mintRecord.Tags,
		Metadata:                 &structpb.Struct{Fields: convertToStructPBMap(mintRecord.Metadata)},
		Requirements:             &structpb.Struct{Fields: convertToStructPBMap(mintRecord.Requirements)},
		LockupOptions:            &structpb.Struct{Fields: convertToStructPBMap(mintRecord.LockupOptions)},
		FeedUrl:                  mintRecord.FeedURL,
		ContractOfSale:           mintRecord.ContractOfSale,
		OwnerAddress:             mintRecord.OwnerAddress,
		SignatureRequirementType: mintMessage.SignatureRequirementType,
		AssetManagers:            mintMessage.AssetManagers,
		MinSignatures:            mintMessage.MinSignatures,
	}

	if len(mintRecord.Metadata) == 0 {
		mintSignaturePayload.Metadata = nil
	}

	if len(mintRecord.Requirements) == 0 {
		mintSignaturePayload.Requirements = nil
	}

	if len(mintRecord.LockupOptions) == 0 {
		mintSignaturePayload.LockupOptions = nil
	}

	err = doge.ValidateSignature(&mintSignaturePayload, envelope.PublicKey, envelope.Signature)
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

	if address != mintRecord.OwnerAddress {
		log.Println("Mint owner address does not match public key")
		return
	}

	id, err := c.store.SaveUnconfirmedMint(ctx, mintRecord)

	if err != nil {
		log.Println("Error saving unconfirmed mint:", err)
		return
	}

	log.Printf("[FE] unconfirmed mint saved: %v", id)
}
