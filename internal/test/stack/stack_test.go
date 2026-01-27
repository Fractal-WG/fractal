package stack

import (
	"fmt"
	"log"
	"testing"
	"time"
)

func TestSimpleFlow(t *testing.T) {
	stacks := makeStackConfigsAndPeer(2)

	seller := stacks[0]
	buyer := stacks[1]
	mintQty := 100
	sellQty := 20

	// Checking for Seller Balance
	Retry(t, func() bool {
		fmt.Printf("Checking balance for %s\n", seller.Address)
		balance, err := seller.IndexerClient.GetBalance(seller.Address)
		if err != nil {
			t.Fatal(err)
		}

		return balance.Available >= 1
	}, 20, 3*time.Second)
	fmt.Println("Doge Balance confirmed")

	// Mint token
	mintHash := Mint(seller)
	AssertEqualWithRetry(t, func() interface{} {
		return GetTokenBalance(seller, mintHash)
	}, mintQty, 10, 3*time.Second)
	fmt.Println("Mint confirmed")

	// Ensure new UTXO is available
	Retry(t, func() bool {
		fmt.Printf("Checking balance for %s\n", seller.Address)
		utxos, err := GetUnspentUtxos(seller, seller.Address)
		if err != nil {
			t.Fatal(err)
		}
		return len(utxos) > 0
	}, 30, 5*time.Second)

	// Create invoice
	invoiceHash := Invoice(seller, buyer.Address, mintHash, sellQty, 20)
	AssertEqualWithRetry(t, func() interface{} {
		return GetPendingTokenBalance(seller, mintHash)
	}, sellQty, 10, 3*time.Second)
	fmt.Println("Invoice confirmed")

	inv, err := seller.TokenisationStore.GetUnconfirmedInvoiceByHash(t.Context(), invoiceHash)
	if err != nil {
		t.Fatal(err)
	}

	invoiceSignatureId := InvoiceSignature(seller, inv)
	fmt.Println("Invoice signature:", invoiceSignatureId)

	// Ensure new UTXO is available
	Retry(t, func() bool {
		fmt.Printf("Checking balance for %s\n", buyer.Address)
		utxos, err := GetUnspentUtxos(buyer, buyer.Address)
		if err != nil {
			t.Fatal(err)
		}
		return len(utxos) > 0
	}, 30, 10*time.Second)

	// Pay for invoice
	paymentTrxn := Payment(buyer, seller, invoiceHash, sellQty, 20)

	// Ensure buyer token balance is updated
	AssertEqualWithRetry(t, func() interface{} {
		return GetTokenBalance(buyer, mintHash)
	}, sellQty, 30, 3*time.Second)

	// Ensure seller token balance is updated
	AssertEqualWithRetry(t, func() interface{} {
		return GetTokenBalance(seller, mintHash)
	}, mintQty-sellQty, 30, 3*time.Second)
	fmt.Println("Payment confirmed")

	log.Println("Mint: ", mintHash)
	log.Println("Invoice: ", invoiceHash)
	log.Println("Payment Trxn: ", paymentTrxn)
}
