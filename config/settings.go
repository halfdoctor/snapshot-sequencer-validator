package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	rpchelper "github.com/powerloom/go-rpc-helper"
	"github.com/powerloom/go-rpc-helper/reporting"
	log "github.com/sirupsen/logrus"
)

// Settings holds all configuration for the decentralized sequencer
type Settings struct {
	// Core Identity
	SequencerID string

	// Ethereum RPC Configuration
	RPCNodes                 []string // Primary RPC nodes for load balancing
	ArchiveRPCNodes          []string // Archive nodes for historical queries
	ProtocolStateContract    string   // Protocol state contract address (manages identities)
	NewProtocolStateContract string   // NEW ProtocolState contract address (VPA-enabled)
	ChainID                  int64

	// Data Market Configuration
	DataMarketAddresses []string         // String addresses
	DataMarketContracts []common.Address // Parsed common.Address types
	// DATA_SOURCES removed - project IDs generated directly from contract addresses

	// Redis Configuration
	RedisHost     string
	RedisPort     string
	RedisDB       int
	RedisPassword string

	// P2P Network Configuration
	P2PPort        int
	P2PPrivateKey  string   // Hex-encoded private key
	P2PPublicIP    string   // Public IP for NAT traversal
	BootstrapPeers []string // Bootstrap peer multiaddrs
	Rendezvous     string   // Rendezvous point for discovery

	// Connection Manager
	ConnManagerLowWater  int
	ConnManagerHighWater int

	// Gossipsub Configuration
	GossipsubHeartbeat time.Duration
	GossipsubParams    map[string]interface{} // Additional gossipsub parameters

	// Level 1 Finalization Delay Configuration
	// Used when snapshot commit/reveal windows are disabled (both zero).
	// When commit/reveal is enabled, Level 1 finalization triggers when snapshot reveal closes.
	// When commit/reveal is disabled, Level 1 finalization triggers after this delay.
	Level1FinalizationDelay time.Duration
	MaxConcurrentWindows    int
	WindowCleanupInterval   time.Duration

	// Event Monitoring
	EventPollInterval   time.Duration
	EventStartBlock     uint64
	EventBlockBatchSize uint64
	BlockFetchTimeout   time.Duration
	ContractABIPath     string

	// Identity & Verification
	// Snapshotter identities managed by protocol state contract only
	SkipIdentityVerification bool     // Skip verification for testing
	FullNodeAddresses        []string // Addresses that are full nodes
	FlaggedSnapshottersCheck bool
	VerificationCacheTTL     time.Duration

	// Dequeuer Configuration
	DequeueWorkers         int
	DequeueBatchSize       int
	DequeueTimeout         time.Duration
	MaxSubmissionsPerEpoch int

	// Deduplication Configuration
	DedupEnabled        bool
	DedupLocalCacheSize int
	DedupTTL            time.Duration

	// Component Toggles
	EnableListener         bool
	EnableDequeuer         bool
	EnableFinalizer        bool
	EnableBatchAggregation bool // P2P exchange of finalized batches for aggregation
	EnableEventMonitor     bool

	// Finalizer Configuration
	FinalizerWorkers      int
	FinalizationBatchSize int

	// Aggregation Configuration
	AggregationWindowDuration time.Duration // Time to wait for validator batches before aggregating (Level 2)

	// Validator Priority Assignment (VPA) Configuration
	ValidatorAddress        string // This validator's address for priority checking
	EnableOnChainSubmission bool   // Enable submission to ProtocolState contract

	// VPA Event Monitoring Configuration
	VPAContractAddress  string // VPA contract address for priority monitoring
	VPAValidatorAddress string // This validator's address for VPA client

	// New Contract Submission Configuration
	RelayerPyEndpoint string // relayer-py service endpoint for new contract submissions
	UseNewContracts   bool   // Enable submission to new VPA-enabled contracts
	NewProtocolState  string // New ProtocolState contract address
	NewDataMarket     string // New DataMarket contract address

	// Stream Configuration for Deterministic Aggregation
	StreamConsumerGroup string        // Consumer group name for aggregator
	StreamConsumerName  string        // Consumer instance name
	StreamReadBlock     time.Duration // Block timeout for stream reads
	StreamBatchSize     int           // Batch size for stream reads
	StreamIdleTimeout   time.Duration // Idle timeout for stream consumers

	// Slot Validation Configuration
	EnableSlotValidation bool

	// Protocol State Cacher Configuration
	EnableProtocolStateCacher bool
	SlotSyncInterval          time.Duration // Fallback cold sync interval
	SlotSyncBatchSize         int           // Batch size for slot fetching // Validate snapshotter addresses against protocol state cache (requires protocol-state-cacher)

	// IPFS Configuration
	IPFSAPI string // IPFS API endpoint (e.g., "/ip4/127.0.0.1/tcp/5001")

	// API Configuration
	APIHost      string
	APIPort      int
	APIAuthToken string

	// Gossipsub Topic Configuration
	GossipsubSnapshotSubmissionPrefix string // Base prefix for snapshot submission topics
	GossipsubFinalizedBatchPrefix     string // Base prefix for finalized batch topics
	GossipsubValidatorPresenceTopic   string // Validator presence/heartbeat topic
	GossipsubConsensusVotesTopic      string // Consensus voting topic
	GossipsubConsensusProposalsTopic  string // Consensus proposals topic

	// Monitoring & Debugging
	SlackWebhookURL string
	MetricsEnabled  bool
	MetricsPort     int
	LogLevel        string
	DebugMode       bool

	// Performance Tuning
	BatchProcessingTimeout time.Duration
	ContractQueryTimeout   time.Duration
}

