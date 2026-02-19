package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol/protocolconnect"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func protoInvoiceToStore(inv *protocol.Invoice) (store.Invoice, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, inv.GetCreatedAt())
	if err != nil {
		return store.Invoice{}, err
	}

	paidAt := sql.NullTime{Valid: false}
	if inv.GetPaidAt() != nil && inv.GetPaidAt().GetValid() {
		t, err := time.Parse(time.RFC3339Nano, inv.GetPaidAt().GetTime())
		if err != nil {
			return store.Invoice{}, err
		}
		paidAt = sql.NullTime{Time: t, Valid: true}
	}

	return store.Invoice{
		Id:                    inv.GetId(),
		Hash:                  inv.GetHash().GetValue(),
		PaymentAddress:        inv.GetPaymentAddress().GetValue(),
		BuyerAddress:          inv.GetBuyerAddress().GetValue(),
		MintHash:              inv.GetMintHash().GetValue(),
		Quantity:              int(inv.GetQuantity()),
		Price:                 int(inv.GetPrice()),
		CreatedAt:             createdAt,
		SellerAddress:         inv.GetSellerAddress().GetValue(),
		BlockHeight:           int64(inv.GetBlockHeight()),
		TransactionHash:       inv.GetTransactionHash().GetValue(),
		PendingTokenBalanceId: inv.GetPendingTokenBalanceId(),
		PublicKey:             inv.GetPublicKey(),
		Signature:             inv.GetSignature(),
		PaidAt:                paidAt,
	}, nil
}

func protoMintToStore(m *protocol.Mint) (store.Mint, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, m.GetCreatedAt())
	if err != nil {
		return store.Mint{}, err
	}

	var metadata store.StringInterfaceMap
	if v := m.GetMetadata(); v != nil && v.GetValue() != nil {
		metadata = store.StringInterfaceMap(v.GetValue().AsMap())
	}

	var requirements store.StringInterfaceMap
	if v := m.GetRequirements(); v != nil && v.GetValue() != nil {
		requirements = store.StringInterfaceMap(v.GetValue().AsMap())
	}

	var lockupOptions store.StringInterfaceMap
	if v := m.GetLockupOptions(); v != nil && v.GetValue() != nil {
		lockupOptions = store.StringInterfaceMap(v.GetValue().AsMap())
	}

	var assetManagers []store.AssetManager
	for _, am := range m.GetAssetManagers() {
		if am == nil {
			continue
		}
		assetManagers = append(assetManagers, store.AssetManager{
			Name:      am.GetName(),
			PublicKey: am.GetPublicKey(),
			URL:       am.GetUrl(),
		})
	}

	var sigReqType store.SignatureRequirementType
	switch m.GetSignatureRequirementType() {
	case protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_ALL_SIGNATURES:
		sigReqType = store.SignatureRequirementType_ALL_SIGNATURES
	case protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_ONE_SIGNATURE:
		sigReqType = store.SignatureRequirementType_ONE_SIGNATURE
	case protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_MIN_SIGNATURES:
		sigReqType = store.SignatureRequirementType_MIN_SIGNATURES
	case protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_NONE:
		sigReqType = store.SignatureRequirementType_NONE
	}

	return store.Mint{
		MintWithoutID: store.MintWithoutID{
			Hash:                     m.GetHash().GetValue(),
			Title:                    m.GetTitle(),
			FractionCount:            int(m.GetFractionCount()),
			Description:              m.GetDescription(),
			Tags:                     store.StringArray(m.GetTags()),
			Metadata:                 metadata,
			TransactionHash:          m.GetTransactionHash().GetValue(),
			BlockHeight:              int64(m.GetBlockHeight()),
			CreatedAt:                createdAt,
			Requirements:             requirements,
			LockupOptions:            lockupOptions,
			FeedURL:                  m.GetFeedUrl(),
			PublicKey:                m.GetPublicKey(),
			OwnerAddress:             m.GetOwnerAddress().GetValue(),
			Signature:                m.GetSignature(),
			ContractOfSale:           m.GetContractOfSale(),
			SignatureRequirementType: sigReqType,
			AssetManagers:            assetManagers,
			MinSignatures:            int(m.GetMinSignatures()),
			AllowExpansion:           m.GetAllowExpansion(),
			CurrentSupply:            int(m.GetCurrentSupply()),
		},
		Id: m.GetId(),
	}, nil
}

