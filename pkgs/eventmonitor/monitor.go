package eventmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	rpchelper "github.com/powerloom/go-rpc-helper"
	rediskeys "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/vpa"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
)

// EventMonitor watches for EpochReleased and PrioritiesAssigned events and manages submission windows
type EventMonitor struct {
	rpcHelper    *rpchelper.RPCHelper
	redisClient  *redis.Client
	contractAddr common.Address
	contractABI  *ContractABI

	// VPA Contract Monitoring
	vpaContractAddr common.Address
	vpaContractABI  *ContractABI
	vpaClient       *vpa.PriorityCachingClient
	vpaEnabled      bool

	// Window management
	windowManager       *WindowManager
	windowConfigFetcher *WindowConfigFetcher // Fetches window config from contract

	// Event tracking
	lastProcessedBlock uint64
	eventChan          chan *EpochReleasedEvent
	epochReleasedSig   common.Hash // Cache the event signature to avoid recomputing

	// VPA Event tracking
	vpaEventChan          chan *PrioritiesAssignedEvent
	prioritiesAssignedSig common.Hash // Cache VPA event signature
	lastProcessedVPABlock uint64      // Separate block tracking for VPA contract

	// Configuration
	pollInterval         time.Duration
	windowDuration       time.Duration // Fallback default if contract fetch fails
	dataMarkets          []string      // List of data market addresses to monitor
	estimatedMaxPriority int           // Estimated max priority for total window calculation (default: 10)

	// Spam protection components (optional)
	spamComponents interface{} // *spam.SpamComponents - using interface{} to avoid circular import

	ctx    context.Context
	cancel context.CancelFunc
}

// EpochReleasedEvent represents an epoch release from the protocol state contract
type EpochReleasedEvent struct {
	EpochID           *big.Int
	EpochEnd          *big.Int
	DataMarketAddress common.Address
	Timestamp         uint64
	TransactionHash   common.Hash
	BlockNumber       uint64
}

// PrioritiesAssignedEvent represents a priority assignment from the VPA contract
type PrioritiesAssignedEvent struct {
	EpochID         *big.Int
	Seed            *big.Int
	Timestamp       uint64
	ValidatorCount  *big.Int
	DataMarket      common.Address
	TransactionHash common.Hash
	BlockNumber     uint64
}

// WindowManager manages submission windows for epochs
type WindowManager struct {
	activeWindows         map[string]*EpochWindow // key: dataMarketAddress:epochID
	mu                    sync.RWMutex
	windowSemaphore       chan struct{}
	maxWindows            int
	redisClient           *redis.Client
	protocolState         string                           // Protocol state contract address for namespacing
	finalizationBatchSize int                              // Number of projects per finalization batch
	keyBuilders           map[string]*rediskeys.KeyBuilder // Cache key builders per data market
}

// EpochWindow represents an active submission window
type EpochWindow struct {
	EpochID           *big.Int
	DataMarketAddress string
	StartTime         time.Time
	WindowDuration    time.Duration
	Timer             *time.Timer
	Done              chan struct{}
	StartBlockNum     uint64
	EndBlockNum       uint64
}

// Config for EventMonitor
type Config struct {
	RPCHelper             *rpchelper.RPCHelper
	ContractAddress       string
	ContractABIPath       string // Path to the contract ABI JSON file
	RedisClient           *redis.Client
	WindowDuration        time.Duration // Default window duration
	StartBlock            uint64
	PollInterval          time.Duration
	DataMarkets           []string // Data market addresses to monitor
	MaxWindows            int      // Max concurrent submission windows
	FinalizationBatchSize int      // Number of projects per finalization batch

	// VPA Configuration (optional)
	VPAContractAddress  string // VPA contract address for priority monitoring
	VPAContractABIPath  string // Path to VPA contract ABI JSON file
	VPAValidatorAddress string // This validator's address for VPA client
	VPARPCURL           string // RPC URL for VPA client (if different from main RPC)
	ProtocolState       string // Protocol state contract address for VPA integration, window config fetching, and namespacing

	// Window Config Configuration
	WindowConfigCacheTTL time.Duration // Cache TTL for window configs (default: 5 minutes)
	EstimatedMaxPriority int           // Estimated max priority for total window calculation (default: 10)

	// Spam protection components (optional)
	SpamComponents interface{} // *spam.SpamComponents - using interface{} to avoid circular import
}

