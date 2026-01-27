package followerer_test

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	test_support "dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/followerer"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"github.com/dogecoinfoundation/chainfollower/pkg/chainfollower"
	"github.com/dogecoinfoundation/chainfollower/pkg/messages"
	"github.com/dogecoinfoundation/chainfollower/pkg/state"
	"github.com/dogecoinfoundation/chainfollower/pkg/types"
	"github.com/shopspring/decimal"
	"gotest.tools/assert"
)

type FakeChainFollower struct {
	chainfollower.ChainFollowerInterface
	Messages chan messages.Message
}

func (f *FakeChainFollower) Start(chainState *state.ChainPos) chan messages.Message {
	return f.Messages
}

func (f *FakeChainFollower) Stop() {
	close(f.Messages)
}

func TestDogeFollower(t *testing.T) {
	tokenisationStore := test_support.SetupTestDB(t)
	ctx := context.Background()

	chainFollower := &FakeChainFollower{
		Messages: make(chan messages.Message),
	}

	dogeFollower := followerer.NewFollowerWithCustomChainFollower(&config.Config{}, tokenisationStore, chainFollower)
	go dogeFollower.Start()

	hash := "MyMintHash123"
	envelope := protocol.NewMintTransactionEnvelope(hash, protocol.ACTION_MINT)
	encodedTransactionBody := envelope.Serialize()

	chainFollower.Messages <- messages.BlockMessage{
		Block: &types.Block{
			Hash:   "1234567890",
			Height: 99,
			Tx: []types.RawTxn{
				{
					Hash: "TX123213123123",
					VOut: []types.RawTxnVOut{
						{
							ScriptPubKey: types.RawTxnScriptPubKey{
								Type:      "pubkeyhash",
								Addresses: []string{"1234567890"},
								Asm:       "OP_RETURN " + hex.EncodeToString(encodedTransactionBody),
							},
							Value: decimal.NewFromInt(100),
						},
					},
				},
			},
		},
		ChainPos: &state.ChainPos{
			BlockHash:   "1234567890",
			BlockHeight: 100,
		},
	}

	time.Sleep(1 * time.Second)

	transactions, err := tokenisationStore.GetOnChainTransactions(ctx, 0, 100)
	if err != nil {
		t.Fatalf("Failed to get on chain transactions: %v", err)
	}

	mintMessage := protocol.MessageEnvelope{}
	err = mintMessage.Deserialize(encodedTransactionBody)
	if err != nil {
		t.Fatalf("Failed to deserialize envelope: %v", err)
	}

	assert.Equal(t, 1, len(transactions))
	assert.Equal(t, "TX123213123123", transactions[0].TxHash)
	assert.Equal(t, int64(99), transactions[0].Height)
	assert.Equal(t, uint8(protocol.ACTION_MINT), transactions[0].ActionType)
	assert.Equal(t, uint8(protocol.DEFAULT_VERSION), transactions[0].ActionVersion)
	assert.Equal(t, hex.EncodeToString(mintMessage.Data), hex.EncodeToString(transactions[0].ActionData))
	assert.Equal(t, "1234567890", transactions[0].Address)
	assert.Assert(t, transactions[0].Values.Equal(store.StringInterfaceMap{"1234567890": 100}))

}
