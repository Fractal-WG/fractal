package rpc_test

import (
	"context"
	"testing"
	"time"

	connect "connectrpc.com/connect"
	"dogecoin.org/fractal-engine/internal/test/support"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"gotest.tools/assert"
)

func TestInvoices(t *testing.T) {
	tokenisationStore, dogenetClient, feClient := SetupRpcTest(t)

	paymentAddress := support.GenerateDogecoinAddress(true)
	buyerAddress := support.GenerateDogecoinAddress(true)
	sellOfferAddress := support.GenerateDogecoinAddress(true)
	mintHash := support.GenerateRandomHash()

	_, err := tokenisationStore.SaveMint(&store.MintWithoutID{
		Title:         "mint1",
		Description:   "description1",
		FractionCount: 100,
		Hash:          mintHash,
	}, "owner")
	assert.NilError(t, err)

	invoicePayload := rpc.CreateInvoiceRequestPayload{
		PaymentAddress: paymentAddress,
		BuyerAddress:   buyerAddress,
		MintHash:       mintHash,
		Quantity:       10,
		Price:          100,
		SellerAddress:  sellOfferAddress,
	}

	privHex, pubHex, _, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	signature, err := doge.SignPayload(invoicePayload, privHex, pubHex)
	assert.NilError(t, err)

	protoPayload := &protocol.CreateInvoiceRequestPayload{}
	protoPayload.SetPaymentAddress(paymentAddress)
	protoPayload.SetBuyerAddress(buyerAddress)
	protoPayload.SetMintHash(mintHash)
	protoPayload.SetQuantity(10)
	protoPayload.SetPrice(100)
	protoPayload.SetSellerAddress(sellOfferAddress)

	invoice := &protocol.CreateInvoiceRequest{}
	invoice.SetPayload(protoPayload)
	invoice.SetPublicKey(pubHex)
	invoice.SetSignature(signature)

	invoiceResponse, err := feClient.CreateInvoice(context.Background(), connect.NewRequest(invoice))
	if err != nil {
		t.Fatalf("Failed to create invoice: %v", err)
	}

	invoices, err := tokenisationStore.GetUnconfirmedInvoices(0, 10, mintHash, buyerAddress)
	if err != nil {
		t.Fatalf("Failed to get invoices: %v", err)
	}

	assert.Equal(t, len(invoices), 1)
	assert.Equal(t, invoices[0].Hash, invoiceResponse.Msg.GetHash())
	assert.Equal(t, invoices[0].PaymentAddress, invoicePayload.PaymentAddress)
	assert.Equal(t, invoices[0].BuyerAddress, invoicePayload.BuyerAddress)
	assert.Equal(t, invoices[0].MintHash, invoicePayload.MintHash)
	assert.Equal(t, invoices[0].Quantity, invoicePayload.Quantity)
	assert.Equal(t, invoices[0].Price, invoicePayload.Price)
	assert.Equal(t, invoices[0].SellerAddress, invoicePayload.SellerAddress)
	assert.Equal(t, invoices[0].Status, "draft")

	assert.Equal(t, len(dogenetClient.invoices), 1)
	assert.Equal(t, dogenetClient.invoices[0].Hash, invoiceResponse.Msg.GetHash())
	assert.Equal(t, dogenetClient.invoices[0].PaymentAddress, invoicePayload.PaymentAddress)
	assert.Equal(t, dogenetClient.invoices[0].BuyerAddress, invoicePayload.BuyerAddress)
	assert.Equal(t, dogenetClient.invoices[0].MintHash, invoicePayload.MintHash)
	assert.Equal(t, dogenetClient.invoices[0].Quantity, invoicePayload.Quantity)
	assert.Equal(t, dogenetClient.invoices[0].Price, invoicePayload.Price)
	assert.Equal(t, dogenetClient.invoices[0].SellerAddress, invoicePayload.SellerAddress)
}