// NewEventMonitor creates a new event monitor
func NewEventMonitor(cfg *Config) (*EventMonitor, error) {
	if cfg.RPCHelper == nil {
		return nil, fmt.Errorf("RPC helper is required")
	}

	// Load the contract ABI
	var contractABI *ContractABI
	if cfg.ContractABIPath != "" {
		abi, err := LoadContractABI(cfg.ContractABIPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load contract ABI: %w", err)
		}
		contractABI = abi
		log.Infof("✅ Loaded contract ABI from %s", cfg.ContractABIPath)

		// Verify the ABI has the EpochReleased event
		if !contractABI.HasEvent("EpochReleased") {
			return nil, fmt.Errorf("ABI does not contain EpochReleased event")
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	// If StartBlock is 0, fetch current block to start from latest
	startBlock := cfg.StartBlock
	if startBlock == 0 {
		currentBlock, err := cfg.RPCHelper.BlockNumber(context.Background())
		if err != nil {
			log.Warnf("Failed to get current block for start, using 0: %v", err)
		} else {
			// Start from current block minus 1 (like legacy collector)
			if currentBlock > 0 {
				startBlock = currentBlock - 1
			}
			log.Infof("Starting event monitor from current block: %d", startBlock)
		}
	}

	// Compute and cache event signature once at startup
	var epochReleasedSig common.Hash
	if contractABI != nil {
		sig, err := contractABI.GetEventHash("EpochReleased")
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to get EpochReleased event hash from ABI: %w", err)
		}
		epochReleasedSig = sig
		log.Infof("✅ Using ABI-derived EpochReleased event signature: %s", epochReleasedSig.Hex())
	} else {
		// Fallback to hardcoded signature for backward compatibility
		epochReleasedSig = getEpochReleasedEventSignature()
		log.Infof("✅ Using hardcoded EpochReleased event signature: %s", epochReleasedSig.Hex())
	}

	// Initialize VPA components if configured
	var vpaContractAddr common.Address
	var vpaContractABI *ContractABI
	var vpaClient *vpa.PriorityCachingClient
	var prioritiesAssignedSig common.Hash
	var vpaEnabled bool

	if cfg.VPAValidatorAddress != "" && cfg.ProtocolState != "" {
		vpaEnabled = true

		// Fetch VPA address from ProtocolState contract
		if cfg.ProtocolState != "" {
			log.Infof("🔍 Fetching VPA address from ProtocolState contract...")

			// Parse RPC URL from VPARPCURL (POWERLOOM_RPC_NODES can be comma-separated or JSON array)
			var rpcURL string
			if strings.Contains(cfg.VPARPCURL, ",") {
				urls := strings.Split(cfg.VPARPCURL, ",")
				rpcURL = strings.TrimSpace(urls[0])
			} else if strings.HasPrefix(cfg.VPARPCURL, "[") {
				var urls []string
				if err := json.Unmarshal([]byte(cfg.VPARPCURL), &urls); err != nil {
					log.Warnf("⚠️  Failed to parse VPARPCURL as JSON: %v", err)
					rpcURL = cfg.VPARPCURL
				} else if len(urls) > 0 {
					rpcURL = urls[0]
				} else {
					log.Warnf("⚠️  Empty VPARPCURL array")
					cancel()
					return nil, fmt.Errorf("empty VPARPCURL array")
				}
			} else {
				rpcURL = cfg.VPARPCURL
			}

			// Use shared VPA fetching function
			fetchedVPAAddress, err := vpa.FetchVPAAddress(rpcURL, cfg.ProtocolState)
			if err != nil {
				log.Warnf("⚠️  Failed to fetch VPA address: %v", err)
				vpaContractAddr = common.Address{}
			} else {
				vpaContractAddr = fetchedVPAAddress
				log.Infof("✅ Successfully fetched VPA address from ProtocolState: %s", vpaContractAddr.Hex())
			}
		}
	}

	// Initialize VPA client after we have the VPA address
	if vpaContractAddr != (common.Address{}) && len(cfg.DataMarkets) > 0 {
		if cfg.VPARPCURL == "" {
			log.Warn("VPA RPC URL not configured, VPA client will not be initialized")
			vpaEnabled = false
		} else {
			var err error
			vpaClient, err = vpa.NewPriorityCachingClient(
				cfg.VPARPCURL,
				vpaContractAddr.Hex(),
				cfg.VPAValidatorAddress,
				cfg.RedisClient,
				cfg.ProtocolState,
				cfg.DataMarkets[0], // Use first data market as default
				cfg.ProtocolState,
			)
			if err != nil {
				cancel()
				return nil, fmt.Errorf("failed to create VPA caching client: %w", err)
			}
			log.Infof("✅ Initialized VPA caching client for validator %s", cfg.VPAValidatorAddress)
		}
	} else {
		log.Info("VPA monitoring disabled - no VPA contract address fetched or no data markets configured")
	}

	// Load VPA contract ABI if path provided
	if cfg.VPAContractABIPath != "" {
		abi, err := LoadContractABI(cfg.VPAContractABIPath)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to load VPA contract ABI: %w", err)
		}
		vpaContractABI = abi
		log.Infof("✅ Loaded VPA contract ABI from %s", cfg.VPAContractABIPath)

		// Verify the ABI has the PrioritiesAssigned event
		if !vpaContractABI.HasEvent("PrioritiesAssigned") {
			cancel()
			return nil, fmt.Errorf("VPA ABI does not contain PrioritiesAssigned event")
		}

		// Get VPA event signature from ABI
		sig, err := vpaContractABI.GetEventHash("PrioritiesAssigned")
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to get PrioritiesAssigned event hash from VPA ABI: %w", err)
		}
		prioritiesAssignedSig = sig
		log.Infof("✅ Using ABI-derived PrioritiesAssigned event signature: %s", prioritiesAssignedSig.Hex())
	} else {
		// Fallback to hardcoded signature
		prioritiesAssignedSig = getPrioritiesAssignedEventSignature()
		log.Infof("✅ Using hardcoded PrioritiesAssigned event signature: %s", prioritiesAssignedSig.Hex())
	}

	windowManager := &WindowManager{
		activeWindows:         make(map[string]*EpochWindow),
		windowSemaphore:       make(chan struct{}, cfg.MaxWindows),
		maxWindows:            cfg.MaxWindows,
		redisClient:           cfg.RedisClient,
		protocolState:         cfg.ContractAddress, // Use protocol state for namespacing
		finalizationBatchSize: cfg.FinalizationBatchSize,
		keyBuilders:           make(map[string]*rediskeys.KeyBuilder),
	}

	// Initialize window config fetcher if NEW ProtocolState contract is configured
	var windowConfigFetcher *WindowConfigFetcher
	estimatedMaxPriority := cfg.EstimatedMaxPriority
	if estimatedMaxPriority == 0 {
		estimatedMaxPriority = 10 // Default safe upper bound
	}

	if cfg.ProtocolState != "" {
		cacheTTL := cfg.WindowConfigCacheTTL
		if cacheTTL == 0 {
			cacheTTL = 5 * time.Minute // Default cache TTL
		}

		fetcher, err := NewWindowConfigFetcher(cfg.RPCHelper, cfg.ProtocolState, cacheTTL)
		if err != nil {
			log.Warnf("⚠️  Failed to initialize window config fetcher, will use fallback duration: %v", err)
		} else {
			windowConfigFetcher = fetcher
			log.WithFields(log.Fields{
				"protocol_state_contract": cfg.ProtocolState,
				"contract_type":           "ProtocolState (VPA-enabled)",
			}).Info("✅ Initialized window config fetcher - will call getDataMarketSubmissionWindowConfig")
		}
	} else {
		log.Info("Window config fetcher disabled - PROTOCOL_STATE_CONTRACT not configured, using fallback duration")
	}

	return &EventMonitor{
		rpcHelper:    cfg.RPCHelper,
		redisClient:  cfg.RedisClient,
		contractAddr: common.HexToAddress(cfg.ContractAddress),
		contractABI:  contractABI,

		// VPA configuration
		vpaContractAddr:       vpaContractAddr,
		vpaContractABI:        vpaContractABI,
		vpaClient:             vpaClient,
		vpaEnabled:            vpaEnabled,
		prioritiesAssignedSig: prioritiesAssignedSig,
		lastProcessedVPABlock: startBlock, // Start from same block as main monitoring

		// Window management
		windowManager:       windowManager,
		windowConfigFetcher: windowConfigFetcher,
		windowDuration:      cfg.WindowDuration, // Fallback default

		// Event tracking
		lastProcessedBlock: startBlock,
		eventChan:          make(chan *EpochReleasedEvent, 100),
		epochReleasedSig:   epochReleasedSig,
		vpaEventChan:       make(chan *PrioritiesAssignedEvent, 100),

		// Configuration
		pollInterval:         cfg.PollInterval,
		dataMarkets:          cfg.DataMarkets,
		estimatedMaxPriority: estimatedMaxPriority,

		// Spam protection components
		spamComponents: cfg.SpamComponents,

		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start begins monitoring for events
func (m *EventMonitor) Start() error {
	log.Info("🚀 Starting event monitor...")

	// Start event processor
	go m.processEvents()

	// Start VPA event processor if enabled
	if m.vpaEnabled {
		go m.processVPAEvents()
		log.Info("✅ VPA event processor started")
	}

	// Start block poller
	go m.pollBlocks()

	return nil
}

// Stop gracefully shuts down the monitor
func (m *EventMonitor) Stop() {
	log.Info("🛑 Stopping event monitor...")
	m.cancel()
	m.windowManager.Shutdown()
}

// pollBlocks continuously polls for new blocks and events
func (m *EventMonitor) pollBlocks() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkForNewEvents()
		}
	}
}

// checkForNewEvents queries for new EpochReleased and PrioritiesAssigned events
func (m *EventMonitor) checkForNewEvents() {
	// Get current block
	currentBlock, err := m.rpcHelper.BlockNumber(m.ctx)
	if err != nil {
		log.Errorf("Failed to get current block: %v", err)
		return
	}

	// Check for legacy EpochReleased events
	m.checkForEpochReleasedEvents(currentBlock)

	// Check for VPA PrioritiesAssigned events if enabled
	if m.vpaEnabled {
		m.checkForPrioritiesAssignedEvents(currentBlock)
	}
}

