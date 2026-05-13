package rpc_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	test_support "dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"dogecoin.org/fractal-engine/pkg/store"
	dogeconnect "github.com/dogeorg/dogeconnect-go"
	"code.dogecoin.org/gossip/dnet"
	"gotest.tools/assert"
)

// SetupDCTest creates a DCHandler wired to a fresh in-memory DB and a
// generated signing key pair.  It returns the httptest.Server, a store for
// pre-populating data, and the context to use in store calls.
func SetupDCTest(t *testing.T) (*httptest.Server, *store.TokenisationStore) {
	t.Helper()

	tokenStore := test_support.SetupTestDB(t)

	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)

	cfg := config.NewConfig()
	cfg.DogeNetKeyPair = keyPair

	mux := http.NewServeMux()
	dcHandler := rpc.NewDCHandler(tokenStore, cfg, nil)
	mux.HandleFunc("GET /dc/mint/{hash}", dcHandler.ServeMint)
	mux.HandleFunc("GET /dc/invoice/{hash}", dcHandler.ServeInvoice)
	mux.HandleFunc("GET /dc/payment/{hash}", dcHandler.ServePayment)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, tokenStore
}

// decodeEnvelopePayload parses the ConnectEnvelope from a response body and
// decodes the base64 payload into a ConnectPayment.
func decodeEnvelopePayload(t *testing.T, body []byte) dogeconnect.ConnectPayment {
	t.Helper()

	var env dogeconnect.ConnectEnvelope
	assert.NilError(t, json.Unmarshal(body, &env))
	assert.Assert(t, env.Payload != "", "envelope payload is empty")

	raw, err := base64.StdEncoding.DecodeString(env.Payload)
	assert.NilError(t, err)

	var payment dogeconnect.ConnectPayment
	assert.NilError(t, json.Unmarshal(raw, &payment))
	return payment
}

// --- ServeMint ---------------------------------------------------------

func TestServeMint_UnconfirmedFound(t *testing.T) {
	srv, tokenStore := SetupDCTest(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "Test Mint",
		Description:   "A test mint",
		FractionCount: 100,
	})
	assert.NilError(t, err)

	resp, err := srv.Client().Get(srv.URL + "/dc/mint/" + mintHash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	var buf [65536]byte
	n, _ := resp.Body.Read(buf[:])
	payment := decodeEnvelopePayload(t, buf[:n])

	assert.Equal(t, string(payment.Type), "payment")
	assert.Equal(t, payment.ID, "mint:"+mintHash)
	assert.Equal(t, payment.VendorName, "Test Mint")
	assert.Equal(t, payment.Total, "0.00000000")
	assert.Equal(t, len(payment.Items), 1)
	assert.Equal(t, payment.Items[0].ID, mintHash)
	assert.Equal(t, len(payment.Outputs), 1)
	assert.Equal(t, string(payment.Outputs[0].Type), "data")
}

func TestServeMint_ConfirmedFound(t *testing.T) {
	srv, tokenStore := SetupDCTest(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "Confirmed Mint",
		Description:   "A confirmed mint",
		FractionCount: 50,
	}, "owner")
	assert.NilError(t, err)

	resp, err := srv.Client().Get(srv.URL + "/dc/mint/" + mintHash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	var buf [65536]byte
	n, _ := resp.Body.Read(buf[:])
	payment := decodeEnvelopePayload(t, buf[:n])

	assert.Equal(t, payment.ID, "mint:"+mintHash)
	assert.Equal(t, payment.VendorName, "Confirmed Mint")
}

func TestServeMint_NotFound(t *testing.T) {
	srv, _ := SetupDCTest(t)

	hash := test_support.GenerateRandomHash()
	resp, err := srv.Client().Get(srv.URL + "/dc/mint/" + hash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

func TestServeMint_InvalidHash(t *testing.T) {
	srv, _ := SetupDCTest(t)

	resp, err := srv.Client().Get(srv.URL + "/dc/mint/not-a-valid-hash")
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestServeMint_NoSigningKey(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)

	// Config with empty key pair (no signing key).
	cfg := config.NewConfig()
	mux := http.NewServeMux()
	dcHandler := rpc.NewDCHandler(tokenStore, cfg, nil)
	mux.HandleFunc("GET /dc/mint/{hash}", dcHandler.ServeMint)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx := context.Background()
	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "Mint",
		FractionCount: 1,
	})
	assert.NilError(t, err)

	resp, err := srv.Client().Get(srv.URL + "/dc/mint/" + mintHash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusServiceUnavailable)
}

