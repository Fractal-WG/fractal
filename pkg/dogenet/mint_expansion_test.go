package dogenet_test

import (
	"bufio"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"code.dogecoin.org/gossip/dnet"

	test_support "dogecoin.org/fractal-engine/internal/test/support"

	"dogecoin.org/fractal-engine/pkg/config"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/dogenet"
	"dogecoin.org/fractal-engine/pkg/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gotest.tools/assert"
)

// mintExpansionSigPayload mirrors rpc.ExpandMintRequestPayload for signing in tests.
type mintExpansionSigPayload struct {
	MintHash         string `json:"mint_hash"`
	AdditionalSupply int    `json:"additional_supply"`
}

func setupDogeNetClient(t *testing.T) (*dogenet.DogeNetClient, net.Conn, *bufio.Reader) {
	t.Helper()
	tokenStore := test_support.SetupTestDB(t)
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
	})

	go func() {
		defer func() { recover() }()
		client.StartWithConn(serverConn)
	}()

	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}
	clientConn.Write(br_buf[:])

	test_support.WaitForDogeNetClient(client)

	return client, clientConn, reader
}

func TestGossipMintExpansion(t *testing.T) {
	client, _, reader := setupDogeNetClient(t)

	testTime := time.Now()
	expansion := store.UnconfirmedMintExpansion{
		Hash:             "expansionhash123",
		MintHash:         "minthash456",
		AdditionalSupply: 500,
		OwnerAddress:     "nTestOwnerAddress1234567890123456",
		PublicKey:        "pubkey789",
		Signature:        "sig123",
		CreatedAt:        testTime,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- client.GossipMintExpansion(expansion)
	}()

	msg, err := dnet.ReadMessage(reader)
	assert.NilError(t, err)

	assert.Equal(t, dogenet.ChanFE.String(), msg.Chan.String())
	assert.Equal(t, dogenet.TagMintExpansion.String(), msg.Tag.String())

	envelope := protocol.MintExpansionMessageEnvelope{}
	err = proto.Unmarshal(msg.Payload, &envelope)
	assert.NilError(t, err)

	assert.Equal(t, int32(protocol.ACTION_MINT_EXPANSION), envelope.Type)
	assert.Equal(t, int32(protocol.DEFAULT_VERSION), envelope.Version)
	assert.Equal(t, "pubkey789", envelope.PublicKey)
	assert.Equal(t, "sig123", envelope.Signature)

	payload := envelope.Payload
	assert.Equal(t, "expansionhash123", payload.Hash)
	assert.Equal(t, "minthash456", payload.MintHash)
	assert.Equal(t, int32(500), payload.AdditionalSupply)
	assert.Equal(t, "nTestOwnerAddress1234567890123456", payload.OwnerAddress)
	assert.Assert(t, payload.CreatedAt != nil)
	assert.Assert(t, payload.CreatedAt.AsTime().Sub(testTime) < time.Second)

	select {
	case err := <-errCh:
		assert.NilError(t, err)
	case <-time.After(100 * time.Millisecond):
	}

	client.Stop()
}

func TestRecvMintExpansionViaStartWithConn(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		defer func() { recover() }()
		client.StartWithConn(serverConn)
	}()

	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}
	clientConn.Write(br_buf[:])

	test_support.WaitForDogeNetClient(client)

	privKey, dogePubKey, dogeAddress, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	mintHash := test_support.GenerateRandomHash()

	mint := &store.MintWithoutID{
		Hash:           mintHash,
		Title:          "Test Mint",
		FractionCount:  1000,
		AllowExpansion: true,
		PublicKey:      dogePubKey,
	}
	_, err = tokenStore.SaveMint(ctx, mint, dogeAddress)
	assert.NilError(t, err)

	const additionalSupply = 250

	sigPayload := mintExpansionSigPayload{
		MintHash:         mintHash,
		AdditionalSupply: additionalSupply,
	}
	signature, err := doge.SignPayload(sigPayload, privKey, dogePubKey)
	assert.NilError(t, err)

	nonce := uuid.New().String()
	expansionForHash := &store.UnconfirmedMintExpansion{
		MintHash:         mintHash,
		AdditionalSupply: additionalSupply,
		OwnerAddress:     dogeAddress,
		PublicKey:        dogePubKey,
		Signature:        signature,
		Nonce:            nonce,
	}
	expansionHash, err := expansionForHash.GenerateHash()
	assert.NilError(t, err)

	expansionMessage := &protocol.MintExpansionMessage{
		Hash:             expansionHash,
		MintHash:         mintHash,
		AdditionalSupply: additionalSupply,
		OwnerAddress:     dogeAddress,
		CreatedAt:        timestamppb.New(time.Now()),
		Nonce:            nonce,
	}

	envelope := &protocol.MintExpansionMessageEnvelope{
		Type:      protocol.ACTION_MINT_EXPANSION,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   expansionMessage,
		PublicKey: dogePubKey,
		Signature: signature,
	}

	data, err := proto.Marshal(envelope)
	assert.NilError(t, err)

	encodedMsg := dnet.EncodeMessageRaw(dogenet.ChanFE, dogenet.TagMintExpansion, keyPair, data)
	err = encodedMsg.Send(clientConn)
	assert.NilError(t, err)

	time.Sleep(100 * time.Millisecond)

	var count int
	err = tokenStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM unconfirmed_mint_expansions").Scan(&count)
	assert.NilError(t, err)
	assert.Equal(t, 1, count)

	var saved store.UnconfirmedMintExpansion
	err = tokenStore.DB.QueryRowContext(ctx,
		"SELECT hash, mint_hash, additional_supply, owner_address, public_key, signature FROM unconfirmed_mint_expansions LIMIT 1",
	).Scan(&saved.Hash, &saved.MintHash, &saved.AdditionalSupply, &saved.OwnerAddress, &saved.PublicKey, &saved.Signature)
	assert.NilError(t, err)

	assert.Equal(t, expansionHash, saved.Hash)
	assert.Equal(t, mintHash, saved.MintHash)
	assert.Equal(t, additionalSupply, saved.AdditionalSupply)
	assert.Equal(t, dogeAddress, saved.OwnerAddress)
	assert.Equal(t, dogePubKey, saved.PublicKey)
	assert.Equal(t, signature, saved.Signature)

	client.Stop()
}

