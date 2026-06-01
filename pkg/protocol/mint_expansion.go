package protocol

import (
	"encoding/hex"

	"google.golang.org/protobuf/proto"
)

func NewMintExpansionTransactionEnvelope(mintHash string, expansionHash string, additionalSupply int32) MessageEnvelope {
	mintHashBytes, err := hex.DecodeString(mintHash)
	if err != nil {
		return MessageEnvelope{}
	}

	expansionHashBytes, err := hex.DecodeString(expansionHash)
	if err != nil {
		return MessageEnvelope{}
	}

	message := &OnChainMintExpansionMessage{
		MintHash:         mintHashBytes,
		AdditionalSupply: additionalSupply,
		ExpansionHash:    expansionHashBytes,
	}

	protoBytes, err := proto.Marshal(message)
	if err != nil {
		return MessageEnvelope{}
	}

	return NewMessageEnvelope(ACTION_MINT_EXPANSION, DEFAULT_VERSION, protoBytes)
}
