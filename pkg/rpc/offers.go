package rpc

import (
	"context"
	"errors"
	"time"

	connect "connectrpc.com/connect"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
)

func (s *ConnectRpcService) GetSellOffers(_ context.Context, req *connect.Request[protocol.GetSellOffersRequest]) (*connect.Response[protocol.GetSellOffersResponse], error) {
	limit := int32(100)
	if req.Msg.GetLimit() != nil && req.Msg.GetLimit().GetValue() > 0 && req.Msg.GetLimit().GetValue() < limit {
		limit = req.Msg.GetLimit().GetValue()
	}

	page := int32(0)
	if req.Msg.GetPage() != nil && req.Msg.GetPage().GetValue() > 0 {
		page = req.Msg.GetPage().GetValue()
	}

	start := int(page * limit)
	end := int(start + int(limit))

	mintHash := ""
	if req.Msg.GetMintHash() != nil {
		mintHash = req.Msg.GetMintHash().Value
	}

	offererAddress := ""
	if req.Msg.GetOffererAddress() != nil {
		offererAddress = req.Msg.GetOffererAddress().Value
	}

	offers, err := s.store.GetSellOffers(start, end, mintHash, offererAddress)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if start >= len(offers) {
		return connect.NewResponse(&protocol.GetSellOffersResponse{}), nil
	}

	if end > len(offers) {
		end = len(offers)
	}

	offersWithMints := make([]*protocol.SellOfferWithMint, 0, len(offers))
	for _, offer := range offers {
		mint, err := s.store.GetMintByHash(offer.MintHash)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		protoOffer, err := toProtoSellOfferWithMint(offer, mint)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		offersWithMints = append(offersWithMints, protoOffer)
	}

	resp := &protocol.GetSellOffersResponse{}
	resp.SetOffers(offersWithMints[start:end])
	resp.SetTotal(int32(len(offers)))
	resp.SetPage(page)
	resp.SetLimit(limit)
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) CreateSellOffer(_ context.Context, req *connect.Request[protocol.CreateSellOfferRequest]) (*connect.Response[protocol.CreateSellOfferResponse], error) {
	request, err := toCreateSellOfferRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := request.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	count, err := s.store.CountSellOffers(request.Payload.MintHash, request.Payload.OffererAddress)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if count >= s.cfg.SellOfferLimit {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("sell offer limit reached"))
	}

	newOfferWithoutId := &store.SellOfferWithoutID{
		OffererAddress: request.Payload.OffererAddress,
		MintHash:       request.Payload.MintHash,
		Quantity:       request.Payload.Quantity,
		Price:          request.Payload.Price,
		CreatedAt:      time.Now(),
		PublicKey:      request.PublicKey,
		Signature:      request.Signature,
	}
	newOfferWithoutId.Hash, err = newOfferWithoutId.GenerateHash()
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	id, err := s.store.SaveSellOffer(newOfferWithoutId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	newOffer := &store.SellOffer{
		SellOfferWithoutID: *newOfferWithoutId,
		Id:                 id,
	}

	if err := s.gossipClient.GossipSellOffer(*newOffer); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.CreateSellOfferResponse{}
	resp.SetId(id)
	resp.SetHash(newOfferWithoutId.Hash)
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) DeleteSellOffer(_ context.Context, req *connect.Request[protocol.DeleteSellOfferRequest]) (*connect.Response[protocol.DeleteSellOfferResponse], error) {
	request, err := toDeleteSellOfferRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := request.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.store.DeleteSellOffer(request.Payload.OfferHash, request.PublicKey); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.gossipClient.GossipDeleteSellOffer(request.Payload.OfferHash, request.PublicKey, request.Signature); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.DeleteSellOfferResponse{}
	resp.SetValue("Sell offer deleted")
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) GetBuyOffers(_ context.Context, req *connect.Request[protocol.GetBuyOffersRequest]) (*connect.Response[protocol.GetBuyOffersResponse], error) {
	limit := int32(100)
	if req.Msg.GetLimit() != nil && req.Msg.GetLimit().GetValue() > 0 && req.Msg.GetLimit().GetValue() < limit {
		limit = req.Msg.GetLimit().GetValue()
	}

	page := int32(0)
	if req.Msg.GetPage() != nil && req.Msg.GetPage().GetValue() > 0 {
		page = req.Msg.GetPage().GetValue()
	}

	start := int(page * limit)
	end := int(start + int(limit))

	mintHash := ""
	if req.Msg.GetMintHash() != nil {
		mintHash = req.Msg.GetMintHash().Value
	}

	sellerAddress := ""
	if req.Msg.GetSellerAddress() != nil {
		sellerAddress = req.Msg.GetSellerAddress().Value
	}

	offers, err := s.store.GetBuyOffersByMintAndSellerAddress(start, end, mintHash, sellerAddress)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if start >= len(offers) {
		return connect.NewResponse(&protocol.GetBuyOffersResponse{}), nil
	}

	if end > len(offers) {
		end = len(offers)
	}

	offersWithMints := make([]*protocol.BuyOfferWithMint, 0, len(offers))
	for _, offer := range offers {
		mint, err := s.store.GetMintByHash(offer.MintHash)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		protoOffer, err := toProtoBuyOfferWithMint(offer, mint)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		offersWithMints = append(offersWithMints, protoOffer)
	}

	resp := &protocol.GetBuyOffersResponse{}
	resp.SetOffers(offersWithMints[start:end])
	resp.SetTotal(int32(len(offers)))
	resp.SetPage(page)
	resp.SetLimit(limit)
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) CreateBuyOffer(_ context.Context, req *connect.Request[protocol.CreateBuyOfferRequest]) (*connect.Response[protocol.CreateBuyOfferResponse], error) {
	request, err := toCreateBuyOfferRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := request.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	count, err := s.store.CountBuyOffers(request.Payload.MintHash, request.Payload.OffererAddress, request.Payload.SellerAddress)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if count >= s.cfg.BuyOfferLimit {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("buy offer limit reached"))
	}

	newOfferWithoutId := &store.BuyOfferWithoutID{
		OffererAddress: request.Payload.OffererAddress,
		MintHash:       request.Payload.MintHash,
		SellerAddress:  request.Payload.SellerAddress,
		Quantity:       request.Payload.Quantity,
		Price:          request.Payload.Price,
		CreatedAt:      time.Now(),
		PublicKey:      request.PublicKey,
	}
	newOfferWithoutId.Hash, err = newOfferWithoutId.GenerateHash()
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	id, err := s.store.SaveBuyOffer(newOfferWithoutId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	newOffer := &store.BuyOffer{
		BuyOfferWithoutID: *newOfferWithoutId,
		Id:                id,
	}

	if err := s.gossipClient.GossipBuyOffer(*newOffer); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.CreateBuyOfferResponse{}
	resp.SetId(id)
	resp.SetHash(newOfferWithoutId.Hash)
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) DeleteBuyOffer(_ context.Context, req *connect.Request[protocol.DeleteBuyOfferRequest]) (*connect.Response[protocol.DeleteBuyOfferResponse], error) {
	request, err := toDeleteBuyOfferRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := request.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.store.DeleteBuyOffer(request.Payload.OfferHash, request.PublicKey); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.gossipClient.GossipDeleteBuyOffer(request.Payload.OfferHash, request.PublicKey, request.Signature); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.DeleteBuyOfferResponse{}
	resp.SetValue("Buy offer deleted")
	return connect.NewResponse(resp), nil
}
