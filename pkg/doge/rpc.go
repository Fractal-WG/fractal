package doge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/rpc"
	"sync/atomic"

	"dogecoin.org/fractal-engine/pkg/config"
)

type rpcRequest struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
	Id     uint64 `json:"id"`
}

type rpcResponse struct {
	Id     uint64           `json:"id"`
	Result *json.RawMessage `json:"result"`
	Error  any              `json:"error"`
}

type RpcClient struct {
	RpcClient *rpc.Client
	Config    *config.Config
	Id        atomic.Uint64
}

func NewRpcClient(config *config.Config) *RpcClient {
	return &RpcClient{
		Config: config,
	}
}

func (t *RpcClient) ListUnspent(ctx context.Context, address string) ([]UTXO, error) {
	res, err := t.Request(ctx, "listunspent", []any{0, 99999999, []string{address}})
	if err != nil {
		return []UTXO{}, err
	}

	var result []UTXO
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return []UTXO{}, err
	}

	return result, nil
}

func (t *RpcClient) Generate(ctx context.Context, n int) ([]string, error) {
	res, err := t.Request(ctx, "generate", []any{n})
	if err != nil {
		return []string{}, err
	}

	var result []string
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return []string{}, err
	}

	return result, nil
}

func (t *RpcClient) AddPeer(ctx context.Context, address string) error {
	_, err := t.Request(ctx, "addnode", []any{address, "add"})
	if err != nil && err.Error() != "json-rpc no result or error was returned" {
		return err
	}

	return nil
}

