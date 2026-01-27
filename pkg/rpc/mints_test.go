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
	ctx := context.Background()

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
	protoMetadata.SetValue(metadataStruct)
	protoRequirements := &protocol.StringInterfaceMap{}
	protoRequirements.SetValue(emptyStruct)
	protoLockupOptions := &protocol.StringInterfaceMap{}
	protoLockupOptions.SetValue(emptyStruct)

	ownerAddressProto := &protocol.Address{}
	ownerAddressProto.SetValue(payload.OwnerAddress)

	protoPayload := &protocol.CreateMintRequestPayload{}
	protoPayload.SetTitle(payload.Title)
	protoPayload.SetDescription(payload.Description)
	protoPayload.SetFractionCount(int32(payload.FractionCount))
	protoPayload.SetTags(payload.Tags)
	protoPayload.SetMetadata(protoMetadata)
	protoPayload.SetRequirements(protoRequirements)
	protoPayload.SetLockupOptions(protoLockupOptions)
	protoPayload.SetFeedUrl(payload.FeedURL)
	protoPayload.SetOwnerAddress(ownerAddressProto)

	protoRequest := &protocol.CreateMintRequest{}
	protoRequest.SetPayload(protoPayload)
	protoRequest.SetPublicKey(pubHex)
	protoRequest.SetSignature(signature)

	mintResponse, err := feClient.CreateMint(ctx, connect.NewRequest(protoRequest))
	if err != nil {
		t.Fatalf("Failed to create mint: %v", err)
	}

	mints, err := tokenisationStore.GetUnconfirmedMints(ctx, 0, 10)
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