func protoSellOfferToStore(o *protocol.SellOffer) (store.SellOffer, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, o.GetCreatedAt())
	if err != nil {
		return store.SellOffer{}, err
	}
	return store.SellOffer{
		SellOfferWithoutID: store.SellOfferWithoutID{
			Hash:           o.GetHash().GetValue(),
			MintHash:       o.GetMintHash().GetValue(),
			OffererAddress: o.GetOffererAddress().GetValue(),
			Quantity:       int(o.GetQuantity()),
			Price:          int(o.GetPrice()),
			CreatedAt:      createdAt,
			PublicKey:      o.GetPublicKey(),
			Signature:      o.GetSignature(),
		},
		Id: o.GetId(),
	}, nil
}

type TokenisationClient struct {
	baseUrl string
	client  protocolconnect.FractalEngineRpcServiceClient
	privHex string
	pubHex  string
}

func NewTokenisationClient(baseUrl string, privHex string, pubHex string) *TokenisationClient {
	httpClient := &http.Client{}
	client := protocolconnect.NewFractalEngineRpcServiceClient(httpClient, baseUrl)

	return &TokenisationClient{baseUrl: baseUrl, client: client, privHex: privHex, pubHex: pubHex}
}

func (c *TokenisationClient) CreateInvoice(invoice *rpc.CreateInvoiceRequest) (rpc.CreateInvoiceResponse, error) {
	signature, err := doge.SignPayload(invoice.Payload, c.privHex, c.pubHex)
	if err != nil {
		return rpc.CreateInvoiceResponse{}, err
	}

	paymentAddr := &protocol.Address{}
	paymentAddr.SetValue(invoice.Payload.PaymentAddress)

	buyerAddr := &protocol.Address{}
	buyerAddr.SetValue(invoice.Payload.BuyerAddress)

	mintHash := &protocol.Hash{}
	mintHash.SetValue(invoice.Payload.MintHash)

	sellerAddr := &protocol.Address{}
	sellerAddr.SetValue(invoice.Payload.SellerAddress)

	payload := &protocol.CreateInvoiceRequestPayload{}
	payload.SetPaymentAddress(paymentAddr)
	payload.SetBuyerAddress(buyerAddr)
	payload.SetMintHash(mintHash)
	payload.SetQuantity(int32(invoice.Payload.Quantity))
	payload.SetPrice(int32(invoice.Payload.Price))
	payload.SetSellerAddress(sellerAddr)

	req := &protocol.CreateInvoiceRequest{}
	req.SetPayload(payload)
	req.SetPublicKey(c.pubHex)
	req.SetSignature(signature)

	resp, err := c.client.CreateInvoice(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.CreateInvoiceResponse{}, err
	}

	return rpc.CreateInvoiceResponse{
		Hash:                   resp.Msg.GetHash().GetValue(),
		EncodedTransactionBody: resp.Msg.GetEncodedTransactionBody(),
	}, nil
}

func (c *TokenisationClient) GetInvoices(page int, limit int, mintHash string, address string) (rpc.GetInvoicesResponse, error) {
	addr := &protocol.Address{}
	addr.SetValue(address)

	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	req := &protocol.GetInvoicesRequest{}
	req.SetAddress(addr)
	req.SetPage(wrapperspb.Int32(int32(page)))
	req.SetLimit(wrapperspb.Int32(int32(limit)))
	req.SetMintHash(mintHashProto)

	resp, err := c.client.GetInvoices(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.GetInvoicesResponse{}, err
	}

	invoices := make([]store.Invoice, 0, len(resp.Msg.GetInvoices()))
	for _, inv := range resp.Msg.GetInvoices() {
		storeInv, err := protoInvoiceToStore(inv)
		if err != nil {
			return rpc.GetInvoicesResponse{}, err
		}
		invoices = append(invoices, storeInv)
	}

	return rpc.GetInvoicesResponse{
		Invoices: invoices,
		Total:    int(resp.Msg.GetTotal()),
		Page:     int(resp.Msg.GetPage()),
		Limit:    int(resp.Msg.GetLimit()),
	}, nil
}