func (t *RpcClient) GetNewAddress(ctx context.Context) (string, error) {
	res, err := t.Request(ctx, "getnewaddress", []any{})
	if err != nil {
		return "", err
	}

	var result string
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (t *RpcClient) DumpPrivKey(ctx context.Context, address string) (string, error) {
	res, err := t.Request(ctx, "dumpprivkey", []any{address})
	if err != nil {
		return "", err
	}

	var result string
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (t *RpcClient) SendToAddress(ctx context.Context, address string, amount int64) (string, error) {
	res, err := t.Request(ctx, "sendtoaddress", []any{address, amount})
	if err != nil {
		return "", err
	}

	var result string
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (t *RpcClient) GetWalletInfo(ctx context.Context) (WalletInfo, error) {
	res, err := t.Request(ctx, "getwalletinfo", []any{})
	if err != nil {
		return WalletInfo{}, err
	}

	var result WalletInfo
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return WalletInfo{}, err
	}

	return result, nil
}

func (t *RpcClient) GetBestBlockHash(ctx context.Context) (string, error) {
	res, err := t.Request(ctx, "getbestblockhash", []any{})
	if err != nil {
		return "", err
	}

	var result string
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (t *RpcClient) GetBlockHex(ctx context.Context, blockHash string) (string, error) {
	res, err := t.Request(ctx, "getblock", []any{blockHash, 0})
	if err != nil {
		return "", err
	}

	var result string
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (t *RpcClient) GetBlock(ctx context.Context, blockHash string) (Block, error) {
	res, err := t.Request(ctx, "getblock", []any{blockHash, 1})
	if err != nil {
		return Block{}, err
	}

	var block Block
	err = json.Unmarshal(*res, &block)
	if err != nil {
		return Block{}, err
	}
	return block, nil
}

func (t *RpcClient) GetBlockWithTransactions(ctx context.Context, blockHash string) (BlockWithTransactions, error) {
	res, err := t.Request(ctx, "getblock", []any{blockHash, 2})
	if err != nil {
		return BlockWithTransactions{}, err
	}

	var block BlockWithTransactions
	err = json.Unmarshal(*res, &block)
	if err != nil {
		return BlockWithTransactions{}, err
	}

	return block, nil
}

func (t *RpcClient) GetBlockchainInfo(ctx context.Context) (BlockchainInfo, error) {
	res, err := t.Request(ctx, "getblockchaininfo", []any{})
	if err != nil {
		return BlockchainInfo{}, err
	}

	var result BlockchainInfo
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return BlockchainInfo{}, err
	}
	return result, nil
}

func (t *RpcClient) GetBlockCount(ctx context.Context) (int64, error) {
	res, err := t.Request(ctx, "getblockcount", []any{})
	if err != nil {
		return 0, err
	}

	var result int64
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return 0, err
	}

	return result, nil
}

// Doesn't seem to be implemented in Dogecoin Core
// func (t *RpcClient) GetBlockFilter(blockHash string, filterType string) (BlockFilter, error) {
// 	res, err := t.Request(ctx, "getblockfilter", []any{blockHash, filterType})
// 	if err != nil {
// 		return BlockFilter{}, err
// 	}

// 	var result BlockFilter
// 	err = json.Unmarshal(*res, &result)
// 	if err != nil {
// 		return BlockFilter{}, err
// 	}
// 	return result, nil
// }

func (t *RpcClient) GetBlockHash(ctx context.Context, blockHeight int) (string, error) {
	res, err := t.Request(ctx, "getblockhash", []any{blockHeight})
	if err != nil {
		return "", err
	}

	var result string
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (t *RpcClient) GetBlockHeaderHex(ctx context.Context, blockHash string) (string, error) {
	res, err := t.Request(ctx, "getblockheader", []any{blockHash, false})
	if err != nil {
		return "", err
	}

	var result string
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (t *RpcClient) GetBlockHeader(ctx context.Context, blockHash string) (BlockHeader, error) {
	res, err := t.Request(ctx, "getblockheader", []any{blockHash, true})
	if err != nil {
		return BlockHeader{}, err
	}

	var result BlockHeader
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return BlockHeader{}, err
	}

	return result, nil
}

func (t *RpcClient) GetBlockStatsFromHash(ctx context.Context, blockHash string, stats []string) (BlockStats, error) {
	res, err := t.Request(ctx, "getblockstats", []any{blockHash, stats})
	if err != nil {
		return BlockStats{}, err
	}

	var result BlockStats
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return BlockStats{}, err
	}

	return result, nil
}

func (t *RpcClient) GetChainTips(ctx context.Context) ([]ChainTip, error) {
	res, err := t.Request(ctx, "getchaintips", []any{})
	if err != nil {
		return []ChainTip{}, err
	}

	var result []ChainTip
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return []ChainTip{}, err
	}

	return result, nil
}

func (t *RpcClient) GetDifficulty(ctx context.Context) (float64, error) {
	res, err := t.Request(ctx, "getdifficulty", []any{})
	if err != nil {
		return 0, err
	}

	var result float64
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return 0, err
	}

	return result, nil
}

// func (t *RpcClient) GetMempoolAncestorsHexs(txId string) ([]string, error) {
// 	res, err := t.Request(ctx, "getmempoolancestors", []any{txId})
// 	if err != nil {
// 		return []string{}, err
// 	}

// 	var result []string
// 	err = json.Unmarshal(*res, &result)
// 	if err != nil {
// 		return []string{}, err
// 	}

// 	return result, nil
// }

// func (t *RpcClient) GetMempoolAncestors(txId string) (MempoolAncestors, error) {
// 	res, err := t.Request(ctx, "getmempoolancestors", []any{txId})
// 	if err != nil {
// 		return MempoolAncestors{}, err
// 	}

// 	var result MempoolAncestors
// 	err = json.Unmarshal(*res, &result)
// 	if err != nil {
// 		return MempoolAncestors{}, err
// 	}

// 	return result, nil
// }

// Doesnt seem to be implemented in Dogecoin Core
// func (t *RpcClient) GetChainTxStats(nBlocks int, blockHash string) (ChainTxStats, error) {
// 	res, err := t.Request(ctx, "getchaintxstats", []any{nBlocks, blockHash})
// 	if err != nil {
// 		return ChainTxStats{}, err
// 	}

