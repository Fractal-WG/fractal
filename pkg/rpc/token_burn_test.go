package rpc_test

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	"dogecoin.org/fractal-engine/pkg/doge"
	"dogecoin.org/fractal-engine/pkg/rpc"
	"dogecoin.org/fractal-engine/pkg/rpc/protocol"
	"dogecoin.org/fractal-engine/pkg/store"
	"gotest.tools/assert"
)

// buildBurnRequest signs and assembles a BurnTokensRequest proto.
func buildBurnRequest(t *testing.T, privHex, pubHex, mintHash string, quantity int) *connect.Request[protocol.BurnTokensRequest] {
	t.Helper()

	payload := rpc.BurnTokensRequestPayload{
		MintHash:     mintHash,
		BurnQuantity: quantity,
	}
	signature, err := doge.SignPayload(payload, privHex, pubHex)
	assert.NilError(t, err)

	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)

	protoPayload := &protocol.BurnTokensRequestPayload{}
	protoPayload.SetMintHash(mintHashProto)
	protoPayload.SetBurnQuantity(int32(quantity))

	req := &protocol.BurnTokensRequest{}
	req.SetPayload(protoPayload)
	req.SetPublicKey(pubHex)
	req.SetSignature(signature)

	return connect.NewRequest(req)
}

// setupBurnableMint saves a confirmed burnable mint and optionally seeds a token balance.
func setupBurnableMint(t *testing.T, s *store.TokenisationStore, burnAuthority store.BurnAuthorityType, ownerAddress, ownerPubKey string, fractionCount int) string {
	t.Helper()

	mintHash, err := (&store.MintWithoutID{
		Title:           "Burn Test Mint",
		FractionCount:   fractionCount,
		Description:     "For burn tests",
		Tags:            store.StringArray{},
		Metadata:        store.StringInterfaceMap{},
		Requirements:    store.StringInterfaceMap{},
		LockupOptions:   store.StringInterfaceMap{},
		PublicKey:       ownerPubKey,
		TransactionHash: "confirmedTxHash",
		Burnable:        true,
		BurnAuthority:   burnAuthority,
	}).GenerateHash()
	assert.NilError(t, err)

	mint := &store.MintWithoutID{
		Hash:            mintHash,
		Title:           "Burn Test Mint",
		FractionCount:   fractionCount,
		Description:     "For burn tests",
		Tags:            store.StringArray{},
		Metadata:        store.StringInterfaceMap{},
		Requirements:    store.StringInterfaceMap{},
		LockupOptions:   store.StringInterfaceMap{},
		PublicKey:       ownerPubKey,
		TransactionHash: "confirmedTxHash",
		Burnable:        true,
		BurnAuthority:   burnAuthority,
	}
	_, err = s.SaveMint(context.Background(), mint, ownerAddress)
	assert.NilError(t, err)

	return mintHash
}

// --- owner_only authority ---

func TestBurnTokens_OwnerOnly_OwnerCanBurnOwnTokens(t *testing.T) {
	s, gossip, feClient := SetupRpcTest(t)
	ctx := context.Background()

	privHex, pubHex, ownerAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	mintHash := setupBurnableMint(t, s, store.BurnAuthorityOwnerOnly, ownerAddr, pubHex, 1000)
	err = s.UpsertTokenBalance(ctx, ownerAddr, mintHash, 1000)
	assert.NilError(t, err)

	resp, err := feClient.BurnTokens(ctx, buildBurnRequest(t, privHex, pubHex, mintHash, 100))
	assert.NilError(t, err)
	assert.Assert(t, resp.Msg.GetBurnHash().GetValue() != "")
	assert.Assert(t, resp.Msg.GetEncodedTransactionBody() != "")

	// gossip should have received the burn
	assert.Equal(t, len(gossip.tokenBurns), 1)
	assert.Equal(t, gossip.tokenBurns[0].BurnerAddress, ownerAddr)
	assert.Equal(t, gossip.tokenBurns[0].MintHash, mintHash)
	assert.Equal(t, gossip.tokenBurns[0].BurnQuantity, 100)
}

