package client

import (
	"context"

	"connectrpc.com/connect"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol"
)

func (c *TokenisationClient) TopUpBalance(ctx context.Context, address string) error {
	addr := &protocol.Address{}
	addr.SetValue(address)

	req := &protocol.DogeTopUpRequest{}
	req.SetAddress(addr)

	_, err := c.client.DogeTopUp(ctx, connect.NewRequest(req))
	return err
}
