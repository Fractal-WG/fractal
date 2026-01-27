package stack

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	feclient "dogecoin.org/fractal-engine/pkg/client"
	fecfg "dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/dogenet"
	"dogecoin.org/fractal-engine/pkg/indexer"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"dogecoin.org/fractal-engine/pkg/store"

	"github.com/dogeorg/doge/koinu"
)

type StackConfig struct {
	InstanceId          int
	BasePort            int
	DogePort            int
	DogeHost            string
	DogeP2PPort         int
	FractalPort         int
	FractalHost         string
	DogeNetPort         int
	DogeNetHost         string
	DogeNetChain        string
	DogeNetBindPort     int
	DogeNetPubKey       string
	DogeNetWebPort      int
	IndexerURL          string
	PortgresPort        int
	PostgresHost        string
	DogeNetHandlerPort  int
	PrivKey             string
	PubKey              string
	AssetManagerPrivKey string
	AssetManagerPubKey  string
	AssetManagerAddress string
	Address             string
	TokenisationClient  *feclient.TokenisationClient
	IndexerClient       *indexer.IndexerClient
	DogeClient          *doge.RpcClient
	DogeNetClient       dogenet.GossipClient
	TokenisationStore   *store.TokenisationStore
}

var nodePubKeyRe = regexp.MustCompile(`DogeNet PubKey is:\s*([0-9a-fA-F]{64})`)
var spentUtxos []string

func NewStackConfig(instanceId int, chain string) StackConfig {
	prefixByte, err := doge.GetPrefix(chain)
	if err != nil {
		panic(err)
	}

	basePortFirst := 8600 + (1 * 100)
	basePort := 8600 + (instanceId * 100)
	privHex, pubHex, address, err := doge.GenerateDogecoinKeypair(prefixByte)
	if err != nil {
		panic(err)
	}

	assetManagerPrivHex, assetManagerPubHex, assetManagerAddress, err := doge.GenerateDogecoinKeypair(prefixByte)
	if err != nil {
		panic(err)
	}

	stackConfig := StackConfig{
		InstanceId:          instanceId,
		BasePort:            basePort,
		DogePort:            basePortFirst + 14556,
		DogeHost:            "0.0.0.0",
		DogeP2PPort:         basePortFirst + 10,
		DogeNetChain:        chain,
		FractalPort:         basePort + 20,
		FractalHost:         "0.0.0.0",
		DogeNetPort:         basePort + 30,
		DogeNetWebPort:      basePort + 40,
		DogeNetHost:         "0.0.0.0",
		IndexerURL:          "http://0.0.0.0:" + strconv.Itoa(basePortFirst+50),
		DogeNetHandlerPort:  basePort + 70,
		DogeNetBindPort:     basePort + 73,
		Address:             address,
		PrivKey:             privHex,
		PubKey:              pubHex,
		AssetManagerPrivKey: assetManagerPrivHex,
		AssetManagerPubKey:  assetManagerPubHex,
		AssetManagerAddress: assetManagerAddress,
		PortgresPort:        basePort + 60,
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error:", err)
	}

	dogenetLogFile := home + "/.fractal-stack-" + strconv.Itoa(instanceId) + "/logs/fractalengine.log"

	_, err = os.Stat(dogenetLogFile)
	if err != nil {
		fmt.Println("Error:", err)
	}

	logFile, err := os.ReadFile(dogenetLogFile)
	if err != nil {
		fmt.Println("Error:", err)
	}

	logFileStr := string(logFile)
	nodePubKey, ok := ExtractNodePubKey(logFileStr)
	if !ok {
		fmt.Println("Error: could not extract node public key")
	}
	stackConfig.DogeNetPubKey = nodePubKey

	stackConfig.TokenisationClient = feclient.NewTokenisationClient("http://"+stackConfig.FractalHost+":"+strconv.Itoa(stackConfig.FractalPort), stackConfig.PrivKey, stackConfig.PubKey)
	stackConfig.IndexerClient = indexer.NewIndexerClient(stackConfig.IndexerURL)
	stackConfig.DogeClient = doge.NewRpcClient(&fecfg.Config{
		DogeScheme:   "http",
		DogeHost:     "localhost",
		DogePort:     strconv.Itoa(stackConfig.DogePort),
		DogeUser:     "dogecoinrpc",
		DogePassword: "changeme1",
	})

	tokenStore, err := store.NewTokenisationStore("postgres://fractalstore:fractalstore@0.0.0.0:"+strconv.Itoa(stackConfig.PortgresPort)+"/fractalstore?sslmode=disable", fecfg.Config{})
	if err != nil {
		panic(err)
	}

	stackConfig.TokenisationStore = tokenStore

	stackConfig.DogeNetClient = dogenet.NewDogeNetClient(&fecfg.Config{
		DogeNetWebAddress: "localhost" + ":" + strconv.Itoa(stackConfig.DogeNetWebPort),
	}, tokenStore)

	TopUp(&stackConfig)
	ConfirmBlocks(&stackConfig)

	return stackConfig
}

