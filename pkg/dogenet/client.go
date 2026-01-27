package dogenet

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"dogecoin.org/fractal-engine/pkg/config"

	"code.dogecoin.org/gossip/dnet"
	"code.dogecoin.org/governor"
	"dogecoin.org/fractal-engine/pkg/store"
	"google.golang.org/protobuf/types/known/structpb"
)

type NodePubKeyMsg struct {
	PubKey []byte
}

type AddPeer struct {
	Key  string `json:"key"`
	Addr string `json:"addr"`
}

type NodeInfo struct {
	Key      string `json:"pubkey"`
	Addr     string `json:"address"`
	Identity string `json:"identity"`
}

type GetNodesResponse []NodeInfo

type GossipClient interface {
	GossipMint(record store.Mint) error
	GossipBuyOffer(record store.BuyOffer) error
	GossipSellOffer(record store.SellOffer) error
	GossipDeleteBuyOffer(hash string, publicKey string, signature string) error
	GossipDeleteSellOffer(hash string, publicKey string, signature string) error
	GossipUnconfirmedInvoice(record store.UnconfirmedInvoice) error
	GossipInvoiceSignature(record store.InvoiceSignature) error
	GetNodes() (GetNodesResponse, error)
	AddPeer(addPeer AddPeer) error
	CheckRunning() error
	Run()
	Stop()
}

type DogeNetClient struct {
	governor.ServiceCtx
	GossipClient
	cfg           *config.Config
	store         *store.TokenisationStore
	sock          net.Conn
	feKey         dnet.KeyPair
	Stopping      bool
	Messages      chan dnet.Message
	Running       bool
	dogeNetCtx    context.Context
	dogeNetCancel context.CancelFunc
}

const GossipInterval = 71 * time.Second // gossip a random identity to peers

func convertToStructPBMap(m map[string]interface{}) map[string]*structpb.Value {
	fields := make(map[string]*structpb.Value)
	for k, v := range m {
		fields[k] = &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: v.(string)}}
	}
	return fields
}

func NewDogeNetClient(cfg *config.Config, store *store.TokenisationStore) *DogeNetClient {
	return &DogeNetClient{
		cfg:      cfg,
		store:    store,
		Stopping: false,
		feKey:    cfg.DogeNetKeyPair,
		Messages: make(chan dnet.Message),
	}
}

func (c *DogeNetClient) SocketReady() bool {
	return c.sock != nil
}

func (c *DogeNetClient) ServerReady() (bool, error) {
	sock, err := net.Dial(c.cfg.DogeNetNetwork, c.cfg.DogeNetAddress)
	if err != nil {
		return false, err
	}
	defer sock.Close()

	return true, nil
}

func (c *DogeNetClient) GetNodes() (GetNodesResponse, error) {
	resp, err := http.Get("http://" + c.cfg.DogeNetWebAddress + "/nodes")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %s", err)
	}

	var nodes GetNodesResponse

	err = json.Unmarshal(body, &nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %s", err)
	}

	return nodes, nil
}

func (c *DogeNetClient) AddPeer(addPeer AddPeer) error {
	payload, err := json.Marshal(addPeer)
	if err != nil {
		return err
	}

	resp, err := http.Post("http://"+c.cfg.DogeNetWebAddress+"/addpeer", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %s", err)
		}

		fmt.Println(string(body))

		return fmt.Errorf("failed to add peer: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %s", err)
	}

	fmt.Println(string(body))

	return nil
}

func (c *DogeNetClient) CheckRunning() error {
	resp, err := http.Get("http://" + c.cfg.DogeNetWebAddress + "/")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *DogeNetClient) StartWithConn(conn net.Conn) {
	c.sock = conn
	c.Run()
}