func (c *TokenisationClient) GetMyInvoices(page int, limit int, address string) (rpc.GetInvoicesResponse, error) {
	addr := &protocol.Address{}
	addr.SetValue(address)

	req := &protocol.GetInvoicesRequest{}
	req.SetAddress(addr)
	req.SetPage(wrapperspb.Int32(int32(page)))
	req.SetLimit(wrapperspb.Int32(int32(limit)))

	resp, err := c.client.GetInvoices(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.GetInvoicesResponse{}, err
	}

	invoices := make([]store.Invoice, 0, len(resp.Msg.GetInvoices()))
	for _, inv := range resp.Msg.GetInvoices() {
		storeInv, err := protoInvoiceToStore(inv)
		if err != nil {
			return rpc.GetInvoicesResponse{}, err
		}
		invoices = append(invoices, storeInv)
	}

	return rpc.GetInvoicesResponse{
		Invoices: invoices,
		Total:    int(resp.Msg.GetTotal()),
		Page:     int(resp.Msg.GetPage()),
		Limit:    int(resp.Msg.GetLimit()),
	}, nil
}

func (c *TokenisationClient) GetHealth() (rpc.GetHealthResponse, error) {
	resp, err := c.client.GetHealth(context.Background(), connect.NewRequest(&protocol.GetHealthRequest{}))
	if err != nil {
		return rpc.GetHealthResponse{}, err
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, resp.Msg.GetUpdatedAt())
	if err != nil {
		return rpc.GetHealthResponse{}, err
	}

	return rpc.GetHealthResponse{
		Chain:              resp.Msg.GetChain(),
		CurrentBlockHeight: int64(resp.Msg.GetCurrentBlockHeight()),
		LatestBlockHeight:  int64(resp.Msg.GetLatestBlockHeight()),
		WalletsEnabled:     resp.Msg.GetWalletsEnabled(),
		UpdatedAt:          updatedAt,
	}, nil
}

func (c *TokenisationClient) CreateBuyOffer(offer *rpc.CreateBuyOfferRequest) (rpc.CreateOfferResponse, error) {
	signature, err := doge.SignPayload(offer.Payload, c.privHex, c.pubHex)
	if err != nil {
		return rpc.CreateOfferResponse{}, err
	}

	offererAddr := &protocol.Address{}
	offererAddr.SetValue(offer.Payload.OffererAddress)

	sellerAddr := &protocol.Address{}
	sellerAddr.SetValue(offer.Payload.SellerAddress)

	mintHash := &protocol.Hash{}
	mintHash.SetValue(offer.Payload.MintHash)

	payload := &protocol.CreateBuyOfferRequestPayload{}
	payload.SetOffererAddress(offererAddr)
	payload.SetSellerAddress(sellerAddr)
	payload.SetMintHash(mintHash)
	payload.SetQuantity(int32(offer.Payload.Quantity))
	payload.SetPrice(int32(offer.Payload.Price))

	req := &protocol.CreateBuyOfferRequest{}
	req.SetPayload(payload)
	req.SetPublicKey(c.pubHex)
	req.SetSignature(signature)

	resp, err := c.client.CreateBuyOffer(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.CreateOfferResponse{}, err
	}

	return rpc.CreateOfferResponse{
		Id:   resp.Msg.GetId(),
		Hash: resp.Msg.GetHash().GetValue(),
	}, nil
}

func (c *TokenisationClient) DeleteBuyOffer(offer *rpc.DeleteBuyOfferRequest) (string, error) {
	signature, err := doge.SignPayload(offer.Payload, c.privHex, c.pubHex)
	if err != nil {
		return "", err
	}

	offerHash := &protocol.Hash{}
	offerHash.SetValue(offer.Payload.OfferHash)

	payload := &protocol.DeleteBuyOfferRequestPayload{}
	payload.SetOfferHash(offerHash)

	req := &protocol.DeleteBuyOfferRequest{}
	req.SetPayload(payload)
	req.SetPublicKey(c.pubHex)
	req.SetSignature(signature)

	resp, err := c.client.DeleteBuyOffer(context.Background(), connect.NewRequest(req))
	if err != nil {
		return "", err
	}

	return resp.Msg.GetValue(), nil
}

