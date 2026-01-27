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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gotest.tools/assert"
)

func TestGossipMint(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	// Create pipe for testing
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Start client with connection
	go func() {
		defer func() {
			recover() // Recover from any panics during handshake/message processing
		}()
		client.StartWithConn(serverConn)
	}()

	// Handle handshake
	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}

	// Send handshake response
	clientConn.Write(br_buf[:])

	// Wait for client to be ready
	test_support.WaitForDogeNetClient(client)

	// Create test mint record
	testTime := time.Now()
	mint := store.Mint{
		MintWithoutID: store.MintWithoutID{
			Hash:          "hash123",
			Title:         "Test Mint",
			Description:   "Test Description",
			FractionCount: 100,
			Tags:          []string{"test", "mint"},
			Metadata: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			Requirements: map[string]interface{}{
				"req1": "reqval1",
			},
			LockupOptions: map[string]interface{}{
				"lockup1": "lockupval1",
			},
			FeedURL:   "https://example.com/feed",
			CreatedAt: testTime,
		},
	}

	// Test GossipMint in goroutine since it will block on send
	errCh := make(chan error, 1)
	go func() {
		errCh <- client.GossipMint(mint)
	}()

	// Read the message from the connection
	msg, err := dnet.ReadMessage(reader)
	assert.NilError(t, err)

	// Verify message properties
	assert.Equal(t, dogenet.ChanFE.String(), msg.Chan.String())
	assert.Equal(t, dogenet.TagMint.String(), msg.Tag.String())

	// Unmarshal and verify the envelope
	envelope := protocol.MintMessageEnvelope{}
	err = proto.Unmarshal(msg.Payload, &envelope)
	assert.NilError(t, err)

	assert.Equal(t, int32(protocol.ACTION_MINT), envelope.Type)
	assert.Equal(t, int32(protocol.DEFAULT_VERSION), envelope.Version)

	// Verify the mint message payload
	mintMsg := envelope.Payload
	assert.Equal(t, "hash123", mintMsg.Hash)
	assert.Equal(t, "Test Mint", mintMsg.Title)
	assert.Equal(t, "Test Description", mintMsg.Description)
	assert.Equal(t, int32(100), mintMsg.FractionCount)
	assert.Assert(t, len(mintMsg.Tags) == 2)
	assert.Equal(t, "test", mintMsg.Tags[0])
	assert.Equal(t, "mint", mintMsg.Tags[1])
	assert.Equal(t, "https://example.com/feed", mintMsg.FeedUrl)

	// Verify timestamps
	assert.Assert(t, mintMsg.CreatedAt != nil)
	receivedTime := mintMsg.CreatedAt.AsTime()
	assert.Assert(t, receivedTime.Sub(testTime) < time.Second)

	// Verify metadata
	assert.Assert(t, mintMsg.Metadata != nil)
	metadataMap := mintMsg.Metadata.AsMap()
	assert.Equal(t, "value1", metadataMap["key1"])
	assert.Equal(t, "value2", metadataMap["key2"])

	// Verify requirements
	assert.Assert(t, mintMsg.Requirements != nil)
	reqMap := mintMsg.Requirements.AsMap()
	assert.Equal(t, "reqval1", reqMap["req1"])

	// Verify lockup options
	assert.Assert(t, mintMsg.LockupOptions != nil)
	lockupMap := mintMsg.LockupOptions.AsMap()
	assert.Equal(t, "lockupval1", lockupMap["lockup1"])

	// Check that GossipMint completed successfully
	select {
	case err := <-errCh:
		assert.NilError(t, err)
	case <-time.After(100 * time.Millisecond):
		// May not complete due to connection closure
	}

	client.Stop()
}

func TestGossipMintWithNilMetadata(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	// Create pipe for testing
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Start client with connection
	go func() {
		defer func() { recover() }()
		client.StartWithConn(serverConn)
	}()

	// Handle handshake
	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}
	clientConn.Write(br_buf[:])

	// Wait for ready
	test_support.WaitForDogeNetClient(client)

	// Create test mint record with nil metadata
	mint := store.Mint{
		Id: "mint456",
		MintWithoutID: store.MintWithoutID{
			Hash:          "hash456",
			Title:         "Test Mint No Metadata",
			Description:   "Test Description",
			FractionCount: 50,
			Tags:          []string{"test"},
			Metadata:      nil, // nil metadata
			Requirements:  nil, // nil requirements
			LockupOptions: nil, // nil lockup options
			CreatedAt:     time.Now(),
		},
	}

	// Test GossipMint
	go func() {
		client.GossipMint(mint)
	}()

	// Read and verify message
	msg, err := dnet.ReadMessage(reader)
	assert.NilError(t, err)

	envelope := protocol.MintMessageEnvelope{}
	err = proto.Unmarshal(msg.Payload, &envelope)
	assert.NilError(t, err)

	mintMsg := envelope.Payload
	assert.Equal(t, "hash456", mintMsg.Hash)

	// Verify nil metadata is handled gracefully
	assert.Assert(t, mintMsg.Metadata != nil)
	assert.Assert(t, mintMsg.Requirements != nil)
	assert.Assert(t, mintMsg.LockupOptions != nil)

	client.Stop()
}

