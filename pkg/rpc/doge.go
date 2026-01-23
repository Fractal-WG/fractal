package rpc

import (
	"context"
	"encoding/json"
	"errors"

	connect "connectrpc.com/connect"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
)

func (s *ConnectRpcService) DogeConfirm(_ context.Context, _ *connect.Request[protocol.DogeConfirmRequest]) (*connect.Response[protocol.DogeConfirmResponse], error) {
	_, err := s.dogeClient.Request("generate", []interface{}{10})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.DogeConfirmResponse{}
	resp.Values = map[string]string{}
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) DogeSend(_ context.Context, req *connect.Request[protocol.DogeSendRequest]) (*connect.Response[protocol.DogeSendResponse], error) {
	res, err := s.dogeClient.Request("sendrawtransaction", []interface{}{req.Msg.GetEncodedTransactionHex(), true})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	var txid string
	if err := json.Unmarshal(*res, &txid); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	resp := &protocol.DogeSendResponse{}
	resp.TransactionId = txid
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) DogeTopUp(_ context.Context, req *connect.Request[protocol.DogeTopUpRequest]) (*connect.Response[protocol.DogeTopUpResponse], error) {
	address := req.Msg.GetAddress()
	if address == nil || address.GetValue() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("address is required"))
	}

	if _, err := s.dogeClient.Generate(101); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if _, err := s.dogeClient.SendToAddress(address.GetValue(), 1000); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if _, err := s.dogeClient.Generate(1); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.DogeTopUpResponse{}
	resp.Value = address.GetValue()
	return connect.NewResponse(resp), nil
}