func (c *TokenisationClient) DeleteSellOffer(offer *rpc.DeleteSellOfferRequest) (string, error) {
	signature, err := doge.SignPayload(offer.Payload, c.privHex, c.pubHex)
	if err != nil {
		return "", err
	}

	offerHash := &protocol.Hash{}
	offerHash.SetValue(offer.Payload.OfferHash)

	payload := &protocol.DeleteSellOfferRequestPayload{}
	payload.SetOfferHash(offerHash)

	req := &protocol.DeleteSellOfferRequest{}
	req.SetPayload(payload)
	req.SetPublicKey(c.pubHex)
	req.SetSignature(signature)

	resp, err := c.client.DeleteSellOffer(context.Background(), connect.NewRequest(req))
	if err != nil {
		return "", err
	}

	return resp.Msg.GetValue(), nil
}

func (c *TokenisationClient) CreateSellOffer(offer *rpc.CreateSellOfferRequest) (rpc.CreateOfferResponse, error) {
	signature, err := doge.SignPayload(offer.Payload, c.privHex, c.pubHex)
	if err != nil {
		return rpc.CreateOfferResponse{}, err
	}

	offererAddr := &protocol.Address{}
	offererAddr.SetValue(offer.Payload.OffererAddress)

	mintHash := &protocol.Hash{}
	mintHash.SetValue(offer.Payload.MintHash)

	payload := &protocol.CreateSellOfferRequestPayload{}
	payload.SetOffererAddress(offererAddr)
	payload.SetMintHash(mintHash)
	payload.SetQuantity(int32(offer.Payload.Quantity))
	payload.SetPrice(int32(offer.Payload.Price))

	req := &protocol.CreateSellOfferRequest{}
	req.SetPayload(payload)
	req.SetPublicKey(c.pubHex)
	req.SetSignature(signature)

	resp, err := c.client.CreateSellOffer(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.CreateOfferResponse{}, err
	}

	return rpc.CreateOfferResponse{
		Id:   resp.Msg.GetId(),
		Hash: resp.Msg.GetHash().GetValue(),
	}, nil
}

func (c *TokenisationClient) GetMintByHash(hash string) (rpc.GetMintResponse, error) {
	mintHash := &protocol.Hash{}
	mintHash.SetValue(hash)

	req := &protocol.GetMintRequest{}
	req.SetHash(mintHash)

	resp, err := c.client.GetMint(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.GetMintResponse{}, err
	}

	mint, err := protoMintToStore(resp.Msg.GetMint())
	if err != nil {
		return rpc.GetMintResponse{}, err
	}

	return rpc.GetMintResponse{Mint: mint}, nil
}

func (c *TokenisationClient) GetSellOffersByMintHash(page int, limit int, mintHash string) (rpc.GetSellOffersResponse, error) {
	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	req := &protocol.GetSellOffersRequest{}
	req.SetPage(wrapperspb.Int32(int32(page)))
	req.SetLimit(wrapperspb.Int32(int32(limit)))
	req.SetMintHash(mintHashProto)

	resp, err := c.client.GetSellOffers(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.GetSellOffersResponse{}, err
	}

	offers := make([]rpc.SellOfferWithMint, 0, len(resp.Msg.GetOffers()))
	for _, o := range resp.Msg.GetOffers() {
		sellOffer, err := protoSellOfferToStore(o.GetOffer())
		if err != nil {
			return rpc.GetSellOffersResponse{}, err
		}
		mint, err := protoMintToStore(o.GetMint())
		if err != nil {
			return rpc.GetSellOffersResponse{}, err
		}
		offers = append(offers, rpc.SellOfferWithMint{Offer: sellOffer, Mint: mint})
	}

	return rpc.GetSellOffersResponse{
		Offers: offers,
		Total:  int(resp.Msg.GetTotal()),
		Page:   int(resp.Msg.GetPage()),
		Limit:  int(resp.Msg.GetLimit()),
	}, nil
}

func protoBuyOfferToStore(o *protocol.BuyOffer) (store.BuyOffer, error) {
	createdAt, err := time.Parse(time.RFC3339Nano, o.GetCreatedAt())
	if err != nil {
		return store.BuyOffer{}, err
	}
	return store.BuyOffer{
		BuyOfferWithoutID: store.BuyOfferWithoutID{
			Hash:           o.GetHash().GetValue(),
			MintHash:       o.GetMintHash().GetValue(),
			OffererAddress: o.GetOffererAddress().GetValue(),
			SellerAddress:  o.GetSellerAddress().GetValue(),
			Quantity:       int(o.GetQuantity()),
			Price:          int(o.GetPrice()),
			CreatedAt:      createdAt,
			PublicKey:      o.GetPublicKey(),
			Signature:      o.GetSignature(),
		},
		Id: o.GetId(),
	}, nil
}

