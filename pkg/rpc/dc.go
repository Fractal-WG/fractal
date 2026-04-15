package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/doge"
	engineprotocol "dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	dogeconnect "github.com/dogeorg/dogeconnect-go"
)

const (
	// dcRequiredConfirmations is the number of block confirmations the relay
	// reports as required before a transaction is considered final.
	dcRequiredConfirmations = 6
	// dcBlockTimeSec is the approximate Dogecoin block interval in seconds.
	dcBlockTimeSec = 60

	// Payment ID prefixes — type:hash format used in ConnectPayment.ID and
	// parsed by ServeRelayStatus to route the confirmation check.
	dcIDPrefixMint    = "mint"
	dcIDPrefixInvoice = "invoice"
	dcIDPrefixPayment = "payment"
)

// pubKeyHashStr encodes the first 15 bytes of the SHA256 of the Gateway Public Key
// in URL-safe Base64 (RFC 4648); 15 is divisible by 3, which avoids Base64 padding.
func pubKeyHashStr(pubKey []byte) string {
	pkHash := sha256.Sum256(pubKey)
	return base64.URLEncoding.EncodeToString(pkHash[0:15]) // 15 bytes -> 20 chars
}

type DCHandler struct {
	store      *store.TokenisationStore
	cfg        *config.Config
	dogeClient *doge.RpcClient
}

func NewDCHandler(s *store.TokenisationStore, cfg *config.Config, dogeClient *doge.RpcClient) *DCHandler {
	return &DCHandler{store: s, cfg: cfg, dogeClient: dogeClient}
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
		ID:         dcIDPrefixMint + ":" + hash,
		Issued:     now.Format(time.RFC3339),
		Timeout:    9999999999999, // How much should i set this too??
		Relay:      h.cfg.RelayURL,
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

// ServeRelayPay handles POST /dc/relay/pay.
// Wallets submit a signed transaction here; the relay broadcasts it and returns payment status.
func (h *DCHandler) ServeRelayPay(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	var submission dogeconnect.PaymentSubmission
	if err := json.NewDecoder(r.Body).Decode(&submission); err != nil {
		respondJSON(w, http.StatusBadRequest, dogeconnect.ErrorResponse{
			Error:   "invalid_request",
			Message: "could not parse request body",
		})
		return
	}

	if _, errs := submission.Parse(); errs.Err() != nil {
		respondJSON(w, http.StatusBadRequest, dogeconnect.ErrorResponse{
			Error:   "invalid_request",
			Message: errs.Err().Error(),
		})
		return
	}

	if h.dogeClient == nil {
		respondJSON(w, http.StatusServiceUnavailable, dogeconnect.ErrorResponse{
			Error:   "node_unavailable",
			Message: "dogecoin node not configured",
		})
		return
	}

	txid, err := h.dogeClient.SendRawTransaction(r.Context(), submission.Tx)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, dogeconnect.ErrorResponse{
			Error:   "broadcast_failed",
			Message: err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, dogeconnect.PaymentStatusResponse{
		ID:     submission.ID,
		Status: dogeconnect.PaymentStatusAccepted,
		TxID:   txid,
	})
}

// ServeRelayStatus handles POST /dc/relay/status.
// Parses the type:hash payment ID and checks the store directly.
func (h *DCHandler) ServeRelayStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	var query dogeconnect.StatusQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		respondJSON(w, http.StatusBadRequest, dogeconnect.ErrorResponse{
			Error:   "invalid_request",
			Message: "could not parse request body",
		})
		return
	}

	if errs := query.Validate(); errs.Err() != nil {
		respondJSON(w, http.StatusBadRequest, dogeconnect.ErrorResponse{
			Error:   "invalid_request",
			Message: errs.Err().Error(),
		})
		return
	}

	status, err := h.checkConfirmationStatus(r.Context(), query.ID)
	if err != nil {
		respondJSON(w, http.StatusNotFound, dogeconnect.ErrorResponse{
			Error:   dogeconnect.ErrorCodeNotFound,
			Message: err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, status)
}

// checkConfirmationStatus parses a type:hash payment ID, queries the store,
// and enriches the response with live confirmation counts from the node.
func (h *DCHandler) checkConfirmationStatus(ctx context.Context, id string) (dogeconnect.PaymentStatusResponse, error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return dogeconnect.PaymentStatusResponse{}, errors.New("invalid payment id format")
	}
	kind, hash := parts[0], parts[1]

	var (
		status dogeconnect.PaymentStatusResponse
		err    error
	)
	switch kind {
	case dcIDPrefixMint:
		status, err = h.mintConfirmationStatus(ctx, id, hash)
	case dcIDPrefixInvoice:
		status, err = h.invoiceConfirmationStatus(ctx, id, hash)
	case dcIDPrefixPayment:
		status, err = h.paymentConfirmationStatus(ctx, id, hash)
	default:
		return dogeconnect.PaymentStatusResponse{}, errors.New("unknown payment id prefix")
	}
	if err != nil {
		return dogeconnect.PaymentStatusResponse{}, err
	}

	return h.enrichWithConfirmations(ctx, status), nil
}

