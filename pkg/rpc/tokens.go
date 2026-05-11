package rpc

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
)

func (s *ConnectRpcService) GetPendingTokenBalances(ctx context.Context, req *connect.Request[protocol.GetPendingTokenBalancesRequest]) (*connect.Response[protocol.GetPendingTokenBalancesResponse], error) {
	address := req.Msg.GetAddress()
	if address == nil || address.GetValue() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("address is required"))
	}

	mintHash := ""
	if req.Msg.GetMintHash() != nil {
		mintHash = req.Msg.GetMintHash().GetValue()
	}

	tokenBalances, err := s.store.GetPendingTokenBalances(ctx, address.GetValue(), mintHash)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	responseBalances := make([]*protocol.TokenBalance, 0, len(tokenBalances))
	for _, balance := range tokenBalances {
		responseBalances = append(responseBalances, toProtoTokenBalance(balance))
	}

	resp := &protocol.GetPendingTokenBalancesResponse{}
	resp.SetBalances(responseBalances)
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) GetTokenBalances(ctx context.Context, req *connect.Request[protocol.GetTokenBalancesRequest]) (*connect.Response[protocol.GetTokenBalancesResponse], error) {
	address := req.Msg.GetAddress()
	if address == nil || address.GetValue() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("address is required"))
	}

	includeMintDetails := false
	if req.Msg.GetIncludeMintDetails() != nil {
		includeMintDetails = req.Msg.GetIncludeMintDetails().Value
	}

	resp := &protocol.GetTokenBalancesResponse{}

	if includeMintDetails {
		limit := int32(100)
		if req.Msg.GetLimit() != nil && req.Msg.GetLimit().GetValue() > 0 && req.Msg.GetLimit().GetValue() <= limit {
			limit = req.Msg.GetLimit().GetValue()
		}

		page := int32(0)
		if req.Msg.GetPage() != nil && req.Msg.GetPage().GetValue() > 0 && req.Msg.GetPage().GetValue() <= 1000 {
			page = req.Msg.GetPage().GetValue()
		}

		start := int(page * limit)

		tokenBalances, err := s.store.GetMyMintTokenBalances(ctx, address.GetValue(), start, int(limit))
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		protoMints := make([]*protocol.TokenBalanceWithMint, 0, len(tokenBalances))
		for _, balance := range tokenBalances {
			protoBalance, err := toProtoTokenBalanceWithMint(balance)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
			protoMints = append(protoMints, protoBalance)
		}

		resp.SetMints(protoMints)
		resp.SetTotal(int32(len(tokenBalances)))
		resp.SetPage(page)
		resp.SetLimit(limit)

		return connect.NewResponse(resp), nil
	}

	mintHash := ""
	if req.Msg.GetMintHash() != nil {
		mintHash = req.Msg.GetMintHash().GetValue()
	}

	tokenBalances, err := s.store.GetTokenBalances(ctx, address.GetValue(), mintHash)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	responseBalances := make([]*protocol.TokenBalance, 0, len(tokenBalances))
	for _, balance := range tokenBalances {
		responseBalances = append(responseBalances, toProtoTokenBalance(balance))
	}
	resp.SetBalances(responseBalances)

	return connect.NewResponse(resp), nil
}
