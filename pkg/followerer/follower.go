package followerer

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"

	fecfg "dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"

	"github.com/dogecoinfoundation/chainfollower/pkg/chainfollower"
	"github.com/dogecoinfoundation/chainfollower/pkg/config"
	"github.com/dogecoinfoundation/chainfollower/pkg/messages"
	"github.com/dogecoinfoundation/chainfollower/pkg/rpc"
	"github.com/dogecoinfoundation/chainfollower/pkg/state"
	"github.com/dogecoinfoundation/chainfollower/pkg/types"
)

type DogeFollower struct {
	cfg           *fecfg.Config
	store         *store.TokenisationStore
	chainfollower chainfollower.ChainFollowerInterface
	Running       bool
	msgChan       chan messages.Message
	context       context.Context
	cancel        context.CancelFunc
	rpcClient     rpc.RpcTransportInterface
}

func NewFollower(cfg *fecfg.Config, store *store.TokenisationStore) *DogeFollower {
	rpcClient := rpc.NewRpcTransport(&config.Config{
		RpcUrl:  cfg.DogeScheme + "://" + cfg.DogeHost + ":" + cfg.DogePort,
		RpcUser: cfg.DogeUser,
		RpcPass: cfg.DogePassword,
	})

	chainfollower := chainfollower.NewChainFollower(rpcClient)

	ctx, cancel := context.WithCancel(context.Background())

	return &DogeFollower{cfg: cfg, store: store, chainfollower: chainfollower, Running: false, context: ctx, cancel: cancel}
}

func NewFollowerWithCustomChainFollower(cfg *fecfg.Config, store *store.TokenisationStore, chainfollower chainfollower.ChainFollowerInterface) *DogeFollower {
	ctx, cancel := context.WithCancel(context.Background())
	return &DogeFollower{cfg: cfg, store: store, chainfollower: chainfollower, Running: false, context: ctx, cancel: cancel}
}

func (f *DogeFollower) Start() error {
	f.Running = true

	blockHeight, blockHash, _, err := f.store.GetChainPosition(f.context)
	if err != nil {
		return err
	}

	f.msgChan = f.chainfollower.Start(&state.ChainPos{
		BlockHash:   blockHash,
		BlockHeight: blockHeight,
	})

	for {
		select {
		case <-f.context.Done():
			fmt.Println("Exiting follower")
			return nil
		case msg := <-f.msgChan:

			switch msg := msg.(type) {
			case messages.BlockMessage:
				transactionNumber := 0
				for _, tx := range msg.Block.Tx {
					fractalMessage, err := GetFractalMessageFromVout(tx.VOut)
					if err != nil {
						continue
					}

					address, err := GetAddressFromVout(tx.VOut)
					if err != nil {
						continue
					}

					addressValues := make(map[string]interface{})
					for _, vout := range tx.VOut {
						if len(vout.ScriptPubKey.Addresses) == 1 {
							addy := vout.ScriptPubKey.Addresses[0]
							value := vout.Value.InexactFloat64()
							if _, ok := addressValues[addy]; !ok {
								addressValues[addy] = value
							} else {
								addressValues[addy] = addressValues[addy].(float64) + value
							}
						}
					}

					_, err = f.store.SaveOnChainTransaction(f.context, tx.Hash, msg.Block.Height, blockHash, transactionNumber, fractalMessage.Action, fractalMessage.Version, fractalMessage.Data, address, addressValues)
					if err != nil {
						log.Println("Error saving on chain transaction:", err)
					}

					transactionNumber++
				}

				if f.cfg.PersistFollower {
					err := f.store.UpsertChainPosition(f.context, msg.ChainPos.BlockHeight, msg.ChainPos.BlockHash, msg.ChainPos.WaitingForNextHash)
					if err != nil {
						log.Println("Error setting chain position:", err)
					}
				}

			case messages.RollbackMessage:
				log.Println("Received rollback message from chainfollower:")
				if f.cfg.PersistFollower {
					err := f.store.UpsertChainPosition(f.context, msg.NewChainPos.BlockHeight, msg.NewChainPos.BlockHash, msg.NewChainPos.WaitingForNextHash)
					if err != nil {
						log.Println("Error setting chain position:", err)
					}
				}

			default:
				log.Printf("Received unknown message from chainfollower: %v\n", msg)
			}
		}
	}
}

func GetFractalMessageFromVout(vout []types.RawTxnVOut) (protocol.MessageEnvelope, error) {
	var bytes []byte
	for _, vout := range vout {
		bytes = ParseOpReturnData(vout)
		if bytes == nil {
			continue
		}

		message := protocol.MessageEnvelope{}
		err := message.Deserialize(bytes)
		if err != nil {
			log.Println("Error deserializing message envelope:", err)
			continue
		}

		if message.IsFractalEngineMessage() {
			return message, nil
		}
	}

	return protocol.MessageEnvelope{}, errors.New("no fractal engine message")
}

func GetAddressFromVout(vout []types.RawTxnVOut) (string, error) {
	for _, vout := range vout {
		if vout.ScriptPubKey.Type == "pubkeyhash" {
			return vout.ScriptPubKey.Addresses[0], nil
		}
	}

	return "", errors.New("no address found")
}

func ParseOpReturnData(vout types.RawTxnVOut) []byte {
	asm := vout.ScriptPubKey.Asm
	parts := strings.Split(asm, " ")

	if len(parts) > 0 {
		op := parts[0]

		if op == "OP_RETURN" {
			bytes, err := hex.DecodeString(parts[1])
			if err != nil {
				return nil
			}

			return bytes
		}
	}

	return nil
}

func (f *DogeFollower) Stop() {
	fmt.Println("Stopping follower")
	if f.Running {
		f.chainfollower.Stop()
		f.cancel()
		f.Running = false
	}

}
