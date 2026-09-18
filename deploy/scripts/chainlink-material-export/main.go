// 本程序使用项目固定的 Chainlink 官方部署包装器生成受控 VRF、OCR2 和 Automation 合约材料。
package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	automationconfig "github.com/smartcontractkit/chainlink-automation/pkg/v2/config"
	"github.com/smartcontractkit/chainlink/devenv/contracts"
	"github.com/smartcontractkit/chainlink/devenv/products"
	legacyoffchainaggregator "github.com/smartcontractkit/libocr/gethwrappers/offchainaggregator"
	"github.com/smartcontractkit/libocr/gethwrappers2/ocr2aggregator"
	"github.com/smartcontractkit/libocr/offchainreporting2/confighelper"
	"github.com/smartcontractkit/libocr/offchainreporting2/reportingplugin/median"
	ocrtypes "github.com/smartcontractkit/libocr/offchainreporting2/types"
	ocrplusconfighelper "github.com/smartcontractkit/libocr/offchainreporting2plus/confighelper"
	ocrplustypes "github.com/smartcontractkit/libocr/offchainreporting2plus/types"
)

type material struct {
	SchemaVersion    int    `json:"schema_version"`
	Protocol         string `json:"protocol"`
	Version          string `json:"version"`
	ChainID          uint64 `json:"chain_id"`
	BlockhashStore   string `json:"blockhash_store"`
	BatchBlockhash   string `json:"batch_blockhash_store"`
	Coordinator      string `json:"coordinator"`
	BatchCoordinator string `json:"batch_coordinator"`
	LinkToken        string `json:"link_token"`
	LinkNativeFeed   string `json:"link_native_feed"`
	Consumer         string `json:"consumer"`
	SubscriptionID   string `json:"subscription_id"`
	VRFKeyHash       string `json:"vrf_key_hash"`
	VRFPublicKey     string `json:"vrf_public_key"`
}

type ocrNode struct {
	Transmitter           string `json:"transmitter"`
	P2PPeerID             string `json:"p2p_peer_id"`
	OCR2ConfigPublicKey   string `json:"ocr2_config_public_key"`
	OCR2OffchainPublicKey string `json:"ocr2_offchain_public_key"`
	OCR2OnchainPublicKey  string `json:"ocr2_onchain_public_key"`
}

type ocrMaterial struct {
	SchemaVersion int      `json:"schema_version"`
	Protocol      string   `json:"protocol"`
	Version       string   `json:"version"`
	ChainID       uint64   `json:"chain_id"`
	LinkToken     string   `json:"link_token"`
	Aggregator    string   `json:"aggregator"`
	SourceFeed    string   `json:"source_feed"`
	LinkETHFeed   string   `json:"link_eth_feed"`
	Transmitters  []string `json:"transmitters"`
	F             uint8    `json:"f"`
}

type automationMaterial struct {
	SchemaVersion int    `json:"schema_version"`
	Protocol      string `json:"protocol"`
	Version       string `json:"version"`
	ChainID       uint64 `json:"chain_id"`
	LinkToken     string `json:"link_token"`
	WETH          string `json:"weth"`
	LinkETHFeed   string `json:"link_eth_feed"`
	GasFeed       string `json:"gas_feed"`
	Transcoder    string `json:"transcoder"`
	Registry      string `json:"registry"`
	Registrar     string `json:"registrar"`
	Upkeep        string `json:"upkeep"`
	UpkeepID      string `json:"upkeep_id"`
	Transmitter   string `json:"transmitter"`
	F             uint8  `json:"f"`
}