var (
	// SettingsObj is the global settings instance
	SettingsObj *Settings
)

// LoadConfig loads configuration from environment variables
func LoadConfig() error {
	SettingsObj = &Settings{
		// Core Identity
		SequencerID: getEnv("SEQUENCER_ID", "unified-sequencer-1"),

		// Ethereum RPC Configuration
		// ChainID will be set by loadRPCConfig() from RPC or env var

		// Redis Configuration - Read directly from env
		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisDB:       getEnvAsInt("REDIS_DB", 0),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		// P2P Network Configuration
		P2PPort:       getEnvAsInt("P2P_PORT", 9001),
		P2PPrivateKey: getEnv("PRIVATE_KEY", ""),
		P2PPublicIP:   getEnv("PUBLIC_IP", ""),
		Rendezvous:    getEnv("RENDEZVOUS_POINT", "powerloom-snapshot-sequencer-network"),

		// Connection Manager
		// CRITICAL: DSV nodes serve thousands of snapshotter peers (local collectors)
		// Default limits are too low - need much higher limits to prevent pruning publishers
		// LowWater: minimum connections to maintain (should be high enough for all active publishers)
		// HighWater: maximum before pruning starts (should accommodate peak load)
		ConnManagerLowWater:  getEnvAsInt("CONN_MANAGER_LOW_WATER", 2000),  // Default: 2000 (was 100)
		ConnManagerHighWater: getEnvAsInt("CONN_MANAGER_HIGH_WATER", 5000), // Default: 5000 (was 400)

		// Gossipsub Configuration
		GossipsubHeartbeat: time.Duration(getEnvAsInt("GOSSIPSUB_HEARTBEAT_MS", 700)) * time.Millisecond,
		GossipsubParams:    make(map[string]interface{}),

		// Level 1 Finalization Delay Configuration
		// NOTE: This is used when snapshot commit/reveal windows are disabled (both zero).
		// When commit/reveal is enabled, Level 1 finalization triggers when snapshot reveal closes.
		// When commit/reveal is disabled, Level 1 finalization triggers after this delay.
		// This delay must be BEFORE P1 window closes to allow time for:
		//   - Level 1 local finalization to complete
		//   - Level 2 network-wide aggregation to complete
		//   - Priority 1 validator to commit on-chain during P1 window
		Level1FinalizationDelay: time.Duration(getEnvAsInt("LEVEL1_FINALIZATION_DELAY_SECONDS", 10)) * time.Second,
		MaxConcurrentWindows:    getEnvAsInt("MAX_CONCURRENT_WINDOWS", 100),
		WindowCleanupInterval:   5 * time.Minute,

		// Event Monitoring
		EventPollInterval:   time.Duration(getEnvAsInt("EVENT_POLL_INTERVAL", 12)) * time.Second,
		EventStartBlock:     uint64(getEnvAsInt("EVENT_START_BLOCK", 0)),
		EventBlockBatchSize: uint64(getEnvAsInt("EVENT_BLOCK_BATCH_SIZE", 1000)),
		BlockFetchTimeout:   time.Duration(getEnvAsInt("BLOCK_FETCH_TIMEOUT", 30)) * time.Second,
		ContractABIPath:     getEnv("CONTRACT_ABI_PATH", "./abi/ProtocolContract.json"),

		// Identity & Verification
		SkipIdentityVerification: getBoolEnv("SKIP_IDENTITY_VERIFICATION", false),
		FlaggedSnapshottersCheck: getBoolEnv("CHECK_FLAGGED_SNAPSHOTTERS", true),
		VerificationCacheTTL:     time.Duration(getEnvAsInt("VERIFICATION_CACHE_TTL", 600)) * time.Second,

		// Dequeuer Configuration
		DequeueWorkers:         getEnvAsInt("DEQUEUER_WORKERS", 5),
		DequeueBatchSize:       getEnvAsInt("DEQUEUE_BATCH_SIZE", 10),
		DequeueTimeout:         5 * time.Second,
		MaxSubmissionsPerEpoch: getEnvAsInt("MAX_SUBMISSIONS_PER_EPOCH", 100),

		// Deduplication Configuration
		DedupEnabled:        getBoolEnv("DEDUP_ENABLED", true),
		DedupLocalCacheSize: getEnvAsInt("DEDUP_LOCAL_CACHE_SIZE", 10000),
		DedupTTL:            time.Duration(getEnvAsInt("DEDUP_TTL_SECONDS", 7200)) * time.Second,

		// Component Toggles
		EnableListener:         getBoolEnv("ENABLE_LISTENER", true),
		EnableDequeuer:         getBoolEnv("ENABLE_DEQUEUER", true),
		EnableFinalizer:        getBoolEnv("ENABLE_FINALIZER", true),
		EnableBatchAggregation: getBoolEnv("ENABLE_BATCH_AGGREGATION", true),
		EnableEventMonitor:     getBoolEnv("ENABLE_EVENT_MONITOR", false),

		// Finalizer Configuration
		FinalizerWorkers:      getEnvAsInt("FINALIZER_WORKERS", 5),
		FinalizationBatchSize: getEnvAsInt("FINALIZATION_BATCH_SIZE", 20),

		// Aggregation Configuration
		AggregationWindowDuration: time.Duration(getEnvAsInt("AGGREGATION_WINDOW_SECONDS", 20)) * time.Second,

		// Validator Priority Assignment (VPA) Configuration
		ValidatorAddress:        getEnv("VALIDATOR_ADDRESS", ""),
		EnableOnChainSubmission: getBoolEnv("ENABLE_ONCHAIN_SUBMISSION", false),

		// VPA Event Monitoring Configuration
		VPAContractAddress:  getEnv("VPA_CONTRACT_ADDRESS", ""),
		VPAValidatorAddress: getEnv("VPA_VALIDATOR_ADDRESS", ""),

		// New Contract Submission Configuration
		RelayerPyEndpoint: getEnv("RELAYER_PY_ENDPOINT", ""),
		UseNewContracts:   getBoolEnv("USE_NEW_CONTRACTS", false),
		NewProtocolState:  getEnv("NEW_PROTOCOL_STATE_CONTRACT", ""),
		NewDataMarket:     getEnv("NEW_DATA_MARKET_CONTRACT", ""),

		// Stream Configuration for Deterministic Aggregation
		StreamConsumerGroup: getEnv("STREAM_CONSUMER_GROUP", "aggregator-group"),
		StreamConsumerName:  getEnv("STREAM_CONSUMER_NAME", "aggregator-instance"),
		StreamReadBlock:     time.Duration(getEnvAsInt("STREAM_READ_BLOCK_MS", 2000)) * time.Millisecond,
		StreamBatchSize:     getEnvAsInt("STREAM_BATCH_SIZE", 10),
		StreamIdleTimeout:   time.Duration(getEnvAsInt("STREAM_IDLE_TIMEOUT_MS", 30000)) * time.Millisecond,

		// Slot Validation Configuration
		EnableSlotValidation: getBoolEnv("ENABLE_SLOT_VALIDATION", false),

		// Protocol State Cacher Configuration
		EnableProtocolStateCacher: getBoolEnv("ENABLE_PROTOCOL_STATE_CACHER", false),
		SlotSyncInterval:          time.Duration(getEnvAsInt("SLOT_SYNC_INTERVAL_SECONDS", 3600)) * time.Second,
		SlotSyncBatchSize:         getEnvAsInt("SLOT_SYNC_BATCH_SIZE", 20),

		// API Configuration
		APIHost:      getEnv("API_HOST", "0.0.0.0"),
		APIPort:      getEnvAsInt("API_PORT", 8080),
		APIAuthToken: getEnv("API_AUTH_TOKEN", ""),

		// Gossipsub Topic Configuration
		GossipsubSnapshotSubmissionPrefix: getEnv("GOSSIPSUB_SNAPSHOT_SUBMISSION_PREFIX", "/powerloom/snapshot-submissions"),
		GossipsubFinalizedBatchPrefix:     getEnv("GOSSIPSUB_FINALIZED_BATCH_PREFIX", "/powerloom/finalized-batches"),
		GossipsubValidatorPresenceTopic:   getEnv("GOSSIPSUB_VALIDATOR_PRESENCE_TOPIC", "/powerloom/validator/presence"),
		GossipsubConsensusVotesTopic:      getEnv("GOSSIPSUB_CONSENSUS_VOTES_TOPIC", "/powerloom/consensus/votes"),
		GossipsubConsensusProposalsTopic:  getEnv("GOSSIPSUB_CONSENSUS_PROPOSALS_TOPIC", "/powerloom/consensus/proposals"),

		// Monitoring & Debugging
		SlackWebhookURL: getEnv("SLACK_WEBHOOK_URL", ""),
		MetricsEnabled:  getBoolEnv("METRICS_ENABLED", false),
		MetricsPort:     getEnvAsInt("METRICS_PORT", 9090),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		DebugMode:       getBoolEnv("DEBUG_MODE", false),

		// Performance Tuning
		BatchProcessingTimeout: 5 * time.Minute,
		ContractQueryTimeout:   30 * time.Second,

		// IPFS Configuration
		IPFSAPI: getEnv("IPFS_HOST", ""),

		// Contract Addresses
		ProtocolStateContract:    getEnv("PROTOCOL_STATE_CONTRACT", ""),
		NewProtocolStateContract: getEnv("NEW_PROTOCOL_STATE_CONTRACT", ""),
	}

	// Load complex configurations that require additional parsing
	if err := loadRPCConfig(); err != nil {
		return fmt.Errorf("failed to load RPC config: %w", err)
	}

	if err := loadDataMarkets(); err != nil {
		return fmt.Errorf("failed to load data markets: %w", err)
	}

	loadBootstrapPeers()
	loadFullNodeAddresses()

	// DATA_SOURCES removed - project IDs generated directly from contract addresses

	// Configure logging
	configureLogging()

	// Validate configuration
	if err := validateConfig(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Log configuration summary
	logConfigSummary()

	return nil
}

// loadRPCConfig loads RPC node configuration
func loadRPCConfig() error {
	// Load Powerloom Protocol Chain RPC nodes (all interactions in this component)
	powerloomNodesStr := getEnv("POWERLOOM_RPC_NODES", "")
	if powerloomNodesStr != "" {
		// Support comma-separated format (simplest)
		if !strings.HasPrefix(powerloomNodesStr, "[") {
			SettingsObj.RPCNodes = strings.Split(powerloomNodesStr, ",")
		} else {
			// JSON array format
			if err := json.Unmarshal([]byte(powerloomNodesStr), &SettingsObj.RPCNodes); err != nil {
				return fmt.Errorf("failed to parse POWERLOOM_RPC_NODES as JSON array: %w", err)
			}
		}
	}

	// Load Powerloom Archive nodes (optional)
	powerloomArchiveStr := getEnv("POWERLOOM_ARCHIVE_RPC_NODES", "")
	if powerloomArchiveStr != "" && powerloomArchiveStr != "[]" {
		if !strings.HasPrefix(powerloomArchiveStr, "[") {
			SettingsObj.ArchiveRPCNodes = strings.Split(powerloomArchiveStr, ",")
		} else {
			json.Unmarshal([]byte(powerloomArchiveStr), &SettingsObj.ArchiveRPCNodes)
		}
	}

	// Clean quotes from URLs
	for i := range SettingsObj.RPCNodes {
		SettingsObj.RPCNodes[i] = strings.Trim(SettingsObj.RPCNodes[i], "\" ")
	}
	for i := range SettingsObj.ArchiveRPCNodes {
		SettingsObj.ArchiveRPCNodes[i] = strings.Trim(SettingsObj.ArchiveRPCNodes[i], "\" ")
	}

	// Fetch chain ID from RPC if nodes are configured
	if len(SettingsObj.RPCNodes) > 0 {
		log.Infof("Fetching chain ID from RPC: %s", SettingsObj.RPCNodes[0])
		if chainID, err := fetchChainIDFromRPC(SettingsObj.RPCNodes[0]); err == nil {
			SettingsObj.ChainID = chainID
			log.Infof("✅ Chain ID fetched from RPC: %d", chainID)
		} else {
			log.Warnf("❌ Failed to fetch chain ID from RPC (%v), falling back to CHAIN_ID env var", err)
			SettingsObj.ChainID = int64(getEnvAsInt("CHAIN_ID", 1))
		}
	} else {
		log.Warnf("No RPC nodes configured (POWERLOOM_RPC_NODES empty), using CHAIN_ID env var")
		SettingsObj.ChainID = int64(getEnvAsInt("CHAIN_ID", 1))
	}

	log.Infof("Final Chain ID for EIP-712: %d", SettingsObj.ChainID)

	return nil
}

// loadDataMarkets loads data market addresses
func loadDataMarkets() error {
	dataMarketsStr := getEnv("DATA_MARKET_ADDRESSES", "")

	if dataMarketsStr == "" {
		return nil
	}

	// Support comma-separated format (simplest and most reliable)
	// Example: 0x123...,0x456...,0x789...
	if !strings.HasPrefix(dataMarketsStr, "[") {
		SettingsObj.DataMarketAddresses = strings.Split(dataMarketsStr, ",")
	} else {
		// If it starts with [, require proper JSON with quoted strings
		// Example: ["0x123...","0x456..."]
		if err := json.Unmarshal([]byte(dataMarketsStr), &SettingsObj.DataMarketAddresses); err != nil {
			return fmt.Errorf("failed to parse DATA_MARKET_ADDRESSES as JSON array (addresses must be quoted): %w", err)
		}
	}

	// Clean and convert to common.Address
	SettingsObj.DataMarketContracts = make([]common.Address, 0, len(SettingsObj.DataMarketAddresses))
	for _, addr := range SettingsObj.DataMarketAddresses {
		addr = strings.TrimSpace(strings.Trim(addr, "\""))
		if addr != "" && addr != "0x0" {
			SettingsObj.DataMarketContracts = append(SettingsObj.DataMarketContracts, common.HexToAddress(addr))
		}
	}

	return nil
}

// loadBootstrapPeers loads bootstrap peer addresses
func loadBootstrapPeers() {
	// DEBUG: Show what environment variables are actually set
	bootstrapPeersEnv := getEnv("BOOTSTRAP_PEERS", "")
	bootstrapMultiEnv := getEnv("BOOTSTRAP_MULTIADDR", "")
	log.Printf("DEBUG: BOOTSTRAP_PEERS env: '%s'", bootstrapPeersEnv)
	log.Printf("DEBUG: BOOTSTRAP_MULTIADDR env: '%s'", bootstrapMultiEnv)

	// Try BOOTSTRAP_PEERS first (can be JSON array or comma-separated)
	peersStr := bootstrapPeersEnv
	if peersStr != "" {
		if strings.HasPrefix(peersStr, "[") {
			json.Unmarshal([]byte(peersStr), &SettingsObj.BootstrapPeers)
		} else {
			SettingsObj.BootstrapPeers = strings.Split(peersStr, ",")
		}
	} else {
		// Fallback to BOOTSTRAP_MULTIADDR (backward compatibility)
		multiPeer := bootstrapMultiEnv
		if multiPeer != "" {
			// Support comma-separated bootstrap addresses in BOOTSTRAP_MULTIADDR
			SettingsObj.BootstrapPeers = strings.Split(multiPeer, ",")
		}
	}

	// Clean the peer addresses
	for i := range SettingsObj.BootstrapPeers {
		SettingsObj.BootstrapPeers[i] = strings.TrimSpace(strings.Trim(SettingsObj.BootstrapPeers[i], "\""))
		log.Printf("DEBUG: Bootstrap peer[%d]: '%s'", i, SettingsObj.BootstrapPeers[i])
	}
	log.Printf("DEBUG: Total bootstrap peers: %d", len(SettingsObj.BootstrapPeers))
}

// loadFullNodeAddresses loads full node addresses
func loadFullNodeAddresses() {
	fullNodesStr := getEnv("FULL_NODE_ADDRESSES", "")

	if strings.HasPrefix(fullNodesStr, "[") {
		json.Unmarshal([]byte(fullNodesStr), &SettingsObj.FullNodeAddresses)
	} else if fullNodesStr != "" {
		SettingsObj.FullNodeAddresses = strings.Split(fullNodesStr, ",")
	}

	// Clean and lowercase
	for i := range SettingsObj.FullNodeAddresses {
		SettingsObj.FullNodeAddresses[i] = strings.ToLower(strings.TrimSpace(SettingsObj.FullNodeAddresses[i]))
	}
}

// configureLogging sets up the logger based on configuration
func configureLogging() {
	// Set log level
	switch strings.ToLower(SettingsObj.LogLevel) {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn", "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	// Override with debug mode
	if SettingsObj.DebugMode {
		log.SetLevel(log.DebugLevel)
	}

	// Set formatter
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})
}

