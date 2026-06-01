package store

import (
	"bytes"
	"crypto/sha256"

	"database/sql"

	"database/sql/driver"
	"encoding/hex"

	"encoding/json"

	"fmt"

	"time"

	"dogecoin.org/fractal-engine/pkg/doge"
)

type StringInterfaceMap map[string]interface{}

func (m StringInterfaceMap) Value() (driver.Value, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (m StringInterfaceMap) Equal(other StringInterfaceMap) bool {
	// Marshal both sides to canonical JSON and compare bytes
	// encoding/json sorts map keys, and both 10 and 10.0 render as "10"
	aj, errA := json.Marshal(m)
	if errA != nil {
		return false
	}
	bj, errB := json.Marshal(other)
	if errB != nil {
		return false
	}
	return bytes.Equal(aj, bj)
}

func (m *StringInterfaceMap) Scan(src interface{}) error {
	var source []byte
	switch src := src.(type) {
	case string:
		source = []byte(src)
	case []byte:
		source = src
	case nil:
		*m = nil
		return nil
	default:
		return fmt.Errorf("unsupported type: %T", src)
	}
	return json.Unmarshal(source, m)
}

type AssetManager struct {
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
	URL       string `json:"url"`
}

func (a *AssetManager) Value() (driver.Value, error) {
	return json.Marshal(a)
}

func (a *AssetManager) Scan(src interface{}) error {
	return json.Unmarshal(src.([]byte), a)
}

type AssetManagers []AssetManager

// Value implements driver.Valuer — converts to JSON for DB insertion.
func (a AssetManagers) Value() (driver.Value, error) {
	// nil slice -> NULL in DB
	if a == nil {
		return nil, nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, fmt.Errorf("marshal AssetManagers: %w", err)
	}
	return string(b), nil // or return b ([]byte) — both work
}

// Scan implements sql.Scanner — converts DB value to the slice.
func (a *AssetManagers) Scan(src interface{}) error {
	if a == nil {
		return fmt.Errorf("AssetManagers: Scan on nil pointer")
	}
	if src == nil {
		*a = nil
		return nil
	}

	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("unsupported scan type for AssetManagers: %T", src)
	}

	if len(data) == 0 {
		*a = nil
		return nil
	}

	return json.Unmarshal(data, a)
}

type SignatureRequirementType string

const (
	SignatureRequirementType_ALL_SIGNATURES SignatureRequirementType = "REQUIRES_ALL_SIGNATURES"
	SignatureRequirementType_ONE_SIGNATURE  SignatureRequirementType = "REQUIRES_ONE_SIGNATURE"
	SignatureRequirementType_MIN_SIGNATURES SignatureRequirementType = "REQUIRES_MIN_SIGNATURES"
	SignatureRequirementType_NONE           SignatureRequirementType = "NONE"
)

type MintWithoutID struct {
	Hash                     string                   `json:"hash"`
	Title                    string                   `json:"title"`
	FractionCount            int                      `json:"fraction_count"`
	Description              string                   `json:"description"`
	Tags                     StringArray              `json:"tags"`
	Metadata                 StringInterfaceMap       `json:"metadata"`
	TransactionHash          string                   `json:"transaction_hash"`
	BlockHeight              int64                    `json:"block_height"`
	CreatedAt                time.Time                `json:"created_at"`
	Requirements             StringInterfaceMap       `json:"requirements"`
	LockupOptions            StringInterfaceMap       `json:"lockup_options"`
	FeedURL                  string                   `json:"feed_url"`
	PublicKey                string                   `json:"public_key"`
	OwnerAddress             string                   `json:"owner_address"`
	Signature                string                   `json:"signature"`
	ContractOfSale           string                   `json:"contract_of_sale"`
	SignatureRequirementType SignatureRequirementType `json:"signature_requirement_type"`
	AssetManagers            AssetManagers            `json:"asset_managers"`
	MinSignatures            int                      `json:"min_signatures"`
	AllowExpansion           bool                     `json:"allow_expansion"`
	CurrentSupply            int                      `json:"current_supply"`
}

