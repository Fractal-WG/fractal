package support

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/dogenet"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"dogecoin.org/fractal-engine/pkg/store"
	"github.com/dogecoinfoundation/dogetest/pkg/dogetest"
)

func SetupTestDB() *store.TokenisationStore {
	ctx := context.Background()
	randoDb := rand.Intn(10000)

	url := "file:memdb" + fmt.Sprintf("%d", randoDb) + "?mode=memory&cache=shared"

	tokenStore, err := store.NewTokenisationStore(url, config.Config{})
	if err != nil {
		log.Fatal(err)
	}

	err = tokenStore.Migrate(ctx)
	if err != nil && err.Error() != "no change" {
		log.Fatal(err)
	}

	return tokenStore
}

func WriteMintToCore(dogeTest *dogetest.DogeTest, addressBook *dogetest.AddressBook, mintResponse *rpc.CreateMintResponse) error {
	unspent, err := dogeTest.Rpc.ListUnspent(addressBook.Addresses[0].Address)
	if err != nil {
		log.Fatal(err)
	}

	if len(unspent) == 0 {
		return fmt.Errorf("no unspent outputs found")
	}

	selectedUTXO := unspent[0]

	inputs := []map[string]interface{}{
		{
			"txid": selectedUTXO.TxID,
			"vout": selectedUTXO.Vout,
		},
	}

	change := selectedUTXO.Amount - 0.5

	envelope := protocol.NewMintTransactionEnvelope(mintResponse.Hash, protocol.ACTION_MINT)
	encodedTransactionBody := envelope.Serialize()

	outputs := map[string]interface{}{
		addressBook.Addresses[0].Address: change,
		"data":                           hex.EncodeToString(encodedTransactionBody),
	}

	createResp, err := dogeTest.Rpc.Request("createrawtransaction", []interface{}{inputs, outputs})

	if err != nil {
		log.Fatalf("Error creating raw transaction: %v", err)
	}

	var rawTx string

	if err := json.Unmarshal(*createResp, &rawTx); err != nil {
		log.Fatalf("Error parsing raw transaction: %v", err)
	}

	// Step 3: Add OP_RETURN output to the transaction
	rawTxBytes, err := hex.DecodeString(rawTx)
	if err != nil {
		log.Fatalf("Error decoding raw transaction hex: %v", err)
	}

	prevTxs := []map[string]interface{}{
		{

			"txid":         selectedUTXO.TxID,
			"vout":         selectedUTXO.Vout,
			"scriptPubKey": selectedUTXO.ScriptPubKey,
			"amount":       selectedUTXO.Amount,
		},
	}

	// Prepare privkeys (private keys for signing)
	privkeys := []string{addressBook.Addresses[0].PrivateKey}

	signResp, err := dogeTest.Rpc.Request("signrawtransaction", []interface{}{hex.EncodeToString(rawTxBytes), prevTxs, privkeys})
	if err != nil {
		log.Fatalf("Error signing raw transaction: %v", err)
	}

	var signResult map[string]interface{}
	if err := json.Unmarshal(*signResp, &signResult); err != nil {
		log.Fatalf("Error parsing signed transaction: %v", err)
	}

	signedTx, ok := signResult["hex"].(string)
	if !ok {
		log.Fatal("Error retrieving signed transaction hex.")
	}

	// Step 5: Broadcast the signed transaction
	sendResp, err := dogeTest.Rpc.Request("sendrawtransaction", []interface{}{signedTx})
	if err != nil {
		log.Fatalf("Error broadcasting transaction: %v", err)
	}

	var txID string
	if err := json.Unmarshal(*sendResp, &txID); err != nil {
		log.Fatalf("Error parsing transaction ID: %v", err)
	}

	fmt.Printf("Transaction sent successfully! TXID: %s\n", txID)

	time.Sleep(2 * time.Second)

	blockies, err := dogeTest.ConfirmBlocks()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Blockies:", blockies)

	return nil
}

// GenerateDogecoinAddress generates a string that matches the Dogecoin address format
// This creates a valid-looking address for testing purposes (not a real spendable address)
// Dogecoin addresses start with 'D' for mainnet or 'n' for testnet
func GenerateDogecoinAddress(testnet bool) string {
	// Base58 alphabet used in Bitcoin/Dogecoin addresses
	base58Alphabet := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	// Address length is typically 34 characters
	addressLength := 34

	// Start with appropriate prefix
	var address strings.Builder
	if testnet {
		address.WriteString("n")
	} else {
		address.WriteString("D")
	}

	// Generate random characters from base58 alphabet
	for i := 1; i < addressLength; i++ {
		randomBytes := make([]byte, 1)
		_, err := rand.Read(randomBytes)
		if err != nil {
			// Fallback to a deterministic character if random fails
			address.WriteByte(base58Alphabet[i%len(base58Alphabet)])
			continue
		}
		// Use the random byte to pick a character from the alphabet
		index := int(randomBytes[0]) % len(base58Alphabet)
		address.WriteByte(base58Alphabet[index])
	}

	return address.String()
}

// GenerateRandomHash generates a random 64-character hexadecimal hash
// This is useful for testing purposes where a hash-like string is needed
func GenerateRandomHash() string {
	// 32 bytes will give us 64 hex characters
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to a deterministic hash if random fails
		return "0000000000000000000000000000000000000000000000000000000000000000"
	}
	return hex.EncodeToString(randomBytes)
}

func WaitForDogeNetClient(client *dogenet.DogeNetClient) {
	counter := 0
	for {
		if client.Running {
			break
		}

		if counter > 100 {
			log.Fatal("DogeNet did not start.")
		}

		counter++

		time.Sleep(100 * time.Millisecond)
	}
}
