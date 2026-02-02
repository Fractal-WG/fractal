package rpc

import (
	"encoding/json"
	"fmt"
	"time"

	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/store"
	"dogecoin.org/fractal-engine/pkg/validation"
)

type SignedRequest struct {
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
}

type PrepareMintRequest struct {
	Payload CreateMintRequestPayload `json:"payload"`
}

func (req *PrepareMintRequest) Validate() error {
	if err := validation.ValidateTitle(req.Payload.Title); err != nil {
		return err
	}

	if err := validation.ValidateDescription(req.Payload.Description); err != nil {
		return err
	}

	if err := validation.ValidateQuantity("fraction_count", req.Payload.FractionCount); err != nil {
		return err
	}

	if err := validation.ValidateFeedURL(req.Payload.FeedURL); err != nil {
		return err
	}

	if err := validation.ValidateTags(req.Payload.Tags); err != nil {
		return err
	}

	// Validate metadata size
	if req.Payload.Metadata != nil {
		metadataBytes, err := json.Marshal(req.Payload.Metadata)
		if err != nil {
			return fmt.Errorf("invalid metadata format: %w", err)
		}
		if err := validation.ValidateMetadataSize("metadata", metadataBytes); err != nil {
			return err
		}
	}

	// Validate requirements size
	if req.Payload.Requirements != nil {
		reqBytes, err := json.Marshal(req.Payload.Requirements)
		if err != nil {
			return fmt.Errorf("invalid requirements format: %w", err)
		}
		if err := validation.ValidateMetadataSize("requirements", reqBytes); err != nil {
			return err
		}
	}

	// Validate lockup options size
	if req.Payload.LockupOptions != nil {
		lockupBytes, err := json.Marshal(req.Payload.LockupOptions)
		if err != nil {
			return fmt.Errorf("invalid lockup_options format: %w", err)
		}
		if err := validation.ValidateMetadataSize("lockup_options", lockupBytes); err != nil {
			return err
		}
	}

	return nil
}

type CreateMintRequest struct {
	SignedRequest
	Payload CreateMintRequestPayload `json:"payload"`
}

type CreateMintRequestPayload struct {
	Title                    string                         `json:"title"`
	FractionCount            int                            `json:"fraction_count"`
	Description              string                         `json:"description"`
	Tags                     store.StringArray              `json:"tags,omitempty"`
	Metadata                 store.StringInterfaceMap       `json:"metadata,omitempty"`
	Requirements             store.StringInterfaceMap       `json:"requirements,omitempty"`
	LockupOptions            store.StringInterfaceMap       `json:"lockup_options,omitempty"`
	FeedURL                  string                         `json:"feed_url,omitempty"`
	ContractOfSale           string                         `json:"contract_of_sale,omitempty"`
	OwnerAddress             string                         `json:"owner_address"`
	SignatureRequirementType store.SignatureRequirementType `json:"signature_requirement_type,omitempty"`
	AssetManagers            []store.AssetManager           `json:"asset_managers,omitempty"`
	MinSignatures            int                            `json:"min_signatures,omitempty"`
}

func (req *CreateMintRequest) Validate() error {
	if err := validation.ValidateAddress(req.Payload.OwnerAddress); err != nil {
		return fmt.Errorf("invalid address: %w", err)
	}

	if err := validation.ValidatePublicKey(req.PublicKey); err != nil {
		return fmt.Errorf("invalid public_key: %w", err)
	}

	if err := doge.ValidateSignature(req.Payload, req.PublicKey, req.Signature); err != nil {
		return err
	}

	return nil
}

type CreateMintResponse struct {
	Hash                   string `json:"hash"`
	EncodedTransactionBody string `json:"encoded_transaction_body"`
}

type GetTokenBalanceResponse struct {
	MintHash string `json:"mint_hash"`
	Balance  int    `json:"balance"`
}

type GetTokenBalanceWithMintsResponse struct {
	Mints []store.TokenBalanceWithMint `json:"mints"`
	Total int                          `json:"total"`
	Page  int                          `json:"page"`
	Limit int                          `json:"limit"`
}

type GetMintsResponse struct {
	Mints []store.Mint `json:"mints"`
	Total int          `json:"total"`
	Page  int          `json:"page"`
	Limit int          `json:"limit"`
}

type GetStatsResponse struct {
	Stats map[string]int `json:"stats"`
}

type CreateBuyOfferRequest struct {
	SignedRequest
	Payload CreateBuyOfferRequestPayload `json:"payload"`
}

type CreateBuyOfferRequestPayload struct {
	OffererAddress string `json:"offerer_address"`
	SellerAddress  string `json:"seller_address"`
	MintHash       string `json:"mint_hash"`
	Quantity       int    `json:"quantity"`
	Price          int    `json:"price"`
}

type DeleteBuyOfferRequest struct {
	SignedRequest
	Payload DeleteBuyOfferRequestPayload `json:"payload"`
}

type DeleteBuyOfferRequestPayload struct {
	OfferHash string `json:"offer_hash"`
}

type DeleteSellOfferRequest struct {
	SignedRequest
	Payload DeleteSellOfferRequestPayload `json:"payload"`
}

type DeleteSellOfferRequestPayload struct {
	OfferHash string `json:"offer_hash"`
}

func (req *DeleteBuyOfferRequest) Validate() error {
	if err := validation.ValidateHash(req.Payload.OfferHash); err != nil {
		return fmt.Errorf("invalid offer_hash: %w", err)
	}

	if err := doge.ValidateSignature(req.Payload, req.PublicKey, req.Signature); err != nil {
		return err
	}

	return nil
}

