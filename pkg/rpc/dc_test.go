package rpc_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