func TestRecvMintExpansionInvalidEnvelope(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		defer func() { recover() }()
		client.StartWithConn(serverConn)
	}()

	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}
	clientConn.Write(br_buf[:])

	test_support.WaitForDogeNetClient(client)

	invalidData := []byte("invalid protobuf data")
	encodedMsg := dnet.EncodeMessageRaw(dogenet.ChanFE, dogenet.TagMintExpansion, keyPair, invalidData)
	err = encodedMsg.Send(clientConn)
	assert.NilError(t, err)

	time.Sleep(100 * time.Millisecond)

	var count int
	err = tokenStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM unconfirmed_mint_expansions").Scan(&count)
	assert.NilError(t, err)
	assert.Equal(t, 0, count)

	client.Stop()
}

func TestRecvMintExpansionWrongActionType(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		defer func() { recover() }()
		client.StartWithConn(serverConn)
	}()

	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}
	clientConn.Write(br_buf[:])

	test_support.WaitForDogeNetClient(client)

	expansionMessage := &protocol.MintExpansionMessage{
		Hash:             "hash123",
		MintHash:         "minthash456",
		AdditionalSupply: 100,
		CreatedAt:        timestamppb.New(time.Now()),
	}

	envelope := &protocol.MintExpansionMessageEnvelope{
		Type:    protocol.ACTION_PAYMENT, // Wrong action type
		Version: protocol.DEFAULT_VERSION,
		Payload: expansionMessage,
	}

	data, err := proto.Marshal(envelope)
	assert.NilError(t, err)

	encodedMsg := dnet.EncodeMessageRaw(dogenet.ChanFE, dogenet.TagMintExpansion, keyPair, data)
	err = encodedMsg.Send(clientConn)
	assert.NilError(t, err)

	time.Sleep(100 * time.Millisecond)

	var count int
	err = tokenStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM unconfirmed_mint_expansions").Scan(&count)
	assert.NilError(t, err)
	assert.Equal(t, 0, count)

	client.Stop()
}

func TestRecvMintExpansionInvalidSignature(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		defer func() { recover() }()
		client.StartWithConn(serverConn)
	}()

	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}
	clientConn.Write(br_buf[:])

	test_support.WaitForDogeNetClient(client)

	_, dogePubKey, dogeAddress, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	expansionMessage := &protocol.MintExpansionMessage{
		Hash:             test_support.GenerateRandomHash(),
		MintHash:         test_support.GenerateRandomHash(),
		AdditionalSupply: 100,
		OwnerAddress:     dogeAddress,
		CreatedAt:        timestamppb.New(time.Now()),
	}

	envelope := &protocol.MintExpansionMessageEnvelope{
		Type:      protocol.ACTION_MINT_EXPANSION,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   expansionMessage,
		PublicKey: dogePubKey,
		Signature: "invalidsignature",
	}

	data, err := proto.Marshal(envelope)
	assert.NilError(t, err)

	encodedMsg := dnet.EncodeMessageRaw(dogenet.ChanFE, dogenet.TagMintExpansion, keyPair, data)
	err = encodedMsg.Send(clientConn)
	assert.NilError(t, err)

	time.Sleep(100 * time.Millisecond)

	var count int
	err = tokenStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM unconfirmed_mint_expansions").Scan(&count)
	assert.NilError(t, err)
	assert.Equal(t, 0, count)

	client.Stop()
}

func TestRecvMintExpansionAddressMismatch(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		defer func() { recover() }()
		client.StartWithConn(serverConn)
	}()

	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}
	clientConn.Write(br_buf[:])

	test_support.WaitForDogeNetClient(client)

	privKey, dogePubKey, _, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	mintHash := test_support.GenerateRandomHash()
	const additionalSupply = 100

	sigPayload := mintExpansionSigPayload{
		MintHash:         mintHash,
		AdditionalSupply: additionalSupply,
	}
	signature, err := doge.SignPayload(sigPayload, privKey, dogePubKey)
	assert.NilError(t, err)

	expansionMessage := &protocol.MintExpansionMessage{
		Hash:             test_support.GenerateRandomHash(),
		MintHash:         mintHash,
		AdditionalSupply: additionalSupply,
		OwnerAddress:     "nWrongAddress000000000000000000000", // doesn't match dogePubKey
		CreatedAt:        timestamppb.New(time.Now()),
	}

	envelope := &protocol.MintExpansionMessageEnvelope{
		Type:      protocol.ACTION_MINT_EXPANSION,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   expansionMessage,
		PublicKey: dogePubKey,
		Signature: signature,
	}

	data, err := proto.Marshal(envelope)
	assert.NilError(t, err)

	encodedMsg := dnet.EncodeMessageRaw(dogenet.ChanFE, dogenet.TagMintExpansion, keyPair, data)
	err = encodedMsg.Send(clientConn)
	assert.NilError(t, err)

	time.Sleep(100 * time.Millisecond)

	var count int
	err = tokenStore.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM unconfirmed_mint_expansions").Scan(&count)
	assert.NilError(t, err)
	assert.Equal(t, 0, count)

	client.Stop()
}
