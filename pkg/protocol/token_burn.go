package protocol

import (
	"encoding/hex"

	"google.golang.org/protobuf/proto"
)

func NewTokenBurnTransactionEnvelope(mintHash, burnHash string, burnQuantity int32) MessageEnvelope {
	mintHashBytes, err := hex.DecodeString(mintHash)
	if err != nil {
		return MessageEnvelope{}
	}

	burnHashBytes, err := hex.DecodeString(burnHash)
	if err != nil {
		return MessageEnvelope{}
	}

	message := &OnChainTokenBurnMessage{
		MintHash:     mintHashBytes,
		BurnQuantity: burnQuantity,
		BurnHash:     burnHashBytes,
	}

	protoBytes, err := proto.Marshal(message)
	if err != nil {
		return MessageEnvelope{}
	}

	return NewMessageEnvelope(ACTION_TOKEN_BURN, DEFAULT_VERSION, protoBytes)
}