func TestBurnTokens_OwnerOnly_NonOwnerRejected(t *testing.T) {
	s, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	_, ownerPubHex, ownerAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	holderPrivHex, holderPubHex, holderAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	mintHash := setupBurnableMint(t, s, store.BurnAuthorityOwnerOnly, ownerAddr, ownerPubHex, 1000)
	// give the holder a real balance — but they're not the owner
	err = s.UpsertTokenBalance(ctx, holderAddr, mintHash, 500)
	assert.NilError(t, err)

	_, err = feClient.BurnTokens(ctx, buildBurnRequest(t, holderPrivHex, holderPubHex, mintHash, 100))
	assert.Assert(t, err != nil, "non-owner should be rejected with owner_only authority")
}

// --- holder_only authority ---

func TestBurnTokens_HolderOnly_HolderCanBurnOwnTokens(t *testing.T) {
	s, gossip, feClient := SetupRpcTest(t)
	ctx := context.Background()

	_, ownerPubHex, ownerAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	holderPrivHex, holderPubHex, holderAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	mintHash := setupBurnableMint(t, s, store.BurnAuthorityHolderOnly, ownerAddr, ownerPubHex, 1000)
	err = s.UpsertTokenBalance(ctx, holderAddr, mintHash, 500)
	assert.NilError(t, err)

	resp, err := feClient.BurnTokens(ctx, buildBurnRequest(t, holderPrivHex, holderPubHex, mintHash, 200))
	assert.NilError(t, err)
	assert.Assert(t, resp.Msg.GetBurnHash().GetValue() != "")

	assert.Equal(t, len(gossip.tokenBurns), 1)
	// burn is against the holder's own address, not the owner's
	assert.Equal(t, gossip.tokenBurns[0].BurnerAddress, holderAddr)
}

func TestBurnTokens_HolderOnly_ZeroBalanceRejected(t *testing.T) {
	s, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	_, ownerPubHex, ownerAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	// person with no tokens at all
	noTokensPrivHex, noTokensPubHex, _, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	mintHash := setupBurnableMint(t, s, store.BurnAuthorityHolderOnly, ownerAddr, ownerPubHex, 1000)
	// do NOT seed any balance for noTokensAddr

	_, err = feClient.BurnTokens(ctx, buildBurnRequest(t, noTokensPrivHex, noTokensPubHex, mintHash, 1))
	assert.Assert(t, err != nil, "address with no tokens should be rejected")
}

func TestBurnTokens_HolderOnly_CannotBurnMoreThanOwned(t *testing.T) {
	s, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	_, ownerPubHex, ownerAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	holderPrivHex, holderPubHex, holderAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	mintHash := setupBurnableMint(t, s, store.BurnAuthorityHolderOnly, ownerAddr, ownerPubHex, 1000)
	err = s.UpsertTokenBalance(ctx, holderAddr, mintHash, 50)
	assert.NilError(t, err)

	// try to burn 51 when only holding 50
	_, err = feClient.BurnTokens(ctx, buildBurnRequest(t, holderPrivHex, holderPubHex, mintHash, 51))
	assert.Assert(t, err != nil, "should not be able to burn more than owned balance")
}

// --- owner_or_holder authority ---

func TestBurnTokens_OwnerOrHolder_OwnerCanBurnOwnTokens(t *testing.T) {
	s, gossip, feClient := SetupRpcTest(t)
	ctx := context.Background()

	privHex, pubHex, ownerAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	mintHash := setupBurnableMint(t, s, store.BurnAuthorityOwnerOrHolder, ownerAddr, pubHex, 1000)
	err = s.UpsertTokenBalance(ctx, ownerAddr, mintHash, 1000)
	assert.NilError(t, err)

	resp, err := feClient.BurnTokens(ctx, buildBurnRequest(t, privHex, pubHex, mintHash, 300))
	assert.NilError(t, err)
	assert.Assert(t, resp.Msg.GetBurnHash().GetValue() != "")
	assert.Equal(t, gossip.tokenBurns[0].BurnerAddress, ownerAddr)
}

