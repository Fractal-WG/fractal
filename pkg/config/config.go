package config

import "code.dogecoin.org/gossip/dnet"

type Config struct {
	RpcServerHost      string
	RpcServerPort      string
	RpcApiKey          string
	DogeNetChain       string
	DogeNetNetwork     string
	DogeNetAddress     string
	DogeNetWebAddress  string
	DogeNetKeyPair     dnet.KeyPair
	DogeHost           string
	DogeScheme         string
	DogePort           string
	DogeUser           string
	DogePassword       string
	DatabaseURL        string
	PersistFollower    bool
	RateLimitPerSecond int
	InvoiceLimit       int
	BuyOfferLimit      int
	SellOfferLimit     int
	CORSAllowedOrigins string
	RelayURL           string
}

func NewConfig() *Config {
	return &Config{
		RpcServerHost:      "0.0.0.0",
		RpcServerPort:      "8891",
		DogeNetChain:       "regtest",
		DogeNetNetwork:     "tcp",
		DogeNetAddress:     "0.0.0.0:42069",
		DogeNetWebAddress:  "0.0.0.0:8085",
		DogeNetKeyPair:     dnet.KeyPair{},
		DogeScheme:         "http",
		DogeHost:           "dogecoin",
		DogePort:           "22555",
		DogeUser:           "test",
		DogePassword:       "test",
		DatabaseURL:        "sqlite://fractal-engine.db",
		PersistFollower:    true,
		RateLimitPerSecond: 10,
		InvoiceLimit:       10,
		BuyOfferLimit:      10,
		SellOfferLimit:     10,
		CORSAllowedOrigins: "*",
		RelayURL:           "http://10.0.2.2:8891/dc/relay",
	}
}
