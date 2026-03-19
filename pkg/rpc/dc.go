package rpc

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"dogecoin.org/fractal-engine/pkg/config"
	engineprotocol "dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"github.com/dogeorg/doge"
)

type DCHandler struct {
	store *store.TokenisationStore
	cfg   *config.Config
}

func NewDCHandler(s *store.TokenisationStore, cfg *config.Config) *DCHandler {
	return &DCHandler{store: s, cfg: cfg}
}

type connectEnvelope struct {
	Version string `json:"version"`
	Payload string `json:"payload"`
	PubKey  string `json:"pubkey"`
	Sig     string `json:"sig"`
}

type connectPayment struct {
	Type          string          `json:"type"`
	ID            string          `json:"id"`
	Issued        string          `json:"issued"`
	Timeout       int             `json:"timeout"`
	Relay         string          `json:"relay,omitempty"`
	FeePerKb      string          `json:"fee_per_kb"`
	MaxSize       int             `json:"max_size"`
	VendorName    string          `json:"vendor_name,omitempty"`
	VendorAddress string          `json:"vendor_address,omitempty"`
	Total         string          `json:"total"`
	Fees          string          `json:"fees"`
	Taxes         string          `json:"taxes"`
	Items         []connectItem   `json:"items"`
	Outputs       []connectOutput `json:"outputs"`
}

type connectItem struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Desc string `json:"desc,omitempty"`
}

type connectOutput struct {
	Type    string `json:"type"`
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
	// Assume 0's mean no limit?
	payment := connectPayment{
		Type:       "payment",
		ID:         hash[:16],
		Issued:     now.Format(time.RFC3339),
		Timeout:    0,
		FeePerKb:   "0",
		MaxSize:    0,
		VendorName: mint.Title,
		Total:      "0.00000000",
		Fees:       "0.00000000",
		Taxes:      "0.00000000",
		Items: []connectItem{
			{
				Type: "signing",
				ID:   hash,
				Name: mint.Title,
				Desc: mint.Description,
			},
		},
		Outputs: []connectOutput{
			{
				Type: "data",
				Data: opReturnHex,
			},
		},
	}

	payloadBytes, err := json.Marshal(payment)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	payloadB64 := base64.StdEncoding.EncodeToString(payloadBytes)

	// Sign with the engine's Dogecoin Schnorr key (EC-Schnorr-Dogecoin over BLAKE-256)
	keyPair := h.cfg.DogeNetKeyPair
	if keyPair.Priv == nil || keyPair.Pub == nil {
		http.Error(w, "signing key not configured", http.StatusServiceUnavailable)
		return
	}

	sig, err := doge.SignMessage(keyPair.Priv, payloadBytes)
	if err != nil {
		http.Error(w, "signing error", http.StatusInternalServerError)
		return
	}

	result := connectEnvelope{
		Version: "1.0",
		Payload: payloadB64,
		PubKey:  hex.EncodeToString(keyPair.Pub[:]),
		Sig:     hex.EncodeToString(sig[:]),
	}

	respondJSON(w, http.StatusOK, result)
}