func TestBurnTokens_OwnerOrHolder_HolderCanBurnOwnTokens(t *testing.T) {
	s, gossip, feClient := SetupRpcTest(t)
	ctx := context.Background()

	_, ownerPubHex, ownerAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	holderPrivHex, holderPubHex, holderAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	mintHash := setupBurnableMint(t, s, store.BurnAuthorityOwnerOrHolder, ownerAddr, ownerPubHex, 1000)
	err = s.UpsertTokenBalance(ctx, holderAddr, mintHash, 400)
	assert.NilError(t, err)

	resp, err := feClient.BurnTokens(ctx, buildBurnRequest(t, holderPrivHex, holderPubHex, mintHash, 100))
	assert.NilError(t, err)
	assert.Assert(t, resp.Msg.GetBurnHash().GetValue() != "")
	// burn targets the holder's own address
	assert.Equal(t, gossip.tokenBurns[0].BurnerAddress, holderAddr)
}

// --- non-burnable mint ---

func TestBurnTokens_NotBurnable_Rejected(t *testing.T) {
	s, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	privHex, pubHex, ownerAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	// mint with Burnable=false
	mint := &store.MintWithoutID{
		Hash:            "nonburnablehash0000000000000000000000000000000000000000000000000000",
		Title:           "Non-Burnable Mint",
		FractionCount:   500,
		Description:     "Cannot be burned",
		Tags:            store.StringArray{},
		Metadata:        store.StringInterfaceMap{},
		Requirements:    store.StringInterfaceMap{},
		LockupOptions:   store.StringInterfaceMap{},
		PublicKey:       pubHex,
		TransactionHash: "txHash",
		Burnable:        false,
	}
	_, err = s.SaveMint(ctx, mint, ownerAddr)
	assert.NilError(t, err)
	err = s.UpsertTokenBalance(ctx, ownerAddr, mint.Hash, 500)
	assert.NilError(t, err)

	_, err = feClient.BurnTokens(ctx, buildBurnRequest(t, privHex, pubHex, mint.Hash, 100))
	assert.Assert(t, err != nil, "burn should be rejected when mint is not burnable")
}

// --- signature validation ---

func TestBurnTokens_InvalidSignature_Rejected(t *testing.T) {
	s, _, feClient := SetupRpcTest(t)
	ctx := context.Background()

	_, pubHex, ownerAddr, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	// generate a different key to produce a mismatched signature
	wrongPrivHex, _, _, err := doge.GenerateDogecoinKeypair(doge.PrefixRegtest)
	assert.NilError(t, err)

	mintHash := setupBurnableMint(t, s, store.BurnAuthorityHolderOnly, ownerAddr, pubHex, 1000)
	err = s.UpsertTokenBalance(ctx, ownerAddr, mintHash, 1000)
	assert.NilError(t, err)

	// sign with wrong private key but present the correct public key
	payload := rpc.BurnTokensRequestPayload{MintHash: mintHash, BurnQuantity: 100}
	badSig, err := doge.SignPayload(payload, wrongPrivHex, pubHex)
	assert.NilError(t, err)

	mintHashProto := &protocol.Hash{}
	mintHashProto.SetValue(mintHash)
	protoPayload := &protocol.BurnTokensRequestPayload{}
	protoPayload.SetMintHash(mintHashProto)
	protoPayload.SetBurnQuantity(100)
	req := &protocol.BurnTokensRequest{}
	req.SetPayload(protoPayload)
	req.SetPublicKey(pubHex)
	req.SetSignature(badSig)

	_, err = feClient.BurnTokens(ctx, connect.NewRequest(req))
	assert.Assert(t, err != nil, "mismatched signature should be rejected")
}