// enrichWithConfirmations populates Required, Confirmed, DueSec, and
// ConfirmedAt for accepted/confirmed responses. Required and DueSec are
// always set; Confirmed is populated from the node when possible (best-effort).
func (h *DCHandler) enrichWithConfirmations(ctx context.Context, status dogeconnect.PaymentStatusResponse) dogeconnect.PaymentStatusResponse {
	if status.Status != dogeconnect.PaymentStatusAccepted && status.Status != dogeconnect.PaymentStatusConfirmed {
		return status
	}

	required := dcRequiredConfirmations
	status.Required = &required

	// Best-effort: query the node for the live confirmation count.
	confirmed := 0
	if h.dogeClient != nil && status.TxID != "" {
		if confs, err := h.dogeClient.GetTransactionConfirmations(ctx, status.TxID); err == nil {
			confirmed = int(confs)
		}
	}
	status.Confirmed = &confirmed

	switch status.Status {
	case dogeconnect.PaymentStatusAccepted:
		dueSec := max(dcRequiredConfirmations-confirmed, 0) * dcBlockTimeSec
		status.DueSec = &dueSec
	case dogeconnect.PaymentStatusConfirmed:
		status.ConfirmedAt = time.Now().UTC().Format(time.RFC3339)
	}

	return status
}

// mintConfirmationStatus checks the mints and unconfirmed_mints tables.
// The txid is read from the store record so enrichWithConfirmations can query the node.
func (h *DCHandler) mintConfirmationStatus(ctx context.Context, id, hash string) (dogeconnect.PaymentStatusResponse, error) {
	mint, err := h.store.GetMintByHash(ctx, hash)
	if err == nil && mint.Hash != "" {
		return dogeconnect.PaymentStatusResponse{
			ID:     id,
			Status: dogeconnect.PaymentStatusConfirmed,
			TxID:   mint.TransactionHash,
		}, nil
	}

	unconfirmed, err := h.store.GetUnconfirmedMintByHash(ctx, hash)
	if err == nil && unconfirmed.Hash != "" {
		return dogeconnect.PaymentStatusResponse{
			ID:     id,
			Status: dogeconnect.PaymentStatusAccepted,
			TxID:   unconfirmed.TransactionHash,
		}, nil
	}

	return dogeconnect.PaymentStatusResponse{}, errors.New("mint not found")
}

// invoiceConfirmationStatus checks the invoices and unconfirmed_invoices tables.
func (h *DCHandler) invoiceConfirmationStatus(ctx context.Context, id, hash string) (dogeconnect.PaymentStatusResponse, error) {
	invoice, err := h.store.GetInvoiceByHash(ctx, hash)
	if err == nil && invoice.Id != "" {
		return dogeconnect.PaymentStatusResponse{
			ID:     id,
			Status: dogeconnect.PaymentStatusConfirmed,
			TxID:   invoice.TransactionHash,
		}, nil
	}

	unconfirmed, err := h.store.GetUnconfirmedInvoiceByHash(ctx, hash)
	if err == nil && unconfirmed.Id != "" {
		return dogeconnect.PaymentStatusResponse{
			ID:     id,
			Status: dogeconnect.PaymentStatusAccepted,
		}, nil
	}

	return dogeconnect.PaymentStatusResponse{}, errors.New("invoice not found")
}

// paymentConfirmationStatus checks whether the invoice has been paid.
// There is no separate payments table — paid_at on the invoice signals completion.
func (h *DCHandler) paymentConfirmationStatus(ctx context.Context, id, invoiceHash string) (dogeconnect.PaymentStatusResponse, error) {
	invoice, err := h.store.GetInvoiceByHash(ctx, invoiceHash)
	if err != nil || invoice.Id == "" {
		return dogeconnect.PaymentStatusResponse{}, errors.New("invoice not found")
	}

	if invoice.PaidAt.Valid {
		return dogeconnect.PaymentStatusResponse{
			ID:     id,
			Status: dogeconnect.PaymentStatusConfirmed,
			TxID:   invoice.TransactionHash,
		}, nil
	}

	return dogeconnect.PaymentStatusResponse{
		ID:     id,
		Status: dogeconnect.PaymentStatusAccepted,
		TxID:   invoice.TransactionHash,
	}, nil
}
