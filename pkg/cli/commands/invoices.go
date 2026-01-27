package commands

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	fecli "dogecoin.org/fractal-engine/pkg/cli"
	"dogecoin.org/fractal-engine/pkg/cli/keys"
	fecfg "dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/indexer"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v3"
)

var InvoiceCommand = &cli.Command{
	Name:  "invoices",
	Usage: "Manage invoices",
	Commands: []*cli.Command{
		{
			Name:   "create",
			Usage:  "Create an invoice",
			Action: createInvoiceAction,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "config-path",
					Usage: "Path to the config file",
					Value: "config.toml",
				},
			},
		},
		{
			Name:   "list",
			Usage:  "List invoices",
			Action: listInvoicesAction,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "config-path",
					Usage: "Path to the config file",
					Value: "config.toml",
				},
			},
		},
		{
			Name:   "pay",
			Usage:  "Pay an invoice",
			Action: payInvoiceAction,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "config-path",
					Usage: "Path to the config file",
					Value: "config.toml",
				},
			},
		},
	},
}

func listInvoicesAction(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config-path")

	config, err := fecli.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	store := keys.NewSecureStore()
	address, err := store.Get(config.ActiveKey + "_address")
	if err != nil {
		log.Fatal(err)
	}

	tokenisationClient, err := getTokenisationClient(ctx, cmd)
	if err != nil {
		log.Fatal(err)
	}

	invoices, err := tokenisationClient.GetMyInvoices(0, 10, address)
	if err != nil {
		log.Fatal(err)
	}

	prettyJSON, err := json.MarshalIndent(invoices, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return err
	}
	fmt.Println(string(prettyJSON))

	return nil
}

func createInvoiceAction(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config-path")

	config, err := fecli.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	store := keys.NewSecureStore()
	address, err := store.Get(config.ActiveKey + "_address")
	if err != nil {
		log.Fatal(err)
	}

	tokenisationClient, err := getTokenisationClient(ctx, cmd)
	if err != nil {
		log.Fatal(err)
	}

	var mintHash string
	var buyerAddress string
	var quantity string
	var pricePer string

	group := huh.NewGroup(
		huh.NewInput().
			Title("What is the token hash?").
			Value(&mintHash),
		huh.NewInput().
			Title("What is the buyer address?").
			Value(&buyerAddress),
		huh.NewInput().
			Title("What is the quantity?").
			Value(&quantity),
		huh.NewInput().
			Title("What is the price per?").
			Value(&pricePer),
	)

	form := huh.NewForm(group)
	err = form.Run()
	if err != nil {
		log.Fatal(err)
	}

	pubHex, err := store.Get(config.ActiveKey + "_public_key")
	if err != nil {
		log.Fatal(err)
	}

	privHex, err := store.Get(config.ActiveKey + "_private_key")
	if err != nil {
		log.Fatal(err)
	}

	chain, err := store.Get(config.ActiveKey + "_chain")
	if err != nil {
		log.Fatal(err)
	}

	chainByte, err := doge.GetPrefix(chain)
	if err != nil {
		log.Fatal(err)
	}
	chainCfg := doge.GetChainCfg(chainByte)

	quantityInt, err := strconv.Atoi(quantity)
	if err != nil {
		log.Fatal(err)
	}

	pricePerInt, err := strconv.Atoi(pricePer)
	if err != nil {
		log.Fatal(err)
	}

	invoiceRequest := rpc.CreateInvoiceRequest{
		Payload: rpc.CreateInvoiceRequestPayload{
			PaymentAddress: address,
			BuyerAddress:   buyerAddress,
			MintHash:       mintHash,
			Quantity:       quantityInt,
			Price:          pricePerInt,
			SellerAddress:  address,
		},
	}

	invoiceRequest.PublicKey = pubHex

	payloadBytes, err := json.Marshal(invoiceRequest.Payload)
	if err != nil {
		log.Fatal(err)
	}

	signature, err := doge.SignPayload(payloadBytes, privHex, pubHex)
	if err != nil {
		log.Fatal(err)
	}

	invoiceRequest.Signature = signature

	response, err := tokenisationClient.CreateInvoice(&invoiceRequest)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Created invoice: " + response.Hash)

	indexerClient := indexer.NewIndexerClient(config.IndexerURL)

	utxos, err := indexerClient.GetUTXO(address)
	if err != nil {
		log.Fatal(err)
	}

	if len(utxos.UTXOs) == 0 {
		log.Fatal("No utxos found for address", address)
	}

	envelope := protocol.NewInvoiceTransactionEnvelope(response.Hash, mintHash, int32(quantityInt), protocol.ACTION_INVOICE)
	encodedTransactionBody := envelope.Serialize()

	inputs := []interface{}{
		map[string]interface{}{
			"txid": utxos.UTXOs[0].TxID,
			"vout": utxos.UTXOs[0].VOut,
		},
	}

	amount := utxos.UTXOs[0].Value

	outputs := map[string]interface{}{
		"data":  hex.EncodeToString(encodedTransactionBody),
		address: amount - 1,
	}

	dogeClient := doge.NewRpcClient(&fecfg.Config{
		DogeScheme:   config.DogeScheme,
		DogeHost:     config.DogeHost,
		DogePort:     config.DogePort,
		DogeUser:     config.DogeUser,
		DogePassword: config.DogePassword,
	})

	rawTx, err := dogeClient.Request(ctx, "createrawtransaction", []interface{}{inputs, outputs})
	if err != nil {
		log.Fatal(err)
	}

	var rawTxResponse string
	if err := json.Unmarshal(*rawTx, &rawTxResponse); err != nil {
		log.Fatal(err)
	}

	encodedTx, err := doge.SignRawTransaction(rawTxResponse, privHex, []doge.PrevOutput{
		{
			Address: address,
			Amount:  int64(amount),
		},
	}, chainCfg)

	if err != nil {
		log.Fatal(err)
	}

	res, err := dogeClient.Request(ctx, "sendrawtransaction", []interface{}{encodedTx})
	if err != nil {
		log.Println("error sending raw transaction", err)
		return err
	}

	var txid string

	if err := json.Unmarshal(*res, &txid); err != nil {
		log.Println("error parsing send raw transaction response", err)
		return err
	}

	fmt.Println("Transaction sent: " + txid)

	return nil
}
