package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/ethdb/leveldb"
	"github.com/ethereum/go-ethereum/ethdb/pebble"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/go-ethereum/triedb"
)

type historyManifest struct {
	SchemaVersion int         `json:"schema_version"`
	Engine        string      `json:"engine"`
	BlockNumber   string      `json:"block_number"`
	BlockHash     common.Hash `json:"block_hash"`
	StateRoot     common.Hash `json:"state_root"`
	ChainID       string      `json:"chain_id"`
	MaxBytes      uint64      `json:"max_bytes"`
}

type historyAPI struct {
	db      ethdb.Database
	trieDB  *triedb.Database
	stateDB state.Database
	head    *types.Block
	config  *params.ChainConfig
}

func parseManifest(root string) (historyManifest, error) {
	var m historyManifest
	f, err := os.Open(filepath.Join(root, "snapshot.json"))
	if err != nil {
		return m, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return m, err
	}
	if info.Size() > 8192 {
		return m, errors.New("history manifest exceeds size limit")
	}
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return m, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return m, errors.New("snapshot manifest has trailing data")
	}
	if m.SchemaVersion != 1 || (m.Engine != "leveldb" && m.Engine != "pebble") || m.BlockHash == (common.Hash{}) || m.StateRoot == (common.Hash{}) || m.MaxBytes == 0 || m.MaxBytes > 1<<40 {
		return m, errors.New("invalid history manifest")
	}
	number, err := strconv.ParseUint(m.BlockNumber, 10, 63)
	if err != nil || strconv.FormatUint(number, 10) != m.BlockNumber {
		return m, errors.New("invalid history block number")
	}
	n, ok := new(big.Int).SetString(m.ChainID, 10)
	if !ok || n.Sign() <= 0 || n.BitLen() > 256 || n.String() != m.ChainID {
		return m, errors.New("invalid history chain id")
	}
	return m, nil
}

func openHistory(root string, m historyManifest) (*historyAPI, error) {
	datadir := filepath.Join(root, "chaindata")
	if fi, err := os.Stat(datadir); err != nil || !fi.IsDir() {
		return nil, errors.New("history chaindata is missing")
	}
	var kv ethdb.KeyValueStore
	var err error
	if m.Engine == "leveldb" {
		kv, err = leveldb.New(datadir, 128, 128, "chaimir-history", true)
	} else {
		kv, err = pebble.New(datadir, 128, 128, "chaimir-history", true)
	}
	if err != nil {
		return nil, err
	}
	ancient := filepath.Join(datadir, "ancient")
	if _, err := os.Stat(ancient); os.IsNotExist(err) {
		ancient = ""
	} else if err != nil {
		return nil, errors.Join(err, kv.Close())
	}
	db, err := rawdb.Open(kv, rawdb.OpenOptions{Ancient: ancient, ReadOnly: true})
	if err != nil {
		return nil, errors.Join(err, kv.Close())
	}
	api, err := newHistoryAPI(db, m)
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return api, nil
}