// checkForEpochReleasedEvents queries for new EpochReleased events from protocol state contract
func (m *EventMonitor) checkForEpochReleasedEvents(currentBlock uint64) {
	// Don't scan if we're already up to date
	if m.lastProcessedBlock >= currentBlock {
		return
	}

	// Limit scan range to avoid overwhelming the node
	toBlock := m.lastProcessedBlock + 1000
	if toBlock > currentBlock {
		toBlock = currentBlock
	}

	// Use cached event signature (computed once at startup)
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(m.lastProcessedBlock + 1)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: []common.Address{m.contractAddr},
		Topics:    [][]common.Hash{{m.epochReleasedSig}},
	}

	logs, err := m.rpcHelper.FilterLogs(m.ctx, query)
	if err != nil {
		log.Errorf("Failed to filter EpochReleased logs: %v", err)
		return
	}

	for _, vLog := range logs {
		event := m.parseEpochReleasedEvent(vLog)
		if event != nil {
			// Only process events for configured data markets
			if m.isValidDataMarket(event.DataMarketAddress.Hex()) {
				m.eventChan <- event
			}
		}
	}

	m.lastProcessedBlock = toBlock
}

// checkForPrioritiesAssignedEvents queries for new PrioritiesAssigned events from VPA contract
func (m *EventMonitor) checkForPrioritiesAssignedEvents(currentBlock uint64) {
	// Don't scan if we're already up to date
	if m.lastProcessedVPABlock >= currentBlock {
		return
	}

	// Limit scan range to avoid overwhelming the node
	toBlock := m.lastProcessedVPABlock + 1000
	if toBlock > currentBlock {
		toBlock = currentBlock
	}

	// Use cached VPA event signature
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(m.lastProcessedVPABlock + 1)),
		ToBlock:   big.NewInt(int64(toBlock)),
		Addresses: []common.Address{m.vpaContractAddr},
		Topics:    [][]common.Hash{{m.prioritiesAssignedSig}},
	}

	logs, err := m.rpcHelper.FilterLogs(m.ctx, query)
	if err != nil {
		log.Errorf("Failed to filter PrioritiesAssigned logs: %v", err)
		return
	}

	for _, vLog := range logs {
		event := m.parsePrioritiesAssignedEvent(vLog)
		if event != nil {
			// Only process events for configured data markets
			if m.isValidDataMarket(event.DataMarket.Hex()) {
				m.vpaEventChan <- event
			}
		}
	}

	m.lastProcessedVPABlock = toBlock
}

// parseEpochReleasedEvent parses the log into an EpochReleasedEvent
func (m *EventMonitor) parseEpochReleasedEvent(vLog types.Log) *EpochReleasedEvent {
	// Event: EpochReleased(address indexed dataMarketAddress, uint256 indexed epochId, uint256 begin, uint256 end, uint256 timestamp)
	// topics[0] = event signature
	// topics[1] = dataMarketAddress (indexed)
	// topics[2] = epochId (indexed)
	// data contains: begin, end, timestamp (non-indexed)

	if len(vLog.Topics) < 3 {
		log.Warnf("Invalid EpochReleased event: expected at least 3 topics, got %d", len(vLog.Topics))
		return nil
	}

	// Parse indexed fields from topics
	dataMarketAddress := common.HexToAddress(vLog.Topics[1].Hex())
	epochID := new(big.Int).SetBytes(vLog.Topics[2].Bytes())

	// Parse non-indexed fields from data
	// The data contains: begin (uint256), end (uint256), timestamp (uint256)
	if len(vLog.Data) < 96 { // 3 * 32 bytes
		log.Warnf("Invalid EpochReleased event data: expected at least 96 bytes, got %d", len(vLog.Data))
		return nil
	}

	// Each uint256 is 32 bytes
	// begin := new(big.Int).SetBytes(vLog.Data[0:32])  // Not needed for our purposes
	// end := new(big.Int).SetBytes(vLog.Data[32:64])   // Not needed for our purposes
	timestamp := new(big.Int).SetBytes(vLog.Data[64:96])

	log.Debugf("Parsed EpochReleased event: DataMarket=%s, EpochID=%s, Timestamp=%s",
		dataMarketAddress.Hex(), epochID.String(), timestamp.String())

	return &EpochReleasedEvent{
		EpochID:           epochID,
		DataMarketAddress: dataMarketAddress,
		BlockNumber:       vLog.BlockNumber,
		TransactionHash:   vLog.TxHash,
		Timestamp:         timestamp.Uint64(),
	}
}

// parsePrioritiesAssignedEvent parses the log into a PrioritiesAssignedEvent
func (m *EventMonitor) parsePrioritiesAssignedEvent(vLog types.Log) *PrioritiesAssignedEvent {
	// Event: PrioritiesAssigned(address indexed dataMarket, uint256 indexed epochId, uint256 seed, uint256 timestamp, uint256 validatorCount)
	// topics[0] = event signature
	// topics[1] = dataMarket (indexed)
	// topics[2] = epochId (indexed)
	// data contains: seed, timestamp, validatorCount (non-indexed)

	if len(vLog.Topics) < 3 {
		log.Warnf("Invalid PrioritiesAssigned event: expected at least 3 topics, got %d", len(vLog.Topics))
		return nil
	}

	// Parse indexed fields from topics
	dataMarket := common.HexToAddress(vLog.Topics[1].Hex())
	epochID := new(big.Int).SetBytes(vLog.Topics[2].Bytes())

	// Parse non-indexed fields from data
	// The data contains: seed (uint256), timestamp (uint256), validatorCount (uint256)
	if len(vLog.Data) < 96 { // 3 * 32 bytes
		log.Warnf("Invalid PrioritiesAssigned event data: expected at least 96 bytes, got %d", len(vLog.Data))
		return nil
	}

	// Each uint256 is 32 bytes
	seed := new(big.Int).SetBytes(vLog.Data[0:32])
	timestamp := new(big.Int).SetBytes(vLog.Data[32:64])
	validatorCount := new(big.Int).SetBytes(vLog.Data[64:96])

	log.Debugf("Parsed PrioritiesAssigned event: DataMarket=%s, EpochID=%s, Seed=%s, Timestamp=%s, ValidatorCount=%s",
		dataMarket.Hex(), epochID.String(), seed.String(), timestamp.String(), validatorCount.String())

	return &PrioritiesAssignedEvent{
		EpochID:         epochID,
		Seed:            seed,
		Timestamp:       timestamp.Uint64(),
		ValidatorCount:  validatorCount,
		DataMarket:      dataMarket,
		TransactionHash: vLog.TxHash,
		BlockNumber:     vLog.BlockNumber,
	}
}

// isValidDataMarket checks if the data market is in our configured list
func (m *EventMonitor) isValidDataMarket(address string) bool {
	for _, market := range m.dataMarkets {
		if market == address {
			return true
		}
	}
	return false
}

// getEpochReleasedEventSignature computes the keccak256 hash of the event signature
func getEpochReleasedEventSignature() common.Hash {
	// Event signature: EpochReleased(address,uint256,uint256,uint256,uint256)
	// This matches the event definition:
	// event EpochReleased(
	//     address indexed dataMarketAddress,
	//     uint256 indexed epochId,
	//     uint256 begin,
	//     uint256 end,
	//     uint256 timestamp
	// );
	eventSignature := []byte("EpochReleased(address,uint256,uint256,uint256,uint256)")
	hash := crypto.Keccak256Hash(eventSignature)

	log.Debugf("Computed EpochReleased event signature: %s", hash.Hex())
	return hash
}