// --- ServeInvoice ------------------------------------------------------

func TestServeInvoice_UnconfirmedFound(t *testing.T) {
	srv, tokenStore := SetupDCTest(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "Invoice Mint",
		Description:   "mint for invoice test",
		FractionCount: 100,
	}, "owner")
	assert.NilError(t, err)

	invoice := store.UnconfirmedInvoice{
		Id:             "inv1",
		MintHash:       mintHash,
		Quantity:       5,
		Price:          1000000, // 0.01 DOGE per token
		BuyerAddress:   test_support.GenerateDogecoinAddress(true),
		SellerAddress:  test_support.GenerateDogecoinAddress(true),
		PaymentAddress: test_support.GenerateDogecoinAddress(true),
		PublicKey:      "pubkey",
		Signature:      "sig",
		CreatedAt:      time.Now(),
	}
	var hashErr error
	invoice.Hash, hashErr = invoice.GenerateHash()
	assert.NilError(t, hashErr)

	_, err = tokenStore.SaveUnconfirmedInvoice(ctx, &invoice)
	assert.NilError(t, err)

	resp, err := srv.Client().Get(srv.URL + "/dc/invoice/" + invoice.Hash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	var buf [65536]byte
	n, _ := resp.Body.Read(buf[:])
	payment := decodeEnvelopePayload(t, buf[:n])

	assert.Equal(t, string(payment.Type), "payment")
	assert.Equal(t, payment.ID, "invoice:"+invoice.Hash)
	assert.Equal(t, payment.VendorName, "Invoice Mint")
	assert.Equal(t, payment.Total, "0.00000000")
	assert.Equal(t, len(payment.Items), 1)
	assert.Equal(t, payment.Items[0].ID, invoice.Hash)
	assert.Equal(t, len(payment.Outputs), 1)
	assert.Equal(t, string(payment.Outputs[0].Type), "data")
}