func TestInvoicesWithSignatureRequired(t *testing.T) {
	tokenisationStore, dogenetClient, feClient := SetupRpcTest(t)

	paymentAddress := support.GenerateDogecoinAddress(true)
	sellOfferAddress := support.GenerateDogecoinAddress(true)
	buyerAddress := support.GenerateDogecoinAddress(true)

	// Save a confirmed mint
	confirmedMint := &store.MintWithoutID{
		Title:                    "Confirmed Mint",
		FractionCount:            100,
		Description:              "Confirmed mint",
		Tags:                     store.StringArray{},
		Metadata:                 store.StringInterfaceMap{},
		Requirements:             store.StringInterfaceMap{},
		LockupOptions:            store.StringInterfaceMap{},
		PublicKey:                "testPubKey",
		TransactionHash:          "txHash",
		SignatureRequirementType: store.SignatureRequirementType_ALL_SIGNATURES,
		MinSignatures:            1,
		AssetManagers: store.AssetManagers{
			{
				Name:      "asset manager",
				PublicKey: "publicKey123",
				URL:       "https://example.com/assetManager",
			},
		},
	}
	var err error
	confirmedMint.Hash, err = confirmedMint.GenerateHash()
	assert.NilError(t, err)

	_, err = tokenisationStore.SaveMint(confirmedMint, "owner")
	assert.NilError(t, err)

	invoice := rpc.CreateInvoiceRequest{
		Payload: rpc.CreateInvoiceRequestPayload{
			PaymentAddress: paymentAddress,
			BuyerAddress:   buyerAddress,
			MintHash:       confirmedMint.Hash,
			Quantity:       10,
			Price:          100,
			SellerAddress:  sellOfferAddress,
		},
	}

	privHex, pubHex, _, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	signature, err := doge.SignPayload(invoice.Payload, privHex, pubHex)
	assert.NilError(t, err)

	protoPayload := &protocol.CreateInvoiceRequestPayload{}
	protoPayload.SetPaymentAddress(paymentAddress)
	protoPayload.SetBuyerAddress(buyerAddress)
	protoPayload.SetMintHash(confirmedMint.Hash)
	protoPayload.SetQuantity(10)
	protoPayload.SetPrice(100)
	protoPayload.SetSellerAddress(sellOfferAddress)

	protoInvoice := &protocol.CreateInvoiceRequest{}
	protoInvoice.SetPayload(protoPayload)
	protoInvoice.SetPublicKey(pubHex)
	protoInvoice.SetSignature(signature)

	invoiceResponse, err := feClient.CreateInvoice(context.Background(), connect.NewRequest(protoInvoice))
	if err != nil {
		t.Fatalf("Failed to create invoice: %v", err)
	}

	invoices, err := tokenisationStore.GetUnconfirmedInvoices(0, 10, confirmedMint.Hash, buyerAddress)
	if err != nil {
		t.Fatalf("Failed to get invoices: %v", err)
	}

	assert.Equal(t, len(invoices), 1)
	assert.Equal(t, invoices[0].Hash, invoiceResponse.Msg.GetHash())
	assert.Equal(t, invoices[0].PaymentAddress, invoice.Payload.PaymentAddress)
	assert.Equal(t, invoices[0].BuyerAddress, invoice.Payload.BuyerAddress)
	assert.Equal(t, invoices[0].MintHash, invoice.Payload.MintHash)
	assert.Equal(t, invoices[0].Quantity, invoice.Payload.Quantity)
	assert.Equal(t, invoices[0].Price, invoice.Payload.Price)
	assert.Equal(t, invoices[0].SellerAddress, invoice.Payload.SellerAddress)
	assert.Equal(t, invoices[0].Status, "pending_signatures")

	assert.Equal(t, len(dogenetClient.invoices), 1)
	assert.Equal(t, dogenetClient.invoices[0].Hash, invoiceResponse.Msg.GetHash())
	assert.Equal(t, dogenetClient.invoices[0].PaymentAddress, invoice.Payload.PaymentAddress)
	assert.Equal(t, dogenetClient.invoices[0].BuyerAddress, invoice.Payload.BuyerAddress)
	assert.Equal(t, dogenetClient.invoices[0].MintHash, invoice.Payload.MintHash)
	assert.Equal(t, dogenetClient.invoices[0].Quantity, invoice.Payload.Quantity)
	assert.Equal(t, dogenetClient.invoices[0].Price, invoice.Payload.Price)
	assert.Equal(t, dogenetClient.invoices[0].SellerAddress, invoice.Payload.SellerAddress)
}

