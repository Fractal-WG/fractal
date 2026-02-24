package commands

import (
	"context"
	"encoding/json"
	"log"
	"strconv"

	fecli "dogecoin.org/fractal-engine/pkg/cli"
	climodels "dogecoin.org/fractal-engine/pkg/cli/climodels"
	"dogecoin.org/fractal-engine/pkg/cli/keys"
	fecfg "dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/indexer"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/urfave/cli/v3"
)

var TokensCommand = &cli.Command{
	Name:  "tokens",
	Usage: "Manage tokens",
	Commands: []*cli.Command{
		{
			Name:   "list",
			Usage:  "List tokens",
			Action: listTokensAction,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "config-path",
					Usage: "Path to the config file",
					Value: "config.toml",
				},
			},
		},
		{
			Name:   "burn",
			Usage:  "Burn tokens",
			Action: burnTokensAction,
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

func burnTokensAction(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config-path")

	config, err := fecli.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	secureStore := keys.NewSecureStore()
	address, err := secureStore.Get(config.ActiveKey + "_address")
	if err != nil {
		log.Fatal(err)
	}
	privHex, err := secureStore.Get(config.ActiveKey + "_private_key")
	if err != nil {
		log.Fatal(err)
	}
	chain, err := secureStore.Get(config.ActiveKey + "_chain")
	if err != nil {
		log.Fatal(err)
	}

	var mintHash string
	var burnQuantityStr string

	group := huh.NewGroup(
		huh.NewInput().
			Title("What is the Mint Hash?").
			Value(&mintHash),
		huh.NewInput().
			Title("How many tokens do you want to burn?").
			Value(&burnQuantityStr),
	)

	form := huh.NewForm(group)
	err = form.Run()
	if err != nil {
		log.Fatal(err)
	}

	burnQuantity, err := strconv.Atoi(burnQuantityStr)
	if err != nil {
		log.Fatal("Invalid burn quantity:", err)
	}

	tokenisationClient, err := getTokenisationClient(ctx, cmd)
	if err != nil {
		log.Fatal(err)
	}

	burnResponse, err := tokenisationClient.BurnTokens(&rpc.BurnTokensRequest{
		Payload: rpc.BurnTokensRequestPayload{
			MintHash:     mintHash,
			BurnQuantity: burnQuantity,
		},
	})
	if err != nil {
		log.Fatal("Error burning tokens:", err)
	}

	log.Println("Burn hash:", burnResponse.BurnHash)
	log.Println("Encoded transaction body:", burnResponse.EncodedTransactionBody)

	chainByte, err := doge.GetPrefix(chain)
	if err != nil {
		log.Fatal(err)
	}
	chainCfg := doge.GetChainCfg(chainByte)

	indexerClient := indexer.NewIndexerClient(config.IndexerURL)
	utxos, err := indexerClient.GetUTXO(address)
	if err != nil {
		log.Fatal(err)
	}

	if len(utxos.UTXOs) == 0 {
		log.Fatal("No utxos found for address", address)
	}

	inputs := []interface{}{
		map[string]interface{}{
			"txid": utxos.UTXOs[0].TxID,
			"vout": utxos.UTXOs[0].VOut,
		},
	}

	outputs := map[string]interface{}{
		"data":    burnResponse.EncodedTransactionBody,
		address:   utxos.UTXOs[0].Value - 1,
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
			Amount:  int64(utxos.UTXOs[0].Value),
		},
	}, chainCfg)
	if err != nil {
		log.Fatal(err)
	}

	res, err := dogeClient.Request(ctx, "sendrawtransaction", []interface{}{encodedTx})
	if err != nil {
		log.Println("Error sending raw transaction:", err)
		return err
	}

	var txid string
	if err := json.Unmarshal(*res, &txid); err != nil {
		log.Println("Error parsing send raw transaction response:", err)
		return err
	}

	log.Println("Token burn transaction sent:", txid)
	return nil
}

func listTokensAction(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config-path")

	config, err := fecli.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	secureStore := keys.NewSecureStore()
	address, err := secureStore.Get(config.ActiveKey + "_address")
	if err != nil {
		log.Fatal(err)
	}

	tokenisationClient, err := getTokenisationClient(ctx, cmd)
	if err != nil {
		log.Fatal(err)
	}

	var mintHash string

	group := huh.NewGroup(
		huh.NewInput().
			Title("What is the Mint Hash?").
			Value(&mintHash),
	)

	form := huh.NewForm(group)
	err = form.Run()
	if err != nil {
		log.Fatal(err)
	}

	tokenBalances, err := tokenisationClient.GetTokenBalance(address, mintHash)
	if err != nil {
		log.Fatal(err)
	}

	rows := []table.Row{}
	for _, tokenBalance := range tokenBalances {
		rows = append(rows, table.Row{tokenBalance.Address, tokenBalance.MintHash, strconv.Itoa(tokenBalance.Quantity)})
	}

	mintTable := climodels.CliTableModel{
		Table: table.New(
			table.WithColumns([]table.Column{
				{Title: "Address", Width: 20},
				{Title: "Mint Hash", Width: 64},
				{Title: "Quantity", Width: 10},
			}),
		),
	}

	mintTable.Table.SetRows(rows)

	p := tea.NewProgram(mintTable)
	_, err = p.Run()
	if err != nil {
		log.Fatal(err)
	}

	return nil
}
