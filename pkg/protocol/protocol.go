package protocol

import (
	"bytes"
	"encoding/binary"

	structpb "google.golang.org/protobuf/types/known/structpb"
)

// Version
// PROTOBUF
// 1.0.0

const (
	FRACTAL_ENGINE_IDENTIFIER = 0xFE0001FE
	DEFAULT_VERSION           = 1
	ACTION_MINT               = 0x01
	ACTION_BUY_OFFER          = 0x02
	ACTION_SELL_OFFER         = 0x03
	ACTION_INVOICE            = 0x04
	ACTION_PAYMENT            = 0x05
	ACTION_DELETE_BUY_OFFER   = 0x06
	ACTION_DELETE_SELL_OFFER  = 0x07
	ACTION_INVOICE_SIGNATURE  = 0x08
	ACTION_MINT_EXPANSION     = 0x09
	ACTION_TOKEN_BURN         = 0x0A
)

type MessageEnvelope struct {
	EngineIdentifier uint32
	Action           uint8
	Version          uint8
	Data             []byte
}

func NewMessageEnvelope(action uint8, version uint8, data []byte) MessageEnvelope {
	return MessageEnvelope{
		EngineIdentifier: FRACTAL_ENGINE_IDENTIFIER,
		Action:           action,
		Version:          version,
		Data:             data,
	}
}

func (m *MessageEnvelope) IsFractalEngineMessage() bool {
	return m.EngineIdentifier == FRACTAL_ENGINE_IDENTIFIER
}

func (m *MessageEnvelope) Serialize() []byte {
	bufIdentifier := make([]byte, 4)
	binary.BigEndian.PutUint32(bufIdentifier, m.EngineIdentifier)

	buf := new(bytes.Buffer)
	buf.Write(bufIdentifier)
	buf.WriteByte(m.Action)
	buf.WriteByte(m.Version)
	buf.Write(m.Data)

	return buf.Bytes()
}

func (m *MessageEnvelope) Deserialize(data []byte) error {
	buf := bytes.NewBuffer(data)

	bufIdentifier := make([]byte, 4)
	buf.Read(bufIdentifier)
	m.EngineIdentifier = binary.BigEndian.Uint32(bufIdentifier)

	action, err := buf.ReadByte()
	if err != nil {
		return err
	}

	m.Action = action

	version, err := buf.ReadByte()
	if err != nil {
		return err
	}

	m.Version = version

	m.Data = buf.Bytes()
	return nil
}

func ConvertToStructPBMap(m map[string]interface{}) map[string]*structpb.Value {
	fields := make(map[string]*structpb.Value)
	for k, v := range m {
		fields[k] = &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: v.(string)}}
	}
	return fields
}
