package rpc

import (
	"encoding/json"
	"errors"
	"time"

	protocol "dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/types/known/structpb"
)

func toCreateMintRequest(req *protocol.CreateMintRequest) (*CreateMintRequest, error) {
	if req == nil || req.GetPayload() == nil {
		return nil, errors.New("payload is required")
	}

	payload := req.GetPayload()
	return &CreateMintRequest{
		SignedRequest: SignedRequest{
			PublicKey: req.GetPublicKey(),
			Signature: req.GetSignature(),
		},
		Payload: CreateMintRequestPayload{
			Title:                    payload.GetTitle(),
			FractionCount:            int(payload.GetFractionCount()),
			Description:              payload.GetDescription(),
			Tags:                     store.StringArray(payload.GetTags()),
			Metadata:                 toStoreStringInterfaceMap(payload.GetMetadata()),
			Requirements:             toStoreStringInterfaceMap(payload.GetRequirements()),
			LockupOptions:            toStoreStringInterfaceMap(payload.GetLockupOptions()),
			FeedURL:                  payload.GetFeedUrl(),
			ContractOfSale:           payload.GetContractOfSale(),
			OwnerAddress:             payload.GetOwnerAddress().GetValue(),
			SignatureRequirementType: toStoreSignatureRequirementType(payload.GetSignatureRequirementType()),
			AssetManagers:            toStoreAssetManagers(payload.GetAssetManagers()),
			MinSignatures:            int(payload.GetMinSignatures()),
		},
	}, nil
}

func toCreateInvoiceRequest(req *protocol.CreateInvoiceRequest) (*CreateInvoiceRequest, error) {
	if req == nil || req.GetPayload() == nil {
		return nil, errors.New("payload is required")
	}

	payload := req.GetPayload()
	return &CreateInvoiceRequest{
		SignedRequest: SignedRequest{
			PublicKey: req.GetPublicKey(),
			Signature: req.GetSignature(),
		},
		Payload: CreateInvoiceRequestPayload{
			PaymentAddress: payload.GetPaymentAddress().GetValue(),
			BuyerAddress:   payload.GetBuyerAddress().GetValue(),
			MintHash:       payload.GetMintHash().GetValue(),
			Quantity:       int(payload.GetQuantity()),
			Price:          int(payload.GetPrice()),
			SellerAddress:  payload.GetSellerAddress().GetValue(),
		},
	}, nil
}

func toCreateSellOfferRequest(req *protocol.CreateSellOfferRequest) (*CreateSellOfferRequest, error) {
	if req == nil || req.GetPayload() == nil {
		return nil, errors.New("payload is required")
	}

	payload := req.GetPayload()
	return &CreateSellOfferRequest{
		SignedRequest: SignedRequest{
			PublicKey: req.GetPublicKey(),
			Signature: req.GetSignature(),
		},
		Payload: CreateSellOfferRequestPayload{
			OffererAddress: payload.GetOffererAddress().GetValue(),
			MintHash:       payload.GetMintHash().GetValue(),
			Quantity:       int(payload.GetQuantity()),
			Price:          int(payload.GetPrice()),
		},
	}, nil
}

func toCreateBuyOfferRequest(req *protocol.CreateBuyOfferRequest) (*CreateBuyOfferRequest, error) {
	if req == nil || req.GetPayload() == nil {
		return nil, errors.New("payload is required")
	}

	payload := req.GetPayload()
	return &CreateBuyOfferRequest{
		SignedRequest: SignedRequest{
			PublicKey: req.GetPublicKey(),
			Signature: req.GetSignature(),
		},
		Payload: CreateBuyOfferRequestPayload{
			OffererAddress: payload.GetOffererAddress().GetValue(),
			SellerAddress:  payload.GetSellerAddress().GetValue(),
			MintHash:       payload.GetMintHash().GetValue(),
			Quantity:       int(payload.GetQuantity()),
			Price:          int(payload.GetPrice()),
		},
	}, nil
}