// validateConfig validates the loaded configuration
func validateConfig() error {
	// Check required fields for enabled components
	if SettingsObj.EnableListener || SettingsObj.EnableBatchAggregation {
		if len(SettingsObj.BootstrapPeers) == 0 {
			log.Warn("No bootstrap peers configured - P2P networking may not work")
		}
	}

	if SettingsObj.EnableEventMonitor {
		if SettingsObj.ProtocolStateContract == "" {
			return fmt.Errorf("PROTOCOL_STATE_CONTRACT required when event monitor is enabled")
		}
		if len(SettingsObj.RPCNodes) == 0 {
			return fmt.Errorf("POWERLOOM_RPC_NODES required when event monitor is enabled")
		}
		if len(SettingsObj.DataMarketAddresses) == 0 {
			log.Warn("No data markets configured - event monitor will not track any markets")
		}
	}

	if SettingsObj.EnableDequeuer {
		if SettingsObj.RedisHost == "" {
			return fmt.Errorf("redis configuration required when dequeuer is enabled")
		}
	}

	// Validate stream configuration (mandatory for deterministic aggregation)
	if SettingsObj.StreamConsumerGroup == "" {
		return fmt.Errorf("STREAM_CONSUMER_GROUP required for deterministic aggregation")
	}
	if SettingsObj.StreamConsumerName == "" {
		return fmt.Errorf("STREAM_CONSUMER_NAME required for deterministic aggregation")
	}
	if SettingsObj.StreamBatchSize <= 0 {
		return fmt.Errorf("STREAM_BATCH_SIZE must be greater than 0")
	}
	if SettingsObj.StreamReadBlock <= 0 {
		return fmt.Errorf("STREAM_READ_BLOCK_MS must be greater than 0")
	}
	if SettingsObj.StreamIdleTimeout <= 0 {
		return fmt.Errorf("STREAM_IDLE_TIMEOUT_MS must be greater than 0")
	}

	log.WithFields(log.Fields{
		"consumer_group": SettingsObj.StreamConsumerGroup,
		"consumer_name":  SettingsObj.StreamConsumerName,
		"batch_size":     SettingsObj.StreamBatchSize,
		"read_block":     SettingsObj.StreamReadBlock,
		"idle_timeout":   SettingsObj.StreamIdleTimeout,
	}).Info("Stream notifications configured (mandatory for deterministic aggregation)")

	return nil
}

