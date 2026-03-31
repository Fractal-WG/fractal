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
		Relay:      "http://10.0.0.2:8891",
		FeePerKB:   "0",
		MaxSize:    100000, // What should this be???
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
				Total:       "0",
				UnitCount:   1,
				UnitCost:    "0",
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
