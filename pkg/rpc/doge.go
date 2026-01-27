package rpc

import (
	"context"
	"encoding/json"
	"errors"

	connect "connectrpc.com/connect"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
)

func (s *ConnectRpcService) DogeConfirm(ctx context.Context, _ *connect.Request[protocol.DogeConfirmRequest]) (*connect.Response[protocol.DogeConfirmResponse], error) {
	_, err := s.dogeClient.Request(ctx, "generate", []interface{}{10})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.DogeConfirmResponse{}
	resp.SetValues(map[string]string{})
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) DogeSend(ctx context.Context, req *connect.Request[protocol.DogeSendRequest]) (*connect.Response[protocol.DogeSendResponse], error) {
	res, err := s.dogeClient.Request(ctx, "sendrawtransaction", []interface{}{req.Msg.GetEncodedTransactionHex(), true})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var txid string
	if err := json.Unmarshal(*res, &txid); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	resp := &protocol.DogeSendResponse{}
	resp.SetTransactionId(txid)
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) DogeTopUp(ctx context.Context, req *connect.Request[protocol.DogeTopUpRequest]) (*connect.Response[protocol.DogeTopUpResponse], error) {
	address := req.Msg.GetAddress()
	if address == nil || address.GetValue() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("address is required"))
	}

	if _, err := s.dogeClient.Generate(ctx, 101); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if _, err := s.dogeClient.SendToAddress(ctx, address.GetValue(), 1000); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if _, err := s.dogeClient.Generate(ctx, 1); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.DogeTopUpResponse{}
	resp.SetValue(address.GetValue())
	return connect.NewResponse(resp), nil
}