// 	var result ChainTxStats
// 	err = json.Unmarshal(*res, &result)
// 	if err != nil {
// 		return ChainTxStats{}, err
// 	}

// 	return result, nil
// }

// Doesnt seem like the height version of this function is implemented in Dogecoin Core
// func (t *RpcClient) GetBlockStatsFromHeight(blockHeight int, stats []string) (BlockStats, error) {
// 	res, err := t.Request(ctx, "getblockstats", []any{blockHeight, stats})
// 	if err != nil {
// 		return BlockStats{}, err
// 	}

// 	var result BlockStats
// 	err = json.Unmarshal(*res, &result)
// 	if err != nil {
// 		return BlockStats{}, err
// 	}

// 	return result, nil
// }

func (t *RpcClient) GetMempoolInfo(ctx context.Context) (MempoolInfo, error) {
	res, err := t.Request(ctx, "getmempoolinfo", []any{})
	if err != nil {
		return MempoolInfo{}, err
	}

	var result MempoolInfo
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return MempoolInfo{}, err
	}

	return result, nil
}

func (t *RpcClient) GetRawMempoolInfo(ctx context.Context) (RawMempoolInfo, error) {
	res, err := t.Request(ctx, "getrawmempool", []any{})
	if err != nil {
		return RawMempoolInfo{}, err
	}

	var result RawMempoolInfo
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return RawMempoolInfo{}, err
	}

	return result, nil
}

func (t *RpcClient) GetTxOut(ctx context.Context, txId string, vout int) (TxOut, error) {
	res, err := t.Request(ctx, "gettxout", []any{txId, vout})
	if err != nil {
		return TxOut{}, err
	}

	var result TxOut
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return TxOut{}, err
	}

	return result, nil
}

func (t *RpcClient) GetTxOutProof(ctx context.Context, txIds []string, blockHash string) ([]string, error) {
	res, err := t.Request(ctx, "gettxoutproof", []any{txIds, blockHash})
	if err != nil {
		return []string{}, err
	}

	var result []string
	err = json.Unmarshal(*res, &result)
	if err != nil {
		return []string{}, err
	}

	return result, nil
}

func (t *RpcClient) Request(ctx context.Context, method string, params []any) (*json.RawMessage, error) {
	id := t.Id.Add(1)

	body := rpcRequest{
		Method: method,
		Params: params,
		Id:     id,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("json-rpc marshal request: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", t.Config.DogeScheme+"://"+t.Config.DogeHost+":"+t.Config.DogePort, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("json-rpc request: %v", err)
	}

	if t.Config.DogeUser != "" {
		req.SetBasicAuth(t.Config.DogeUser, t.Config.DogePassword)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("json-rpc transport: %v", err)
	}
	// we MUST read all of res.Body and call res.Close,
	// otherwise the underlying connection cannot be re-used.
	defer res.Body.Close()
	res_bytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("json-rpc read response: %v", err)
	}
	// check for error response
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("json-rpc error status: %v | %v", res.StatusCode, string(res_bytes))
	}
	// cannot use json.NewDecoder: "The decoder introduces its own buffering
	// and may read data from r beyond the JSON values requested."
	var rpcres rpcResponse
	err = json.Unmarshal(res_bytes, &rpcres)
	if err != nil {
		return nil, fmt.Errorf("json-rpc unmarshal response: %v | %v", err, string(res_bytes))
	}
	if rpcres.Id != body.Id {
		return nil, fmt.Errorf("json-rpc wrong ID returned: %v vs %v", rpcres.Id, body.Id)
	}
	if rpcres.Error != nil {
		enc, err := json.Marshal(rpcres.Error)
		if err == nil {
			return nil, fmt.Errorf("json-rpc: error from Core Node: %v", string(enc))
		} else {
			return nil, fmt.Errorf("json-rpc: error from Core Node: %v", rpcres.Error)
		}
	}
	if rpcres.Result == nil {
		return nil, fmt.Errorf("json-rpc no result or error was returned")
	}

	return rpcres.Result, nil
}