// logConfigSummary logs a summary of the configuration
func logConfigSummary() {
	log.Info("=== Configuration Loaded ===")
	log.Infof("Sequencer ID: %s", SettingsObj.SequencerID)
	log.Infof("Components: Listener=%v, Dequeuer=%v, Finalizer=%v, BatchAggregation=%v, EventMonitor=%v",
		SettingsObj.EnableListener, SettingsObj.EnableDequeuer, SettingsObj.EnableFinalizer,
		SettingsObj.EnableBatchAggregation, SettingsObj.EnableEventMonitor)

	if len(SettingsObj.RPCNodes) > 0 {
		log.Infof("RPC Nodes: %d configured", len(SettingsObj.RPCNodes))
	}

	if SettingsObj.ProtocolStateContract != "" {
		log.Infof("Protocol State Contract: %s", SettingsObj.ProtocolStateContract)
	}

	if len(SettingsObj.DataMarketContracts) > 0 {
		log.Infof("Data Markets: %d configured", len(SettingsObj.DataMarketContracts))
	}

	log.Infof("Redis: %s:%s (DB %d)", SettingsObj.RedisHost, SettingsObj.RedisPort, SettingsObj.RedisDB)
	log.Infof("P2P: Port %d, Bootstrap peers: %d", SettingsObj.P2PPort, len(SettingsObj.BootstrapPeers))

	if SettingsObj.DedupEnabled {
		log.Infof("Deduplication: Enabled (TTL: %v, Cache: %d)", SettingsObj.DedupTTL, SettingsObj.DedupLocalCacheSize)
	}

	log.Info("============================")
}

