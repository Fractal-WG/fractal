package rpc

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
)

func (s *ConnectRpcService) GetPendingTokenBalances(_ context.Context, req *connect.Request[protocol.GetPendingTokenBalancesRequest]) (*connect.Response[protocol.GetPendingTokenBalancesResponse], error) {
	address := req.Msg.GetAddress()
	if address == nil || address.GetValue() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("address is required"))
	}

	mintHash := ""
	if req.Msg.GetMintHash() != nil {
		mintHash = req.Msg.GetMintHash().GetValue()
	}

	tokenBalances, err := s.store.GetPendingTokenBalances(address.GetValue(), mintHash)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	responseBalances := make([]*protocol.TokenBalance, 0, len(tokenBalances))
	for _, balance := range tokenBalances {
		responseBalances = append(responseBalances, toProtoTokenBalance(balance))
	}

	resp := &protocol.GetPendingTokenBalancesResponse{}
	resp.Balances = responseBalances
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) GetTokenBalances(_ context.Context, req *connect.Request[protocol.GetTokenBalancesRequest]) (*connect.Response[protocol.GetTokenBalancesResponse], error) {
	address := req.Msg.GetAddress()
	if address == nil || address.GetValue() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("address is required"))
	}

	includeMintDetails := false
	if req.Msg.GetIncludeMintDetails() != nil {
		includeMintDetails = req.Msg.GetIncludeMintDetails().Value
	}

	mintHash := ""
	if req.Msg.GetMintHash() != nil {
		mintHash = req.Msg.GetMintHash().GetValue()
	}

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
		end := int(start + int(limit))

		tokenBalances, err := s.store.GetMyMintTokenBalances(address.GetValue(), start, end)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		var responseData interface{}
		if start >= len(tokenBalances) {
			responseData = GetTokenBalanceWithMintsResponse{}
		} else {
			if end > len(tokenBalances) {
				end = len(tokenBalances)
			}
			responseData = GetTokenBalanceWithMintsResponse{
				Mints: tokenBalances[start:end],
				Total: len(tokenBalances),
				Page:  int(page),
				Limit: int(limit),
			}
		}

		data, err := toStructPB(responseData)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}

		resp := &protocol.GetTokenBalancesResponse{}
		resp.Data = data
		return connect.NewResponse(resp), nil
	}

	tokenBalances, err := s.store.GetTokenBalances(address.GetValue(), mintHash)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	data, err := toStructPB(map[string]interface{}{"balances": tokenBalances})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.GetTokenBalancesResponse{}
	resp.Data = data
	return connect.NewResponse(resp), nil
}