func ExtractNodePubKey(logFileStr string) (string, bool) {
	matches := nodePubKeyRe.FindAllStringSubmatch(logFileStr, -1)
	if len(matches) == 0 {
		return "", false
	}
	return strings.ToLower(matches[len(matches)-1][1]), true
}

func GetUnspentUtxos(stackConfig *StackConfig, address string) ([]indexer.UTXOItem, error) {
	utxo, err := stackConfig.IndexerClient.GetUTXO(address)
	if err != nil {
		return nil, err
	}

	var unspent []indexer.UTXOItem
	for _, utxo := range utxo.UTXOs {
		inSpentUtxos := false
		for _, spentUtxo := range spentUtxos {
			if utxo.TxID == spentUtxo {
				inSpentUtxos = true
				break
			}
		}

		if !inSpentUtxos {
			unspent = append(unspent, utxo)
		}

	}

	return unspent, nil
}

func WriteToBlockchain(stackConfig *StackConfig, paymentAddress string, hexBody string, amount int64) string {
	ctx := context.Background()
	blockChainInfo, err := stackConfig.DogeClient.GetBlockchainInfo(ctx)
	if err != nil {
		panic(err)
	}

	chainByte, err := doge.GetPrefix(blockChainInfo.Chain)
	if err != nil {
		log.Fatal(err)
	}
	chainCfg := doge.GetChainCfg(chainByte)

	var selectedUtxo indexer.UTXOItem
	retries := 0
	maxRetries := 10

	for {
		utxos, err := GetUnspentUtxos(stackConfig, stackConfig.Address)
		if err != nil {
			panic(err)
		}

		if len(utxos) == 0 {
			panic(errors.New("No utxos found"))
		}

		for _, utxo := range utxos {
			_, err := stackConfig.DogeClient.Request(ctx, "gettxout", []interface{}{utxo.TxID, utxo.VOut, true})
			if err != nil {
				continue
			}

			selectedUtxo = utxo
		}

		if selectedUtxo.TxID != "" {
			break
		}

		time.Sleep(20 * time.Second)
		retries++
		if retries >= maxRetries {
			panic(errors.New("Max retries exceeded"))
		}
	}

	fmt.Println("Selected UTXO:", selectedUtxo.TxID)
	fmt.Println("", selectedUtxo.VOut)
	fmt.Println("", selectedUtxo.Value)

	inputs := []interface{}{
		map[string]interface{}{
			"txid": selectedUtxo.TxID,
			"vout": selectedUtxo.VOut,
		},
	}

	address := stackConfig.Address
	fee, _ := koinu.ParseKoinu("1")
	koinuAmount := koinu.Koinu(amount * koinu.OneDoge)

	var outputs map[string]interface{}
	if paymentAddress == "" && paymentAddress == address {
		outputs = map[string]interface{}{
			"data":  hexBody,
			address: selectedUtxo.Value - koinuAmount - fee,
		}
	} else {
		outputs = map[string]interface{}{
			"data":         hexBody,
			paymentAddress: koinuAmount,
			address:        selectedUtxo.Value - koinuAmount - fee,
		}
	}

	rawTx, err := stackConfig.DogeClient.Request(ctx, "createrawtransaction", []interface{}{inputs, outputs})
	if err != nil {
		panic(err)
	}

	var rawTxResponse string
	if err := json.Unmarshal(*rawTx, &rawTxResponse); err != nil {
		panic(err)
	}

	encodedTx, err := doge.SignRawTransaction(rawTxResponse, stackConfig.PrivKey, []doge.PrevOutput{
		{
			Address: address,
			Amount:  int64(selectedUtxo.Value),
		},
	}, chainCfg)

	if err != nil {
		panic(err)
	}

	res, err := stackConfig.DogeClient.Request(ctx, "sendrawtransaction", []interface{}{encodedTx})
	if err != nil {
		panic(err)
	}

	var txid string

	if err := json.Unmarshal(*res, &txid); err != nil {
		panic(err)
	}

	spentUtxos = append(spentUtxos, selectedUtxo.TxID)

	TopUp(stackConfig)
	ConfirmBlocks(stackConfig)

	return txid
}