// Helper functions

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		value = strings.ToLower(value)
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

// IsFullNode checks if an address is configured as a full node
func IsFullNode(address string) bool {
	address = strings.ToLower(address)
	for _, fullNode := range SettingsObj.FullNodeAddresses {
		if strings.ToLower(fullNode) == address {
			return true
		}
	}
	return false
}

// IsValidDataMarket checks if an address is a configured data market
func IsValidDataMarket(address string) bool {
	address = strings.ToLower(address)
	for _, market := range SettingsObj.DataMarketAddresses {
		if strings.ToLower(market) == address {
			return true
		}
	}
	return false
}

// ToRPCConfig converts Settings to go-rpc-helper RPCConfig
func (s *Settings) ToRPCConfig() *rpchelper.RPCConfig {
	config := &rpchelper.RPCConfig{
		Nodes: func() []rpchelper.NodeConfig {
			var nodes []rpchelper.NodeConfig
			for _, url := range s.RPCNodes {
				nodes = append(nodes, rpchelper.NodeConfig{URL: url})
			}
			return nodes
		}(),
		ArchiveNodes: func() []rpchelper.NodeConfig {
			var nodes []rpchelper.NodeConfig
			for _, url := range s.ArchiveRPCNodes {
				nodes = append(nodes, rpchelper.NodeConfig{URL: url})
			}
			return nodes
		}(),
		MaxRetries:     3,
		RetryDelay:     500 * time.Millisecond,
		MaxRetryDelay:  30 * time.Second,
		RequestTimeout: 30 * time.Second,
	}

	// Configure webhook if SlackWebhookURL is provided
	if s.SlackWebhookURL != "" {
		config.WebhookConfig = &reporting.WebhookConfig{
			URL:     s.SlackWebhookURL,
			Timeout: 30 * time.Second,
			Retries: 3,
		}
	}

	return config
}

// fetchChainIDFromRPC fetches the chain ID from an RPC endpoint
func fetchChainIDFromRPC(rpcURL string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to RPC: %w", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch chain ID: %w", err)
	}

	return chainID.Int64(), nil
}

// GetSnapshotSubmissionTopics returns the discovery and submission topics
func (s *Settings) GetSnapshotSubmissionTopics() (discoveryTopic, submissionsTopic string) {
	return s.GossipsubSnapshotSubmissionPrefix + "/0", s.GossipsubSnapshotSubmissionPrefix + "/all"
}

// GetFinalizedBatchTopics returns the discovery and batch topics
func (s *Settings) GetFinalizedBatchTopics() (discoveryTopic, batchTopic string) {
	return s.GossipsubFinalizedBatchPrefix + "/0", s.GossipsubFinalizedBatchPrefix + "/all"
}
