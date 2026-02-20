package dogenet

import "code.dogecoin.org/gossip/dnet"

var ChanFE = dnet.NewTag("FrkE")
var TagMint = dnet.NewTag("Mint")
var TagBuyOffer = dnet.NewTag("BuyO")
var TagSellOffer = dnet.NewTag("SellO")
var TagInvoice = dnet.NewTag("Invo")
var TagInvoiceSignature = dnet.NewTag("Sign")
var TagDeleteBuyOffer = dnet.NewTag("DBuyO")
var TagDeleteSellOffer = dnet.NewTag("DSell")
var TagMintExpansion = dnet.NewTag("MExp")

type GossipMessage struct {
	Topic string `json:"topic"`
	Data  []byte `json:"data"`
}

type GossipMessageListener func(message GossipMessage)