func GetTokenBalance(stackConfig *StackConfig, mintHash string) int {
	log.Printf("GetTokenBalance %s %s \n", stackConfig.Address, mintHash)

	tokens, err := stackConfig.TokenisationClient.GetTokenBalance(stackConfig.Address, mintHash)
	if err != nil {
		panic(err)
	}

	balance := 0
	for _, token := range tokens {
		balance += token.Quantity
	}

	log.Printf("GetTokenBalance %s %s : %d\n", stackConfig.Address, mintHash, balance)

	return balance
}

func GetPendingTokenBalance(stackConfig *StackConfig, mintHash string) int {
	tokens, err := stackConfig.TokenisationClient.GetPendingTokenBalance(stackConfig.Address, mintHash)
	if err != nil {
		panic(err)
	}

	balance := 0
	for _, token := range tokens {
		balance += token.Quantity
	}

	return balance
}

func ConfirmBlocks(stackConfig *StackConfig) {
	ctx := context.Background()
	_, err := stackConfig.DogeClient.Request(ctx, "generate", []interface{}{10})
	if err != nil {
		panic(err)
	}
}

func TopUp(stackConfig *StackConfig) {
	ctx := context.Background()
	err := stackConfig.TokenisationClient.TopUpBalance(ctx, stackConfig.Address)
	if err != nil {
		panic(err)
	}

	fmt.Println("Topped up address " + stackConfig.Address)
}

func Payment(buyerConfig *StackConfig, sellerConfig *StackConfig, invoiceHash string, quantity int, price int) string {
	envelope := protocol.NewPaymentTransactionEnvelope(invoiceHash, protocol.ACTION_PAYMENT)
	encodedTransactionBody := envelope.Serialize()

	total := int64(quantity * price)

	txId := WriteToBlockchain(buyerConfig, sellerConfig.Address, hex.EncodeToString(encodedTransactionBody), total)
	ConfirmBlocks(sellerConfig)
	ConfirmBlocks(buyerConfig)

	return txId
}

func Invoice(stackConfig *StackConfig, buyerAddress string, mintHash string, quantity int, price int) string {
	invoicePayload := rpc.CreateInvoiceRequestPayload{
		PaymentAddress: stackConfig.Address,
		BuyerAddress:   buyerAddress,
		MintHash:       mintHash,
		Quantity:       quantity,
		Price:          price,
		SellerAddress:  stackConfig.Address,
	}

	invoiceRequest := rpc.CreateInvoiceRequest{
		Payload: invoicePayload,
	}

	signature, err := doge.SignPayload(invoicePayload, stackConfig.PrivKey, stackConfig.PubKey)
	if err != nil {
		panic(err)
	}

	invoiceRequest.Signature = signature
	invoiceRequest.PublicKey = stackConfig.PubKey

	res, err := stackConfig.TokenisationClient.CreateInvoice(&invoiceRequest)
	if err != nil {
		panic(err)
	}

	envelope := protocol.NewInvoiceTransactionEnvelope(res.Hash, mintHash, int32(quantity), protocol.ACTION_INVOICE)
	encodedTransactionBody := envelope.Serialize()

	// just network fees
	WriteToBlockchain(stackConfig, stackConfig.Address, hex.EncodeToString(encodedTransactionBody), int64(5))

	ConfirmBlocks(stackConfig)

	return res.Hash
}

