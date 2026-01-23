package rpc

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	connect "connectrpc.com/connect"
	engineprotocol "dogecoin.org/fractal-engine/pkg/protocol"
	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"dogecoin.org/fractal-engine/pkg/validation"
)

type CreateInvoiceSignatureRequest struct {
	Payload CreateInvoiceSignatureRequestPayload `json:"payload"`
}

type CreateInvoiceSignatureRequestPayload struct {
	InvoiceHash string `json:"invoice_hash"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"public_key"`
}

type CreateInvoiceSignatureResponse struct {
	Id string `json:"id"`
}

func (s *ConnectRpcService) GetInvoices(_ context.Context, req *connect.Request[protocol.GetInvoicesRequest]) (*connect.Response[protocol.GetInvoicesResponse], error) {
	address := req.Msg.GetAddress()
	if address == nil || address.GetValue() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("address is required"))
	}
	if err := validation.ValidateAddress(address.GetValue()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

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

	mintHash := ""
	if req.Msg.GetMintHash() != nil {
		mintHash = req.Msg.GetMintHash().GetValue()
	}

	var invoices []store.Invoice
	var err error
	if mintHash == "" {
		invoices, err = s.store.GetInvoicesForMe(start, end, address.GetValue())
	} else {
		invoices, err = s.store.GetInvoices(start, end, mintHash, address.GetValue())
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if start >= len(invoices) {
		return connect.NewResponse(&protocol.GetInvoicesResponse{}), nil
	}

	if end > len(invoices) {
		end = len(invoices)
	}

	responseInvoices, err := toProtoInvoices(invoices[start:end])
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.GetInvoicesResponse{}
	resp.Invoices = responseInvoices
	resp.Total = int32(len(invoices))
	resp.Page = page
	resp.Limit = limit
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) GetAllInvoices(_ context.Context, req *connect.Request[protocol.GetAllInvoicesRequest]) (*connect.Response[protocol.GetAllInvoicesResponse], error) {
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

	mintHash := ""
	if req.Msg.GetMintHash() != nil {
		mintHash = req.Msg.GetMintHash().GetValue()
	}

	invoices, err := s.store.GetAllInvoices(start, end, mintHash)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if start >= len(invoices) {
		return connect.NewResponse(&protocol.GetAllInvoicesResponse{}), nil
	}

	if end > len(invoices) {
		end = len(invoices)
	}

	responseInvoices, err := toProtoInvoices(invoices[start:end])
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.GetAllInvoicesResponse{}
	resp.Invoices = responseInvoices
	resp.Total = int32(len(invoices))
	resp.Page = page
	resp.Limit = limit
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) CreateInvoice(_ context.Context, req *connect.Request[protocol.CreateInvoiceRequest]) (*connect.Response[protocol.CreateInvoiceResponse], error) {
	request, err := toCreateInvoiceRequest(req.Msg)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := request.Validate(); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	count, err := s.store.CountUnconfirmedInvoices(request.Payload.MintHash, request.Payload.BuyerAddress)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if count >= s.cfg.InvoiceLimit {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invoice limit reached"))
	}

	mint, err := s.store.GetMintByHash(request.Payload.MintHash)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	initialStatus := "pending_signatures"
	if mint.SignatureRequirementType == store.SignatureRequirementType_NONE || mint.SignatureRequirementType == "" {
		initialStatus = "draft"
	}

	newInvoiceWithoutId := &store.UnconfirmedInvoice{
		MintHash:       request.Payload.MintHash,
		Quantity:       request.Payload.Quantity,
		Price:          request.Payload.Price,
		BuyerAddress:   request.Payload.BuyerAddress,
		PaymentAddress: request.Payload.PaymentAddress,
		CreatedAt:      time.Now(),
		SellerAddress:  request.Payload.SellerAddress,
		PublicKey:      request.PublicKey,
		Signature:      request.Signature,
		Status:         initialStatus,
	}

	newInvoiceWithoutId.Hash, err = newInvoiceWithoutId.GenerateHash()
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	id, err := s.store.SaveUnconfirmedInvoice(newInvoiceWithoutId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	newInvoiceWithoutId.Id = id

	if err := s.gossipClient.GossipUnconfirmedInvoice(*newInvoiceWithoutId); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	envelope := engineprotocol.NewInvoiceTransactionEnvelope(newInvoiceWithoutId.Hash, newInvoiceWithoutId.MintHash, int32(newInvoiceWithoutId.Quantity), engineprotocol.ACTION_INVOICE)
	encodedTransactionBody := envelope.Serialize()

	resp := &protocol.CreateInvoiceResponse{}
	resp.Hash = &protocol.Hash{Value: &newInvoiceWithoutId.Hash}
	resp.EncodedTransactionBody = hex.EncodeToString(encodedTransactionBody)
	return connect.NewResponse(resp), nil
}

func (s *ConnectRpcService) CreateInvoiceSignature(_ context.Context, req *connect.Request[protocol.CreateInvoiceSignatureRequest]) (*connect.Response[protocol.CreateInvoiceSignatureResponse], error) {
	payload := req.Msg.GetPayload()
	if payload == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("payload is required"))
	}

	newInvoiceSignature := &store.InvoiceSignature{
		InvoiceHash: payload.GetInvoiceHash(),
		Signature:   payload.GetSignature(),
		PublicKey:   payload.GetPublicKey(),
		CreatedAt:   time.Now(),
	}

	invoice, err := s.store.GetUnconfirmedInvoiceByHash(payload.GetInvoiceHash())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	mint, err := s.store.GetMintByHash(invoice.MintHash)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := newInvoiceSignature.Validate(mint, invoice); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	id, err := s.store.SaveApprovedInvoiceSignature(newInvoiceSignature)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	if err := s.gossipClient.GossipInvoiceSignature(*newInvoiceSignature); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &protocol.CreateInvoiceSignatureResponse{}
	resp.Id = id
	return connect.NewResponse(resp), nil
}