type MintHash struct {
	Title                    string                   `json:"title"`
	FractionCount            int                      `json:"fraction_count"`
	Description              string                   `json:"description"`
	Tags                     StringArray              `json:"tags"`
	Metadata                 StringInterfaceMap       `json:"metadata"`
	Requirements             StringInterfaceMap       `json:"requirements"`
	LockupOptions            StringInterfaceMap       `json:"lockup_options"`
	OwnerAddress             string                   `json:"owner_address"`
	PublicKey                string                   `json:"public_key"`
	ContractOfSale           string                   `json:"contract_of_sale"`
	SignatureRequirementType SignatureRequirementType `json:"signature_requirement_type"`
	AssetManagers            AssetManagers            `json:"asset_managers"`
	MinSignatures            int                      `json:"min_signatures"`
}

type OnChainTransaction struct {
	Id                string             `json:"id"`
	TxHash            string             `json:"tx_hash"`
	Height            int64              `json:"height"`
	BlockHash         string             `json:"block_hash"`
	ActionType        uint8              `json:"action_type"`
	ActionVersion     uint8              `json:"action_version"`
	ActionData        []byte             `json:"action_data"`
	Address           string             `json:"address"`
	Values            StringInterfaceMap `json:"values"`
	TransactionNumber int                `json:"transaction_number"`
}

func (m *MintWithoutID) GenerateHash() (string, error) {
	input := MintHash{
		Title:                    m.Title,
		FractionCount:            m.FractionCount,
		Description:              m.Description,
		Tags:                     m.Tags,
		Metadata:                 m.Metadata,
		Requirements:             m.Requirements,
		LockupOptions:            m.LockupOptions,
		PublicKey:                m.PublicKey,
		ContractOfSale:           m.ContractOfSale,
		SignatureRequirementType: m.SignatureRequirementType,
		AssetManagers:            m.AssetManagers,
		MinSignatures:            m.MinSignatures,
	}

	// Serialize to JSON with sorted keys
	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	// Generate SHA-256 hash (32 bytes)
	hash := sha256.Sum256(jsonBytes)

	// Return as byte slice (length 32)
	return hex.EncodeToString(hash[:]), nil
}

type Mint struct {
	MintWithoutID
	Id string `json:"id"`
}

func (m *Mint) SignatureRequired() bool {
	if m.SignatureRequirementType == SignatureRequirementType_NONE || m.SignatureRequirementType == "" {
		return false
	}

	return true
}

func (m *Mint) HasRequiredSignatures(signatures []InvoiceSignature) bool {
	switch m.SignatureRequirementType {
	case SignatureRequirementType_ALL_SIGNATURES:
		return len(signatures) == len(m.AssetManagers)
	case SignatureRequirementType_ONE_SIGNATURE:
		return len(signatures) == 1
	case SignatureRequirementType_MIN_SIGNATURES:
		return len(signatures) >= m.MinSignatures
	}

	return false
}

type OnChainMint struct {
	MintId          string `json:"mint_id"`
	TransactionHash string `json:"transaction_hash"`
	Address         string `json:"address"`
}

type BuyOfferWithoutID struct {
	Hash           string    `json:"hash"`
	MintHash       string    `json:"mint_hash"`
	OffererAddress string    `json:"offerer_address"`
	SellerAddress  string    `json:"seller_address"`
	Quantity       int       `json:"quantity"`
	Price          int       `json:"price"`
	CreatedAt      time.Time `json:"created_at"`
	PublicKey      string    `json:"public_key"`
	Signature      string    `json:"signature"`
}

type SellOfferWithoutID struct {
	Hash           string    `json:"hash"`
	MintHash       string    `json:"mint_hash"`
	OffererAddress string    `json:"offerer_address"`
	Quantity       int       `json:"quantity"`
	Price          int       `json:"price"`
	CreatedAt      time.Time `json:"created_at"`
	PublicKey      string    `json:"public_key"`
	Signature      string    `json:"signature"`
}