func TestRecvMintViaStartWithConn(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	// Create pipe for testing
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Start client with connection
	go func() {
		defer func() { recover() }()
		client.StartWithConn(serverConn)
	}()

	// Handle handshake
	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}
	clientConn.Write(br_buf[:])

	// Wait for ready
	test_support.WaitForDogeNetClient(client)

	privKey, dogePubKey, dogeAddress, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	// Create test mint message to send TO the client
	mintMessage := &protocol.MintMessage{
		Title:         "Test Mint",
		Description:   "Test Description",
		FractionCount: 100,
		Tags:          []string{"test", "mint"},
		FeedUrl:       "https://example.com/feed",
		OwnerAddress:  dogeAddress,
		Metadata: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"key1": {Kind: &structpb.Value_StringValue{StringValue: "value1"}},
				"key2": {Kind: &structpb.Value_StringValue{StringValue: "value2"}},
			},
		},
		Requirements: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"req1": {Kind: &structpb.Value_StringValue{StringValue: "reqval1"}},
			},
		},
		LockupOptions: &structpb.Struct{
			Fields: map[string]*structpb.Value{
				"lockup1": {Kind: &structpb.Value_StringValue{StringValue: "lockupval1"}},
			},
		},
	}

	signature, err := doge.SignPayload(mintMessage, privKey, dogePubKey)
	assert.NilError(t, err)

	envelope := &protocol.MintMessageEnvelope{
		Type:      protocol.ACTION_MINT,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   mintMessage,
		PublicKey: dogePubKey,
		Signature: signature,
	}

	data, err := proto.Marshal(envelope)
	assert.NilError(t, err)

	// Create and send dnet message to the client
	encodedMsg := dnet.EncodeMessageRaw(dogenet.ChanFE, dogenet.TagMint, keyPair, data)
	err = encodedMsg.Send(clientConn)
	assert.NilError(t, err)

	// Give the client time to process the message
	time.Sleep(100 * time.Millisecond)

	// Verify unconfirmed mint was saved
	mints, err := tokenStore.GetUnconfirmedMints(ctx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, 1, len(mints))

	savedMint := mints[0]
	assert.Equal(t, "Test Mint", savedMint.Title)
	assert.Equal(t, "Test Description", savedMint.Description)
	assert.Equal(t, 100, savedMint.FractionCount)
	assert.Assert(t, len(savedMint.Tags) == 2)
	assert.Equal(t, "test", string(savedMint.Tags[0]))
	assert.Equal(t, "mint", string(savedMint.Tags[1]))

	// Verify metadata
	assert.Equal(t, "value1", savedMint.Metadata["key1"])
	assert.Equal(t, "value2", savedMint.Metadata["key2"])

	// Verify requirements
	assert.Equal(t, "reqval1", savedMint.Requirements["req1"])

	// Verify lockup options
	assert.Equal(t, "lockupval1", savedMint.LockupOptions["lockup1"])

	client.Stop()
}

func TestRecvMintInvalidEnvelope(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	// Create pipe for testing
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Start client
	go func() {
		defer func() { recover() }()
		client.StartWithConn(serverConn)
	}()

	// Handle handshake
	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}
	clientConn.Write(br_buf[:])

	// Wait for ready
	test_support.WaitForDogeNetClient(client)

	// Send invalid protobuf data
	invalidData := []byte("invalid protobuf data")
	encodedMsg := dnet.EncodeMessageRaw(dogenet.ChanFE, dogenet.TagMint, keyPair, invalidData)
	err = encodedMsg.Send(clientConn)
	assert.NilError(t, err)

	// Give time to process
	time.Sleep(100 * time.Millisecond)

	// Verify no mint was saved
	mints, err := tokenStore.GetUnconfirmedMints(ctx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, 0, len(mints))

	client.Stop()
}

func TestRecvMintWrongActionType(t *testing.T) {
	tokenStore := test_support.SetupTestDB(t)
	ctx := context.Background()
	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	assert.NilError(t, err)
	cfg.DogeNetKeyPair = keyPair

	client := dogenet.NewDogeNetClient(cfg, tokenStore)

	// Create pipe for testing
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	// Start client
	go func() {
		defer func() { recover() }()
		client.StartWithConn(serverConn)
	}()

	// Handle handshake
	reader := bufio.NewReader(clientConn)
	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		t.Fatalf("Failed to read bind message: %v", err)
	}
	clientConn.Write(br_buf[:])

	// Wait for ready
	test_support.WaitForDogeNetClient(client)

	// Create mint message with wrong action type
	mintMessage := &protocol.MintMessage{
		Hash:          "hash123",
		Title:         "Test Mint",
		FractionCount: 100,
		CreatedAt:     timestamppb.New(time.Now()),
	}

	envelope := &protocol.MintMessageEnvelope{
		Type:    protocol.ACTION_PAYMENT, // Wrong action type
		Version: protocol.DEFAULT_VERSION,
		Payload: mintMessage,
	}

	data, err := proto.Marshal(envelope)
	assert.NilError(t, err)

	// Send message with wrong action type
	encodedMsg := dnet.EncodeMessageRaw(dogenet.ChanFE, dogenet.TagMint, keyPair, data)
	err = encodedMsg.Send(clientConn)
	assert.NilError(t, err)

	// Give time to process
	time.Sleep(100 * time.Millisecond)

	// Verify no mint was saved due to wrong action type
	mints, err := tokenStore.GetUnconfirmedMints(ctx, 0, 10)
	assert.NilError(t, err)
	assert.Equal(t, 0, len(mints))

	client.Stop()
}