func main() {
	rpcURL := flag.String("rpc", "", "EVM HTTP RPC")
	privateKey := flag.String("private-key", "", "受控部署账户私钥")
	chainID := flag.Uint64("chain-id", 0, "EVM chain id")
	publicKey := flag.String("vrf-public-key", "", "Chainlink 节点生成的未压缩 VRF 公钥")
	protocol := flag.String("protocol", "vrf", "受控协议类型: vrf、ocr2 或 automation")
	nodePath := flag.String("node-json", "", "OCR2 节点公开身份 JSON 路径")
	outPath := flag.String("out", "", "材料 JSON 输出路径")
	flag.Parse()
	if *protocol == "ocr2" {
		generateOCR2(*rpcURL, *privateKey, *chainID, *nodePath, *outPath)
		return
	}
	if *protocol == "automation" {
		generateAutomation(*rpcURL, *privateKey, *chainID, *nodePath, *outPath)
		return
	}
	if *protocol != "vrf" {
		fail(errors.New("-protocol must be vrf、ocr2 或 automation"))
	}

	for name, value := range map[string]string{
		"-rpc":            *rpcURL,
		"-private-key":    *privateKey,
		"-vrf-public-key": *publicKey,
		"-out":            *outPath,
	} {
		if strings.TrimSpace(value) == "" {
			fail(errors.New(name + " is required"))
		}
	}
	if *chainID == 0 {
		fail(errors.New("-chain-id must be positive"))
	}
	key := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(*privateKey), "0x"), "0X")
	if len(key) != 64 {
		fail(errors.New("-private-key must contain 32 bytes"))
	}
	for _, character := range key {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			fail(errors.New("-private-key must contain hexadecimal characters"))
		}
	}

	client, err := products.InitSeth(*rpcURL, []string{key}, chainID)
	if err != nil {
		fail(fmt.Errorf("initialize fixed Chainlink deployment client: %w", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 先部署官方 VRF 依赖，再绑定 coordinator 的正式配置。
	bhs, err := contracts.DeployBlockhashStore(client)
	if err != nil {
		fail(fmt.Errorf("deploy BlockhashStore: %w", err))
	}
	batchBHS, err := contracts.DeployBatchBlockhashStore(client, bhs.Address())
	if err != nil {
		fail(fmt.Errorf("deploy BatchBlockhashStore: %w", err))
	}
	coordinator, err := contracts.DeployVRFCoordinatorV2_5(client, bhs.Address())
	if err != nil {
		fail(fmt.Errorf("deploy VRFCoordinatorV2_5: %w", err))
	}
	batchCoordinator, err := contracts.DeployBatchVRFCoordinatorV2Plus(client, coordinator.Address())
	if err != nil {
		fail(fmt.Errorf("deploy BatchVRFCoordinatorV2Plus: %w", err))
	}
	linkToken, err := contracts.DeployLinkTokenContract(zerolog.Nop(), client)
	if err != nil {
		fail(fmt.Errorf("deploy LINK token: %w", err))
	}
	feed, err := contracts.DeployMockLINKETHFeed(client, big.NewInt(1e18))
	if err != nil {
		fail(fmt.Errorf("deploy LINK/native feed: %w", err))
	}
	if err := coordinator.SetLINKAndLINKNativeFeed(linkToken.Address(), feed.Address()); err != nil {
		fail(fmt.Errorf("configure LINK/native feed: %w", err))
	}
	if err := coordinator.SetConfig(1, 2500000, 86400, 33285, big.NewInt(1e16), 100, 0, 0, 0); err != nil {
		fail(fmt.Errorf("configure VRF coordinator: %w", err))
	}

	provingKey, err := contracts.EncodeOnChainVRFProvingKey(*publicKey)
	if err != nil {
		fail(fmt.Errorf("encode Chainlink VRF public key: %w", err))
	}
	if err := coordinator.RegisterProvingKey(provingKey, products.EtherToGwei(big.NewFloat(100)).Uint64()); err != nil {
		fail(fmt.Errorf("register Chainlink VRF proving key: %w", err))
	}
	keyHash, err := coordinator.HashOfKey(ctx, provingKey)
	if err != nil {
		fail(fmt.Errorf("read Chainlink VRF key hash: %w", err))
	}

	subscriptionTx, err := coordinator.CreateSubscription()
	if err != nil {
		fail(fmt.Errorf("create VRF subscription: %w", err))
	}
	receipt, err := client.Client.TransactionReceipt(ctx, subscriptionTx.Hash())
	if err != nil {
		fail(fmt.Errorf("read VRF subscription receipt: %w", err))
	}
	subscriptionID, err := contracts.FindSubscriptionID(receipt)
	if err != nil {
		fail(fmt.Errorf("parse VRF subscription id: %w", err))
	}
	consumer, err := contracts.DeployVRFv2PlusLoadTestConsumer(client, coordinator.Address())
	if err != nil {
		fail(fmt.Errorf("deploy VRF consumer: %w", err))
	}
	if err := coordinator.AddConsumer(subscriptionID, consumer.Address()); err != nil {
		fail(fmt.Errorf("authorize VRF consumer: %w", err))
	}
	if err := coordinator.FundSubscriptionWithNative(subscriptionID, big.NewInt(1e18)); err != nil {
		fail(fmt.Errorf("fund VRF subscription: %w", err))
	}
	stored, err := coordinator.GetSubscription(ctx, subscriptionID)
	if err != nil {
		fail(fmt.Errorf("read funded VRF subscription: %w", err))
	}
	if stored.NativeBalance.Sign() <= 0 || len(stored.Consumers) != 1 || stored.Consumers[0].Hex() != consumer.Address() {
		fail(errors.New("VRF subscription state does not contain funded authorized consumer"))
	}

	result := material{
		SchemaVersion: 1, Protocol: "chainlink-vrf", Version: "2.61.0", ChainID: *chainID,
		BlockhashStore: bhs.Address(), BatchBlockhash: batchBHS.Address(), Coordinator: coordinator.Address(),
		BatchCoordinator: batchCoordinator.Address(), LinkToken: linkToken.Address(), LinkNativeFeed: feed.Address(),
		Consumer: consumer.Address(), SubscriptionID: subscriptionID.String(), VRFKeyHash: fmt.Sprintf("0x%x", keyHash),
		VRFPublicKey: *publicKey,
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(fmt.Errorf("encode Chainlink VRF material: %w", err))
	}
	if err := os.WriteFile(*outPath, append(data, '\n'), 0o600); err != nil {
		fail(fmt.Errorf("write Chainlink VRF material: %w", err))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// generateOCR2 使用官方 OCR2 aggregator 和节点公开身份生成受控喂价材料。
func generateOCR2(rpcURL, privateKey string, chainID uint64, nodePath, outPath string) {
	if strings.TrimSpace(rpcURL) == "" || strings.TrimSpace(privateKey) == "" || strings.TrimSpace(nodePath) == "" || strings.TrimSpace(outPath) == "" {
		fail(errors.New("ocr2 requires -rpc, -private-key, -chain-id, -node-json and -out"))
	}
	key := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(privateKey), "0x"), "0X")
	if len(key) != 64 {
		fail(errors.New("-private-key must contain 32 bytes"))
	}
	for _, character := range key {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			fail(errors.New("-private-key must contain hexadecimal characters"))
		}
	}
	if chainID == 0 {
		fail(errors.New("-chain-id must be positive"))
	}
	nodeBytes, err := os.ReadFile(nodePath)
	if err != nil {
		fail(fmt.Errorf("read OCR2 node identity: %w", err))
	}
	var nodes []ocrNode
	if err := json.Unmarshal(nodeBytes, &nodes); err != nil {
		fail(fmt.Errorf("decode OCR2 node identities: %w", err))
	}
	if err := validateOCRNodes(nodes); err != nil {
		fail(err)
	}

	client, err := products.InitSeth(rpcURL, []string{key}, &chainID)
	if err != nil {
		fail(fmt.Errorf("initialize fixed Chainlink deployment client: %w", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	linkToken, err := contracts.DeployLinkTokenContract(zerolog.Nop(), client)
	if err != nil {
		fail(fmt.Errorf("deploy OCR2 LINK token: %w", err))
	}
	sourceFeed, err := contracts.DeployMockETHUSDFeed(client, big.NewInt(12345))
	if err != nil {
		fail(fmt.Errorf("deploy OCR2 controlled source feed: %w", err))
	}
	linkETHFeed, err := contracts.DeployMockLINKETHFeed(client, big.NewInt(1e18))
	if err != nil {
		fail(fmt.Errorf("deploy OCR2 controlled LINK/ETH feed: %w", err))
	}
	aggregatorAddress, aggregatorTx, aggregator, err := ocr2aggregator.DeployOCR2Aggregator(
		client.NewTXOpts(), client.Client, common.HexToAddress(linkToken.Address()), big.NewInt(1), big.NewInt(50000000000000000), common.Address{}, common.Address{}, 8, "chaimir-ocr2",
	)
	if err != nil {
		fail(fmt.Errorf("deploy official OCR2 aggregator: %w", err))
	}
	if _, err := bind.WaitDeployed(ctx, client.Client, aggregatorTx); err != nil {
		fail(fmt.Errorf("wait for OCR2 aggregator deployment: %w", err))
	}
	transmitterAddresses := make([]common.Address, 0, len(nodes))
	for _, node := range nodes {
		transmitterAddresses = append(transmitterAddresses, common.HexToAddress(node.Transmitter))
	}
	payees := make([]common.Address, len(transmitterAddresses))
	for index := range payees {
		payees[index] = client.MustGetRootKeyAddress()
	}
	if _, err := client.Decode(aggregator.SetPayees(client.NewTXOpts(), transmitterAddresses, payees)); err != nil {
		fail(fmt.Errorf("set OCR2 payee: %w", err))
	}
	if err := linkToken.Transfer(aggregatorAddress.Hex(), big.NewInt(1000000000000000000)); err != nil {
		fail(fmt.Errorf("fund OCR2 aggregator billing balance: %w", err))
	}

	identities := make([]confighelper.OracleIdentityExtra, 0, len(nodes))
	for _, node := range nodes {
		identity, identityErr := buildOCRIdentity(node)
		if identityErr != nil {
			fail(identityErr)
		}
		identities = append(identities, identity)
	}
	s := make([]int, len(identities))
	for index := range s {
		s[index] = 1
	}
	signerKeys, transmitterAccounts, f, _, offchainConfigVersion, offchainConfig, err := confighelper.ContractSetConfigArgsForTests(
		10*time.Second, 10*time.Second, 2500*time.Millisecond, 40*time.Millisecond, 15*time.Second, 3,
		s, identities,
		median.OffchainConfig{AlphaReportPPB: 1, AlphaAcceptPPB: 1, DeltaC: 30 * time.Second}.Encode(), nil,
		20*time.Millisecond, 1600*time.Millisecond, 800*time.Millisecond, 20*time.Millisecond, 20*time.Millisecond, 1, nil,
	)
	if err != nil {
		fail(fmt.Errorf("build OCR2 config: %w", err))
	}
	signers := make([]common.Address, 0, len(signerKeys))
	for _, signer := range signerKeys {
		signers = append(signers, common.BytesToAddress(signer))
	}
	transmitters := make([]common.Address, 0, len(transmitterAccounts))
	for _, account := range transmitterAccounts {
		transmitters = append(transmitters, common.HexToAddress(string(account)))
	}
	onchainConfig, err := (median.StandardOnchainConfigCodec{}).Encode(ctx, median.OnchainConfig{Min: big.NewInt(1), Max: big.NewInt(50000000000000000)})
	if err != nil {
		fail(fmt.Errorf("encode OCR2 onchain config: %w", err))
	}
	if _, err := client.Decode(aggregator.SetConfig(client.NewTXOpts(), signers, transmitters, f, onchainConfig, offchainConfigVersion, offchainConfig)); err != nil {
		fail(fmt.Errorf("set official OCR2 aggregator config: %w", err))
	}
	configDetails, err := aggregator.LatestConfigDetails(nil)
	if err != nil {
		fail(fmt.Errorf("read official OCR2 aggregator config after set: %w", err))
	}
	if configDetails.ConfigCount == 0 || configDetails.BlockNumber == 0 || configDetails.ConfigDigest == [32]byte{} {
		fail(fmt.Errorf("official OCR2 aggregator config was not persisted: count=%d block=%d digest=%x", configDetails.ConfigCount, configDetails.BlockNumber, configDetails.ConfigDigest))
	}
	// Chainlink 节点的 offchainreporting2 tracker 必须读取同一地址的官方 legacy wrapper 视图。
	legacyAggregator, err := legacyoffchainaggregator.NewOffchainAggregator(aggregatorAddress, client.Client)
	if err != nil {
		fail(fmt.Errorf("initialize Chainlink OCR2 tracker contract view: %w", err))
	}
	legacyConfigDetails, err := legacyAggregator.LatestConfigDetails(nil)
	if err != nil {
		fail(fmt.Errorf("read Chainlink OCR2 tracker config after set: %w", err))
	}
	if legacyConfigDetails.ConfigCount == 0 || legacyConfigDetails.BlockNumber == 0 || legacyConfigDetails.ConfigDigest == [16]byte{} {
		fail(fmt.Errorf("Chainlink OCR2 tracker cannot observe official aggregator config: count=%d block=%d digest=%x full_digest=%x", legacyConfigDetails.ConfigCount, legacyConfigDetails.BlockNumber, legacyConfigDetails.ConfigDigest, configDetails.ConfigDigest))
	}
	if len(signers) != len(nodes) || len(transmitters) != len(nodes) || f != 1 {
		fail(fmt.Errorf("OCR2 config did not produce the expected four-node configuration: nodes=%d signers=%d transmitters=%d f=%d", len(nodes), len(signers), len(transmitters), f))
	}
	transmitterStrings := make([]string, 0, len(transmitters))
	for _, transmitter := range transmitters {
		transmitterStrings = append(transmitterStrings, transmitter.Hex())
	}
	result := ocrMaterial{SchemaVersion: 1, Protocol: "chainlink-ocr2-median", Version: "2.61.0", ChainID: chainID, LinkToken: linkToken.Address(), Aggregator: aggregatorAddress.Hex(), SourceFeed: sourceFeed.Address(), LinkETHFeed: linkETHFeed.Address(), Transmitters: transmitterStrings, F: f}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(fmt.Errorf("encode OCR2 material: %w", err))
	}
	if err := os.WriteFile(outPath, append(data, '\n'), 0o600); err != nil {
		fail(fmt.Errorf("write OCR2 material: %w", err))
	}
}

func validateOCRNode(node ocrNode) error {
	if common.HexToAddress(node.Transmitter) == (common.Address{}) || !common.IsHexAddress(node.Transmitter) {
		return errors.New("OCR2 transmitter must be a non-zero EVM address")
	}
	if _, err := normalizeP2PPeerID(node.P2PPeerID); err != nil {
		return err
	}
	for name, value := range map[string]string{"config": node.OCR2ConfigPublicKey, "offchain": node.OCR2OffchainPublicKey, "onchain": node.OCR2OnchainPublicKey} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("OCR2 %s public key is required", name)
		}
	}
	return nil
}

// validateOCRNodes 校验四节点 OCR2/Automation 组合的身份数量和唯一性。
func validateOCRNodes(nodes []ocrNode) error {
	if len(nodes) != 4 {
		return fmt.Errorf("OCR2/Automation requires exactly four EVM nodes, got %d", len(nodes))
	}
	seenTransmitters := make(map[string]struct{}, len(nodes))
	seenPeers := make(map[string]struct{}, len(nodes))
	for index, node := range nodes {
		if err := validateOCRNode(node); err != nil {
			return fmt.Errorf("validate OCR2 node %d: %w", index+1, err)
		}
		transmitter := strings.ToLower(common.HexToAddress(node.Transmitter).Hex())
		if _, exists := seenTransmitters[transmitter]; exists {
			return fmt.Errorf("OCR2 transmitter is duplicated: %s", transmitter)
		}
		seenTransmitters[transmitter] = struct{}{}
		peerID, peerErr := normalizeP2PPeerID(node.P2PPeerID)
		if peerErr != nil {
			return fmt.Errorf("validate OCR2 node %d peer id: %w", index+1, peerErr)
		}
		if _, exists := seenPeers[peerID]; exists {
			return fmt.Errorf("OCR2 P2P peer id is duplicated: %s", node.P2PPeerID)
		}
		seenPeers[peerID] = struct{}{}
	}
	return nil
}

// generateAutomation 使用官方 Automation v2.0 合约、OCR2 配置和 upkeep 生成受控材料。
func generateAutomation(rpcURL, privateKey string, chainID uint64, nodePath, outPath string) {
	if strings.TrimSpace(rpcURL) == "" || strings.TrimSpace(privateKey) == "" || strings.TrimSpace(nodePath) == "" || strings.TrimSpace(outPath) == "" {
		fail(errors.New("automation requires -rpc, -private-key, -chain-id, -node-json and -out"))
	}
	key := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(privateKey), "0x"), "0X")
	if len(key) != 64 {
		fail(errors.New("-private-key must contain 32 bytes"))
	}
	for _, character := range key {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			fail(errors.New("-private-key must contain hexadecimal characters"))
		}
	}
	if chainID == 0 {
		fail(errors.New("-chain-id must be positive"))
	}
	nodeBytes, err := os.ReadFile(nodePath)
	if err != nil {
		fail(fmt.Errorf("read Automation node identity: %w", err))
	}
	var nodes []ocrNode
	if err := json.Unmarshal(nodeBytes, &nodes); err != nil {
		fail(fmt.Errorf("decode Automation node identities: %w", err))
	}
	if err := validateOCRNodes(nodes); err != nil {
		fail(err)
	}

	client, err := products.InitSeth(rpcURL, []string{key}, &chainID)
	if err != nil {
		fail(fmt.Errorf("initialize fixed Chainlink deployment client: %w", err))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 先部署官方 Automation 的计费依赖，再部署 registry 和 registrar。
	linkToken, err := contracts.DeployLinkTokenContract(zerolog.Nop(), client)
	if err != nil {
		fail(fmt.Errorf("deploy Automation LINK token: %w", err))
	}
	weth, err := contracts.DeployWETHTokenContract(zerolog.Nop(), client)
	if err != nil {
		fail(fmt.Errorf("deploy Automation WETH token: %w", err))
	}
	linkETHFeed, err := contracts.DeployMockLINKETHFeed(client, big.NewInt(1e18))
	if err != nil {
		fail(fmt.Errorf("deploy Automation LINK/ETH feed: %w", err))
	}
	gasFeed, err := contracts.DeployMockGASFeed(client, big.NewInt(2e11))
	if err != nil {
		fail(fmt.Errorf("deploy Automation gas feed: %w", err))
	}
	transcoder, err := contracts.DeployUpkeepTranscoder(client)
	if err != nil {
		fail(fmt.Errorf("deploy Automation upkeep transcoder: %w", err))
	}
	settings := contracts.KeeperRegistrySettings{
		PaymentPremiumPPB: 200000000, FlatFeeMicroLINK: 0, BlockCountPerTurn: big.NewInt(1),
		CheckGasLimit: 2500000, StalenessSeconds: big.NewInt(90000), GasCeilingMultiplier: 1,
		MinUpkeepSpend: big.NewInt(0), MaxPerformGas: 5000000, FallbackGasPrice: big.NewInt(2e11),
		FallbackLinkPrice: big.NewInt(2e18), FallbackNativePrice: big.NewInt(2e18),
		MaxCheckDataSize: 5000, MaxPerformDataSize: 5000, MaxRevertDataSize: 5000,
		RegistryVersion: contracts.RegistryVersion_2_0,
	}
	registry, err := contracts.DeployKeeperRegistry(client, &contracts.KeeperRegistryOpts{
		RegistryVersion: contracts.RegistryVersion_2_0, LinkAddr: linkToken.Address(),
		ETHFeedAddr: linkETHFeed.Address(), GasFeedAddr: gasFeed.Address(), TranscoderAddr: transcoder.Address(),
		Settings: settings, WrappedNativeAddr: weth.Address(),
	})
	if err != nil {
		fail(fmt.Errorf("deploy official Automation registry: %w", err))
	}
	registrar, err := contracts.DeployKeeperRegistrar(client, contracts.RegistryVersion_2_0, linkToken.Address(), contracts.KeeperRegistrarSettings{
		AutoApproveConfigType: 2, AutoApproveMaxAllowed: 65535, RegistryAddr: registry.Address(),
		MinLinkJuels: big.NewInt(0), WETHTokenAddr: weth.Address(),
	})
	if err != nil {
		fail(fmt.Errorf("deploy official Automation registrar: %w", err))
	}
	identities := make([]ocrplusconfighelper.OracleIdentityExtra, 0, len(nodes))
	for _, node := range nodes {
		identity, identityErr := buildAutomationOCRIdentity(node)
		if identityErr != nil {
			fail(identityErr)
		}
		identities = append(identities, identity)
	}
	offchain, err := json.Marshal(automationconfig.OffchainConfig{
		Version: "v2", PerformLockoutWindow: 300000, TargetProbability: "0.999", TargetInRounds: 1,
		SamplingJobDuration: 5000, MinConfirmations: 0, GasLimitPerReport: 5300000,
		GasOverheadPerUpkeep: 300000, MaxUpkeepBatchSize: 1, ReportBlockLag: 0, MercuryLookup: false,
	})
	if err != nil {
		fail(fmt.Errorf("encode Automation plugin config: %w", err))
	}
	onchain := settings.Encode20OnchainConfig(registrar.Address())
	s := make([]int, len(identities))
	for index := range s {
		s[index] = 1
	}
	signerKeys, transmitterKeys, f, _, offchainConfigVersion, offchainConfig, err := ocrplusconfighelper.ContractSetConfigArgsForTests(
		10*time.Second, 15*time.Second, 1*time.Second, 200*time.Millisecond, 30*time.Second, 24,
		s, identities, offchain, nil,
		20*time.Millisecond, 1600*time.Millisecond, 800*time.Millisecond, 20*time.Millisecond, 20*time.Millisecond, 1, onchain,
	)
	if err != nil {
		fail(fmt.Errorf("build Automation OCR2 config: %w", err))
	}
	if len(signerKeys) != len(nodes) || len(transmitterKeys) != len(nodes) || f != 1 {
		fail(fmt.Errorf("Automation OCR2 config is not the expected four-node configuration: nodes=%d signers=%d transmitters=%d f=%d", len(nodes), len(signerKeys), len(transmitterKeys), f))
	}
	signers := make([]common.Address, 0, len(signerKeys))
	for _, signer := range signerKeys {
		signers = append(signers, common.BytesToAddress(signer))
	}
	transmitters := make([]common.Address, 0, len(transmitterKeys))
	for _, transmitter := range transmitterKeys {
		transmitters = append(transmitters, common.HexToAddress(string(transmitter)))
	}
	if err := registry.SetConfig(settings, contracts.OCRv2Config{
		Signers: signers, Transmitters: transmitters, F: f,
		OnchainConfig: onchain, OffchainConfigVersion: offchainConfigVersion, OffchainConfig: offchainConfig,
	}); err != nil {
		fail(fmt.Errorf("set official Automation registry OCR2 config: %w", err))
	}

	// 官方 upkeep counter 负责真实 check/perform 语义，不以人工调用 performUpkeep 代替节点联动。
	upkeep, err := contracts.DeployUpkeepCounterFromKey(client, 0, big.NewInt(999999), big.NewInt(1))
	if err != nil {
		fail(fmt.Errorf("deploy official Automation upkeep counter: %w", err))
	}
	deployer := client.MustGetRootKeyAddress().Hex()
	linkAmount := big.NewInt(1e18)
	registration, err := registrar.EncodeRegisterRequest("chaimir-automation-upkeep", nil, upkeep.Address(), 500000, deployer, nil, linkAmount, 0, deployer, false, false, linkToken.Address())
	if err != nil {
		fail(fmt.Errorf("encode official Automation upkeep registration: %w", err))
	}
	registrationTx, err := linkToken.TransferAndCall(registrar.Address(), linkAmount, registration)
	if err != nil {
		fail(fmt.Errorf("register official Automation upkeep: %w", err))
	}
	registrationReceipt, err := client.Client.TransactionReceipt(ctx, registrationTx.Hash())
	if err != nil {
		fail(fmt.Errorf("read Automation upkeep registration receipt: %w", err))
	}
	var upkeepID *big.Int
	for _, log := range registrationReceipt.Logs {
		if log.Address.Hex() != registry.Address() {
			continue
		}
		candidate, parseErr := registry.ParseUpkeepIDFromRegisteredLog(log)
		if parseErr == nil {
			upkeepID = candidate
			break
		}
	}
	if upkeepID == nil || upkeepID.Sign() <= 0 {
		fail(errors.New("Automation upkeep registration receipt did not contain a valid upkeep id"))
	}
	upkeepInfo, err := registry.GetUpkeepInfo(ctx, upkeepID)
	if err != nil {
		fail(fmt.Errorf("read registered Automation upkeep: %w", err))
	}
	if !strings.EqualFold(upkeepInfo.Target, upkeep.Address()) || upkeepInfo.Balance.Cmp(linkAmount) < 0 {
		fail(fmt.Errorf("Automation upkeep state mismatch: target=%s balance=%s", upkeepInfo.Target, upkeepInfo.Balance.String()))
	}
	result := automationMaterial{
		SchemaVersion: 1, Protocol: "chainlink-automation-ocr2-v2.0", Version: "2.61.0", ChainID: chainID,
		LinkToken: linkToken.Address(), WETH: weth.Address(), LinkETHFeed: linkETHFeed.Address(), GasFeed: gasFeed.Address(),
		Transcoder: transcoder.Address(), Registry: registry.Address(), Registrar: registrar.Address(), Upkeep: upkeep.Address(),
		UpkeepID: upkeepID.String(), Transmitter: nodes[0].Transmitter, F: f,
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(fmt.Errorf("encode Automation material: %w", err))
	}
	if err := os.WriteFile(outPath, append(data, '\n'), 0o600); err != nil {
		fail(fmt.Errorf("write Automation material: %w", err))
	}
}

