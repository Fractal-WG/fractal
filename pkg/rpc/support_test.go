package rpc_test

import (
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	test_support "dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/dogenet"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol/protocolconnect"
	"dogecoin.org/fractal-engine/pkg/store"
)

type FakeGossipClient struct {
	dogenet.GossipClient
	buyOffers         []store.BuyOffer
	sellOffers        []store.SellOffer
	mints             []store.Mint
	invoices          []store.UnconfirmedInvoice
	invoiceSignatures []store.InvoiceSignature
}

func (g *FakeGossipClient) GossipBuyOffer(offer store.BuyOffer) error {
	g.buyOffers = append(g.buyOffers, offer)
	return nil
}

func (g *FakeGossipClient) GossipSellOffer(offer store.SellOffer) error {
	g.sellOffers = append(g.sellOffers, offer)
	return nil
}

func (g *FakeGossipClient) GossipMint(mint store.Mint) error {
	g.mints = append(g.mints, mint)
	return nil
}

func (g *FakeGossipClient) GossipUnconfirmedInvoice(invoice store.UnconfirmedInvoice) error {
	g.invoices = append(g.invoices, invoice)
	return nil
}

func (g *FakeGossipClient) GossipDeleteBuyOffer(hash string, publicKey string, signature string) error {
	for i, offer := range g.buyOffers {
		if offer.Hash == hash {
			log.Printf("Deleted buy offer: %v", offer)
			g.buyOffers = append(g.buyOffers[:i], g.buyOffers[i+1:]...)
			return nil
		}
	}

	return nil
}

func (g *FakeGossipClient) GossipDeleteSellOffer(hash string, publicKey string, signature string) error {
	for i, offer := range g.sellOffers {
		if offer.Hash == hash {
			log.Printf("Deleted sell offer: %v", offer)
			g.sellOffers = append(g.sellOffers[:i], g.sellOffers[i+1:]...)
			return nil
		}
	}

	return nil
}

func (g *FakeGossipClient) GossipInvoiceSignature(invoiceSignature store.InvoiceSignature) error {
	g.invoiceSignatures = append(g.invoiceSignatures, invoiceSignature)
	return nil
}

func SetupRpcTest(t *testing.T) (*store.TokenisationStore, *FakeGossipClient, protocolconnect.FractalEngineRpcServiceClient) {
	t.Helper()

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	dogenetClient := &FakeGossipClient{
		buyOffers:         []store.BuyOffer{},
		sellOffers:        []store.SellOffer{},
		mints:             []store.Mint{},
		invoices:          []store.UnconfirmedInvoice{},
		invoiceSignatures: []store.InvoiceSignature{},
	}

	cfg := config.NewConfig()
	tokenisationStore := test_support.SetupTestDB(t)
	connectService := rpc.NewConnectRpcService(tokenisationStore, dogenetClient, cfg, doge.NewRpcClient(cfg))
	connectPath, connectHandler := protocolconnect.NewFractalEngineRpcServiceHandler(connectService)
	mux.Handle(connectPath, connectHandler)

	feClient := protocolconnect.NewFractalEngineRpcServiceClient(server.Client(), server.URL)

	return tokenisationStore, dogenetClient, feClient
}