type BuyOfferHash struct {
	MintHash       string `json:"mint_hash"`
	OffererAddress string `json:"offerer_address"`
	SellerAddress  string `json:"seller_address"`
	Quantity       int    `json:"quantity"`
	Price          int    `json:"price"`
	PublicKey      string `json:"public_key"`
}

func (o *BuyOfferWithoutID) GenerateHash() (string, error) {
	input := BuyOfferHash{
		MintHash:       o.MintHash,
		OffererAddress: o.OffererAddress,
		SellerAddress:  o.SellerAddress,
		Quantity:       o.Quantity,
		Price:          o.Price,
		PublicKey:      o.PublicKey,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonBytes)

	return hex.EncodeToString(hash[:]), nil
}

type SellOfferHash struct {
	MintHash       string `json:"mint_hash"`
	OffererAddress string `json:"offerer_address"`
	Quantity       int    `json:"quantity"`
	Price          int    `json:"price"`
	PublicKey      string `json:"public_key"`
	Signature      string `json:"signature"`
}

func (o *SellOfferWithoutID) GenerateHash() (string, error) {
	input := SellOfferHash{
		MintHash:       o.MintHash,
		OffererAddress: o.OffererAddress,
		Quantity:       o.Quantity,
		Price:          o.Price,
		PublicKey:      o.PublicKey,
		Signature:      o.Signature,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonBytes)

	return hex.EncodeToString(hash[:]), nil
}

type BuyOffer struct {
	BuyOfferWithoutID
	Id string `json:"id"`
}

type SellOffer struct {
	SellOfferWithoutID
	Id string `json:"id"`
}

type UnconfirmedInvoice struct {
	Id             string    `json:"id"`
	Hash           string    `json:"hash"`
	BuyerAddress   string    `json:"buyer_address"`
	MintHash       string    `json:"mint_hash"`
	Quantity       int       `json:"quantity"`
	Price          int       `json:"price"`
	CreatedAt      time.Time `json:"created_at"`
	PaymentAddress string    `json:"payment_address"`
	SellerAddress  string    `json:"seller_address"`
	PublicKey      string    `json:"public_key"`
	Signature      string    `json:"signature"`
	Status         string    `json:"status"`
}

func (u *UnconfirmedInvoice) GenerateHash() (string, error) {
	input := UnconfirmedInvoiceHash{
		MintHash:      u.MintHash,
		Quantity:      u.Quantity,
		Price:         u.Price,
		BuyerAddress:  u.BuyerAddress,
		SellerAddress: u.SellerAddress,
		PublicKey:     u.PublicKey,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonBytes)

	return hex.EncodeToString(hash[:]), nil
}

type UnconfirmedInvoiceHash struct {
	MintHash       string `json:"mint_hash"`
	Quantity       int    `json:"quantity"`
	Price          int    `json:"price"`
	BuyerAddress   string `json:"buyer_address"`
	PaymentAddress string `json:"payment_address"`
	SellerAddress  string `json:"seller_address"`
	PublicKey      string `json:"public_key"`
	Signature      string `json:"signature"`
}

type InvoiceHash struct {
	MintHash       string `json:"mint_hash"`
	Quantity       int    `json:"quantity"`
	Price          int    `json:"price"`
	PaymentAddress string `json:"payment_address"`
	SellerAddress  string `json:"seller_address"`
	PublicKey      string `json:"public_key"`
	Signature      string `json:"signature"`
}

type Invoice struct {
	Id                    string       `json:"id"`
	Hash                  string       `json:"hash"`
	PaymentAddress        string       `json:"payment_address"`
	BuyerAddress          string       `json:"buyer_address"`
	MintHash              string       `json:"mint_hash"`
	Quantity              int          `json:"quantity"`
	Price                 int          `json:"price"`
	CreatedAt             time.Time    `json:"created_at"`
	SellerAddress         string       `json:"seller_address"`
	BlockHeight           int64        `json:"block_height"`
	TransactionHash       string       `json:"transaction_hash"`
	PendingTokenBalanceId string       `json:"pending_token_balance_id"`
	PublicKey             string       `json:"public_key"`
	Signature             string       `json:"signature"`
	PaidAt                sql.NullTime `json:"paid_at"`
}