// buildAutomationOCRIdentity 将 Chainlink 节点公开密钥转换为官方 Automation 配置格式。
func buildAutomationOCRIdentity(node ocrNode) (ocrplusconfighelper.OracleIdentityExtra, error) {
	peerID, err := normalizeP2PPeerID(node.P2PPeerID)
	if err != nil {
		return ocrplusconfighelper.OracleIdentityExtra{}, err
	}
	offchain, err := decodeOCRKey(node.OCR2OffchainPublicKey, "ocr2off_evm_")
	if err != nil {
		return ocrplusconfighelper.OracleIdentityExtra{}, err
	}
	configKey, err := decodeOCRKey(node.OCR2ConfigPublicKey, "ocr2cfg_evm_")
	if err != nil {
		return ocrplusconfighelper.OracleIdentityExtra{}, err
	}
	onchain, err := decodeOCROnchainKey(node.OCR2OnchainPublicKey)
	if err != nil {
		return ocrplusconfighelper.OracleIdentityExtra{}, err
	}
	var offchainFixed [ed25519.PublicKeySize]byte
	var configFixed [ed25519.PublicKeySize]byte
	copy(offchainFixed[:], offchain)
	copy(configFixed[:], configKey)
	return ocrplusconfighelper.OracleIdentityExtra{
		OracleIdentity: ocrplusconfighelper.OracleIdentity{
			OffchainPublicKey: offchainFixed,
			OnchainPublicKey:  onchain,
			PeerID:            peerID,
			TransmitAccount:   ocrplustypes.Account(node.Transmitter),
		},
		ConfigEncryptionPublicKey: configFixed,
	}, nil
}