func newHistoryAPI(db ethdb.Database, m historyManifest) (*historyAPI, error) {
	number, err := strconv.ParseUint(m.BlockNumber, 10, 63)
	if err != nil {
		return nil, err
	}
	hash := rawdb.ReadCanonicalHash(db, number)
	if hash != m.BlockHash {
		return nil, errors.New("history canonical block hash mismatch")
	}
	head, err := readHistoryBlock(db, hash, number)
	if err != nil {
		return nil, err
	}
	if head.Root() != m.StateRoot {
		return nil, errors.New("history block or state root mismatch")
	}
	genesis := rawdb.ReadCanonicalHash(db, 0)
	config := rawdb.ReadChainConfig(db, genesis)
	if config == nil || config.ChainID == nil || config.ChainID.String() != m.ChainID {
		return nil, errors.New("history chain identity mismatch")
	}
	if scheme := rawdb.ReadStateScheme(db); scheme != rawdb.HashScheme {
		return nil, errors.New("history snapshot requires hash state scheme")
	}
	tdb := triedb.NewDatabase(db, triedb.HashDefaults)
	api := &historyAPI{db: db, trieDB: tdb, stateDB: state.NewMPTDatabase(tdb, state.NewCodeDB(db)), head: head, config: config}
	if _, err := state.New(head.Root(), api.stateDB); err != nil {
		return nil, errors.Join(err, tdb.Close())
	}
	parent := head.Header()
	for index := uint64(0); index < 256 && parent.Number.Sign() > 0; index++ {
		n := parent.Number.Uint64() - 1
		block, err := readHistoryBlock(db, parent.ParentHash, n)
		if err != nil || rawdb.ReadCanonicalHash(db, n) != parent.ParentHash {
			return nil, errors.Join(errors.New("history ancestor chain is incomplete"), err, tdb.Close())
		}
		parent = block.Header()
	}
	return api, nil
}

func readHistoryBlock(db ethdb.Database, hash common.Hash, number uint64) (*types.Block, error) {
	block := rawdb.ReadBlock(db, hash, number)
	if block == nil {
		return nil, errors.New("history block is unavailable")
	}
	if !block.Number().IsUint64() || block.NumberU64() != number || block.Hash() != hash {
		return nil, errors.New("history block header identity mismatch")
	}
	header := block.Header()
	if types.DeriveSha(block.Transactions(), trie.NewStackTrie(nil)) != header.TxHash || types.CalcUncleHash(block.Uncles()) != header.UncleHash {
		return nil, errors.New("history block body commitment mismatch")
	}
	if header.WithdrawalsHash != nil {
		if block.Withdrawals() == nil || types.DeriveSha(block.Withdrawals(), trie.NewStackTrie(nil)) != *header.WithdrawalsHash {
			return nil, errors.New("history block withdrawals commitment mismatch")
		}
	} else if block.Withdrawals() != nil {
		return nil, errors.New("history block has unexpected withdrawals")
	}
	return block, nil
}

func (a *historyAPI) close() error { return errors.Join(a.trieDB.Close(), a.db.Close()) }

func verifyHistory(ctx context.Context, root string, m historyManifest, a *historyAPI) error {
	var size uint64
	if err := filepath.WalkDir(root, func(path string, item os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if item.Type()&os.ModeSymlink != 0 {
			return errors.New("history snapshot contains a symlink")
		}
		if item.IsDir() {
			return nil
		}
		if !item.Type().IsRegular() {
			return errors.New("history snapshot contains a special file")
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		if info.Size() < 0 || uint64(info.Size()) > m.MaxBytes-size {
			return errors.New("history snapshot exceeds declared capacity")
		}
		size += uint64(info.Size())
		return nil
	}); err != nil {
		return err
	}
	tree, err := trie.New(trie.StateTrieID(a.head.Root()), a.trieDB)
	if err != nil {
		return err
	}
	nodes, err := tree.NodeIterator(nil)
	if err != nil {
		return err
	}
	for nodes.Next(true) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := verifyNode(nodes); err != nil {
			return err
		}
		if !nodes.Leaf() {
			continue
		}
		var account types.StateAccount
		if err := rlp.DecodeBytes(nodes.LeafBlob(), &account); err != nil {
			return err
		}
		if len(account.CodeHash) != 32 {
			return errors.New("invalid account code hash")
		}
		if !bytes.Equal(account.CodeHash, types.EmptyCodeHash.Bytes()) {
			code := rawdb.ReadCode(a.db, common.BytesToHash(account.CodeHash))
			if len(code) == 0 || !bytes.Equal(crypto.Keccak256(code), account.CodeHash) {
				return errors.New("history contract code is missing or corrupt")
			}
		}
		storage, err := trie.New(trie.StorageTrieID(a.head.Root(), common.BytesToHash(nodes.LeafKey()), account.Root), a.trieDB)
		if err != nil {
			return err
		}
		slots, err := storage.NodeIterator(nil)
		if err != nil {
			return err
		}
		for slots.Next(true) {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := verifyNode(slots); err != nil {
				return err
			}
		}
		if err := slots.Error(); err != nil {
			return err
		}
	}
	return nodes.Error()
}

