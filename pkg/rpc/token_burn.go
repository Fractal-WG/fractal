package rpc

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	connect "connectrpc.com/connect"
	"dogecoin.org/fractal-engine/pkg/doge"
	engineprotocol "dogecoin.org/fractal-engine/pkg/protocol"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"dogecoin.org/fractal-engine/pkg/validation"
)

func (s *ConnectRpcService) BurnTokens(ctx context.Context, req *connect.Request[protocol.BurnTokensRequest]) (*connect.Response[protocol.BurnTokensResponse], error) {
	request, err := toBurnTokensRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := request.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	prefix, err := doge.GetPrefix(s.cfg.DogeNetChain)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("chain config error: %w", err))
	}

	burnerAddress, err := doge.PublicKeyToDogeAddress(request.PublicKey, prefix)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid public_key: %w", err))
	}

	mint, err := s.store.GetMintByHash(ctx, request.Payload.MintHash)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("mint not found: %w", err))
	}

	if mint.Hash == "" {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("mint not found"))
	}

	if !mint.Burnable {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("mint does not allow token burning"))
	}

	switch mint.BurnAuthority {
	case store.BurnAuthorityOwnerOnly:
		if err := validation.ValidateAddressPublicKeyMatch(mint.OwnerAddress, request.PublicKey); err != nil {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("only the mint owner can burn tokens"))
		}
	case store.BurnAuthorityHolderOnly:
		// any holder can burn; balance gate below
	case store.BurnAuthorityOwnerOrHolder:
		// either owner or any holder; balance gate below
	default:
		if err := validation.ValidateAddressPublicKeyMatch(mint.OwnerAddress, request.PublicKey); err != nil {
			return nil, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("only the mint owner can burn tokens"))
		}
	}

	// Check available balance for burner
	tokenBalances, err := s.store.GetTokenBalances(ctx, burnerAddress, mint.Hash)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get token balances: %w", err))
	}

	var totalBalance int
	for _, b := range tokenBalances {
		totalBalance += b.Quantity
	}

	pendingBalances, err := s.store.GetPendingTokenBalances(ctx, burnerAddress, mint.Hash)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get pending balances: %w", err))
	}

	var totalPending int
	for _, b := range pendingBalances {
		totalPending += b.Quantity
	}

	available := totalBalance - totalPending
	if available < request.Payload.BurnQuantity {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("insufficient balance: available %d, burn_quantity %d", available, request.Payload.BurnQuantity))
	}

	burn := &store.UnconfirmedTokenBurn{
		MintHash:      mint.Hash,
		BurnQuantity:  request.Payload.BurnQuantity,
		BurnerAddress: burnerAddress,
		PublicKey:     request.PublicKey,
		Signature:     request.Signature,
		CreatedAt:     time.Now(),
	}

	burn.Hash, err = burn.GenerateHash()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	_, err = s.store.SaveUnconfirmedTokenBurn(ctx, burn)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.gossipClient.GossipTokenBurn(*burn); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	envelope := engineprotocol.NewTokenBurnTransactionEnvelope(burn.MintHash, burn.Hash, int32(burn.BurnQuantity))
	encodedTransactionBody := envelope.Serialize()

	resp := &protocol.BurnTokensResponse{}
	resp.SetBurnHash(toProtoHash(burn.Hash))
	resp.SetEncodedTransactionBody(hex.EncodeToString(encodedTransactionBody))
	return connect.NewResponse(resp), nil
}