func buildOCRIdentity(node ocrNode) (confighelper.OracleIdentityExtra, error) {
	peerID, err := normalizeP2PPeerID(node.P2PPeerID)
	if err != nil {
		return confighelper.OracleIdentityExtra{}, err
	}
	offchain, err := decodeOCRKey(node.OCR2OffchainPublicKey, "ocr2off_evm_")
	if err != nil {
		return confighelper.OracleIdentityExtra{}, err
	}
	configKey, err := decodeOCRKey(node.OCR2ConfigPublicKey, "ocr2cfg_evm_")
	if err != nil {
		return confighelper.OracleIdentityExtra{}, err
	}
	onchain, err := decodeOCROnchainKey(node.OCR2OnchainPublicKey)
	if err != nil {
		return confighelper.OracleIdentityExtra{}, err
	}
	var offchainFixed [ed25519.PublicKeySize]byte
	var configFixed [ed25519.PublicKeySize]byte
	copy(offchainFixed[:], offchain)
	copy(configFixed[:], configKey)
	return confighelper.OracleIdentityExtra{
		OracleIdentity: confighelper.OracleIdentity{
			OnchainPublicKey:  onchain,
			OffchainPublicKey: offchainFixed,
			PeerID:            peerID,
			TransmitAccount:   ocrtypes.Account(node.Transmitter),
		},
		ConfigEncryptionPublicKey: configFixed,
	}, nil
}