func (c *TokenisationClient) GetBuyOffersBySellerAddress(page int, limit int, mintHash string, sellerAddress string) (rpc.GetBuyOffersResponse, error) {
	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	sellerAddr := &protocol.Address{}
	sellerAddr.SetValue(sellerAddress)

	req := &protocol.GetBuyOffersRequest{}
	req.SetPage(wrapperspb.Int32(int32(page)))
	req.SetLimit(wrapperspb.Int32(int32(limit)))
	req.SetMintHash(mintHashProto)
	req.SetSellerAddress(sellerAddr)

	resp, err := c.client.GetBuyOffers(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.GetBuyOffersResponse{}, err
	}

	offers := make([]rpc.BuyOfferWithMint, 0, len(resp.Msg.GetOffers()))
	for _, o := range resp.Msg.GetOffers() {
		buyOffer, err := protoBuyOfferToStore(o.GetOffer())
		if err != nil {
			return rpc.GetBuyOffersResponse{}, err
		}
		mint, err := protoMintToStore(o.GetMint())
		if err != nil {
			return rpc.GetBuyOffersResponse{}, err
		}
		offers = append(offers, rpc.BuyOfferWithMint{Offer: buyOffer, Mint: mint})
	}

	return rpc.GetBuyOffersResponse{
		Offers: offers,
		Total:  int(resp.Msg.GetTotal()),
		Page:   int(resp.Msg.GetPage()),
		Limit:  int(resp.Msg.GetLimit()),
	}, nil
}

func (c *TokenisationClient) GetBuyOffers(page int, limit int, mintHash string) (rpc.GetBuyOffersResponse, error) {
	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	req := &protocol.GetBuyOffersRequest{}
	req.SetPage(wrapperspb.Int32(int32(page)))
	req.SetLimit(wrapperspb.Int32(int32(limit)))
	req.SetMintHash(mintHashProto)

	resp, err := c.client.GetBuyOffers(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.GetBuyOffersResponse{}, err
	}

	offers := make([]rpc.BuyOfferWithMint, 0, len(resp.Msg.GetOffers()))
	for _, o := range resp.Msg.GetOffers() {
		buyOffer, err := protoBuyOfferToStore(o.GetOffer())
		if err != nil {
			return rpc.GetBuyOffersResponse{}, err
		}
		mint, err := protoMintToStore(o.GetMint())
		if err != nil {
			return rpc.GetBuyOffersResponse{}, err
		}
		offers = append(offers, rpc.BuyOfferWithMint{Offer: buyOffer, Mint: mint})
	}

	return rpc.GetBuyOffersResponse{
		Offers: offers,
		Total:  int(resp.Msg.GetTotal()),
		Page:   int(resp.Msg.GetPage()),
		Limit:  int(resp.Msg.GetLimit()),
	}, nil
}