func (i *Invoice) GenerateHash() (string, error) {
	input := InvoiceHash{
		MintHash:       i.MintHash,
		Quantity:       i.Quantity,
		Price:          i.Price,
		PaymentAddress: i.PaymentAddress,
		SellerAddress:  i.SellerAddress,
		PublicKey:      i.PublicKey,
		Signature:      i.Signature,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonBytes)

	return hex.EncodeToString(hash[:]), nil
}

type InvoiceSignature struct {
	Id          string    `json:"id"`
	InvoiceHash string    `json:"invoice_hash"`
	Signature   string    `json:"signature"`
	PublicKey   string    `json:"public_key"`
	CreatedAt   time.Time `json:"created_at"`
}

type InvoiceSignatureBody struct {
	Hash           string `json:"hash"`
	MintHash       string `json:"mint_hash"`
	Price          int    `json:"price"`
	Quantity       int    `json:"quantity"`
	BuyerAddress   string `json:"buyer_address"`
	PaymentAddress string `json:"payment_address"`
	SellerAddress  string `json:"seller_address"`
}

func (i *InvoiceSignature) Validate(mint Mint, invoice UnconfirmedInvoice) error {
	var assetManager AssetManager

	for _, am := range mint.AssetManagers {
		if am.PublicKey == i.PublicKey {
			assetManager = am
			break
		}
	}

	if assetManager.PublicKey == "" {
		return fmt.Errorf("public key does not match any asset managers")
	}

	invoiceBody := InvoiceSignatureBody{
		Hash:           invoice.Hash,
		MintHash:       invoice.MintHash,
		Price:          invoice.Price,
		Quantity:       invoice.Quantity,
		BuyerAddress:   invoice.BuyerAddress,
		PaymentAddress: invoice.PaymentAddress,
		SellerAddress:  invoice.SellerAddress,
	}

	err := doge.ValidateSignature(invoiceBody, i.PublicKey, i.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	return nil
}

type UnconfirmedMintExpansion struct {
	Id               string    `json:"id"`
	Hash             string    `json:"hash"`
	MintHash         string    `json:"mint_hash"`
	AdditionalSupply int       `json:"additional_supply"`
	OwnerAddress     string    `json:"owner_address"`
	PublicKey        string    `json:"public_key"`
	Signature        string    `json:"signature"`
	Nonce            string    `json:"nonce"`
	CreatedAt        time.Time `json:"created_at"`
}

type MintExpansionHash struct {
	MintHash         string `json:"mint_hash"`
	AdditionalSupply int    `json:"additional_supply"`
	OwnerAddress     string `json:"owner_address"`
	PublicKey        string `json:"public_key"`
	Signature        string `json:"signature"`
	Nonce            string `json:"nonce"`
}

func (e *UnconfirmedMintExpansion) GenerateHash() (string, error) {
	input := MintExpansionHash{
		MintHash:         e.MintHash,
		AdditionalSupply: e.AdditionalSupply,
		OwnerAddress:     e.OwnerAddress,
		PublicKey:        e.PublicKey,
		Signature:        e.Signature,
		Nonce:            e.Nonce,
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:]), nil
}

type TokenBalanceWithMint struct {
	Mint
	Address   string    `json:"address"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TokenBalance struct {
	MintHash  string    `json:"mint_hash"`
	Address   string    `json:"address"`
	Quantity  int       `json:"quantity"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PendingTokenBalance struct {
	InvoiceHash  string    `json:"invoice_hash"`
	MintHash     string    `json:"mint_hash"`
	Quantity     int       `json:"quantity"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	OwnerAddress string    `json:"owner_address"`
}