func TestCreateInvoiceSignature(t *testing.T) {
	tokenisationStore, _, feClient := SetupRpcTest(t)

	assetManagerPrivKey, assetManagerPubKey, _, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	// Save a confirmed mint
	confirmedMint := &store.MintWithoutID{
		Title:                    "Confirmed Mint",
		FractionCount:            100,
		Description:              "Confirmed mint",
		Tags:                     store.StringArray{},
		Metadata:                 store.StringInterfaceMap{},
		Requirements:             store.StringInterfaceMap{},
		LockupOptions:            store.StringInterfaceMap{},
		PublicKey:                "testPubKey",
		TransactionHash:          "txHash",
		SignatureRequirementType: store.SignatureRequirementType_ALL_SIGNATURES,
		MinSignatures:            1,
		AssetManagers: store.AssetManagers{
			{
				Name:      "asset manager",
				PublicKey: assetManagerPubKey,
				URL:       "https://example.com/assetManager",
			},
		},
	}

	confirmedMint.Hash, err = confirmedMint.GenerateHash()
	assert.NilError(t, err)

	_, err = tokenisationStore.SaveMint(confirmedMint, "owner")
	assert.NilError(t, err)

	paymentAddress := support.GenerateDogecoinAddress(true)
	offererAddress := support.GenerateDogecoinAddress(true)
	sellOfferAddress := support.GenerateDogecoinAddress(true)

	invoice := store.UnconfirmedInvoice{
		Id:             "myId",
		PaymentAddress: paymentAddress,
		BuyerAddress:   offererAddress,
		MintHash:       confirmedMint.Hash,
		Quantity:       10,
		Price:          25,
		CreatedAt:      time.Now(),
		PublicKey:      "myPublicKey",
		SellerAddress:  sellOfferAddress,
		Signature:      "mySignature",
	}

	invoice.Hash, err = invoice.GenerateHash()
	assert.NilError(t, err)

	_, err = tokenisationStore.SaveUnconfirmedInvoice(&invoice)
	if err != nil {
		t.Fatalf("Failed to save invoice: %v", err)
	}

	invoiceBody := store.InvoiceSignatureBody{
		Hash:           invoice.Hash,
		MintHash:       invoice.MintHash,
		Price:          invoice.Price,
		Quantity:       invoice.Quantity,
		BuyerAddress:   invoice.BuyerAddress,
		PaymentAddress: invoice.PaymentAddress,
		SellerAddress:  invoice.SellerAddress,
	}

	signature, err := doge.SignPayload(invoiceBody, assetManagerPrivKey, assetManagerPubKey)
	assert.NilError(t, err)

	protoPayload := &protocol.CreateInvoiceSignatureRequestPayload{}
	protoPayload.SetInvoiceHash(invoice.Hash)
	protoPayload.SetSignature(signature)
	protoPayload.SetPublicKey(assetManagerPubKey)

	protoRequest := &protocol.CreateInvoiceSignatureRequest{}
	protoRequest.SetPayload(protoPayload)

	createInvoiceSignatureResponse, err := feClient.CreateInvoiceSignature(context.Background(), connect.NewRequest(protoRequest))
	if err != nil {
		t.Fatalf("Failed to create invoice signature: %v", err)
	}

	var savedInvoiceHash string
	tokenisationStore.DB.QueryRow("SELECT invoice_hash FROM invoice_signatures WHERE id = $1", createInvoiceSignatureResponse.Msg.GetId()).Scan(&savedInvoiceHash)
	if err != nil {
		t.Fatalf("Failed to get invoice signature: %v", err)
	}

	assert.Equal(t, savedInvoiceHash, invoice.Hash)
}