func (c *TokenisationClient) Mint(mint *rpc.CreateMintRequest) (rpc.CreateMintResponse, error) {
	signature, err := doge.SignPayload(mint.Payload, c.privHex, c.pubHex)
	if err != nil {
		return rpc.CreateMintResponse{}, err
	}

	toProtoMap := func(m store.StringInterfaceMap) (*protocol.StringInterfaceMap, error) {
		if m == nil {
			return nil, nil
		}
		s, err := structpb.NewStruct(map[string]interface{}(m))
		if err != nil {
			return nil, err
		}
		protoMap := &protocol.StringInterfaceMap{}
		protoMap.SetValue(s)
		return protoMap, nil
	}

	metadata, err := toProtoMap(mint.Payload.Metadata)
	if err != nil {
		return rpc.CreateMintResponse{}, err
	}
	requirements, err := toProtoMap(mint.Payload.Requirements)
	if err != nil {
		return rpc.CreateMintResponse{}, err
	}
	lockupOptions, err := toProtoMap(mint.Payload.LockupOptions)
	if err != nil {
		return rpc.CreateMintResponse{}, err
	}

	assetManagers := make([]*protocol.AssetManager, 0, len(mint.Payload.AssetManagers))
	for _, am := range mint.Payload.AssetManagers {
		protoAM := &protocol.AssetManager{}
		protoAM.SetName(am.Name)
		protoAM.SetPublicKey(am.PublicKey)
		protoAM.SetUrl(am.URL)
		assetManagers = append(assetManagers, protoAM)
	}

	var sigReqType protocol.SignatureRequirementType
	switch mint.Payload.SignatureRequirementType {
	case store.SignatureRequirementType_ALL_SIGNATURES:
		sigReqType = protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_ALL_SIGNATURES
	case store.SignatureRequirementType_ONE_SIGNATURE:
		sigReqType = protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_ONE_SIGNATURE
	case store.SignatureRequirementType_MIN_SIGNATURES:
		sigReqType = protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_REQUIRES_MIN_SIGNATURES
	case store.SignatureRequirementType_NONE:
		sigReqType = protocol.SignatureRequirementType_SIGNATURE_REQUIREMENT_TYPE_NONE
	}

	ownerAddr := &protocol.Address{}
	ownerAddr.SetValue(mint.Payload.OwnerAddress)

	payload := &protocol.CreateMintRequestPayload{}
	payload.SetTitle(mint.Payload.Title)
	payload.SetFractionCount(int32(mint.Payload.FractionCount))
	payload.SetDescription(mint.Payload.Description)
	payload.SetTags([]string(mint.Payload.Tags))
	payload.SetMetadata(metadata)
	payload.SetRequirements(requirements)
	payload.SetLockupOptions(lockupOptions)
	payload.SetFeedUrl(mint.Payload.FeedURL)
	payload.SetContractOfSale(mint.Payload.ContractOfSale)
	payload.SetOwnerAddress(ownerAddr)
	payload.SetSignatureRequirementType(sigReqType)
	payload.SetAssetManagers(assetManagers)
	payload.SetMinSignatures(int32(mint.Payload.MinSignatures))
	payload.SetAllowExpansion(mint.Payload.AllowExpansion)

	req := &protocol.CreateMintRequest{}
	req.SetPayload(payload)
	req.SetPublicKey(c.pubHex)
	req.SetSignature(signature)

	resp, err := c.client.CreateMint(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.CreateMintResponse{}, err
	}

	return rpc.CreateMintResponse{
		Hash:                   resp.Msg.GetHash().GetValue(),
		EncodedTransactionBody: resp.Msg.GetEncodedTransactionBody(),
	}, nil
}

func (c *TokenisationClient) MintExpansion(expand *rpc.ExpandMintRequest) (rpc.ExpandMintResponse, error) {
	signature, err := doge.SignPayload(expand.Payload, c.privHex, c.pubHex)
	if err != nil {
		return rpc.ExpandMintResponse{}, err
	}

	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(expand.Payload.MintHash)

	payload := &protocol.ExpandMintRequestPayload{}
	payload.SetMintHash(mintHashProto)
	payload.SetAdditionalSupply(int32(expand.Payload.AdditionalSupply))

	req := &protocol.ExpandMintRequest{}
	req.SetPayload(payload)
	req.SetPublicKey(c.pubHex)
	req.SetSignature(signature)

	resp, err := c.client.ExpandMint(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.ExpandMintResponse{}, err
	}

	return rpc.ExpandMintResponse{
		ExpansionHash:          resp.Msg.GetExpansionHash().GetValue(),
		EncodedTransactionBody: resp.Msg.GetEncodedTransactionBody(),
	}, nil
}

func (c *TokenisationClient) GetTokenBalance(address string, mintHash string) ([]store.TokenBalance, error) {
	addr := &protocol.Address{}
	addr.SetValue(address)

	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	req := &protocol.GetTokenBalancesRequest{}
	req.SetAddress(addr)
	req.SetMintHash(mintHashProto)

	resp, err := c.client.GetTokenBalances(context.Background(), connect.NewRequest(req))
	if err != nil {
		return []store.TokenBalance{}, err
	}

	// Server returns {"balances": [...]} as a structpb.Struct
	raw := resp.Msg.GetData().AsMap()
	balancesJSON, err := json.Marshal(raw["balances"])
	if err != nil {
		return []store.TokenBalance{}, err
	}

	var result []store.TokenBalance
	if err := json.Unmarshal(balancesJSON, &result); err != nil {
		return []store.TokenBalance{}, err
	}

	return result, nil
}

