package rpc

import (
	"context"
	"encoding/hex"

	connect "connectrpc.com/connect"
	engineprotocol "dogecoin.org/fractal-engine/pkg/protocol"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
)

func (s *ConnectRpcService) CreateNewPayment(_ context.Context, req *connect.Request[protocol.CreateNewPaymentRequest]) (*connect.Response[protocol.CreateNewPaymentResponse], error) {
	envelope := engineprotocol.NewPaymentTransactionEnvelope(req.Msg.GetInvoiceHash().GetValue(), engineprotocol.ACTION_PAYMENT)
	encodedTransactionBody := envelope.Serialize()

	resp := &protocol.CreateNewPaymentResponse{}
	resp.Values = map[string]string{
		"encoded_transaction_body": hex.EncodeToString(encodedTransactionBody),
	}
	return connect.NewResponse(resp), nil
}