func verifyNode(nodes trie.NodeIterator) error {
	if hash := nodes.Hash(); hash != (common.Hash{}) && crypto.Keccak256Hash(nodes.NodeBlob()) != hash {
		return errors.New("history trie node hash mismatch")
	}
	return nodes.Error()
}

// ChainId 返回镜像内创世配置声明的链身份。
func (a *historyAPI) ChainId() *hexutil.Big {
	return (*hexutil.Big)(new(big.Int).Set(a.config.ChainID))
}

// BlockNumber 始终返回冻结区块，不推进历史库。
func (a *historyAPI) BlockNumber() hexutil.Uint64 { return hexutil.Uint64(a.head.NumberU64()) }

// GasPrice 提供固定区块的基础费，供 Anvil 初始化费用环境。
func (a *historyAPI) GasPrice() *hexutil.Big {
	fee := a.head.BaseFee()
	if fee == nil {
		fee = new(big.Int)
	}
	return (*hexutil.Big)(new(big.Int).Set(fee))
}

// GetBlockByNumber 读取冻结块及可用祖先；latest/safe/finalized 均指向冻结块。
func (a *historyAPI) GetBlockByNumber(number rpc.BlockNumber, full bool) (map[string]any, error) {
	n, err := a.blockNumber(number)
	if err != nil {
		return nil, err
	}
	block, err := readHistoryBlock(a.db, rawdb.ReadCanonicalHash(a.db, n), n)
	if err != nil {
		return nil, err
	}
	return ethapi.RPCMarshalBlock(block, true, full, a.config), nil
}

// GetBlockByHash 仅返回不晚于冻结点的 canonical 区块。
func (a *historyAPI) GetBlockByHash(hash common.Hash, full bool) (map[string]any, error) {
	n, ok := rawdb.ReadHeaderNumber(a.db, hash)
	if !ok || n > a.head.NumberU64() || rawdb.ReadCanonicalHash(a.db, n) != hash {
		return nil, errors.New("history block is unavailable")
	}
	return a.GetBlockByNumber(rpc.BlockNumber(n), full)
}
func (a *historyAPI) blockNumber(number rpc.BlockNumber) (uint64, error) {
	if number == rpc.LatestBlockNumber || number == rpc.SafeBlockNumber || number == rpc.FinalizedBlockNumber {
		return a.head.NumberU64(), nil
	}
	if number < 0 || uint64(number) > a.head.NumberU64() {
		return 0, errors.New("requested block is outside frozen history")
	}
	return uint64(number), nil
}
func (a *historyAPI) frozenState(block rpc.BlockNumberOrHash) (*state.StateDB, error) {
	if hash, ok := block.Hash(); ok {
		if hash != a.head.Hash() {
			return nil, errors.New("state is only available at the frozen block")
		}
	} else if number, ok := block.Number(); !ok {
		return nil, errors.New("block identity is required")
	} else if n, err := a.blockNumber(number); err != nil || n != a.head.NumberU64() {
		return nil, errors.New("state is only available at the frozen block")
	}
	return state.New(a.head.Root(), a.stateDB)
}

// GetBalance 只读冻结状态，缺失节点明确失败。
func (a *historyAPI) GetBalance(address common.Address, block rpc.BlockNumberOrHash) (*hexutil.Big, error) {
	s, err := a.frozenState(block)
	if err != nil {
		return nil, err
	}
	value := s.GetBalance(address)
	return (*hexutil.Big)(value.ToBig()), s.Error()
}

