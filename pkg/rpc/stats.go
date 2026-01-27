package rpc

import (
	"context"

	connect "connectrpc.com/connect"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
)


func (s *ConnectRpcService) GetStats(ctx context.Context, _ *connect.Request[protocol.GetStatsRequest]) (*connect.Response[protocol.GetStatsResponse], error) {
	stats, err := s.store.GetStats(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	responseStats := make(map[string]int32, len(stats))
	for key, value := range stats {
		responseStats[key] = int32(value)
	}

	resp := &protocol.GetStatsResponse{}
	resp.SetStats(responseStats)
	return connect.NewResponse(resp), nil
}
