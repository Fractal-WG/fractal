package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	dn "code.dogecoin.org/dogenet/pkg/dogenet"
	"code.dogecoin.org/dogenet/pkg/spec"
	"code.dogecoin.org/gossip/dnet"
	"code.dogecoin.org/governor"
	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/dogenet"
	"dogecoin.org/fractal-engine/pkg/service"
	"dogecoin.org/fractal-engine/pkg/store"
	"dogecoin.org/fractal-engine/pkg/version"
)

func main() {
	var rpcServerHost string
	var rpcServerPort string
	var rpcApiKey string
	var dogeNetNetwork string
	var dogeNetAddress string
	var dogeNetWebAddress string
	var dogeNetDbFile string
	var dogeNetChain string
	var dogeScheme string
	var dogeHost string
	var dogePort string
	var dogeUser string
	var dogePassword string
	var databaseURL string
	var persistFollower bool
	var rateLimitPerSecond int
	var invoiceLimit int
	var buyOfferLimit int
	var sellOfferLimit int
	var embedDogenet bool
	var corsAllowedOrigins string
	var relayURL string
	var showVersion bool
	var databaseHost string
	var databasePort string
	var databaseName string
	var databaseUsername string
	var databasePassword string

	flag.StringVar(&rpcServerHost, "rpc-server-host", getEnv("RPC_SERVER_HOST", "0.0.0.0"), "RPC Server Host")
	flag.StringVar(&rpcServerPort, "rpc-server-port", getEnv("RPC_SERVER_PORT", "8891"), "RPC Server Port")
	flag.StringVar(&rpcApiKey, "rpc-api-key", getEnv("RPC_API_KEY", ""), "RPC API Key, If set the RPC server is protected")
	flag.StringVar(&dogeNetNetwork, "doge-net-network", getEnv("DOGE_NET_NETWORK", "tcp"), "DogeNet Network")
	flag.StringVar(&dogeNetAddress, "doge-net-address", getEnv("DOGE_NET_ADDRESS", "0.0.0.0:8086"), "DogeNet Address")
	flag.StringVar(&dogeNetWebAddress, "doge-net-web-address", getEnv("DOGE_NET_WEB_ADDRESS", "0.0.0.0:8085"), "DogeNet Web Address")
	flag.StringVar(&dogeNetDbFile, "doge-net-db-file", getEnv("DOGE_NET_DB_FILE", "dogenet.db"), "DogeNet DB File")
	flag.StringVar(&dogeNetChain, "doge-net-chain", getEnv("DOGE_NET_CHAIN", "mainnet"), "DogeNet Chain")
	flag.BoolVar(&embedDogenet, "embed-dogenet", getEnvBool("EMBED_DOGENET", true), "Embed the DogeNet service")
	flag.StringVar(&dogeScheme, "doge-scheme", getEnv("DOGE_SCHEME", "http"), "Doge Scheme")
	flag.StringVar(&dogeHost, "doge-host", getEnv("DOGE_HOST", "0.0.0.0"), "Doge Host")
	flag.StringVar(&dogePort, "doge-port", getEnv("DOGE_PORT", "22556"), "Doge Port")
	flag.StringVar(&dogeUser, "doge-user", getEnv("DOGE_USER", "test"), "Doge User")
	flag.StringVar(&dogePassword, "doge-password", getEnv("DOGE_PASSWORD", "test"), "Doge Password")

	flag.StringVar(&databaseURL, "database-url", getEnv("DATABASE_URL", "postgres://fractalstore:fractalstore@localhost:5432/fractalstore?sslmode=disable"), "Database URL")
	flag.StringVar(&databaseHost, "database-host", getEnv("DATABASE_HOST", ""), "Database Host")
	flag.StringVar(&databasePort, "database-port", getEnv("DATABASE_PORT", ""), "Database Port")
	flag.StringVar(&databaseName, "database-name", getEnv("DATABASE_NAME", ""), "Database Name")
	flag.StringVar(&databaseUsername, "database-username", getEnv("DATABASE_USERNAME", ""), "Database Username")
	flag.StringVar(&databasePassword, "database-password", getEnv("DATABASE_PASSWORD", ""), "Database Password")

	flag.BoolVar(&persistFollower, "persist-follower", getEnvBool("PERSIST_FOLLOWER", true), "Persist Follower")
	flag.IntVar(&rateLimitPerSecond, "api-rate-limit-per-second", getEnvInt("API_RATE_LIMIT_PER_SECOND", 10), "API Rate Limit Per Second")
	flag.IntVar(&invoiceLimit, "invoice-limit", getEnvInt("INVOICE_LIMIT", 100), "Invoice Limit (per mint)")
	flag.IntVar(&buyOfferLimit, "buy-offer-limit", getEnvInt("BUY_OFFER_LIMIT", 3), "Buy Offer Limit (per buyer per mint)")
	flag.IntVar(&sellOfferLimit, "sell-offer-limit", getEnvInt("SELL_OFFER_LIMIT", 3), "Sell Offer Limit (per seller per mint)")
	flag.StringVar(&corsAllowedOrigins, "cors-allowed-origins", getEnv("CORS_ALLOWED_ORIGINS", "*"), "Comma-separated list of allowed CORS origins or *")
	flag.StringVar(&relayURL, "relay-url", getEnv("RELAY_URL", "http://10.0.2.2:8891"), "Relay URL for DogeConnect payment relay")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")

	flag.Parse()

	if showVersion {
		log.Printf("fractal-engine %s\n", version.String())
		return
	}

	if databaseHost != "" {
		databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s", databaseUsername, databasePassword, databaseHost, databasePort, databaseName)
	}

	cfg := &config.Config{
		RpcServerHost:      rpcServerHost,
		RpcServerPort:      rpcServerPort,
		RpcApiKey:          rpcApiKey,
		DogeNetNetwork:     dogeNetNetwork,
		DogeNetAddress:     dogeNetAddress,
		DogeNetWebAddress:  dogeNetWebAddress,
		DogeNetChain:       dogeNetChain,
		DogeScheme:         dogeScheme,
		DogeHost:           dogeHost,
		DogePort:           dogePort,
		DogeUser:           dogeUser,
		DogePassword:       dogePassword,
		DatabaseURL:        databaseURL,
		PersistFollower:    persistFollower,
		RateLimitPerSecond: rateLimitPerSecond,
		InvoiceLimit:       invoiceLimit,
		BuyOfferLimit:      buyOfferLimit,
		SellOfferLimit:     sellOfferLimit,
		CORSAllowedOrigins: corsAllowedOrigins,
		RelayURL:           relayURL,
	}

	tokenStore, err := store.NewTokenisationStore(cfg.DatabaseURL, *cfg)
	if err != nil {
		log.Fatalf("Failed to create tokenisation store: %v", err)
	}

	kp, err := dnet.GenerateKeyPair()
	if err != nil {
		log.Fatalf("Failed to generate key pair: %v", err)
	}

	cfg.DogeNetKeyPair = kp

	gov := governor.New().CatchSignals()

	dogenetClient := dogenet.NewDogeNetClient(cfg, tokenStore)

	if embedDogenet {
		const WebAPIDefaultPort = 8085
		const DefaultStorage = "./storage"

		dogeNetServerKp, err := dnet.GenerateKeyPair()
		if err != nil {
			log.Fatalf("Failed to generate key pair: %v", err)
		}

		// Print this out for the Test stack to read from the logs
		fmt.Printf("DogeNet PubKey is: %s\n", hex.EncodeToString(dogeNetServerKp.Pub[:]))

		var HandlerDefaultBind = spec.BindTo{Network: dogeNetNetwork, Address: dogeNetAddress} // const

		webAddy, err := dnet.ParseAddress(dogeNetWebAddress)
		if err != nil {
			log.Fatalf("Failed to parse web address: %v", err)
		}

		rawAddy := "0.0.0.0:" + strconv.Itoa(int(webAddy.Port)+33)

		addy, err := dnet.ParseAddress(rawAddy)
		if err != nil {
			log.Fatalf("Failed to parse web address: %v", err)
		}

		err = dn.DogeNet(gov, dn.DogeNetConfig{
			Dir:          DefaultStorage,
			DBFile:       dogeNetDbFile,
			Binds:        []dnet.Address{addy},
			BindWeb:      []dnet.Address{webAddy},
			HandlerBind:  HandlerDefaultBind,
			NodeKey:      dogeNetServerKp,
			AllowLocal:   true,
			Public:       addy,
			UseReflector: false,
		})

		if err != nil {
			log.Fatal("Failed to setup DogeNet")
		}
	}

	gov.Start()

	if embedDogenet {
		for {
			log.Println("Checking for dogenet socket...")
			active, _ := dogenetClient.ServerReady()
			if active {
				break
			}

			time.Sleep(200 * time.Millisecond)
		}
	}

	go dogenetClient.Run()

	log.Println("Waiting for DogeNet socket to be ready...")
	for {
		if dogenetClient.SocketReady() {
			break
		}
		log.Println("DogeNet socket not ready, waiting...")
		time.Sleep(200 * time.Millisecond)
	}
	log.Println("DogeNet socket ready")

	service := service.NewTokenisationService(cfg, dogenetClient, tokenStore)
	service.Run()

	gov.WaitForShutdown()
}

func getEnv(key, fallback string) string {
	v := os.Getenv(key)
	if v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
