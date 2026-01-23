package rpc

import (
	"context"
	"encoding/hex"
	"time"

	connect "connectrpc.com/connect"
	engineprotocol "dogecoin.org/fractal-engine/pkg/protocol"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"dogecoin.org/fractal-engine/pkg/validation"
)

func (s *ConnectRpcService) GetMints(ctx context.Context, req *connect.Request[protocol.GetMintsRequest]) (*connect.Response[protocol.GetMintsResponse], error) {
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

	mints, err := s.store.GetMints(ctx, start, end)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if start >= len(mints) {
		return connect.NewResponse(&protocol.GetMintsResponse{}), nil
	}

	if end > len(mints) {
		end = len(mints)
	}

	responseMints, err := toProtoMints(mints[start:end])
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.GetMintsResponse{}
	resp.SetMints(responseMints)
	resp.SetTotal(int32(len(mints)))
	resp.SetPage(page)
	resp.SetLimit(limit)
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) GetMint(ctx context.Context, req *connect.Request[protocol.GetMintRequest]) (*connect.Response[protocol.GetMintResponse], error) {
	hash := req.Msg.GetHash()
	if err := validation.ValidateHash(hash.GetValue()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	mint, err := s.store.GetMintByHash(ctx, hash.GetValue())
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	protoMint, err := toProtoMint(mint)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.GetMintResponse{}
	resp.SetMint(protoMint)
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) CreateMint(ctx context.Context, req *connect.Request[protocol.CreateMintRequest]) (*connect.Response[protocol.CreateMintResponse], error) {
	request, err := toCreateMintRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := request.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	newMintWithoutId := &store.MintWithoutID{
		Title:                    request.Payload.Title,
		FractionCount:            request.Payload.FractionCount,
		Description:              request.Payload.Description,
		Tags:                     request.Payload.Tags,
		Metadata:                 request.Payload.Metadata,
		CreatedAt:                time.Now(),
		Requirements:             request.Payload.Requirements,
		LockupOptions:            request.Payload.LockupOptions,
		SignatureRequirementType: request.Payload.SignatureRequirementType,
		FeedURL:                  request.Payload.FeedURL,
		PublicKey:                request.PublicKey,
		Signature:                request.Signature,
		OwnerAddress:             request.Payload.OwnerAddress,
		ContractOfSale:           request.Payload.ContractOfSale,
		AssetManagers:            request.Payload.AssetManagers,
		MinSignatures:            request.Payload.MinSignatures,
	}

	newMintWithoutId.Hash, err = newMintWithoutId.GenerateHash()
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	id, err := s.store.SaveUnconfirmedMint(ctx, newMintWithoutId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	newMint := &store.Mint{
		MintWithoutID: *newMintWithoutId,
		Id:            id,
	}

	if err := s.gossipClient.GossipMint(*newMint); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	envelope := engineprotocol.NewMintTransactionEnvelope(newMintWithoutId.Hash, engineprotocol.ACTION_MINT)
	encodedTransactionBody := envelope.Serialize()

	resp := &protocol.CreateMintResponse{}
	resp.SetHash(toProtoHash(newMintWithoutId.Hash))
	resp.SetEncodedTransactionBody(hex.EncodeToString(encodedTransactionBody))
	return connect.NewResponse(resp), nil
}
