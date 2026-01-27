package commands

import (
	"context"
	"log"

	fecli "dogecoin.org/fractal-engine/pkg/cli"
	"dogecoin.org/fractal-engine/pkg/cli/keys"
	fecfg "dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/doge"
	"github.com/urfave/cli/v3"
)

var DogeCommand = &cli.Command{
	Name:  "doge",
	Usage: "helper calls for doge",
	Commands: []*cli.Command{
		{
			Name:   "top-up",
			Usage:  "top up balance for a given address (regtest only)",
			Action: topUpBalanceAction,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:  "config-path",
					Usage: "Path to the config file",
					Value: "config.toml",
				},
			},
		},
		{
			Name:   "confirm",
			Usage:  "confirm blocks (regtest only)",
			Action: confirmAction,
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

func confirmAction(ctx context.Context, cmd *cli.Command) error {
	configPath := cmd.String("config-path")

	config, err := fecli.LoadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	dogeClient := doge.NewRpcClient(&fecfg.Config{
		DogeScheme:   config.DogeScheme,
		DogeHost:     config.DogeHost,
		DogePort:     config.DogePort,
		DogeUser:     config.DogeUser,
		DogePassword: config.DogePassword,
	})

	_, err = dogeClient.Request(ctx, "generate", []interface{}{1})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Confirmed blocks")

	return nil
}

func topUpBalanceAction(ctx context.Context, cmd *cli.Command) error {
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

	err = tokenisationClient.TopUpBalance(ctx, address)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Balance topped up for", address)

	return nil
}