func InvoiceSignature(stackConfig *StackConfig, invoice store.UnconfirmedInvoice) string {
	invoiceBody := store.InvoiceSignatureBody{
		Hash:           invoice.Hash,
		MintHash:       invoice.MintHash,
		Price:          invoice.Price,
		Quantity:       invoice.Quantity,
		BuyerAddress:   invoice.BuyerAddress,
		PaymentAddress: invoice.PaymentAddress,
		SellerAddress:  invoice.SellerAddress,
	}

	signature, err := doge.SignPayload(invoiceBody, stackConfig.AssetManagerPrivKey, stackConfig.AssetManagerPubKey)
	if err != nil {
		panic(err)
	}

	createInvoiceSignatureRequest := rpc.CreateInvoiceSignatureRequest{
		Payload: rpc.CreateInvoiceSignatureRequestPayload{
			InvoiceHash: invoice.Hash,
			Signature:   signature,
			PublicKey:   stackConfig.AssetManagerPubKey,
		},
	}

	createInvoiceSignatureResponse, err := stackConfig.TokenisationClient.CreateInvoiceSignature(&createInvoiceSignatureRequest)
	if err != nil {
		panic(err)
	}

	return createInvoiceSignatureResponse.Id
}

func Mint(stackConfig *StackConfig) string {
	mintPayload := rpc.CreateMintRequestPayload{
		Title:                    "Super Lambo",
		FractionCount:            100,
		Description:              "Fast Car",
		ContractOfSale:           "contract of sale",
		Tags:                     []string{"car"},
		FeedURL:                  "https://example.com/feed",
		OwnerAddress:             stackConfig.Address,
		SignatureRequirementType: store.SignatureRequirementType_ALL_SIGNATURES,
		AssetManagers: []store.AssetManager{
			{
				Name:      "asset manager",
				PublicKey: stackConfig.AssetManagerPubKey,
				URL:       "https://example.com/assetManager",
			},
		},
		MinSignatures: 1,
	}

	mintRequest := rpc.CreateMintRequest{
		Payload: mintPayload,
		SignedRequest: rpc.SignedRequest{
			PublicKey: stackConfig.PubKey,
		},
	}

	signature, err := doge.SignPayload(mintPayload, stackConfig.PrivKey, stackConfig.PubKey)
	if err != nil {
		panic(err)
	}

	mintRequest.Signature = signature

	res, err := stackConfig.TokenisationClient.Mint(&mintRequest)
	if err != nil {
		panic(err)
	}

	envelope := protocol.NewMintTransactionEnvelope(res.Hash, protocol.ACTION_MINT)
	encodedTransactionBody := envelope.Serialize()

	// Only need 1 to cover network fees
	WriteToBlockchain(stackConfig, stackConfig.Address, hex.EncodeToString(encodedTransactionBody), 5)

	ConfirmBlocks(stackConfig)

	return res.Hash
}

func makeStackConfigsAndPeer(stackCount int) []*StackConfig {
	var stacks []*StackConfig
	for i := 0; i < stackCount; i++ {
		newConfig := NewStackConfig(i+1, "regtest")
		stacks = append(stacks, &newConfig)
	}

	for i := 0; i < len(stacks)/2; i += 2 {
		stackA := stacks[i]
		stackB := stacks[(i + 1)]

		// Check for nodes, if doesnt exist, then add peer.
		err := stackA.DogeNetClient.AddPeer(dogenet.AddPeer{
			Key:  stackB.DogeNetPubKey,
			Addr: stackB.DogeNetHost + ":" + strconv.Itoa(stackB.DogeNetBindPort),
		})
		if err != nil {
			panic(err)
		}

		for {
			nodesA, err := stackA.DogeNetClient.GetNodes()
			if err != nil {
				panic(err)
			}

			fmt.Println("Nodes A:", nodesA)

			nodesB, err := stackB.DogeNetClient.GetNodes()
			if err != nil {
				panic(err)
			}

			fmt.Println("Nodes B:", nodesB)

			if len(nodesA) >= 1 && len(nodesB) >= 1 {
				break
			}

			time.Sleep(5 * time.Second)
		}

		time.Sleep(1 * time.Minute)

	}

	return stacks
}