// getPrioritiesAssignedEventSignature computes the keccak256 hash of the VPA event signature
func getPrioritiesAssignedEventSignature() common.Hash {
	// Event signature: PrioritiesAssigned(address,uint256,uint256,uint256,uint256)
	// This matches the event definition:
	// event PrioritiesAssigned(
	//     address indexed dataMarket,
	//     uint256 indexed epochId,
	//     uint256 seed,
	//     uint256 timestamp,
	//     uint256 validatorCount
	// );
	eventSignature := []byte("PrioritiesAssigned(address,uint256,uint256,uint256,uint256)")
	hash := crypto.Keccak256Hash(eventSignature)

	log.Debugf("Computed PrioritiesAssigned event signature: %s", hash.Hex())
	return hash
}

// processEvents handles incoming epoch events
func (m *EventMonitor) processEvents() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-m.eventChan:
			m.handleEpochReleased(event)
		}
	}
}

// processVPAEvents handles incoming VPA priority assignment events
func (m *EventMonitor) processVPAEvents() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-m.vpaEventChan:
			m.handlePrioritiesAssigned(event)
		}
	}
}

// handlePrioritiesAssigned processes a new priority assignment
func (m *EventMonitor) handlePrioritiesAssigned(event *PrioritiesAssignedEvent) {
	log.Infof("🎯 Priorities assigned for epoch %s in market %s (seed: %s, validators: %s) at block %d",
		event.EpochID, event.DataMarket.Hex(), event.Seed.String(), event.ValidatorCount.String(), event.BlockNumber)

	// Only cache priorities if VPA client is available
	if m.vpaClient == nil {
		log.Warn("VPA client not available, skipping priority caching")
		return
	}

	// Cache priorities for this epoch and data market
	err := m.vpaClient.CacheEpochPriorities(m.ctx, event.DataMarket.Hex(), event.EpochID.Uint64())
	if err != nil {
		log.Errorf("Failed to cache VPA priorities for epoch %s in market %s: %v",
			event.EpochID, event.DataMarket.Hex(), err)
		return
	}

	log.Infof("✅ Successfully cached VPA priorities for epoch %s in market %s",
		event.EpochID, event.DataMarket.Hex())
}