func toDeleteSellOfferRequest(req *protocol.DeleteSellOfferRequest) (*DeleteSellOfferRequest, error) {
	if req == nil || req.GetPayload() == nil {
		return nil, errors.New("payload is required")
	}

	payload := req.GetPayload()
	return &DeleteSellOfferRequest{
		SignedRequest: SignedRequest{
			PublicKey: req.GetPublicKey(),
			Signature: req.GetSignature(),
		},
		Payload: DeleteSellOfferRequestPayload{
			OfferHash: payload.GetOfferHash().GetValue(),
		},
	}, nil
}

func toDeleteBuyOfferRequest(req *protocol.DeleteBuyOfferRequest) (*DeleteBuyOfferRequest, error) {
	if req == nil || req.GetPayload() == nil {
		return nil, errors.New("payload is required")
	}

	payload := req.GetPayload()
	return &DeleteBuyOfferRequest{
		SignedRequest: SignedRequest{
			PublicKey: req.GetPublicKey(),
			Signature: req.GetSignature(),
		},
		Payload: DeleteBuyOfferRequestPayload{
			OfferHash: payload.GetOfferHash().GetValue(),
		},
	}, nil
}

func toStoreSignatureRequirementType(req protocol.SignatureRequirementType) store.SignatureRequirementType {
	switch req {
	case protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_ALL_SIGNATURES:
		return store.SignatureRequirementType_ALL_SIGNATURES
	case protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_ONE_SIGNATURE:
		return store.SignatureRequirementType_ONE_SIGNATURE
	case protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_MIN_SIGNATURES:
		return store.SignatureRequirementType_MIN_SIGNATURES
	case protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_NONE:
		return store.SignatureRequirementType_NONE
	default:
		return ""
	}
}

func toProtoSignatureRequirementType(req store.SignatureRequirementType) protocol.SignatureRequirementType {
	switch req {
	case store.SignatureRequirementType_ALL_SIGNATURES:
		return protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_ALL_SIGNATURES
	case store.SignatureRequirementType_ONE_SIGNATURE:
		return protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_ONE_SIGNATURE
	case store.SignatureRequirementType_MIN_SIGNATURES:
		return protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_MIN_SIGNATURES
	case store.SignatureRequirementType_NONE:
		return protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_NONE
	default:
		return protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_UNSPECIFIED
	}
}

func toStoreAssetManagers(assetManagers []*protocol.AssetManager) []store.AssetManager {
	if len(assetManagers) == 0 {
		return nil
	}

	result := make([]store.AssetManager, 0, len(assetManagers))
	for _, assetManager := range assetManagers {
		if assetManager == nil {
			continue
		}
		result = append(result, store.AssetManager{
			Name:      assetManager.GetName(),
			PublicKey: assetManager.GetPublicKey(),
			URL:       assetManager.GetUrl(),
		})
	}

	return result
}

func toProtoAssetManagers(assetManagers []store.AssetManager) []*protocol.AssetManager {
	if len(assetManagers) == 0 {
		return nil
	}

	result := make([]*protocol.AssetManager, 0, len(assetManagers))
	for _, assetManager := range assetManagers {
		protoManager := &protocol.AssetManager{}
		protoManager.Name = assetManager.Name
		protoManager.PublicKey = assetManager.PublicKey
		protoManager.Url = assetManager.URL
		result = append(result, protoManager)
	}

	return result
}

func toStoreStringInterfaceMap(value *protocol.StringInterfaceMap) store.StringInterfaceMap {
	if value == nil || value.GetValue() == nil {
		return nil
	}

	return store.StringInterfaceMap(value.GetValue().AsMap())
}

func toProtoStringInterfaceMap(value store.StringInterfaceMap) (*protocol.StringInterfaceMap, error) {
	if value == nil {
		return nil, nil
	}

	payload, err := structpb.NewStruct(map[string]interface{}(value))
	if err != nil {
		return nil, err
	}

	protoValue := &protocol.StringInterfaceMap{}
	protoValue.Value = payload
	return protoValue, nil
}

