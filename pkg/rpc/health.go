package rpc

import (
	"context"
	"database/sql"
	"errors"
	"time"

	connect "connectrpc.com/connect"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/version"
)

func (s *ConnectRpcService) GetHealth(_ context.Context, _ *connect.Request[protocol.GetHealthRequest]) (*connect.Response[protocol.GetHealthResponse], error) {
	currentBlockHeight, latestBlockHeight, chain, walletsEnabled, updatedAt, err := s.store.GetHealth()
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.GetHealthResponse{}
	resp.Chain = chain
	resp.CurrentBlockHeight = int32(currentBlockHeight)
	resp.LatestBlockHeight = int32(latestBlockHeight)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339Nano)
	resp.Version = version.Version
	resp.WalletsEnabled = walletsEnabled
	return connect.NewResponse(resp), nil
}