// handleEpochReleased processes a new epoch release
func (m *EventMonitor) handleEpochReleased(event *EpochReleasedEvent) {
	log.Infof("📅 Epoch %s released for market %s at block %d",
		event.EpochID, event.DataMarketAddress.Hex(), event.BlockNumber)

	dataMarketAddr := event.DataMarketAddress.Hex()

	// Fetch window config from contract if fetcher is available
	var windowDuration time.Duration
	var windowConfig *WindowConfig
	var useFallback bool

	// Always try to fetch window config from contract
	if m.windowConfigFetcher != nil {
		log.WithFields(log.Fields{
			"data_market": dataMarketAddr,
		}).Debug("Fetching window config from ProtocolState contract")
		config, err := m.windowConfigFetcher.FetchWindowConfig(m.ctx, dataMarketAddr)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"data_market":             dataMarketAddr,
				"protocol_state_contract": m.windowConfigFetcher.protocolStateAddr.Hex(),
			}).Warn("⚠️  Failed to fetch window config from ProtocolState contract, using fallback duration")
			windowDuration = m.windowDuration
			useFallback = true
		} else {
			windowConfig = config
			// LocalFinalizationWindow() handles two cases:
			// 1. Snapshot Commit/Reveal enabled: triggers when snapshot reveal closes (snapshotCommit + snapshotReveal)
			// 2. Snapshot Commit/Reveal disabled: triggers when P1 window closes (preSubmissionWindow + p1SubmissionWindow)
			// P1 and PN windows in contract are for on-chain submission AFTER local finalization completes.
			// Note: Validator vote commit/reveal is a separate workflow and doesn't affect Level 1 finalization timing.
			windowDuration = config.LocalFinalizationWindow(m.windowDuration)

			hasSnapshotCommitReveal := config.SnapshotCommitWindow.Uint64() > 0 ||
				config.SnapshotRevealWindow.Uint64() > 0

			logFields := log.Fields{
				"data_market":                    dataMarketAddr,
				"p1_window":                      config.P1SubmissionWindow.Uint64(),
				"pN_window":                      config.PNSubmissionWindow.Uint64(),
				"pre_submission_window":          config.PreSubmissionWindow.Uint64(),
				"finalization_duration":          windowDuration,
				"snapshot_commit_reveal_enabled": hasSnapshotCommitReveal,
			}

			if hasSnapshotCommitReveal {
				logFields["snapshot_commit_window"] = config.SnapshotCommitWindow.Uint64()
				logFields["snapshot_reveal_window"] = config.SnapshotRevealWindow.Uint64()
				log.WithFields(logFields).Info("✅ Using on-chain window config: Level 1 finalization triggers when snapshot reveal closes")
			} else {
				log.WithFields(logFields).Info("✅ Using on-chain window config: Level 1 finalization triggers 2/3rds before P1 window closure")
			}
		}
	} else {
		// Window config fetcher not initialized - use fallback duration
		log.WithFields(log.Fields{
			"data_market": dataMarketAddr,
		}).Warn("⚠️  Window config fetcher not initialized - using fallback duration")
		windowDuration = m.windowDuration
		useFallback = true
	}

	// Skip old epochs whose windows would have already expired
	// This prevents filling up the window manager with historical epochs
	epochAge := time.Since(time.Unix(int64(event.Timestamp), 0))
	if epochAge > windowDuration*2 {
		log.Debugf("Skipping old epoch %s (age: %v, window duration: %v)",
			event.EpochID, epochAge, windowDuration)
		return
	}

	// Store epoch info in Redis
	epochKey := fmt.Sprintf("epoch:%s:%s:info", dataMarketAddr, event.EpochID.String())
	epochData := map[string]interface{}{
		"epoch_id":     event.EpochID.String(),
		"epoch_end":    event.EpochEnd.String(),
		"data_market":  dataMarketAddr,
		"released_at":  event.Timestamp,
		"block_number": event.BlockNumber,
		"tx_hash":      event.TransactionHash.Hex(),
		"window_start": time.Now().Unix(),
		"window_end":   time.Now().Add(windowDuration).Unix(),
		"duration":     windowDuration.Seconds(), // Duration until Level 1 finalization triggers (varies based on commit/reveal enabled)
	}

	// Store window config values if available
	if windowConfig != nil {
		epochData["p1_submission_window"] = windowConfig.P1SubmissionWindow.Uint64()
		epochData["pN_submission_window"] = windowConfig.PNSubmissionWindow.Uint64()
		epochData["pre_submission_window"] = windowConfig.PreSubmissionWindow.Uint64()
		epochData["snapshot_commit_window"] = windowConfig.SnapshotCommitWindow.Uint64()
		epochData["snapshot_reveal_window"] = windowConfig.SnapshotRevealWindow.Uint64()
		epochData["validator_vote_commit_window"] = windowConfig.ValidatorVoteCommitWindow.Uint64()
		epochData["validator_vote_reveal_window"] = windowConfig.ValidatorVoteRevealWindow.Uint64()
		epochData["window_source"] = "contract"
	} else {
		epochData["window_source"] = "fallback"
	}

	// Store with pipeline for efficiency
	pipe := m.redisClient.Pipeline()
	pipe.HMSet(m.ctx, epochKey, epochData)
	pipe.Expire(m.ctx, epochKey, 24*time.Hour)

	// Also add to active epochs set (use namespaced keys)
	// Only refresh TTL when adding NEW epochs (not when epoch already exists)
	// The set is also pruned periodically by state-tracker to remove old epochs
	kb := m.windowManager.getKeyBuilder(dataMarketAddr)
	activeEpochsKey := kb.ActiveEpochs()
	pipe.SAdd(m.ctx, activeEpochsKey, event.EpochID.String())

	results, err := pipe.Exec(m.ctx)
	if err != nil {
		log.Errorf("Failed to store epoch info: %v", err)
		return
	}

	// Check if epoch was actually added (new epoch) - SAdd returns 1 if added, 0 if already exists
	// Pipeline order: [0] HMSet, [1] Expire(epochKey), [2] SAdd(activeEpochsKey)
	if len(results) >= 3 {
		if sAddResult, ok := results[2].(*redis.IntCmd); ok {
			added := sAddResult.Val()
			if added > 0 {
				// Only refresh TTL when adding a NEW epoch
				// Check if TTL exists first to avoid unnecessary refresh
				ttl := m.redisClient.TTL(m.ctx, activeEpochsKey).Val()
				if ttl == -1 { // Key exists but has no TTL
					m.redisClient.Expire(m.ctx, activeEpochsKey, 24*time.Hour)
				}
			}
		}
	}

	// Check if this is a 10-epoch boundary for spam aggregation windows
	// Windows are created at epochs 10, 20, 30, etc. (epochID % 10 == 0)
	epochIDInt := event.EpochID.Uint64()
	if epochIDInt > 0 && epochIDInt%10 == 0 {
		log.Infof("Epoch %d is an aggregation window boundary. Triggering local spam data aggregation.", epochIDInt)
		// This is a window boundary - create window and aggregate local tracking data
		// Note: spamComponents may be nil if spam protection is disabled
		if m.spamComponents != nil {

			scValue := reflect.ValueOf(m.spamComponents)
			if scValue.Kind() == reflect.Ptr && !scValue.IsNil() {
				// Get Aggregator field
				aggregatorField := scValue.Elem().FieldByName("Aggregator")
				trackerField := scValue.Elem().FieldByName("Tracker")

				if aggregatorField.IsValid() && trackerField.IsValid() {
					if !aggregatorField.IsNil() && !trackerField.IsNil() {
						// Call CreateWindowAndAggregateLocalData using reflection
						createWindowMethod := aggregatorField.MethodByName("CreateWindowAndAggregateLocalData")
						if createWindowMethod.IsValid() {
							ctxVal := reflect.ValueOf(m.ctx)
							epochVal := reflect.ValueOf(epochIDInt)
							trackerVal := trackerField

							log.Debugf("Calling CreateWindowAndAggregateLocalData for epoch %d", epochIDInt)
							results := createWindowMethod.Call([]reflect.Value{ctxVal, epochVal, trackerVal})
							if len(results) > 0 && !results[0].IsNil() {
								if err, ok := results[0].Interface().(error); ok && err != nil {
									log.Errorf("Failed to create spam aggregation window at epoch boundary %d: %v", epochIDInt, err)
								} else {
									log.Infof("Successfully created spam aggregation window for epoch %d", epochIDInt)
									// Note: Consensus checking is now scheduled by SpamReportWindowManager
									// after collection window closes and reports are sent, with additional delay
									// for other validators' reports to arrive
								}
							} else {
								log.Infof("Successfully created spam aggregation window for epoch %d (no error returned)", epochIDInt)
								// Note: Consensus checking is now scheduled by SpamReportWindowManager
								// after collection window closes and reports are sent, with additional delay
								// for other validators' reports to arrive
							}
						} else {
							log.Warnf("CreateWindowAndAggregateLocalData method not found on aggregator (type: %s)", aggregatorField.Type())
						}
					} else {
						if aggregatorField.IsNil() {
							log.Warnf("Spam aggregator is nil at epoch boundary %d", epochIDInt)
						}
						if trackerField.IsNil() {
							log.Warnf("Spam tracker is nil at epoch boundary %d", epochIDInt)
						}
					}
				} else {
					log.Warnf("Spam components fields not found (aggregator valid: %v, tracker valid: %v)", aggregatorField.IsValid(), trackerField.IsValid())
				}
			} else {
				log.Warnf("Spam components value is not a valid pointer (kind: %s, isNil: %v)", scValue.Kind(), scValue.IsNil())
			}
		} else {
			log.Warnf("Spam components not initialized - skipping window creation at epoch boundary %d", epochIDInt)
		}
	}

	// Start spam report collection window (if spam protection enabled)
	// This window batches all spam reports for the epoch and sends them after collection window
	// Collection window: LEVEL1_FINALIZATION_DELAY_SECONDS + 10 seconds
	if m.spamComponents != nil {
		scValue := reflect.ValueOf(m.spamComponents)
		if scValue.Kind() == reflect.Ptr && !scValue.IsNil() {
			reporterField := scValue.Elem().FieldByName("Reporter")
			if reporterField.IsValid() && !reporterField.IsNil() {
				// Get window manager from reporter using reflection
				getWindowManagerMethod := reporterField.MethodByName("GetWindowManager")
				if getWindowManagerMethod.IsValid() {
					results := getWindowManagerMethod.Call([]reflect.Value{})
					if len(results) > 0 && !results[0].IsNil() {
						windowManager := results[0]

						// Set consensus check callback if this is a window boundary epoch
						epochIDInt := event.EpochID.Uint64()
						if epochIDInt%10 == 0 {
							setCallbackMethod := windowManager.MethodByName("SetConsensusCheckCallback")
							if setCallbackMethod.IsValid() {
								// Create callback function that calls checkConsensusAfterDelay
								callbackFunc := reflect.MakeFunc(reflect.TypeOf(func(uint64) {}), func(args []reflect.Value) []reflect.Value {
									epID := uint64(args[0].Uint())
									go m.checkConsensusAfterDelay(epID)
									return nil
								})
								setCallbackMethod.Call([]reflect.Value{callbackFunc})
								log.Debugf("Set consensus check callback for window boundary epoch %d", epochIDInt)
							}
						}

						// Call StartReportCollectionWindow using reflection
						startWindowMethod := windowManager.MethodByName("StartReportCollectionWindow")
						if startWindowMethod.IsValid() {
							// Convert timestamp from Unix seconds to time.Time
							releaseTime := time.Unix(int64(event.Timestamp), 0)
							ctxVal := reflect.ValueOf(m.ctx)
							dataMarketVal := reflect.ValueOf(dataMarketAddr)
							epochVal := reflect.ValueOf(epochIDInt)
							releaseTimeVal := reflect.ValueOf(releaseTime)
							callResults := startWindowMethod.Call([]reflect.Value{ctxVal, dataMarketVal, epochVal, releaseTimeVal})
							if len(callResults) > 0 && !callResults[0].IsNil() {
								if err, ok := callResults[0].Interface().(error); ok && err != nil {
									log.Debugf("Failed to start spam report collection window for epoch %d: %v", epochIDInt, err)
								} else {
									log.Debugf("Started spam report collection window for epoch %d", epochIDInt)
								}
							} else {
								log.Debugf("Started spam report collection window for epoch %d", epochIDInt)
							}
						}
					}
				}
			}
		}
	}

	// Start submission window - this window is for collecting snapshot CIDs from snapshotter nodes
	// Window closes when Level 1 finalization should begin
	// Duration varies by contract configuration:
	//   - New contracts with snapshot commit/reveal enabled: snapshotCommitWindow + snapshotRevealWindow (snapshot reveal closes)
	//   - New contracts without snapshot commit/reveal: (PreSubmissionWindow + P1SubmissionWindow) - 2/3 of the P1 window
	// When window closes, triggerFinalization() is called to begin Level 1 local finalization
	// Note: Validator vote commit/reveal is a separate workflow and doesn't affect this timing
	if err := m.windowManager.StartSubmissionWindow(
		m.ctx,
		dataMarketAddr,
		event.EpochID,
		windowDuration,
		event.BlockNumber,
	); err != nil {
		log.Errorf("Failed to start submission window: %v", err)
	}

	if useFallback {
		// Only warn if window config fetcher is not initialized (missing PROTOCOL_STATE_CONTRACT)
		// If it's initialized but failed for this specific data market, that's already logged above
		if m.windowConfigFetcher == nil {
			log.Warnf("⚠️  Using fallback window duration %v for epoch %s (PROTOCOL_STATE_CONTRACT not configured)",
				windowDuration, event.EpochID.String())
		}
		// If window config fetcher exists but we're using fallback, it means contract call failed
		// This is already logged above, so no need for additional warning
	} else {
		// Contract with window config
		hasSnapshotCommitReveal := windowConfig != nil && (windowConfig.SnapshotCommitWindow.Uint64() > 0 ||
			windowConfig.SnapshotRevealWindow.Uint64() > 0)

		if hasSnapshotCommitReveal {
			log.Infof("📋 Level 1 finalization will trigger when snapshot reveal window closes (in %v)", windowDuration)
		} else {
			log.Infof("📋 Level 1 finalization will trigger 2/3rds before P1 window closure (in %v)", windowDuration)
		}
	}
}