func toProtoMint(mint store.Mint) (*protocol.Mint, error) {
	metadata, err := toProtoStringInterfaceMap(mint.Metadata)
	if err != nil {
		return nil, err
	}

	requirements, err := toProtoStringInterfaceMap(mint.Requirements)
	if err != nil {
		return nil, err
	}

	lockupOptions, err := toProtoStringInterfaceMap(mint.LockupOptions)
	if err != nil {
		return nil, err
	}

	protoMint := &protocol.Mint{}
	protoMint.AssetManagers = toProtoAssetManagers(mint.AssetManagers)
	protoMint.BlockHeight = int32(mint.BlockHeight)
	protoMint.ContractOfSale = mint.ContractOfSale
	protoMint.CreatedAt = mint.CreatedAt.Format(time.RFC3339Nano)
	protoMint.Description = mint.Description
	protoMint.FeedUrl = mint.FeedURL
	protoMint.FractionCount = int32(mint.FractionCount)
	protoMint.Hash = &protocol.Hash{Value: &mint.Hash}
	protoMint.Id = mint.Id
	protoMint.LockupOptions = lockupOptions
	protoMint.Metadata = metadata
	protoMint.MinSignatures = int32(mint.MinSignatures)
	protoMint.OwnerAddress = &protocol.Address{Value: &mint.OwnerAddress}
	protoMint.PublicKey = mint.PublicKey
	protoMint.Requirements = requirements
	protoMint.Signature = mint.Signature
	protoMint.SignatureRequirementType = toProtoSignatureRequirementType(mint.SignatureRequirementType)
	protoMint.Tags = []string(mint.Tags)
	protoMint.Title = mint.Title
	protoMint.TransactionHash = &protocol.Hash{Value: &mint.TransactionHash}

	return protoMint, nil
}

func toProtoMints(mints []store.Mint) ([]*protocol.Mint, error) {
	result := make([]*protocol.Mint, 0, len(mints))
	for _, mint := range mints {
		protoMint, err := toProtoMint(mint)
		if err != nil {
			return nil, err
		}
		result = append(result, protoMint)
	}
	return result, nil
}

func toProtoInvoice(invoice store.Invoice) *protocol.Invoice {
	paidAt := &protocol.SqlNullTime{}
	paidAt.Time = ""
	paidAt.Valid = invoice.PaidAt.Valid
	if invoice.PaidAt.Valid {
		paidAt.Time = invoice.PaidAt.Time.Format(time.RFC3339Nano)
	}

	protoInvoice := &protocol.Invoice{}
	protoInvoice.BlockHeight = int32(invoice.BlockHeight)
	protoInvoice.BuyerAddress = &protocol.Address{Value: &invoice.BuyerAddress}
	protoInvoice.CreatedAt = invoice.CreatedAt.Format(time.RFC3339Nano)
	protoInvoice.Hash = &protocol.Hash{Value: &invoice.Hash}
	protoInvoice.Id = invoice.Id
	protoInvoice.MintHash = &protocol.Hash{Value: &invoice.MintHash}
	protoInvoice.PaidAt = paidAt
	protoInvoice.PaymentAddress = &protocol.Address{Value: &invoice.PaymentAddress}
	protoInvoice.PendingTokenBalanceId = invoice.PendingTokenBalanceId
	protoInvoice.Price = int32(invoice.Price)
	protoInvoice.PublicKey = invoice.PublicKey
	protoInvoice.Quantity = int32(invoice.Quantity)
	protoInvoice.SellerAddress = &protocol.Address{Value: &invoice.SellerAddress}
	protoInvoice.Signature = invoice.Signature
	protoInvoice.TransactionHash = &protocol.Hash{Value: &invoice.TransactionHash}
	return protoInvoice
}

func toProtoInvoices(invoices []store.Invoice) ([]*protocol.Invoice, error) {
	result := make([]*protocol.Invoice, 0, len(invoices))
	for _, invoice := range invoices {
		result = append(result, toProtoInvoice(invoice))
	}
	return result, nil
}