func TestServeInvoice_NotFound(t *testing.T) {
	srv, _ := SetupDCTest(t)

	hash := test_support.GenerateRandomHash()
	resp, err := srv.Client().Get(srv.URL + "/dc/invoice/" + hash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

func TestServeInvoice_InvalidHash(t *testing.T) {
	srv, _ := SetupDCTest(t)

	resp, err := srv.Client().Get(srv.URL + "/dc/invoice/short")
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestServeInvoice_MintNotFound(t *testing.T) {
	srv, tokenStore := SetupDCTest(t)
	ctx := context.Background()

	// Save an invoice whose mint does not exist in the store.
	invoice := store.UnconfirmedInvoice{
		Id:             "inv2",
		MintHash:       test_support.GenerateRandomHash(), // no matching mint
		Quantity:       1,
		Price:          100,
		BuyerAddress:   test_support.GenerateDogecoinAddress(true),
		SellerAddress:  test_support.GenerateDogecoinAddress(true),
		PaymentAddress: test_support.GenerateDogecoinAddress(true),
		PublicKey:      "pk",
		Signature:      "sig",
		CreatedAt:      time.Now(),
	}
	var hashErr error
	invoice.Hash, hashErr = invoice.GenerateHash()
	assert.NilError(t, hashErr)

	_, err := tokenStore.SaveUnconfirmedInvoice(ctx, &invoice)
	assert.NilError(t, err)

	resp, err := srv.Client().Get(srv.URL + "/dc/invoice/" + invoice.Hash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

// --- ServePayment -------------------------------------------------------

func TestServePayment_ConfirmedInvoice(t *testing.T) {
	srv, tokenStore := SetupDCTest(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "Payment Mint",
		Description:   "mint for payment test",
		FractionCount: 100,
	}, "owner")
	assert.NilError(t, err)

	sellerAddr := test_support.GenerateDogecoinAddress(true)
	confirmedInvoice := store.Invoice{
		MintHash:        mintHash,
		Quantity:        2,
		Price:           50000000, // 0.5 DOGE per token → total 1 DOGE
		BuyerAddress:    test_support.GenerateDogecoinAddress(true),
		SellerAddress:   sellerAddr,
		PaymentAddress:  test_support.GenerateDogecoinAddress(true),
		PublicKey:       "pubkey",
		Signature:       "sig",
		TransactionHash: "txhash",
		BlockHeight:     1,
		CreatedAt:       time.Now(),
	}
	var hashErr error
	confirmedInvoice.Hash, hashErr = confirmedInvoice.GenerateHash()
	assert.NilError(t, hashErr)

	_, err = tokenStore.SaveInvoice(ctx, &confirmedInvoice)
	assert.NilError(t, err)

	resp, err := srv.Client().Get(srv.URL + "/dc/payment/" + confirmedInvoice.Hash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	var buf [65536]byte
	n, _ := resp.Body.Read(buf[:])
	payment := decodeEnvelopePayload(t, buf[:n])

	assert.Equal(t, string(payment.Type), "payment")
	assert.Equal(t, payment.ID, "payment:"+confirmedInvoice.Hash)
	assert.Equal(t, payment.VendorName, "Payment Mint")
	assert.Equal(t, payment.Total, "1") // 2 * 0.5 DOGE; koinu trims trailing zeros

	assert.Equal(t, len(payment.Items), 1)
	assert.Equal(t, payment.Items[0].UnitCount, 2)
	assert.Equal(t, payment.Items[0].UnitCost, "0.5")
	assert.Equal(t, payment.Items[0].Total, "1")

	// Outputs: p2pkh to seller + OP_RETURN data
	assert.Equal(t, len(payment.Outputs), 2)
	assert.Equal(t, string(payment.Outputs[0].Type), "p2pkh")
	assert.Equal(t, payment.Outputs[0].Address, sellerAddr)
	assert.Equal(t, payment.Outputs[0].Amount, "1")
	assert.Equal(t, string(payment.Outputs[1].Type), "data")
}

func TestServePayment_FallsBackToUnconfirmedInvoice(t *testing.T) {
	srv, tokenStore := SetupDCTest(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "Unconfirmed Payment Mint",
		Description:   "mint for unconfirmed payment test",
		FractionCount: 100,
	}, "owner")
	assert.NilError(t, err)

	sellerAddr := test_support.GenerateDogecoinAddress(true)
	invoice := store.UnconfirmedInvoice{
		Id:             "uinv1",
		MintHash:       mintHash,
		Quantity:       3,
		Price:          10000000, // 0.1 DOGE
		BuyerAddress:   test_support.GenerateDogecoinAddress(true),
		SellerAddress:  sellerAddr,
		PaymentAddress: test_support.GenerateDogecoinAddress(true),
		PublicKey:      "pk",
		Signature:      "sig",
		CreatedAt:      time.Now(),
	}
	var hashErr error
	invoice.Hash, hashErr = invoice.GenerateHash()
	assert.NilError(t, hashErr)

	_, err = tokenStore.SaveUnconfirmedInvoice(ctx, &invoice)
	assert.NilError(t, err)

	resp, err := srv.Client().Get(srv.URL + "/dc/payment/" + invoice.Hash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	var buf [65536]byte
	n, _ := resp.Body.Read(buf[:])
	payment := decodeEnvelopePayload(t, buf[:n])

	assert.Equal(t, payment.ID, "payment:"+invoice.Hash)
	assert.Equal(t, payment.Total, "0.3") // 3 * 0.1 DOGE; koinu trims trailing zeros
	assert.Equal(t, payment.Outputs[0].Address, sellerAddr)
}

func TestServePayment_NotFound(t *testing.T) {
	srv, _ := SetupDCTest(t)

	hash := test_support.GenerateRandomHash()
	resp, err := srv.Client().Get(srv.URL + "/dc/payment/" + hash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

func TestServePayment_InvalidHash(t *testing.T) {
	srv, _ := SetupDCTest(t)

	resp, err := srv.Client().Get(srv.URL + "/dc/payment/tooshort")
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

// --- ServeInvoice (additional) -----------------------------------------

func TestServeInvoice_ConfirmedFound(t *testing.T) {
	srv, tokenStore := SetupDCTest(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "Confirmed Invoice Mint",
		Description:   "mint for confirmed invoice test",
		FractionCount: 100,
	}, "owner")
	assert.NilError(t, err)

	inv := store.Invoice{
		MintHash:        mintHash,
		Quantity:        2,
		Price:           100,
		BuyerAddress:    test_support.GenerateDogecoinAddress(true),
		SellerAddress:   test_support.GenerateDogecoinAddress(true),
		PaymentAddress:  test_support.GenerateDogecoinAddress(true),
		PublicKey:       "pubkey",
		Signature:       "sig",
		TransactionHash: "txhash",
		BlockHeight:     1,
		CreatedAt:       time.Now(),
	}
	var hashErr error
	inv.Hash, hashErr = inv.GenerateHash()
	assert.NilError(t, hashErr)

	_, err = tokenStore.SaveInvoice(ctx, &inv)
	assert.NilError(t, err)

	resp, err := srv.Client().Get(srv.URL + "/dc/invoice/" + inv.Hash)
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	var buf [65536]byte
	n, _ := resp.Body.Read(buf[:])
	payment := decodeEnvelopePayload(t, buf[:n])

	assert.Equal(t, payment.ID, "invoice:"+inv.Hash)
	assert.Equal(t, payment.VendorName, "Confirmed Invoice Mint")
	assert.Equal(t, payment.Total, "0.00000000")
	assert.Equal(t, len(payment.Outputs), 1)
	assert.Equal(t, string(payment.Outputs[0].Type), "data")
}

// setupDCWithRelayEndpoints creates a DCHandler with all 5 routes registered,
// including the relay pay and status endpoints. Used for relay endpoint tests.
func setupDCWithRelayEndpoints(t *testing.T) (*httptest.Server, *store.TokenisationStore) {
	t.Helper()

	tokenStore := test_support.SetupTestDB(t)

	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)

	cfg := config.NewConfig()
	cfg.DogeNetKeyPair = keyPair

	mux := http.NewServeMux()
	dcHandler := rpc.NewDCHandler(tokenStore, cfg, nil)
	mux.HandleFunc("GET /dc/mint/{hash}", dcHandler.ServeMint)
	mux.HandleFunc("GET /dc/invoice/{hash}", dcHandler.ServeInvoice)
	mux.HandleFunc("GET /dc/payment/{hash}", dcHandler.ServePayment)
	mux.HandleFunc("POST /dc/relay/pay", dcHandler.ServeRelayPay)
	mux.HandleFunc("POST /dc/relay/status", dcHandler.ServeRelayStatus)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, tokenStore
}

// postRelayStatus POSTs a StatusQuery to /dc/relay/status and returns the response.
func postRelayStatus(t *testing.T, srv *httptest.Server, id string) *http.Response {
	t.Helper()
	body := `{"id":"` + id + `"}`
	resp, err := srv.Client().Post(srv.URL+"/dc/relay/status", "application/json", strings.NewReader(body))
	assert.NilError(t, err)
	return resp
}

// decodeStatusResponse reads and JSON-decodes a PaymentStatusResponse from a response body.
func decodeStatusResponse(t *testing.T, resp *http.Response) dogeconnect.PaymentStatusResponse {
	t.Helper()
	var buf [4096]byte
	n, _ := resp.Body.Read(buf[:])
	var status dogeconnect.PaymentStatusResponse
	assert.NilError(t, json.Unmarshal(buf[:n], &status))
	return status
}

// --- ServeRelayPay -----------------------------------------------------

func TestServeRelayPay_InvalidJSON(t *testing.T) {
	srv, _ := setupDCWithRelayEndpoints(t)

	resp, err := srv.Client().Post(srv.URL+"/dc/relay/pay", "application/json", strings.NewReader("{not json}"))
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestServeRelayPay_MissingTx(t *testing.T) {
	srv, _ := setupDCWithRelayEndpoints(t)

	// Valid JSON but missing required "tx" field → Parse() fails.
	resp, err := srv.Client().Post(srv.URL+"/dc/relay/pay", "application/json", strings.NewReader(`{"id":"mint:abc123"}`))
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestServeRelayPay_NoDogeClient(t *testing.T) {
	srv, _ := setupDCWithRelayEndpoints(t) // dogeClient is nil

	hash := test_support.GenerateRandomHash()
	body := `{"id":"mint:` + hash + `","tx":"deadbeef"}`
	resp, err := srv.Client().Post(srv.URL+"/dc/relay/pay", "application/json", strings.NewReader(body))
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusServiceUnavailable)

	var buf [4096]byte
	n, _ := resp.Body.Read(buf[:])
	var errResp dogeconnect.ErrorResponse
	assert.NilError(t, json.Unmarshal(buf[:n], &errResp))
	assert.Equal(t, string(errResp.Error), "node_unavailable")
}

// --- ServeRelayStatus --------------------------------------------------

func TestServeRelayStatus_InvalidJSON(t *testing.T) {
	srv, _ := setupDCWithRelayEndpoints(t)

	resp, err := srv.Client().Post(srv.URL+"/dc/relay/status", "application/json", strings.NewReader("{not json}"))
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestServeRelayStatus_EmptyID(t *testing.T) {
	srv, _ := setupDCWithRelayEndpoints(t)

	resp, err := srv.Client().Post(srv.URL+"/dc/relay/status", "application/json", strings.NewReader(`{"id":""}`))
	assert.NilError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusBadRequest)
}

func TestServeRelayStatus_InvalidIDFormat(t *testing.T) {
	srv, _ := setupDCWithRelayEndpoints(t)

	resp := postRelayStatus(t, srv, "nocolon")
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

func TestServeRelayStatus_UnknownPrefix(t *testing.T) {
	srv, _ := setupDCWithRelayEndpoints(t)

	hash := test_support.GenerateRandomHash()
	resp := postRelayStatus(t, srv, "unknown:"+hash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

// --- ServeRelayStatus: mint routing ------------------------------------

func TestServeRelayStatus_MintNotFound(t *testing.T) {
	srv, _ := setupDCWithRelayEndpoints(t)

	hash := test_support.GenerateRandomHash()
	resp := postRelayStatus(t, srv, "mint:"+hash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

func TestServeRelayStatus_MintUnpaid(t *testing.T) {
	srv, tokenStore := setupDCWithRelayEndpoints(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "Unpaid Mint",
		FractionCount: 1,
	})
	assert.NilError(t, err)

	resp := postRelayStatus(t, srv, "mint:"+mintHash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	status := decodeStatusResponse(t, resp)
	assert.Equal(t, status.ID, "mint:"+mintHash)
	assert.Equal(t, string(status.Status), "unpaid")
}

func TestServeRelayStatus_MintAccepted(t *testing.T) {
	srv, tokenStore := setupDCWithRelayEndpoints(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveUnconfirmedMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "Accepted Mint",
		FractionCount: 1,
	})
	assert.NilError(t, err)
	err = tokenStore.UpdateUnconfirmedMintTransactionHash(ctx, mintHash, "faketxid")
	assert.NilError(t, err)

	resp := postRelayStatus(t, srv, "mint:"+mintHash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	status := decodeStatusResponse(t, resp)
	assert.Equal(t, status.ID, "mint:"+mintHash)
	assert.Equal(t, string(status.Status), "accepted")
	assert.Equal(t, status.TxID, "faketxid")
	assert.Assert(t, status.Required != nil)
	assert.Assert(t, status.Confirmed != nil)
}

func TestServeRelayStatus_MintConfirmed(t *testing.T) {
	srv, tokenStore := setupDCWithRelayEndpoints(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "Confirmed Mint",
		FractionCount: 1,
	}, "owner")
	assert.NilError(t, err)

	resp := postRelayStatus(t, srv, "mint:"+mintHash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	status := decodeStatusResponse(t, resp)
	assert.Equal(t, status.ID, "mint:"+mintHash)
	assert.Equal(t, string(status.Status), "confirmed")
}

// --- ServeRelayStatus: invoice routing ---------------------------------

func TestServeRelayStatus_InvoiceNotFound(t *testing.T) {
	srv, _ := setupDCWithRelayEndpoints(t)

	hash := test_support.GenerateRandomHash()
	resp := postRelayStatus(t, srv, "invoice:"+hash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

func TestServeRelayStatus_InvoiceUnpaid(t *testing.T) {
	srv, tokenStore := setupDCWithRelayEndpoints(t)
	ctx := context.Background()

	invoice := store.UnconfirmedInvoice{
		MintHash:       test_support.GenerateRandomHash(),
		Quantity:       1,
		Price:          100,
		BuyerAddress:   test_support.GenerateDogecoinAddress(true),
		SellerAddress:  test_support.GenerateDogecoinAddress(true),
		PaymentAddress: test_support.GenerateDogecoinAddress(true),
		PublicKey:      "pk",
		Signature:      "sig",
		CreatedAt:      time.Now(),
	}
	var hashErr error
	invoice.Hash, hashErr = invoice.GenerateHash()
	assert.NilError(t, hashErr)
	_, err := tokenStore.SaveUnconfirmedInvoice(ctx, &invoice)
	assert.NilError(t, err)

	resp := postRelayStatus(t, srv, "invoice:"+invoice.Hash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	status := decodeStatusResponse(t, resp)
	assert.Equal(t, status.ID, "invoice:"+invoice.Hash)
	assert.Equal(t, string(status.Status), "unpaid")
}

func TestServeRelayStatus_InvoiceAccepted(t *testing.T) {
	srv, tokenStore := setupDCWithRelayEndpoints(t)
	ctx := context.Background()

	invoice := store.UnconfirmedInvoice{
		MintHash:       test_support.GenerateRandomHash(),
		Quantity:       1,
		Price:          100,
		BuyerAddress:   test_support.GenerateDogecoinAddress(true),
		SellerAddress:  test_support.GenerateDogecoinAddress(true),
		PaymentAddress: test_support.GenerateDogecoinAddress(true),
		PublicKey:      "pk",
		Signature:      "sig",
		CreatedAt:      time.Now(),
	}
	var hashErr error
	invoice.Hash, hashErr = invoice.GenerateHash()
	assert.NilError(t, hashErr)
	_, err := tokenStore.SaveUnconfirmedInvoice(ctx, &invoice)
	assert.NilError(t, err)
	err = tokenStore.UpdateUnconfirmedInvoiceTransactionHash(ctx, invoice.Hash, "invoicetxid")
	assert.NilError(t, err)

	resp := postRelayStatus(t, srv, "invoice:"+invoice.Hash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	status := decodeStatusResponse(t, resp)
	assert.Equal(t, status.ID, "invoice:"+invoice.Hash)
	assert.Equal(t, string(status.Status), "accepted")
	assert.Equal(t, status.TxID, "invoicetxid")
	assert.Assert(t, status.Required != nil)
	assert.Assert(t, status.Confirmed != nil)
}

func TestServeRelayStatus_InvoiceConfirmed(t *testing.T) {
	srv, tokenStore := setupDCWithRelayEndpoints(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "M",
		FractionCount: 1,
	}, "owner")
	assert.NilError(t, err)

	inv := store.Invoice{
		MintHash:        mintHash,
		Quantity:        1,
		Price:           100,
		BuyerAddress:    test_support.GenerateDogecoinAddress(true),
		SellerAddress:   test_support.GenerateDogecoinAddress(true),
		PaymentAddress:  test_support.GenerateDogecoinAddress(true),
		PublicKey:       "pk",
		Signature:       "sig",
		TransactionHash: "invtx",
		BlockHeight:     10,
		CreatedAt:       time.Now(),
	}
	inv.Hash, err = inv.GenerateHash()
	assert.NilError(t, err)
	_, err = tokenStore.SaveInvoice(ctx, &inv)
	assert.NilError(t, err)

	resp := postRelayStatus(t, srv, "invoice:"+inv.Hash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	status := decodeStatusResponse(t, resp)
	assert.Equal(t, status.ID, "invoice:"+inv.Hash)
	assert.Equal(t, string(status.Status), "confirmed")
	assert.Equal(t, status.TxID, "invtx")
}

// --- ServeRelayStatus: payment routing ---------------------------------

func TestServeRelayStatus_PaymentNotFound(t *testing.T) {
	srv, _ := setupDCWithRelayEndpoints(t)

	hash := test_support.GenerateRandomHash()
	resp := postRelayStatus(t, srv, "payment:"+hash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusNotFound)
}

// TestServeRelayStatus_PaymentUnpaid verifies that a confirmed invoice
// (on-chain) with no paid_at returns "accepted" status, meaning the DOGE
// payment has not yet been received.
func TestServeRelayStatus_PaymentUnpaid(t *testing.T) {
	srv, tokenStore := setupDCWithRelayEndpoints(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "M",
		FractionCount: 1,
	}, "owner")
	assert.NilError(t, err)

	inv := store.Invoice{
		MintHash:        mintHash,
		Quantity:        1,
		Price:           100,
		BuyerAddress:    test_support.GenerateDogecoinAddress(true),
		SellerAddress:   test_support.GenerateDogecoinAddress(true),
		PaymentAddress:  test_support.GenerateDogecoinAddress(true),
		PublicKey:       "pk",
		Signature:       "sig",
		TransactionHash: "invtx",
		BlockHeight:     5,
		CreatedAt:       time.Now(),
	}
	inv.Hash, err = inv.GenerateHash()
	assert.NilError(t, err)
	_, err = tokenStore.SaveInvoice(ctx, &inv)
	assert.NilError(t, err)

	resp := postRelayStatus(t, srv, "payment:"+inv.Hash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	status := decodeStatusResponse(t, resp)
	assert.Equal(t, status.ID, "payment:"+inv.Hash)
	assert.Equal(t, string(status.Status), "accepted")
}

func TestServeRelayStatus_PaymentConfirmed(t *testing.T) {
	srv, tokenStore := setupDCWithRelayEndpoints(t)
	ctx := context.Background()

	mintHash := test_support.GenerateRandomHash()
	_, err := tokenStore.SaveMint(ctx, &store.MintWithoutID{
		Hash:          mintHash,
		Title:         "M",
		FractionCount: 1,
	}, "owner")
	assert.NilError(t, err)

	inv := store.Invoice{
		MintHash:        mintHash,
		Quantity:        1,
		Price:           100,
		BuyerAddress:    test_support.GenerateDogecoinAddress(true),
		SellerAddress:   test_support.GenerateDogecoinAddress(true),
		PaymentAddress:  test_support.GenerateDogecoinAddress(true),
		PublicKey:       "pk",
		Signature:       "sig",
		TransactionHash: "paidtx",
		BlockHeight:     5,
		CreatedAt:       time.Now(),
	}
	inv.Hash, err = inv.GenerateHash()
	assert.NilError(t, err)
	id, err := tokenStore.SaveInvoice(ctx, &inv)
	assert.NilError(t, err)

	// Mark invoice as paid directly in the DB.
	_, err = tokenStore.DB.ExecContext(ctx, "UPDATE invoices SET paid_at = $1 WHERE id = $2", time.Now().UTC(), id)
	assert.NilError(t, err)

	resp := postRelayStatus(t, srv, "payment:"+inv.Hash)
	defer resp.Body.Close()
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	status := decodeStatusResponse(t, resp)
	assert.Equal(t, status.ID, "payment:"+inv.Hash)
	assert.Equal(t, string(status.Status), "confirmed")
	assert.Assert(t, status.ConfirmedAt != "")
}
