package rpc

import (
	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/dogenet"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol/protocolconnect"
	"dogecoin.org/fractal-engine/pkg/store"
)

type ConnectRpcService struct {
	protocolconnect.UnimplementedFractalEngineRpcServiceHandler
	store        *store.TokenisationStore
	gossipClient dogenet.GossipClient
	cfg          *config.Config
	dogeClient   *doge.RpcClient
}

func NewConnectRpcService(store *store.TokenisationStore, gossipClient dogenet.GossipClient, cfg *config.Config, dogeClient *doge.RpcClient) *ConnectRpcService {
	return &ConnectRpcService{
		store:        store,
		gossipClient: gossipClient,
		cfg:          cfg,
		dogeClient:   dogeClient,
	}
}