func (req *DeleteSellOfferRequest) Validate() error {
	if err := validation.ValidateHash(req.Payload.OfferHash); err != nil {
		return fmt.Errorf("invalid offer_hash: %w", err)
	}

	if err := doge.ValidateSignature(req.Payload, req.PublicKey, req.Signature); err != nil {
		return err
	}

	return nil
}

func (req *CreateBuyOfferRequest) Validate() error {
	if err := validation.ValidateAddress(req.Payload.OffererAddress); err != nil {
		return fmt.Errorf("invalid offerer_address: %w", err)
	}

	if err := validation.ValidateAddress(req.Payload.SellerAddress); err != nil {
		return fmt.Errorf("invalid seller_address: %w", err)
	}

	if err := validation.ValidateHash(req.Payload.MintHash); err != nil {
		return fmt.Errorf("invalid mint_hash: %w", err)
	}

	if err := validation.ValidateQuantity("quantity", req.Payload.Quantity); err != nil {
		return err
	}

	if err := validation.ValidatePrice("price", req.Payload.Price); err != nil {
		return err
	}

	if err := doge.ValidateSignature(req.Payload, req.PublicKey, req.Signature); err != nil {
		return err
	}

	return nil
}

type CreateSellOfferRequest struct {
	SignedRequest
	Payload CreateSellOfferRequestPayload `json:"payload"`
}

type CreateSellOfferRequestPayload struct {
	OffererAddress string `json:"offerer_address"`
	MintHash       string `json:"mint_hash"`
	Quantity       int    `json:"quantity"`
	Price          int    `json:"price"`
}

func (req *CreateSellOfferRequest) Validate() error {
	if err := validation.ValidateAddress(req.Payload.OffererAddress); err != nil {
		return fmt.Errorf("invalid offerer_address: %w", err)
	}

	if err := validation.ValidateHash(req.Payload.MintHash); err != nil {
		return fmt.Errorf("invalid mint_hash: %w", err)
	}

	if err := validation.ValidateQuantity("quantity", req.Payload.Quantity); err != nil {
		return err
	}

	if err := validation.ValidatePrice("price", req.Payload.Price); err != nil {
		return err
	}

	if err := doge.ValidateSignature(req.Payload, req.PublicKey, req.Signature); err != nil {
		return err
	}

	return nil
}

type CreateOfferResponse struct {
	Id   string `json:"id"`
	Hash string `json:"hash"`
}

type GetMintResponse struct {
	Mint store.Mint `json:"mint"`
}

type SellOfferWithMint struct {
	Offer store.SellOffer `json:"offer"`
	Mint  store.Mint      `json:"mint"`
}

type BuyOfferWithMint struct {
	Offer store.BuyOffer `json:"offer"`
	Mint  store.Mint     `json:"mint"`
}

type GetSellOffersResponse struct {
	Offers []SellOfferWithMint `json:"offers"`
	Total  int                 `json:"total"`
	Page   int                 `json:"page"`
	Limit  int                 `json:"limit"`
}

type GetBuyOffersResponse struct {
	Offers []BuyOfferWithMint `json:"offers"`
	Total  int                `json:"total"`
	Page   int                `json:"page"`
	Limit  int                `json:"limit"`
}

type CreateInvoiceRequest struct {
	SignedRequest
	Payload CreateInvoiceRequestPayload `json:"payload"`
}

type CreateNewPaymentRequest struct {
	InvoiceHash string `json:"invoice_hash"`
}

type CreateInvoiceRequestPayload struct {
	PaymentAddress string `json:"payment_address"`
	BuyerAddress   string `json:"buyer_address"`
	MintHash       string `json:"mint_hash"`
	Quantity       int    `json:"quantity"`
	Price          int    `json:"price"`
	SellerAddress  string `json:"seller_address"`
}

func (req *CreateInvoiceRequest) Validate() error {
	if err := validation.ValidateAddress(req.Payload.PaymentAddress); err != nil {
		return fmt.Errorf("invalid payment_address: %w", err)
	}

	if err := validation.ValidateAddress(req.Payload.BuyerAddress); err != nil {
		return fmt.Errorf("invalid buyer_address: %w", err)
	}

	if err := validation.ValidateAddress(req.Payload.SellerAddress); err != nil {
		return fmt.Errorf("invalid seller_address: %w", err)
	}

	if err := validation.ValidateHash(req.Payload.MintHash); err != nil {
		return fmt.Errorf("invalid mint_hash: %w", err)
	}

	if err := validation.ValidateQuantity("quantity", req.Payload.Quantity); err != nil {
		return err
	}

	if err := validation.ValidatePrice("price", req.Payload.Price); err != nil {
		return err
	}

	if err := doge.ValidateSignature(req.Payload, req.PublicKey, req.Signature); err != nil {
		return err
	}

	if err := validation.ValidateAddressPublicKeyMatch(req.Payload.SellerAddress, req.PublicKey); err != nil {
		return err
	}

	return nil
}

type GetInvoicesResponse struct {
	Invoices []store.Invoice `json:"invoices"`
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	Limit    int             `json:"limit"`
}

type CreateInvoiceResponse struct {
	Hash                   string `json:"hash"`
	EncodedTransactionBody string `json:"encoded_transaction_body"`
}

type GetHealthResponse struct {
	CurrentBlockHeight int64     `json:"current_block_height"`
	LatestBlockHeight  int64     `json:"latest_block_height"`
	Chain              string    `json:"chain"`
	WalletsEnabled     bool      `json:"wallets_enabled"`
	UpdatedAt          time.Time `json:"updated_at"`
	Version            string    `json:"version"`
}

type Address struct {
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	Label      string `json:"label"`
}

type SignTxRequest struct {
	Payload                PrepareMintRequest `json:"payload"`
	Signature              string             `json:"signature"`
	PublicKey              string             `json:"public_key"`
	EncodedTransactionBody string             `json:"encoded_transaction_body"`
}
