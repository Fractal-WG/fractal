package commands

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	fecli "dogecoin.org/fractal-engine/pkg/cli"
	climodels "dogecoin.org/fractal-engine/pkg/cli/climodels"
	"dogecoin.org/fractal-engine/pkg/cli/keys"
	fecfg "dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/indexer"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/dogeorg/doge/koinu"
	"github.com/urfave/cli/v3"
)

var PaymentsCommand = &cli.Command{
	Name:  "payments",
	Usage: "Manage payments",
	Commands: []*cli.Command{
		{
			Name:   "pay-invoice",
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

func payInvoiceAction(ctx context.Context, cmd *cli.Command) error {
	tokenisationClient, err := getTokenisationClient(ctx, cmd)
	if err != nil {
		log.Fatal(err)
	}

	configPath := cmd.String("config-path")

	config, err := fecli.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	secureStore := keys.NewSecureStore()

	privHex, err := secureStore.Get(config.ActiveKey + "_private_key")
	if err != nil {
		log.Fatal(err)
	}

	address, err := secureStore.Get(config.ActiveKey + "_address")
	if err != nil {
		log.Fatal(err)
	}

	chain, err := secureStore.Get(config.ActiveKey + "_chain")
	if err != nil {
		log.Fatal(err)
	}

	chainByte, err := doge.GetPrefix(chain)
	if err != nil {
		log.Fatal(err)
	}
	chainCfg := doge.GetChainCfg(chainByte)

	var mintHash string

	group := huh.NewGroup(
		huh.NewInput().
			Title("What is the token hash?").
			Value(&mintHash),
	)

	form := huh.NewForm(group)
	err = form.Run()
	if err != nil {
		log.Fatal(err)
	}

	invoices, err := tokenisationClient.GetInvoices(0, 10, mintHash, address)
	if err != nil {
		log.Fatal(err)
	}

	items := []list.Item{}
	for _, invoice := range invoices.Invoices {
		items = append(items, climodels.SelectSimpleListItem{
			OfferId: invoice.Id,
			Name:    "Invoice: " + invoice.Hash + " (Seller: " + invoice.SellerAddress + ")",
			Desc:    "Price: " + strconv.Itoa(invoice.Price) + " Qty: " + strconv.Itoa(invoice.Quantity),
		})
	}

	m := climodels.SelectSimpleListModel{List: list.New(items, list.NewDefaultDelegate(), 0, 0)}
	m.List.Title = "Invoices"

	p := tea.NewProgram(m)
	_, err = p.Run()
	if err != nil {
		log.Fatal(err)
	}

	selectedInvoiceId := m.List.SelectedItem().(climodels.SelectSimpleListItem)

	var selectedInvoice store.Invoice
	for _, invoice := range invoices.Invoices {
		if invoice.Id == selectedInvoiceId.OfferId {
			selectedInvoice = invoice
			break
		}
	}

	indexerClient := indexer.NewIndexerClient(config.IndexerURL)

	utxos, err := indexerClient.GetUTXO(address)
	if err != nil {
		log.Fatal(err)
	}

	if len(utxos.UTXOs) == 0 {
		log.Fatal("No utxos found for address", address)
	}

	envelope := protocol.NewPaymentTransactionEnvelope(selectedInvoice.Hash, protocol.ACTION_PAYMENT)
	encodedTransactionBody := envelope.Serialize()

	inputs := []interface{}{
		map[string]interface{}{
			"txid": utxos.UTXOs[0].TxID,
			"vout": utxos.UTXOs[0].VOut,
		},
	}

	dogeUtxoValue := utxos.UTXOs[0].Value
	buyOfferValue := koinu.Koinu(selectedInvoice.Quantity * selectedInvoice.Price)
	fee, err := koinu.ParseKoinu("0.002")

	if err != nil {
		log.Fatal("Failed to parse fee value", err)
	}

	if dogeUtxoValue < buyOfferValue {
		log.Fatal("Insufficient balance for invoice", selectedInvoice.Hash)
	}

	change := dogeUtxoValue - buyOfferValue - fee
	sellerAddress := selectedInvoice.SellerAddress

	outputs := map[string]interface{}{
		"data": hex.EncodeToString(encodedTransactionBody),
	}

	if address == sellerAddress {
		outputs[address] = change
	} else {
		outputs[address] = change
		outputs[sellerAddress] = buyOfferValue
	}

	fmt.Println(outputs[address])
	fmt.Println(outputs[sellerAddress])

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

	fmt.Println("rawTxResponse", rawTxResponse)

	encodedTx, err := doge.SignRawTransaction(rawTxResponse, privHex, []doge.PrevOutput{
		{
			Address: address,
			Amount:  int64(utxos.UTXOs[0].Value),
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

	return nil
}