func toProtoTokenBalance(balance store.TokenBalance) *protocol.TokenBalance {
	createdAt := ""
	if !balance.CreatedAt.IsZero() {
		createdAt = balance.CreatedAt.Format(time.RFC3339Nano)
	}
	updatedAt := ""
	if !balance.UpdatedAt.IsZero() {
		updatedAt = balance.UpdatedAt.Format(time.RFC3339Nano)
	}

	protoBalance := &protocol.TokenBalance{}
	protoBalance.Address = &protocol.Address{Value: &balance.Address}
	protoBalance.CreatedAt = createdAt
	protoBalance.MintHash = &protocol.Hash{Value: &balance.MintHash}
	protoBalance.Quantity = int32(balance.Quantity)
	protoBalance.UpdatedAt = updatedAt
	return protoBalance
}

func toProtoBuyOffer(offer store.BuyOffer) *protocol.BuyOffer {
	createdAt := ""
	if !offer.CreatedAt.IsZero() {
		createdAt = offer.CreatedAt.Format(time.RFC3339Nano)
	}

	protoOffer := &protocol.BuyOffer{}
	protoOffer.Hash = &protocol.Hash{Value: &offer.Hash}
	protoOffer.MintHash = &protocol.Hash{Value: &offer.MintHash}
	protoOffer.OffererAddress = &protocol.Address{Value: &offer.OffererAddress}
	protoOffer.SellerAddress = &protocol.Address{Value: &offer.SellerAddress}
	protoOffer.Quantity = int32(offer.Quantity)
	protoOffer.Price = int32(offer.Price)
	protoOffer.CreatedAt = createdAt
	protoOffer.PublicKey = offer.PublicKey
	protoOffer.Signature = offer.Signature
	protoOffer.Id = offer.Id
	return protoOffer
}

func toProtoSellOffer(offer store.SellOffer) *protocol.SellOffer {
	createdAt := ""
	if !offer.CreatedAt.IsZero() {
		createdAt = offer.CreatedAt.Format(time.RFC3339Nano)
	}

	protoOffer := &protocol.SellOffer{}
	protoOffer.Hash = &protocol.Hash{Value: &offer.Hash}
	protoOffer.MintHash = &protocol.Hash{Value: &offer.MintHash}
	protoOffer.OffererAddress = &protocol.Address{Value: &offer.OffererAddress}
	protoOffer.Quantity = int32(offer.Quantity)
	protoOffer.Price = int32(offer.Price)
	protoOffer.CreatedAt = createdAt
	protoOffer.PublicKey = offer.PublicKey
	protoOffer.Signature = offer.Signature
	protoOffer.Id = offer.Id
	return protoOffer
}

func toProtoBuyOfferWithMint(offer store.BuyOffer, mint store.Mint) (*protocol.BuyOfferWithMint, error) {
	protoMint, err := toProtoMint(mint)
	if err != nil {
		return nil, err
	}

	protoOffer := &protocol.BuyOfferWithMint{}
	protoOffer.Offer = toProtoBuyOffer(offer)
	protoOffer.Mint = protoMint
	return protoOffer, nil
}

func toProtoSellOfferWithMint(offer store.SellOffer, mint store.Mint) (*protocol.SellOfferWithMint, error) {
	protoMint, err := toProtoMint(mint)
	if err != nil {
		return nil, err
	}

	protoOffer := &protocol.SellOfferWithMint{}
	protoOffer.Offer = toProtoSellOffer(offer)
	protoOffer.Mint = protoMint
	return protoOffer, nil
}

func toStructPB(value interface{}) (*structpb.Struct, error) {
	data, err := encodeToInterface(value)
	if err != nil {
		return nil, err
	}

	switch typed := data.(type) {
	case map[string]interface{}:
		return structpb.NewStruct(typed)
	default:
		return structpb.NewStruct(map[string]interface{}{"items": typed})
	}
}

func encodeToInterface(value interface{}) (interface{}, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var data interface{}
	if err := json.Unmarshal(encoded, &data); err != nil {
		return nil, err
	}

	return data, nil
}