func (c *TokenisationClient) GetTokenBalanceWithMintDetails(address string) (rpc.GetTokenBalanceWithMintsResponse, error) {
	addr := &protocol.Address{}
	addr.SetValue(address)

	req := &protocol.GetTokenBalancesRequest{}
	req.SetAddress(addr)
	req.SetIncludeMintDetails(wrapperspb.Bool(true))

	resp, err := c.client.GetTokenBalances(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.GetTokenBalanceWithMintsResponse{}, err
	}

	dataJSON, err := json.Marshal(resp.Msg.GetData().AsMap())
	if err != nil {
		return rpc.GetTokenBalanceWithMintsResponse{}, err
	}

	var result rpc.GetTokenBalanceWithMintsResponse
	if err := json.Unmarshal(dataJSON, &result); err != nil {
		return rpc.GetTokenBalanceWithMintsResponse{}, err
	}

	return result, nil
}

func (c *TokenisationClient) GetPendingTokenBalance(address string, mintHash string) ([]store.TokenBalance, error) {
	addr := &protocol.Address{}
	addr.SetValue(address)

	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	req := &protocol.GetPendingTokenBalancesRequest{}
	req.SetAddress(addr)
	req.SetMintHash(mintHashProto)

	resp, err := c.client.GetPendingTokenBalances(context.Background(), connect.NewRequest(req))
	if err != nil {
		return []store.TokenBalance{}, err
	}

	parseTime := func(s string) (time.Time, error) {
		if s == "" {
			return time.Time{}, nil
		}
		return time.Parse(time.RFC3339Nano, s)
	}

	balances := make([]store.TokenBalance, 0, len(resp.Msg.GetBalances()))
	for _, b := range resp.Msg.GetBalances() {
		createdAt, err := parseTime(b.GetCreatedAt())
		if err != nil {
			return []store.TokenBalance{}, err
		}
		updatedAt, err := parseTime(b.GetUpdatedAt())
		if err != nil {
			return []store.TokenBalance{}, err
		}
		balances = append(balances, store.TokenBalance{
			MintHash:  b.GetMintHash().GetValue(),
			Address:   b.GetAddress().GetValue(),
			Quantity:  int(b.GetQuantity()),
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}

	return balances, nil
}

func (c *TokenisationClient) GetMints(page int, limit int, publicKey string, includeUnconfirmed bool) (rpc.GetMintsResponse, error) {
	req := &protocol.GetMintsRequest{}
	req.SetPage(wrapperspb.Int32(int32(page)))
	req.SetLimit(wrapperspb.Int32(int32(limit)))
	req.SetPublicKey(publicKey)
	req.SetIncludeUnconfirmed(wrapperspb.Bool(includeUnconfirmed))

	resp, err := c.client.GetMints(context.Background(), connect.NewRequest(req))
	if err != nil {
		return rpc.GetMintsResponse{}, err
	}

	mints := make([]store.Mint, 0, len(resp.Msg.GetMints()))
	for _, m := range resp.Msg.GetMints() {
		mint, err := protoMintToStore(m)
		if err != nil {
			return rpc.GetMintsResponse{}, err
		}
		mints = append(mints, mint)
	}

	return rpc.GetMintsResponse{
		Mints: mints,
		Total: int(resp.Msg.GetTotal()),
		Page:  int(resp.Msg.GetPage()),
		Limit: int(resp.Msg.GetLimit()),
	}, nil
}

func (c *TokenisationClient) CreateInvoiceSignature(signature *rpc.CreateInvoiceSignatureRequest) (rpc.CreateInvoiceSignatureResponse, error) {
	ctx := context.Background()

	payload := &protocol.CreateInvoiceSignatureRequestPayload{}
	payload.SetInvoiceHash(signature.Payload.InvoiceHash)
	payload.SetPublicKey(signature.Payload.PublicKey)
	payload.SetSignature(signature.Payload.Signature)

	req := &protocol.CreateInvoiceSignatureRequest{}
	req.SetPayload(payload)

	resp, err := c.client.CreateInvoiceSignature(ctx, connect.NewRequest(req))
	if err != nil {
		return rpc.CreateInvoiceSignatureResponse{}, err
	}

	return rpc.CreateInvoiceSignatureResponse{
		Id: resp.Msg.GetId(),
	}, nil
}