// checkConsensusAfterDelay waits 10 seconds then checks consensus for the completed window
// This allows all validators' reports to arrive before checking consensus
func (m *EventMonitor) checkConsensusAfterDelay(epochID uint64) {
	log.Infof("Waiting 10 seconds for validator reports before checking consensus for window %d", epochID)
	time.Sleep(10 * time.Second)

	// Now call CheckWindowForConsensus
	if m.spamComponents != nil {
		scValue := reflect.ValueOf(m.spamComponents)
		if scValue.Kind() == reflect.Ptr && !scValue.IsNil() {
			aggregatorField := scValue.Elem().FieldByName("Aggregator")
			if aggregatorField.IsValid() && !aggregatorField.IsNil() {
				checkConsensusMethod := aggregatorField.MethodByName("CheckWindowForConsensus")
				if checkConsensusMethod.IsValid() {
					ctxVal := reflect.ValueOf(m.ctx)
					epochVal := reflect.ValueOf(epochID)
					log.Infof("Checking consensus for window %d after 10-second delay", epochID)
					results := checkConsensusMethod.Call([]reflect.Value{ctxVal, epochVal})
					if len(results) > 0 && !results[0].IsNil() {
						if err, ok := results[0].Interface().(error); ok && err != nil {
							log.Errorf("Failed to check consensus for window %d: %v", epochID, err)
						} else {
							log.Infof("Successfully checked consensus for window %d", epochID)
						}
					}
				}
			}
		}
	}
}

// WindowManager methods

func (wm *WindowManager) StartSubmissionWindow(ctx context.Context, dataMarket string, epochID *big.Int, duration time.Duration, startBlock uint64) error {
	key := fmt.Sprintf("%s:%s", dataMarket, epochID.String())

	// Check if window already exists
	wm.mu.RLock()
	if _, exists := wm.activeWindows[key]; exists {
		wm.mu.RUnlock()
		return fmt.Errorf("window already active for epoch %s in market %s", epochID, dataMarket)
	}
	wm.mu.RUnlock()

	// Try to acquire semaphore
	select {
	case wm.windowSemaphore <- struct{}{}:
		// Got permission
	case <-time.After(1 * time.Second):
		return fmt.Errorf("too many active windows (%d), refusing new window", wm.GetActiveCount())
	}

	// Create window
	window := &EpochWindow{
		EpochID:           epochID,
		DataMarketAddress: dataMarket,
		StartTime:         time.Now(),
		WindowDuration:    duration,
		Done:              make(chan struct{}),
		StartBlockNum:     startBlock,
	}

	// Add to active windows
	wm.mu.Lock()
	wm.activeWindows[key] = window
	activeCount := len(wm.activeWindows)
	wm.mu.Unlock()

	// Start window timer
	window.Timer = time.AfterFunc(duration, func() {
		wm.closeWindow(dataMarket, epochID)
	})

	// Mark window as open in Redis - namespaced with protocol:market
	kb := wm.getKeyBuilder(dataMarket)
	windowKey := kb.EpochWindow(epochID.String())
	wm.redisClient.Set(context.Background(), windowKey, "open", duration)

	log.Infof("⏰ Submission window opened for epoch %s in market %s (duration: %v, active: %d)",
		epochID, dataMarket, duration, activeCount)

	// Add monitoring metrics for window open
	timestamp := time.Now().Unix()

	// Pipeline for monitoring metrics
	pipe := wm.redisClient.Pipeline()

	// 1. Add to epochs timeline
	pipe.ZAdd(context.Background(), kb.MetricsEpochsTimeline(), redis.Z{
		Score:  float64(timestamp),
		Member: fmt.Sprintf("open:%s", epochID.String()),
	})

	// 2. Store epoch info with TTL
	epochInfoKey := kb.MetricsEpochInfo(epochID.String())
	pipe.HSet(context.Background(), epochInfoKey, map[string]interface{}{
		"status":      "open",
		"start":       timestamp,
		"data_market": dataMarket,
		"duration":    duration.Seconds(),
		"start_block": startBlock,
	})
	pipe.Expire(context.Background(), epochInfoKey, 2*time.Hour)

	// 3. Store comprehensive epoch state hash
	epochStateKey := kb.EpochState(epochID.String())
	windowClosesAt := timestamp + int64(duration.Seconds())
	pipe.HSet(context.Background(), epochStateKey, map[string]interface{}{
		"window_status":     "open",
		"window_opened_at":  timestamp,
		"window_closes_at":  windowClosesAt,
		"phase":             "submission",
		"submissions_count": 0,
		"level1_status":     "pending",
		"level2_status":     "pending",
		"onchain_status":    "pending",
		"last_updated":      timestamp,
	})
	pipe.Expire(context.Background(), epochStateKey, 7*24*time.Hour) // Keep for 7 days

	// 3. Publish state change
	pipe.Publish(context.Background(), "state:change", fmt.Sprintf("epoch:open:%s", epochID.String()))

	// Execute pipeline
	if _, err := pipe.Exec(context.Background()); err != nil {
		log.Debugf("Failed to write monitoring metrics: %v", err)
	}

	return nil
}

