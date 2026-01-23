package rpc_test

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"google.golang.org/protobuf/types/known/structpb"
	"gotest.tools/assert"
)

func TestMints(t *testing.T) {
	tokenisationStore, dogenetClient, feClient := SetupRpcTest(t)

	privHex, pubHex, address, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	if err != nil {
		t.Fatalf("GenerateDogecoinKeypair: %v", err)
	}

	payload := rpc.CreateMintRequestPayload{
		Title:         "Test Mint",
		FractionCount: 100,
		Description:   "Test Description",
		Tags:          []string{"test", "mint"},
		Metadata: map[string]interface{}{
			"test": "test",
		},
		Requirements:  map[string]interface{}{},
		LockupOptions: map[string]interface{}{},
		FeedURL:       "https://test.com",
		OwnerAddress:  address,
	}

	signature, err := doge.SignPayload(payload, privHex, pubHex)
	if err != nil {
		t.Fatalf("Failed to sign payload: %v", err)
	}

	metadataStruct, err := structpb.NewStruct(map[string]interface{}{
		"test": "test",
	})
	assert.NilError(t, err)
	emptyStruct, err := structpb.NewStruct(map[string]interface{}{})
	assert.NilError(t, err)

	protoMetadata := &protocol.StringInterfaceMap{}
	protoMetadata.Value = metadataStruct
	protoRequirements := &protocol.StringInterfaceMap{}
	protoRequirements.Value = emptyStruct
	protoLockupOptions := &protocol.StringInterfaceMap{}
	protoLockupOptions.Value = emptyStruct

	protoPayload := &protocol.CreateMintRequestPayload{}
	protoPayload.Title = payload.Title
	protoPayload.Description = payload.Description
	protoPayload.FractionCount = int32(payload.FractionCount)
	protoPayload.Tags = payload.Tags
	protoPayload.Metadata = protoMetadata
	protoPayload.Requirements = protoRequirements
	protoPayload.LockupOptions = protoLockupOptions
	protoPayload.FeedUrl = payload.FeedURL
	protoPayload.OwnerAddress = &protocol.Address{Value: &payload.OwnerAddress}

	protoRequest := &protocol.CreateMintRequest{}
	protoRequest.Payload = protoPayload
	protoRequest.PublicKey = pubHex
	protoRequest.Signature = signature

	mintResponse, err := feClient.CreateMint(context.Background(), connect.NewRequest(protoRequest))
	if err != nil {
		t.Fatalf("Failed to create mint: %v", err)
	}

	mints, err := tokenisationStore.GetUnconfirmedMints(0, 10)
	if err != nil {
		t.Fatalf("Failed to get mints: %v", err)
	}

	assert.Equal(t, len(mints), 1)
	assert.Equal(t, mints[0].Hash, mintResponse.Msg.GetHash().GetValue())
	assert.Equal(t, mints[0].Title, payload.Title)
	assert.Equal(t, mints[0].FractionCount, payload.FractionCount)
	assert.Equal(t, mints[0].Description, payload.Description)
	assert.DeepEqual(t, mints[0].Tags, payload.Tags)
	assert.DeepEqual(t, mints[0].Metadata, payload.Metadata)
	assert.Equal(t, mints[0].FeedURL, payload.FeedURL)

	assert.Equal(t, len(dogenetClient.mints), 1)
	assert.Equal(t, dogenetClient.mints[0].Hash, mintResponse.Msg.GetHash().GetValue())
	assert.Equal(t, dogenetClient.mints[0].Title, payload.Title)
	assert.Equal(t, dogenetClient.mints[0].FractionCount, payload.FractionCount)
	assert.Equal(t, dogenetClient.mints[0].Description, payload.Description)
	assert.DeepEqual(t, dogenetClient.mints[0].Tags, payload.Tags)
	assert.DeepEqual(t, dogenetClient.mints[0].Metadata, payload.Metadata)
	assert.Equal(t, dogenetClient.mints[0].FeedURL, payload.FeedURL)
}
