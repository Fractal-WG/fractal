package rpc

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"time"

	"dogecoin.org/fractal-engine/pkg/config"
	engineprotocol "dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	dogeconnect "github.com/dogeorg/dogeconnect-go"
)

// pubKeyHashStr encodes the first 15 bytes of the SHA256 of the Gateway Public Key
// in URL-safe Base64 (RFC 4648); 15 is divisible by 3, which avoids Base64 padding.
func pubKeyHashStr(pubKey []byte) string {
	pkHash := sha256.Sum256(pubKey)
	return base64.URLEncoding.EncodeToString(pkHash[0:15]) // 15 bytes -> 20 chars
}

type DCHandler struct {
	store *store.TokenisationStore
	cfg   *config.Config
}

func NewDCHandler(s *store.TokenisationStore, cfg *config.Config) *DCHandler {
	return &DCHandler{store: s, cfg: cfg}
}

// mintPayload mirrors dogeconnect.ConnectPayment but uses mintOutput to support
// OP_RETURN data outputs, which the standard ConnectOutput does not.
type mintPayload struct {
	Type       string                    `json:"type"`
	ID         string                    `json:"id"`
	Issued     string                    `json:"issued"`
	Timeout    int                       `json:"timeout"`
	FeePerKB   string                    `json:"fee_per_kb"`
	MaxSize    int                       `json:"max_size"`
	VendorName string                    `json:"vendor_name,omitempty"`
	Total      string                    `json:"total"`
	Fees       string                    `json:"fees"`
	Taxes      string                    `json:"taxes"`
	Items      []dogeconnect.ConnectItem `json:"items"`
	Outputs    []mintOutput              `json:"outputs"`
}

// mintOutput extends the standard ConnectOutput with a data field for OP_RETURN outputs.
type mintOutput struct {
	Type    string `json:"type,omitempty"`
	Data    string `json:"data,omitempty"`
	Address string `json:"address,omitempty"`
	Amount  string `json:"amount,omitempty"`
}

func (h *DCHandler) ServeMint(w http.ResponseWriter, r *http.Request) {
	hash := r.PathValue("hash")
	if len(hash) != 64 {
		http.Error(w, "invalid mint hash", http.StatusBadRequest)
		return
	}

	// Look up mint: unconfirmed first (pending submission), then confirmed
	mint, err := h.store.GetUnconfirmedMintByHash(r.Context(), hash)
	if err != nil || mint.Hash == "" {
		mint, err = h.store.GetMintByHash(r.Context(), hash)
		if err != nil || mint.Hash == "" {
			http.Error(w, "mint not found", http.StatusNotFound)
			return
		}
	}

	// Build OP_RETURN data for the mint transaction
	txEnvelope := engineprotocol.NewMintTransactionEnvelope(hash, engineprotocol.ACTION_MINT)
	opReturnHex := hex.EncodeToString(txEnvelope.Serialize())

	now := time.Now()
	payment := dogeconnect.ConnectPayment{
		Type:       "payment",
		ID:         hash[:16],
		Issued:     now.Format(time.RFC3339),
		Timeout:    0,
		FeePerKB:   "0",
		MaxSize:    0,
		VendorName: mint.Title,
		Total:      "0.00000000",
		Fees:       "0.00000000",
		Taxes:      "0.00000000",
		Items: []dogeconnect.ConnectItem{
			{
				Type:        "signing",
				ID:          hash,
				Name:        mint.Title,
				Description: mint.Description,
			},
		},
		Outputs: []dogeconnect.ConnectOutput{
			{
				Type: "data",
				Data: opReturnHex,
			},
		},
	}

	keyPair := h.cfg.DogeNetKeyPair
	if keyPair.Priv == nil || keyPair.Pub == nil {
		http.Error(w, "signing key not configured", http.StatusServiceUnavailable)
		return
	}

	envelope, err := dogeconnect.SignPaymentRequest(payment, keyPair.Priv[:])
	if err != nil {
		http.Error(w, "failed to sign request", http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, envelope)
}