func (wm *WindowManager) closeWindow(dataMarket string, epochID *big.Int) {
	key := fmt.Sprintf("%s:%s", dataMarket, epochID.String())

	wm.mu.Lock()
	window, exists := wm.activeWindows[key]
	if !exists {
		wm.mu.Unlock()
		return
	}

	// Remove from active windows
	delete(wm.activeWindows, key)
	activeCount := len(wm.activeWindows)
	wm.mu.Unlock()

	// Release semaphore
	<-wm.windowSemaphore

	// Close the done channel
	close(window.Done)

	// Mark window as closed in Redis - namespaced with protocol:market
	ctx := context.Background()
	kb := wm.getKeyBuilder(dataMarket)
	windowKey := kb.EpochWindow(epochID.String())
	wm.redisClient.Set(ctx, windowKey, "closed", 1*time.Hour)

	// Trigger finalization
	wm.triggerFinalization(dataMarket, epochID, window.StartBlockNum)

	log.Infof("⏱️ Submission window closed for epoch %s in market %s (remaining: %d)",
		epochID, dataMarket, activeCount)

	// Add monitoring metrics for window close
	timestamp := time.Now().Unix()

	// Pipeline for monitoring metrics
	pipe := wm.redisClient.Pipeline()

	// 1. Add to epochs timeline
	pipe.ZAdd(context.Background(), kb.MetricsEpochsTimeline(), redis.Z{
		Score:  float64(timestamp),
		Member: fmt.Sprintf("close:%s", epochID.String()),
	})

	// 2. Update epoch info (already has TTL from open)
	epochInfoKey := kb.MetricsEpochInfo(epochID.String())
	pipe.HSet(context.Background(), epochInfoKey, map[string]interface{}{
		"status": "closed",
		"end":    timestamp,
	})

	// 3. Update epoch state hash - window closed, transition to level1_finalization phase
	epochStateKey := kb.EpochState(epochID.String())
	pipe.HSet(context.Background(), epochStateKey, map[string]interface{}{
		"window_status": "closed",
		"phase":         "level1_finalization",
		"last_updated":  timestamp,
	})
	// Refresh TTL on epoch state (7 days - same as initial creation)
	pipe.Expire(context.Background(), epochStateKey, 7*24*time.Hour)

	// 4. Publish state change
	pipe.Publish(context.Background(), "state:change", fmt.Sprintf("epoch:closed:%s", epochID.String()))

	// Execute pipeline
	if _, err := pipe.Exec(context.Background()); err != nil {
		log.Debugf("Failed to write monitoring metrics: %v", err)
	}
}

func (wm *WindowManager) triggerFinalization(dataMarket string, epochID *big.Int, _ uint64) {
	// Submission window has closed - collect all snapshot CIDs that were submitted during the window
	// These snapshot CIDs will be aggregated into a finalized batch during Level 1 aggregation
	submissions := wm.collectEpochSubmissions(dataMarket, epochID)

	// Check if we have any submissions BEFORE proceeding
	if len(submissions) == 0 {
		log.Errorf("❌ CRITICAL: No submissions found for epoch %s - new deterministic structures may not exist (dequeuer write failure?)", epochID)
		ctx := context.Background()
		kb := wm.getKeyBuilder(dataMarket)
		timestamp := time.Now().Unix()
		epochStateKey := kb.EpochState(epochID.String())
		wm.redisClient.HSet(ctx, epochStateKey, map[string]interface{}{
			"level1_status":     "skipped",
			"level1_started_at": timestamp,
			"level1_reason":     "no_submissions_found",
			"last_updated":      timestamp,
		})
		wm.redisClient.Expire(ctx, epochStateKey, 7*24*time.Hour)
		return // Don't proceed with empty batches
	}

	// Split submissions into smaller batches for parallel processing
	batchSize := wm.finalizationBatchSize
	if batchSize <= 0 {
		batchSize = 20 // Default fallback
	}
	batches := wm.splitIntoBatches(submissions, batchSize)

	// Double-check batches isn't empty (shouldn't happen if submissions isn't empty, but be safe)
	if len(batches) == 0 {
		log.Errorf("❌ CRITICAL: No batches created for epoch %s despite %d submissions - skipping finalization", epochID, len(submissions))
		ctx := context.Background()
		kb := wm.getKeyBuilder(dataMarket)
		timestamp := time.Now().Unix()
		epochStateKey := kb.EpochState(epochID.String())
		wm.redisClient.HSet(ctx, epochStateKey, map[string]interface{}{
			"level1_status":     "skipped",
			"level1_started_at": timestamp,
			"level1_reason":     "no_batches_created",
			"last_updated":      timestamp,
		})
		wm.redisClient.Expire(ctx, epochStateKey, 7*24*time.Hour)
		return
	}

	// Track batch metadata in Redis for aggregation worker
	ctx := context.Background()
	kb := wm.getKeyBuilder(dataMarket)
	batchMetaKey := fmt.Sprintf("%s:%s:epoch:%s:batch:meta",
		wm.protocolState, dataMarket, epochID.String())

	batchMeta := map[string]interface{}{
		"epoch_id":       epochID.String(),
		"total_batches":  len(batches),
		"total_projects": len(submissions),
		"created_at":     time.Now().Unix(),
		"data_market":    dataMarket,
	}

	metaData, _ := json.Marshal(batchMeta)
	wm.redisClient.Set(ctx, batchMetaKey, metaData, 2*time.Hour)

	// Update epoch state hash - Level 1 finalization started
	timestamp := time.Now().Unix()
	epochStateKey := kb.EpochState(epochID.String())
	wm.redisClient.HSet(ctx, epochStateKey, map[string]interface{}{
		"level1_status":     "in_progress",
		"level1_started_at": timestamp,
		"last_updated":      timestamp,
	})
	wm.redisClient.Expire(ctx, epochStateKey, 7*24*time.Hour)

	// Push each batch to finalization queue
	queueKey := kb.FinalizationQueue()
	log.Debugf("Pushing %d batches to finalization queue: %s", len(batches), queueKey)

	for i, batch := range batches {
		batchData := map[string]interface{}{
			"epoch_id":      epochID.String(),
			"batch_id":      i,
			"total_batches": len(batches),
			"projects":      batch,
			"data_market":   dataMarket,
		}

		data, _ := json.Marshal(batchData)
		if err := wm.redisClient.LPush(ctx, queueKey, data).Err(); err != nil {
			log.Errorf("Failed to push batch %d to finalization queue: %v", i, err)
			continue
		}
		log.Debugf("Pushed batch %d to finalization queue: %s", i, queueKey)
	}

	log.Infof("🎯 Split epoch %s into %d batches (%d projects total, batch size %d) for parallel finalization",
		epochID, len(batches), len(submissions), batchSize)
}

func (wm *WindowManager) splitIntoBatches(submissions map[string]interface{}, batchSize int) []map[string]interface{} {
	var batches []map[string]interface{}
	currentBatch := make(map[string]interface{})
	count := 0

	for projectID, submissionData := range submissions {
		currentBatch[projectID] = submissionData
		count++

		if count >= batchSize {
			batches = append(batches, currentBatch)
			currentBatch = make(map[string]interface{})
			count = 0
		}
	}

	// Add remaining projects as final batch
	if len(currentBatch) > 0 {
		batches = append(batches, currentBatch)
	}

	return batches
}