func (c *DogeNetClient) Run() {
	if c.Running {
		log.Println("Dogenet client already running")
		return
	}

	c.Running = true

	if c.sock == nil {
		sock, err := net.Dial(c.cfg.DogeNetNetwork, c.cfg.DogeNetAddress)
		if err != nil {
			log.Printf("[FE] cannot connect: %v", err)
			return
		}
		c.sock = sock
	}

	log.Printf("[FE] connected to dogenet.")
	bind := dnet.BindMessage{Version: 1, Chan: ChanFE, PubKey: *c.feKey.Pub}

	_, err := c.sock.Write(bind.Encode())
	if err != nil {
		log.Printf("[FE] cannot send BindMessage: %v", err)
		c.sock.Close()
		return
	}

	reader := bufio.NewReader(c.sock)

	log.Printf("[FE] reading BindMessage reply.")
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		log.Printf("[FE] reading BindMessage reply: %v", err)
		c.sock.Close()
		return
	}

	log.Printf("[FE] reading DecodeBindMessage reply.")

	if _, ok := dnet.DecodeBindMessage(br_buf[:]); ok {
		// send the node's pubkey to the announce service
		// so it can include the node key in the identity announcement
		// TODO
		log.Printf("[FE] Decoded BindMessage reply.")
	} else {
		log.Printf("[FE] invalid BindMessage reply: %v", err)
		c.sock.Close()
		return
	}
	log.Printf("[FE] completed handshake.")

	if c.dogeNetCancel != nil {
		c.dogeNetCancel()
	}
	c.dogeNetCtx, c.dogeNetCancel = context.WithCancel(context.Background())
	go c.gossipRandomMints(c.dogeNetCtx)
	go c.gossipRandomInvoices(c.dogeNetCtx)
	go c.gossipRandomInvoiceSignatures(c.dogeNetCtx)

	for !c.Stopping {
		msg, err := dnet.ReadMessage(reader)
		if err != nil {
			log.Printf("[FE] cannot receive from peer: %v", err)
			c.sock.Close()
			return
		}

		log.Printf("[FE] received message: [%s][%s]", msg.Chan, msg.Tag)

		// write to channel in a goroutine to avoid blocking
		go func() {
			c.Messages <- msg
		}()

		if msg.Chan != ChanFE {
			log.Printf("[FE] ignored message: [%s][%s]", msg.Chan, msg.Tag)
			continue
		}

		log.Printf("[FE] message received\n")

		switch msg.Tag {
		case TagMint:
			c.recvMint(msg)
		case TagBuyOffer:
			c.recvBuyOffer(c.dogeNetCtx, msg)
		case TagSellOffer:
			c.recvSellOffer(msg)
		case TagInvoice:
			c.recvInvoice(msg)
		case TagDeleteBuyOffer:
			c.recvDeleteBuyOffer(msg)
		case TagDeleteSellOffer:
			c.recvDeleteSellOffer(msg)
		case TagInvoiceSignature:
			c.recvInvoiceSignature(msg)
		default:
			log.Printf("[FE] unknown message: [%s][%s]", msg.Chan, msg.Tag)
		}
	}
}

func (c *DogeNetClient) Stop() {
	fmt.Println("Stopping dogenet client")
	c.Stopping = true

	if c.dogeNetCancel != nil {
		c.dogeNetCancel()
	}

	if c.sock != nil {
		c.sock.Close()
	}
}

func (s *DogeNetClient) gossipRandomMints(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if s.Stopping {
				return
			}
		}
		// wait for next turn
		time.Sleep(GossipInterval)

		// choose a random identity
		mint, err := s.store.ChooseMint(ctx)
		if err != nil {
			log.Printf("[FE] cannot choose mint: %v", err)
			continue
		}

		log.Printf("[FE] Gossiping random mint\n")

		err = s.GossipMint(mint)
		if err != nil {
			log.Printf("[FE] cannot gossip mint: %v", err)
		}
	}
}

func (s *DogeNetClient) gossipRandomInvoices(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if s.Stopping {
				return
			}
		}
		// wait for next turn
		time.Sleep(GossipInterval)

		// choose a random identity
		invoice, err := s.store.ChooseInvoice(ctx)
		log.Println("Choose Invoice")

		if err != nil {
			log.Printf("[FE] cannot choose invoice: %v", err)
			continue
		}

		log.Printf("[FE] Gossiping random invoice\n")
		unconfirmedInvoice := store.UnconfirmedInvoice{
			Id:             invoice.Id,
			Hash:           invoice.Hash,
			PaymentAddress: invoice.PaymentAddress,
			BuyerAddress:   invoice.BuyerAddress,
			MintHash:       invoice.MintHash,
			Quantity:       invoice.Quantity,
			Price:          invoice.Price,
			CreatedAt:      invoice.CreatedAt,
			SellerAddress:  invoice.SellerAddress,
			PublicKey:      invoice.PublicKey,
			Signature:      invoice.Signature,
		}

		err = s.GossipUnconfirmedInvoice(unconfirmedInvoice)
		if err != nil {
			log.Printf("[FE] cannot gossip invoice: %v", err)
		}
	}
}

func (s *DogeNetClient) gossipRandomInvoiceSignatures(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if s.Stopping {
				return
			}
		}
		// wait for next turn
		time.Sleep(GossipInterval)

		// choose a random
		invoiceSignature, err := s.store.ChooseInvoiceSignature(ctx)
		log.Println("Choose Invoice Signature")

		if err != nil {
			log.Printf("[FE] cannot choose invoice: %v", err)
			continue
		}

		log.Printf("[FE] Gossiping random invoice signature\n")
		unconfirmedInvoiceSignature := store.InvoiceSignature{
			Id:          invoiceSignature.Id,
			InvoiceHash: invoiceSignature.InvoiceHash,
			Signature:   invoiceSignature.Signature,
			PublicKey:   invoiceSignature.PublicKey,
			CreatedAt:   invoiceSignature.CreatedAt,
		}

		err = s.GossipInvoiceSignature(unconfirmedInvoiceSignature)
		if err != nil {
			log.Printf("[FE] cannot gossip invoice: %v", err)
		}
	}
}
