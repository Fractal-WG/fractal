package dogenet_test

import (
	"bufio"
	"fmt"
	"io"
	"log"
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
	"gotest.tools/assert"
)

func TestDogenet(t *testing.T) {
	tokenisationStore := test_support.SetupTestDB(t)

	myConn, dogenetConn := net.Pipe()

	cfg := config.NewConfig()
	keyPair, err := dnet.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	cfg.DogeNetKeyPair = keyPair

	dogeClient := dogenet.NewDogeNetClient(cfg, tokenisationStore)

	go dogeClient.StartWithConn(dogenetConn)

	test_support.WaitForDogeNetClient(dogeClient)

	reader := bufio.NewReader(myConn)

	br_buf := [dnet.BindMessageSize]byte{}
	_, err = io.ReadAtLeast(reader, br_buf[:], len(br_buf))
	if err != nil {
		log.Printf("[FE] reading BindMessage reply: %v", err)
	}

	log.Printf("[FE] reading BindMessage reply: %v", br_buf)

	myConn.Write(br_buf[:])

	privKey, dogePubKey, dogeAddress, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	if err != nil {
		log.Fatalf("Failed to generate dogecoin keypair: %v", err)
	}

	record := store.Mint{
		MintWithoutID: store.MintWithoutID{
			Title:         "Test Mint",
			FractionCount: 100,
			Description:   "Test Description",
			Tags:          []string{"test", "mint"},
			Metadata: map[string]interface{}{
				"test": "test",
			},
			OwnerAddress: dogeAddress,
		},
	}
	mintMessage := protocol.MintMessage{
		Title:           record.Title,
		Description:     record.Description,
		FractionCount:   int32(record.FractionCount),
		Tags:            record.Tags,
		TransactionHash: record.TransactionHash,
		FeedUrl:         record.FeedURL,
		OwnerAddress:    record.OwnerAddress,
	}

	signature, err := doge.SignPayload(&mintMessage, privKey, dogePubKey)
	if err != nil {
		log.Fatalf("Failed to sign payload: %v", err)
	}

	envelope := protocol.MintMessageEnvelope{
		Type:      protocol.ACTION_MINT,
		Version:   protocol.DEFAULT_VERSION,
		Payload:   &mintMessage,
		PublicKey: dogePubKey,
		Signature: signature,
	}

	data, err := proto.Marshal(&envelope)
	if err != nil {
		log.Fatalf("Failed to marshal: %v", err)
	}

	encodedMsg := dnet.EncodeMessageRaw(dogenet.ChanFE, dogenet.TagMint, keyPair, data)

	err = encodedMsg.Send(myConn)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	time.Sleep(3 * time.Second)

	unconfMints, err := tokenisationStore.GetUnconfirmedMints(t.Context(), 0, 10)
	if err != nil {
		log.Fatalf("Failed to get unconfirmed mints: %v", err)
	}

	fmt.Println("Unconfirmed mints:")
	fmt.Println(unconfMints)

	assert.Equal(t, len(unconfMints), 1)
	assert.Equal(t, unconfMints[0].Hash, record.Hash)
	assert.Equal(t, unconfMints[0].Title, record.Title)
	assert.Equal(t, unconfMints[0].Description, record.Description)
	assert.Equal(t, unconfMints[0].FractionCount, record.FractionCount)

	dogeClient.Stop()
}
