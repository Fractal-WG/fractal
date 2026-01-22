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
			OwnerAddress:             payload.GetOwnerAddress(),
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
			PaymentAddress: payload.GetPaymentAddress(),
			BuyerAddress:   payload.GetBuyerAddress(),
			MintHash:       payload.GetMintHash(),
			Quantity:       int(payload.GetQuantity()),
			Price:          int(payload.GetPrice()),
			SellerAddress:  payload.GetSellerAddress(),
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
			OffererAddress: payload.GetOffererAddress(),
			MintHash:       payload.GetMintHash(),
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
			OffererAddress: payload.GetOffererAddress(),
			SellerAddress:  payload.GetSellerAddress(),
			MintHash:       payload.GetMintHash(),
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
			OfferHash: payload.GetOfferHash(),
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
			OfferHash: payload.GetOfferHash(),
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
		protoManager.SetName(assetManager.Name)
		protoManager.SetPublicKey(assetManager.PublicKey)
		protoManager.SetUrl(assetManager.URL)
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
	protoValue.SetValue(payload)
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
	protoMint.SetAssetManagers(toProtoAssetManagers(mint.AssetManagers))
	protoMint.SetBlockHeight(int32(mint.BlockHeight))
	protoMint.SetContractOfSale(mint.ContractOfSale)
	protoMint.SetCreatedAt(mint.CreatedAt.Format(time.RFC3339Nano))
	protoMint.SetDescription(mint.Description)
	protoMint.SetFeedUrl(mint.FeedURL)
	protoMint.SetFractionCount(int32(mint.FractionCount))
	protoMint.SetHash(mint.Hash)
	protoMint.SetId(mint.Id)
	protoMint.SetLockupOptions(lockupOptions)
	protoMint.SetMetadata(metadata)
	protoMint.SetMinSignatures(int32(mint.MinSignatures))
	protoMint.SetOwnerAddress(mint.OwnerAddress)
	protoMint.SetPublicKey(mint.PublicKey)
	protoMint.SetRequirements(requirements)
	protoMint.SetSignature(mint.Signature)
	protoMint.SetSignatureRequirementType(toProtoSignatureRequirementType(mint.SignatureRequirementType))
	protoMint.SetTags([]string(mint.Tags))
	protoMint.SetTitle(mint.Title)
	protoMint.SetTransactionHash(mint.TransactionHash)
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
	paidAt.SetTime("")
	paidAt.SetValid(invoice.PaidAt.Valid)
	if invoice.PaidAt.Valid {
		paidAt.SetTime(invoice.PaidAt.Time.Format(time.RFC3339Nano))
	}

	protoInvoice := &protocol.Invoice{}
	protoInvoice.SetBlockHeight(int32(invoice.BlockHeight))
	protoInvoice.SetBuyerAddress(invoice.BuyerAddress)
	protoInvoice.SetCreatedAt(invoice.CreatedAt.Format(time.RFC3339Nano))
	protoInvoice.SetHash(invoice.Hash)
	protoInvoice.SetId(invoice.Id)
	protoInvoice.SetMintHash(invoice.MintHash)
	protoInvoice.SetPaidAt(paidAt)
	protoInvoice.SetPaymentAddress(invoice.PaymentAddress)
	protoInvoice.SetPendingTokenBalanceId(invoice.PendingTokenBalanceId)
	protoInvoice.SetPrice(int32(invoice.Price))
	protoInvoice.SetPublicKey(invoice.PublicKey)
	protoInvoice.SetQuantity(int32(invoice.Quantity))
	protoInvoice.SetSellerAddress(invoice.SellerAddress)
	protoInvoice.SetSignature(invoice.Signature)
	protoInvoice.SetTransactionHash(invoice.TransactionHash)
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
	protoBalance.SetAddress(balance.Address)
	protoBalance.SetCreatedAt(createdAt)
	protoBalance.SetMintHash(balance.MintHash)
	protoBalance.SetQuantity(int32(balance.Quantity))
	protoBalance.SetUpdatedAt(updatedAt)
	return protoBalance
}

func toProtoBuyOffer(offer store.BuyOffer) *protocol.BuyOffer {
	createdAt := ""
	if !offer.CreatedAt.IsZero() {
		createdAt = offer.CreatedAt.Format(time.RFC3339Nano)
	}

	protoOffer := &protocol.BuyOffer{}
	protoOffer.SetHash(offer.Hash)
	protoOffer.SetMintHash(offer.MintHash)
	protoOffer.SetOffererAddress(offer.OffererAddress)
	protoOffer.SetSellerAddress(offer.SellerAddress)
	protoOffer.SetQuantity(int32(offer.Quantity))
	protoOffer.SetPrice(int32(offer.Price))
	protoOffer.SetCreatedAt(createdAt)
	protoOffer.SetPublicKey(offer.PublicKey)
	protoOffer.SetSignature(offer.Signature)
	protoOffer.SetId(offer.Id)
	return protoOffer
}

func toProtoSellOffer(offer store.SellOffer) *protocol.SellOffer {
	createdAt := ""
	if !offer.CreatedAt.IsZero() {
		createdAt = offer.CreatedAt.Format(time.RFC3339Nano)
	}

	protoOffer := &protocol.SellOffer{}
	protoOffer.SetHash(offer.Hash)
	protoOffer.SetMintHash(offer.MintHash)
	protoOffer.SetOffererAddress(offer.OffererAddress)
	protoOffer.SetQuantity(int32(offer.Quantity))
	protoOffer.SetPrice(int32(offer.Price))
	protoOffer.SetCreatedAt(createdAt)
	protoOffer.SetPublicKey(offer.PublicKey)
	protoOffer.SetSignature(offer.Signature)
	protoOffer.SetId(offer.Id)
	return protoOffer
}

func toProtoBuyOfferWithMint(offer store.BuyOffer, mint store.Mint) (*protocol.BuyOfferWithMint, error) {
	protoMint, err := toProtoMint(mint)
	if err != nil {
		return nil, err
	}

	protoOffer := &protocol.BuyOfferWithMint{}
	protoOffer.SetOffer(toProtoBuyOffer(offer))
	protoOffer.SetMint(protoMint)
	return protoOffer, nil
}

func toProtoSellOfferWithMint(offer store.SellOffer, mint store.Mint) (*protocol.SellOfferWithMint, error) {
	protoMint, err := toProtoMint(mint)
	if err != nil {
		return nil, err
	}

	protoOffer := &protocol.SellOfferWithMint{}
	protoOffer.SetOffer(toProtoSellOffer(offer))
	protoOffer.SetMint(protoMint)
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