// collectEpochSubmissions retrieves all processed submissions for an epoch
// Uses deterministic epoch-keyed structures (ZSET + HASH)
func (wm *WindowManager) collectEpochSubmissions(dataMarket string, epochID *big.Int) map[string]interface{} {
	ctx := context.Background()

	// Get key builder for this data market
	kb := wm.getKeyBuilder(dataMarket)
	epochIDStr := epochID.String()

	// Get submission IDs from ZSET (deterministic, ordered by timestamp)
	submissionsIdsKey := kb.EpochSubmissionsIds(epochIDStr)
	submissionIDs, err := wm.redisClient.ZRange(ctx, submissionsIdsKey, 0, -1).Result()
	if err != nil {
		log.Errorf("Failed to get submission IDs from ZSET for epoch %s: %v", epochIDStr, err)
		return make(map[string]interface{})
	}

	if len(submissionIDs) == 0 {
		log.Errorf("❌ CRITICAL: No submission IDs found in ZSET for epoch %s (key: %s) - dequeuer may not have written to new structures", epochIDStr, submissionsIdsKey)
		return make(map[string]interface{})
	}

	log.Debugf("Found %d submission IDs in ZSET for epoch %s", len(submissionIDs), epochIDStr)

	// Get all submission data from HASH (deterministic, single operation)
	submissionsDataKey := kb.EpochSubmissionsData(epochIDStr)
	submissionDataMap, err := wm.redisClient.HGetAll(ctx, submissionsDataKey).Result()
	if err != nil {
		log.Errorf("Failed to get submission data from HASH for epoch %s: %v", epochIDStr, err)
		return make(map[string]interface{})
	}

	log.Debugf("Retrieved %d submission entries from HASH for epoch %s", len(submissionDataMap), epochIDStr)

	// Track CIDs per project with vote counts AND submitter details
	// Structure: map[projectID]map[CID]count
	projectVotes := make(map[string]map[string]int)
	// Track WHO submitted WHAT for challenges/proofs
	submissionMetadata := make(map[string][]map[string]interface{}) // projectID -> list of submissions

	foundCount := 0
	missingCount := 0

	// Process submissions deterministically
	for _, submissionID := range submissionIDs {
		data, exists := submissionDataMap[submissionID]
		if !exists {
			log.Warnf("⚠️  Submission ID %s in ZSET but not in HASH for epoch %s", submissionID, epochIDStr)
			missingCount++
			continue
		}

		foundCount++

		var submission map[string]interface{}
		if err := json.Unmarshal([]byte(data), &submission); err != nil {
			log.Errorf("Failed to unmarshal submission %s: %v", submissionID, err)
			missingCount++
			continue
		}

		if subData, ok := submission["Submission"].(map[string]interface{}); ok {
			if request, ok := subData["request"].(map[string]interface{}); ok {
				if projectID, ok := request["projectId"].(string); ok {
					if snapshotCID, ok := request["snapshotCid"].(string); ok {
						// Track vote counts
						if projectVotes[projectID] == nil {
							projectVotes[projectID] = make(map[string]int)
						}
						projectVotes[projectID][snapshotCID]++

						// Track submission metadata for challenges/proofs
						slotID := uint64(0)
						if slot, ok := request["slotId"].(float64); ok {
							slotID = uint64(slot)
						}

						// Extract submitter info from submission
						// EIP-712 signature verification is done by dequeuer during processing
						// The verified snapshotter EVM address is stored in SnapshotterAddr field
						submitterID := ""
						if addr, ok := submission["SnapshotterAddr"].(string); ok && addr != "" {
							submitterID = addr
						} else {
							// Missing SnapshotterAddr indicates signature verification failed in dequeuer
							// This submission should not have been processed - log error and skip
							log.Errorf("Missing SnapshotterAddr for submission %s (epoch=%d, project=%s, CID=%s) - signature verification may have failed",
								submissionID, submission["Submission"].(map[string]interface{})["request"].(map[string]interface{})["epochId"],
								projectID, snapshotCID)
							continue
						}

						signature := ""
						if sig, ok := subData["signature"].(string); ok {
							signature = sig
						}

						metadata := map[string]interface{}{
							"submitter_id": submitterID,
							"snapshot_cid": snapshotCID,
							"slot_id":      slotID,
							"signature":    signature,
							"timestamp":    time.Now().Unix(),
						}

						submissionMetadata[projectID] = append(submissionMetadata[projectID], metadata)
						log.Debugf("Found submission: project=%s, CID=%s, submitter=%s", projectID, snapshotCID, submitterID)
					} else {
						log.Warnf("No snapshotCid in request: %+v", request)
					}
				} else {
					log.Warnf("No projectId in request: %+v", request)
				}
			} else {
				log.Warnf("No request field in submission: %+v", subData)
			}
		} else {
			log.Warnf("No Submission field in data: %+v", submission)
		}
	}

	projectSubmissions := make(map[string]interface{})
	for projectID, cidVotes := range projectVotes {
		projectSubmissions[projectID] = map[string]interface{}{
			"cid_votes":           cidVotes,
			"total_submissions":   len(cidVotes),
			"submission_metadata": submissionMetadata[projectID], // Add WHO submitted WHAT
		}

		totalVotes := 0
		for _, votes := range cidVotes {
			totalVotes += votes
		}
		log.Debugf("Project %s: Collected %d unique CIDs with %d total submissions from %d submitters",
			projectID, len(cidVotes), totalVotes, len(submissionMetadata[projectID]))
	}

	// Store the collected batch in Redis for finalizer (namespaced)
	// Format: {protocol}:{market}:batch:ready:{epochID}
	batchKey := fmt.Sprintf("%s:%s:batch:ready:%s",
		wm.protocolState, dataMarket, epochID.String())
	batchData, _ := json.Marshal(projectSubmissions)
	wm.redisClient.Set(ctx, batchKey, batchData, 1*time.Hour)

	log.Infof("📦 Collected %d unique projects for epoch %s (found %d/%d submissions deterministically from epoch-keyed structures)",
		len(projectSubmissions), epochID, foundCount, len(submissionIDs))

	if missingCount > 0 {
		log.Warnf("⚠️  %d submission IDs in ZSET but missing from HASH for epoch %s", missingCount, epochID)
	}

	if len(submissionIDs) > 0 && len(projectSubmissions) == 0 {
		log.Errorf("❌ CRITICAL: Found %d submission IDs but collected 0 projects for epoch %s - data may be corrupted",
			len(submissionIDs), epochID)
	}

	return projectSubmissions
}

func (wm *WindowManager) GetActiveCount() int {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return len(wm.activeWindows)
}

// getKeyBuilder returns a cached KeyBuilder for the given data market
func (wm *WindowManager) getKeyBuilder(dataMarket string) *rediskeys.KeyBuilder {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if kb, exists := wm.keyBuilders[dataMarket]; exists {
		return kb
	}

	kb := rediskeys.NewKeyBuilder(wm.protocolState, dataMarket)
	wm.keyBuilders[dataMarket] = kb
	return kb
}

func (wm *WindowManager) IsWindowOpen(dataMarket string, epochID *big.Int) bool {
	key := fmt.Sprintf("%s:%s", dataMarket, epochID.String())
	wm.mu.RLock()
	_, exists := wm.activeWindows[key]
	wm.mu.RUnlock()
	return exists
}

func (wm *WindowManager) Shutdown() {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Stop all timers
	for key, window := range wm.activeWindows {
		window.Timer.Stop()
		close(window.Done)
		delete(wm.activeWindows, key)
	}
}