func decodeOCRKey(value, prefix string) ([]byte, error) {
	encoded := strings.TrimPrefix(strings.TrimSpace(value), prefix)
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("OCR2 key %q must be 32-byte hexadecimal data", prefix)
	}
	return decoded, nil
}

// normalizeP2PPeerID 将 Chainlink API 的公开身份转换为官方 libocr 使用的原始 Base58 Peer ID。
func normalizeP2PPeerID(value string) (string, error) {
	const prefix = "p2p_"
	peerID := strings.TrimSpace(value)
	if !strings.HasPrefix(peerID, prefix) {
		return "", errors.New("OCR2 P2P peer id must use the Chainlink API p2p_ prefix")
	}
	peerID = strings.TrimPrefix(peerID, prefix)
	if peerID == "" {
		return "", errors.New("OCR2 P2P peer id is empty after removing the API prefix")
	}
	const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, character := range peerID {
		if !strings.ContainsRune(base58Alphabet, character) {
			return "", fmt.Errorf("OCR2 P2P peer id contains invalid Base58 character %q", character)
		}
	}
	return peerID, nil
}

// decodeOCROnchainKey 解析 EVM OCR2 的链上签名地址,该类型按官方协议为 20 字节。
func decodeOCROnchainKey(value string) ([]byte, error) {
	const prefix = "ocr2on_evm_"
	encoded := strings.TrimPrefix(strings.TrimSpace(value), prefix)
	decoded, err := hex.DecodeString(encoded)
	if err != nil || len(decoded) != 20 {
		return nil, fmt.Errorf("OCR2 key %q must be 20-byte hexadecimal data", prefix)
	}
	return decoded, nil
}