// GetTransactionCount 返回冻结状态中的账户 nonce。
func (a *historyAPI) GetTransactionCount(address common.Address, block rpc.BlockNumberOrHash) (hexutil.Uint64, error) {
	s, err := a.frozenState(block)
	if err != nil {
		return 0, err
	}
	value := s.GetNonce(address)
	return hexutil.Uint64(value), s.Error()
}

// GetCode 返回冻结状态中的原始合约字节码。
func (a *historyAPI) GetCode(address common.Address, block rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	s, err := a.frozenState(block)
	if err != nil {
		return nil, err
	}
	value := s.GetCode(address)
	return hexutil.Bytes(value), s.Error()
}

// GetStorageAt 按 Geth 约定解码最多 32 字节的十六进制槽地址，只读冻结状态。
func (a *historyAPI) GetStorageAt(address common.Address, key string, block rpc.BlockNumberOrHash) (hexutil.Bytes, error) {
	if strings.HasPrefix(key, "0x") || strings.HasPrefix(key, "0X") {
		key = key[2:]
	}
	if len(key) > common.HashLength*2 {
		return nil, errors.New("storage key exceeds 32 bytes")
	}
	if len(key)%2 != 0 {
		key = "0" + key
	}
	slot, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}
	s, err := a.frozenState(block)
	if err != nil {
		return nil, err
	}
	value := s.GetState(address, common.BytesToHash(slot))
	return hexutil.Bytes(value.Bytes()), s.Error()
}

func historyHandler(a *historyAPI) (*rpc.Server, error) {
	server := rpc.NewServer()
	server.SetHTTPBodyLimit(1 << 20)
	server.SetBatchLimits(32, 8<<20)
	if err := server.RegisterName("eth", a); err != nil {
		server.Stop()
		return nil, err
	}
	return server, nil
}

func runHistory() (result error) {
	root := flag.String("snapshot", "/usr/local/share/chaimir/history", "immutable history snapshot directory")
	verify := flag.Bool("verify", false, "verify the full account/storage/code closure without serving")
	check := flag.Bool("check", false, "check immutable history identity without serving")
	flag.Parse()
	if flag.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	m, err := parseManifest(*root)
	if err != nil {
		return err
	}
	a, err := openHistory(*root, m)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, a.close()) }()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *verify {
		return verifyHistory(ctx, *root, m, a)
	}
	if *check {
		return nil
	}
	expected := map[string]string{"CHAIMIR_EVM_HISTORY_BLOCK_NUMBER": m.BlockNumber, "CHAIMIR_EVM_HISTORY_BLOCK_HASH": m.BlockHash.Hex(), "CHAIMIR_EVM_HISTORY_CHAIN_ID": m.ChainID, "CHAIMIR_EVM_HISTORY_STATE_ROOT": m.StateRoot.Hex()}
	selected := false
	for name := range expected {
		if _, exists := os.LookupEnv(name); exists {
			selected = true
		}
	}
	if selected {
		for name, value := range expected {
			if os.Getenv(name) != value {
				return errors.New("requested history identity differs from immutable snapshot")
			}
		}
	}
	port, err := strconv.Atoi(os.Getenv("CHAIMIR_RUNTIME_RPC_PORT"))
	if err != nil || port < 1 || port > 65535 {
		return errors.New("invalid history RPC port")
	}
	handler, err := historyHandler(a)
	if err != nil {
		return err
	}
	defer handler.Stop()
	server := &http.Server{Addr: fmt.Sprintf("0.0.0.0:%d", port), Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 1 << 16}
	done := make(chan struct{})
	stopped := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
			defer stop()
			err := server.Shutdown(cleanup)
			if err != nil {
				err = errors.Join(err, server.Close())
			}
			stopped <- err
		case <-done:
			stopped <- nil
		}
	}()
	err = server.ListenAndServe()
	close(done)
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return errors.Join(err, <-stopped)
}
func main() {
	if err := runHistory(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
