package vpa

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	abiloader "github.com/powerloom/snapshot-sequencer-validator/pkgs/abi"
	rediskeys "github.com/powerloom/snapshot-sequencer-validator/pkgs/redis"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// ValidatorPriorityAssigner binds to the VPA contract
type ValidatorPriorityAssigner struct {
	client       *ethclient.Client
	contractAddr common.Address
	abi          abi.ABI
	validator    common.Address
}

// PriorityInfo holds validator priority information
type PriorityInfo struct {
	EpochID     uint64
	Priority    int
	CanSubmit   bool
	WindowStart time.Time
	WindowEnd   time.Time
}

// ValidatorPriority represents a validator's priority assignment
type ValidatorPriority struct {
	ValidatorID string `json:"validatorId"`
	Priority    int    `json:"priority"`
	CanSubmit   bool   `json:"canSubmit"`
}

// PriorityMetadata holds metadata about priority assignment
type PriorityMetadata struct {
	EpochID        uint64    `json:"epochId"`
	Seed           string    `json:"seed"`
	Timestamp      time.Time `json:"timestamp"`
	ValidatorCount int       `json:"validatorCount"`
	DataMarket     string    `json:"dataMarket"`
}

// CachedPriorities holds all cached priorities for an epoch
type CachedPriorities struct {
	EpochID      uint64           `json:"epochId"`
	Metadata     PriorityMetadata `json:"metadata"`
	Priorities   map[string]int   `json:"priorities"` // validatorID -> priority
	TopValidator string           `json:"topValidator"`
	CachedAt     time.Time        `json:"cachedAt"`
}

// PriorityCachingClient wraps VPA client with Redis caching
type PriorityCachingClient struct {
	*ValidatorPriorityAssigner
	redisClient       *redis.Client
	keyBuilder        *rediskeys.KeyBuilder
	cacheTTL          time.Duration
	logger            *logrus.Entry
	cachedNodeId      *uint64 // Cached validator nodeId (doesn't change at runtime)
	nodeIdMutex       sync.RWMutex
	protocolStateAddr common.Address // ProtocolState contract address for getPriorities()
	protocolStateABI  abi.ABI        // ProtocolState ABI for getPriorities()
}

// NewPriorityCachingClient creates a new VPA client with Redis caching
// protocolState and dataMarket are used for Redis key building
// newProtocolState is used for getPriorities() contract calls (if empty, falls back to protocolState)
func NewPriorityCachingClient(rpcURL, contractAddr, validatorAddr string,
	redisClient *redis.Client, protocolState, dataMarket, newProtocolState string) (*PriorityCachingClient, error) {

	vpaClient, err := NewValidatorPriorityAssigner(rpcURL, contractAddr, validatorAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create VPA client: %w", err)
	}

	// Use passed addresses for Redis keys
	keyBuilder := rediskeys.NewKeyBuilder(protocolState, dataMarket)

	// Load ProtocolState ABI for getPriorities() calls
	protocolStateABI, err := abiloader.LoadABI("PowerloomProtocolState.abi.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load ProtocolState ABI: %w", err)
	}

	// Use newProtocolState for contract calls if provided, otherwise fall back to protocolState
	contractProtocolState := newProtocolState
	if contractProtocolState == "" {
		contractProtocolState = protocolState
	}

	protocolStateAddr := common.HexToAddress(contractProtocolState)
	if protocolStateAddr == (common.Address{}) {
		return nil, fmt.Errorf("invalid ProtocolState address: %s", contractProtocolState)
	}

	return &PriorityCachingClient{
		ValidatorPriorityAssigner: vpaClient,
		redisClient:               redisClient,
		keyBuilder:                keyBuilder,
		cacheTTL:                  24 * time.Hour, // Cache for 24 hours
		logger:                    logrus.WithField("component", "vpa-caching-client"),
		protocolStateAddr:         protocolStateAddr,
		protocolStateABI:          protocolStateABI,
	}, nil
}

// NewValidatorPriorityAssigner creates a new VPA contract client
func NewValidatorPriorityAssigner(rpcURL string, contractAddr string, validatorAddr string) (*ValidatorPriorityAssigner, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}

	if !common.IsHexAddress(contractAddr) {
		return nil, fmt.Errorf("invalid contract address: %s", contractAddr)
	}
	if !common.IsHexAddress(validatorAddr) {
		return nil, fmt.Errorf("invalid validator address: %s", validatorAddr)
	}

	// Load VPA ABI from file using standardized path resolution
	vpaABI, err := abiloader.LoadABI("ValidatorPriorityAssigner.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load VPA ABI: %w", err)
	}

	return &ValidatorPriorityAssigner{
		client:       client,
		contractAddr: common.HexToAddress(contractAddr),
		abi:          vpaABI,
		validator:    common.HexToAddress(validatorAddr),
	}, nil
}

// CanValidatorSubmit checks if this validator can submit for the given epoch and data market
func (vpa *ValidatorPriorityAssigner) CanValidatorSubmit(ctx context.Context, dataMarketAddr string, epochID uint64) (bool, error) {
	if !common.IsHexAddress(dataMarketAddr) {
		return false, fmt.Errorf("invalid data market address: %s", dataMarketAddr)
	}

	// Create call data for canValidatorSubmit
	data, err := vpa.abi.Pack("canValidatorSubmit",
		common.HexToAddress(dataMarketAddr),
		big.NewInt(int64(epochID)),
		vpa.validator)
	if err != nil {
		return false, fmt.Errorf("failed to pack canValidatorSubmit call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &vpa.contractAddr,
		Data: data,
	}
	result, err := vpa.client.CallContract(ctx, msg, nil)
	if err != nil {
		// Log detailed error information for debugging
		logrus.WithError(err).WithFields(logrus.Fields{
			"epoch":        epochID,
			"data_market":  dataMarketAddr,
			"vpa_contract": vpa.contractAddr.Hex(),
			"validator":    vpa.validator.Hex(),
		}).Debug("CanValidatorSubmit contract call failed")
		return false, fmt.Errorf("failed to call canValidatorSubmit: %w", err)
	}

	// Unpack the result
	var canSubmit bool
	err = vpa.abi.UnpackIntoInterface(&canSubmit, "canValidatorSubmit", result)
	if err != nil {
		return false, fmt.Errorf("failed to unpack canValidatorSubmit result: %w", err)
	}

	return canSubmit, nil
}

// GetMyPriority gets this validator's priority for the given epoch and data market
func (vpa *ValidatorPriorityAssigner) GetMyPriority(ctx context.Context, dataMarketAddr string, epochID uint64) (int, error) {
	if !common.IsHexAddress(dataMarketAddr) {
		return 0, fmt.Errorf("invalid data market address: %s", dataMarketAddr)
	}

	// First get the validator ID from the ValidatorState contract
	validatorID, err := vpa.getValidatorID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get validator ID: %w", err)
	}

	// Create call data for getHistoricalPriority
	data, err := vpa.abi.Pack("getHistoricalPriority",
		common.HexToAddress(dataMarketAddr),
		big.NewInt(int64(epochID)),
		big.NewInt(int64(validatorID)))
	if err != nil {
		return 0, fmt.Errorf("failed to pack getHistoricalPriority call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &vpa.contractAddr,
		Data: data,
	}
	result, err := vpa.client.CallContract(ctx, msg, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to call getHistoricalPriority: %w", err)
	}

	// Unpack the result
	var priority *big.Int
	err = vpa.abi.UnpackIntoInterface(&priority, "getHistoricalPriority", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack getHistoricalPriority result: %w", err)
	}

	if priority == nil {
		return 0, fmt.Errorf("no priority assigned for validator %d in epoch %d", validatorID, epochID)
	}

	return int(priority.Int64()), nil
}

// getValidatorID gets the validator nodeId from the ValidatorState contract
// Calls ValidatorState.getNodeIdForValidator(validatorAddress) via VPA contract's validatorState reference
func (vpa *ValidatorPriorityAssigner) getValidatorID(ctx context.Context) (uint64, error) {
	// First, get ValidatorState contract address from VPA contract
	validatorStateAddr, err := vpa.getValidatorStateAddress(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get ValidatorState address: %w", err)
	}

	if validatorStateAddr == (common.Address{}) {
		return 0, fmt.Errorf("ValidatorState address is zero")
	}

	// Call ValidatorState.getNodeIdForValidator(validatorAddress)
	nodeId, err := vpa.callValidatorStateGetNodeId(ctx, validatorStateAddr)
	if err != nil {
		return 0, fmt.Errorf("failed to get nodeId from ValidatorState: %w", err)
	}

	if nodeId == 0 {
		return 0, fmt.Errorf("validator address %s is not assigned to any node", vpa.validator.Hex())
	}

	return nodeId, nil
}

// getValidatorStateAddress gets the ValidatorState contract address from VPA contract
func (vpa *ValidatorPriorityAssigner) getValidatorStateAddress(ctx context.Context) (common.Address, error) {
	// Call VPA contract's validatorState() public variable
	// This is a view function that returns the ValidatorState contract address
	data, err := vpa.abi.Pack("validatorState")
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to pack validatorState call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &vpa.contractAddr,
		Data: data,
	}

	result, err := vpa.client.CallContract(ctx, msg, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to call validatorState(): %w", err)
	}

	// Unpack the result (address)
	var validatorStateAddr common.Address
	err = vpa.abi.UnpackIntoInterface(&validatorStateAddr, "validatorState", result)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to unpack validatorState result: %w", err)
	}

	return validatorStateAddr, nil
}

// callValidatorStateGetNodeId calls ValidatorState.getNodeIdForValidator(validatorAddress)
func (vpa *ValidatorPriorityAssigner) callValidatorStateGetNodeId(ctx context.Context, validatorStateAddr common.Address) (uint64, error) {
	// Minimal ABI for ValidatorState.getNodeIdForValidator(address) -> uint256
	validatorStateABI := `[{
		"inputs": [{"internalType": "address", "name": "validatorAddress", "type": "address"}],
		"name": "getNodeIdForValidator",
		"outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}],
		"stateMutability": "view",
		"type": "function"
	}]`

	parsedABI, err := abi.JSON(strings.NewReader(validatorStateABI))
	if err != nil {
		return 0, fmt.Errorf("failed to parse ValidatorState ABI: %w", err)
	}

	// Pack the function call
	data, err := parsedABI.Pack("getNodeIdForValidator", vpa.validator)
	if err != nil {
		return 0, fmt.Errorf("failed to pack getNodeIdForValidator call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &validatorStateAddr,
		Data: data,
	}

	result, err := vpa.client.CallContract(ctx, msg, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to call getNodeIdForValidator: %w", err)
	}

	// Unpack the result (uint256)
	var nodeId *big.Int
	err = parsedABI.UnpackIntoInterface(&nodeId, "getNodeIdForValidator", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack getNodeIdForValidator result: %w", err)
	}

	if nodeId == nil || nodeId.Uint64() == 0 {
		return 0, fmt.Errorf("validator address %s is not assigned to any node", vpa.validator.Hex())
	}

	return nodeId.Uint64(), nil
}

// getCachedValidatorID gets the validator nodeId with caching (only queries once)
// Returns 0-based validatorIndex (nodeId - 1) since getPriorities() uses 0-based indices
func (pcc *PriorityCachingClient) getCachedValidatorID(ctx context.Context) (uint64, error) {
	// Check cache first
	pcc.nodeIdMutex.RLock()
	if pcc.cachedNodeId != nil {
		// Convert 1-based nodeId to 0-based validatorIndex
		validatorIndex := *pcc.cachedNodeId - 1
		pcc.nodeIdMutex.RUnlock()
		return validatorIndex, nil
	}
	pcc.nodeIdMutex.RUnlock()

	// Cache miss - query and cache
	pcc.nodeIdMutex.Lock()
	defer pcc.nodeIdMutex.Unlock()

	// Double-check after acquiring write lock (another goroutine might have cached it)
	if pcc.cachedNodeId != nil {
		// Convert 1-based nodeId to 0-based validatorIndex
		return *pcc.cachedNodeId - 1, nil
	}

	// Query validator ID from contract (returns 1-based nodeId)
	nodeId, err := pcc.ValidatorPriorityAssigner.getValidatorID(ctx)
	if err != nil {
		return 0, err
	}

	// Cache the 1-based nodeId
	pcc.cachedNodeId = &nodeId
	pcc.logger.WithFields(logrus.Fields{
		"nodeId":         nodeId,
		"validatorIndex": nodeId - 1, // Show 0-based index in logs
		"validatorAddr":  pcc.validator.Hex(),
	}).Info("Cached validator nodeId (stable for runtime)")

	// Return 0-based validatorIndex
	return nodeId - 1, nil
}

// getPriorityFromProtocolState calls ProtocolState.getPriorities() and looks up priority by validatorIndex
// validatorIndex is already 0-based (converted in getCachedValidatorID)
// NOTE: pcc.protocolStateAddr is the NEW ProtocolState contract (VPA only exists there)
//
//	dataMarketAddr must be the NEW DataMarket address (matching the new protocol state)
func (pcc *PriorityCachingClient) getPriorityFromProtocolState(ctx context.Context, dataMarketAddr string, epochID uint64, validatorIndex uint64) (int, error) {
	// Call ProtocolState.getPriorities(dataMarket, epochId) on NEW ProtocolState contract
	// dataMarketAddr must be the NEW DataMarket address
	data, err := pcc.protocolStateABI.Pack("getPriorities",
		common.HexToAddress(dataMarketAddr),
		big.NewInt(int64(epochID)))
	if err != nil {
		return 0, fmt.Errorf("failed to pack getPriorities call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &pcc.protocolStateAddr,
		Data: data,
	}
	result, err := pcc.client.CallContract(ctx, msg, nil)
	if err != nil {
		pcc.logger.WithError(err).WithFields(logrus.Fields{
			"epochID":           epochID,
			"validatorIndex":    validatorIndex,
			"dataMarketAddr":    dataMarketAddr,
			"protocolStateAddr": pcc.protocolStateAddr.Hex(),
		}).Error("ProtocolState.getPriorities() call reverted - check contract addresses and parameters")
		return 0, fmt.Errorf("failed to call ProtocolState.getPriorities: %w", err)
	}

	// Unpack the result - array of ValidatorIndexPriority structs
	var priorities []struct {
		ValidatorIndex *big.Int `json:"validatorIndex"`
		Priority       *big.Int `json:"priority"`
	}
	err = pcc.protocolStateABI.UnpackIntoInterface(&priorities, "getPriorities", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack getPriorities result: %w", err)
	}

	// Log what we got from the contract
	pcc.logger.WithFields(logrus.Fields{
		"epochID":           epochID,
		"validatorIndex":    validatorIndex,
		"dataMarketAddr":    dataMarketAddr,
		"protocolStateAddr": pcc.protocolStateAddr.Hex(),
		"prioritiesCount":   len(priorities),
	}).Debug("getPriorities() returned priorities array")

	// Log all priorities for debugging
	if len(priorities) > 0 {
		priorityList := make([]map[string]interface{}, 0, len(priorities))
		for _, p := range priorities {
			if p.ValidatorIndex != nil && p.Priority != nil {
				priorityList = append(priorityList, map[string]interface{}{
					"validatorIndex": p.ValidatorIndex.Int64(),
					"priority":       p.Priority.Int64(),
				})
			}
		}
		pcc.logger.WithFields(logrus.Fields{
			"epochID":    epochID,
			"priorities": priorityList,
		}).Debug("All priorities returned from contract")
	}

	// Look up our validatorIndex (0-based) in the priorities array
	validatorIndexBig := big.NewInt(int64(validatorIndex))
	for _, p := range priorities {
		if p.ValidatorIndex != nil && p.ValidatorIndex.Cmp(validatorIndexBig) == 0 {
			if p.Priority != nil {
				priority := int(p.Priority.Int64())
				pcc.logger.WithFields(logrus.Fields{
					"epochID":        epochID,
					"validatorIndex": validatorIndex,
					"priority":       priority,
					"dataMarketAddr": dataMarketAddr,
				}).Info("Found priority for validator")
				return priority, nil
			}
		}
	}

	// ValidatorIndex not found in priorities array = no priority assigned
	pcc.logger.WithFields(logrus.Fields{
		"epochID":         epochID,
		"validatorIndex":  validatorIndex,
		"dataMarketAddr":  dataMarketAddr,
		"prioritiesCount": len(priorities),
	}).Warn("ValidatorIndex not found in priorities array - returning priority 0")
	return 0, nil
}

// GetPriorityInfo gets comprehensive priority information for the validator
func (vpa *ValidatorPriorityAssigner) GetPriorityInfo(ctx context.Context, dataMarketAddr string, epochID uint64) (*PriorityInfo, error) {
	canSubmit, err := vpa.CanValidatorSubmit(ctx, dataMarketAddr, epochID)
	if err != nil {
		return nil, fmt.Errorf("failed to check canValidatorSubmit: %w", err)
	}

	priority, err := vpa.GetMyPriority(ctx, dataMarketAddr, epochID)
	if err != nil {
		return nil, fmt.Errorf("failed to get priority: %w", err)
	}

	// Calculate submission window times (simplified - should get from DataMarket contract)
	now := time.Now()
	windowStart := now.Add(-1 * time.Minute) // Assume window opened 1 minute ago
	windowEnd := now.Add(4 * time.Minute)    // Assume window closes in 4 minutes

	return &PriorityInfo{
		EpochID:     epochID,
		Priority:    priority,
		CanSubmit:   canSubmit,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	}, nil
}

// IsTopPriority checks if this validator is the top priority (priority 1)
func (vpa *ValidatorPriorityAssigner) IsTopPriority(ctx context.Context, dataMarketAddr string, epochID uint64) (bool, error) {
	priority, err := vpa.GetMyPriority(ctx, dataMarketAddr, epochID)
	if err != nil {
		return false, err
	}
	return priority == 1, nil
}

// WaitForSubmissionWindow waits until the validator can submit
// It first checks if the window is already open, and only starts polling if it's not yet open
func (vpa *ValidatorPriorityAssigner) WaitForSubmissionWindow(ctx context.Context, dataMarketAddr string, epochID uint64) error {
	// First check: if window is already open, return immediately
	canSubmit, err := vpa.CanValidatorSubmit(ctx, dataMarketAddr, epochID)
	if err != nil {
		errorMsg := err.Error()
		// If check fails, check if it's because window is closed (expected) vs other error
		if strings.Contains(errorMsg, "Submission window closed") || strings.Contains(errorMsg, "execution reverted") {
			// Window is closed, start polling
			logrus.WithFields(logrus.Fields{
				"epoch":        epochID,
				"data_market":  dataMarketAddr,
				"vpa_contract": vpa.contractAddr.Hex(),
				"error":        errorMsg,
			}).Debug("Submission window not yet open, starting to poll...")
		} else {
			// Other error, log it and return
			logrus.WithError(err).WithFields(logrus.Fields{
				"epoch":        epochID,
				"data_market":  dataMarketAddr,
				"vpa_contract": vpa.contractAddr.Hex(),
			}).Error("❌ CanValidatorSubmit failed with unexpected error")
			return fmt.Errorf("failed to check submission window status: %w", err)
		}
	} else if canSubmit {
		// Window is already open, no need to wait
		logrus.WithFields(logrus.Fields{
			"epoch":       epochID,
			"data_market": dataMarketAddr,
		}).Info("✅ Submission window is already open")
		return nil
	}

	// Window is not open yet, start polling
	ticker := time.NewTicker(2 * time.Second) // Poll every 2 seconds instead of 1 to reduce RPC calls
	defer ticker.Stop()

	// Log first attempt
	logrus.WithFields(logrus.Fields{
		"epoch":       epochID,
		"data_market": dataMarketAddr,
	}).Info("⏳ Waiting for submission window to open...")

	pollCount := 0
	maxPollLogInterval := 10 // Log every 10 polls (20 seconds) to avoid spam
	lastErrorType := ""
	consecutiveErrors := 0
	maxConsecutiveErrors := 30 // Stop after 30 consecutive errors (1 minute) of the same type

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pollCount++
			canSubmit, err := vpa.CanValidatorSubmit(ctx, dataMarketAddr, epochID)
			if err != nil {
				errorMsg := err.Error()
				isWindowClosedError := strings.Contains(errorMsg, "Submission window closed") ||
					strings.Contains(errorMsg, "Submission window not open") ||
					strings.Contains(errorMsg, "execution reverted")

				// Check for fatal errors that mean we should stop polling
				isFatalError := strings.Contains(errorMsg, "Epoch not released") ||
					strings.Contains(errorMsg, "Priorities not assigned") ||
					strings.Contains(errorMsg, "Validator not allowed to submit")

				// Track consecutive errors of the same type
				if errorMsg == lastErrorType {
					consecutiveErrors++
				} else {
					consecutiveErrors = 1
					lastErrorType = errorMsg
				}

				// If fatal error or too many consecutive errors, give up
				if isFatalError || consecutiveErrors >= maxConsecutiveErrors {
					logrus.WithError(err).WithFields(logrus.Fields{
						"epoch":              epochID,
						"data_market":        dataMarketAddr,
						"polls":              pollCount,
						"consecutive_errors": consecutiveErrors,
						"vpa_contract":       vpa.contractAddr.Hex(),
						"is_fatal":           isFatalError,
					}).Error("❌ CanValidatorSubmit failed with fatal/unrecoverable error, stopping polling")
					return fmt.Errorf("canValidatorSubmit failed: %w", err)
				}

				// Log errors that aren't "window closed" errors, or if we have many consecutive errors
				if !isWindowClosedError {
					if pollCount%maxPollLogInterval == 0 || consecutiveErrors >= 5 {
						logrus.WithError(err).WithFields(logrus.Fields{
							"epoch":              epochID,
							"data_market":        dataMarketAddr,
							"polls":              pollCount,
							"consecutive_errors": consecutiveErrors,
							"vpa_contract":       vpa.contractAddr.Hex(),
						}).Warn("⚠️ CanValidatorSubmit failed (not window closed error)")
					}
				} else {
					// Log "window closed" errors periodically to show we're still polling
					if pollCount%maxPollLogInterval == 0 {
						logrus.WithFields(logrus.Fields{
							"epoch":       epochID,
							"data_market": dataMarketAddr,
							"polls":       pollCount,
							"elapsed":     time.Duration(pollCount*2) * time.Second,
						}).Debug("Still waiting for submission window to open...")
					}
				}
				continue
			}
			if canSubmit {
				logrus.WithFields(logrus.Fields{
					"epoch":       epochID,
					"data_market": dataMarketAddr,
					"polls":       pollCount,
				}).Info("✅ Submission window is now open")
				return nil
			}
			// Log progress every 10 polls (20 seconds) to show we're still waiting
			if pollCount%maxPollLogInterval == 0 {
				logrus.WithFields(logrus.Fields{
					"epoch":       epochID,
					"data_market": dataMarketAddr,
					"polls":       pollCount,
					"elapsed":     time.Duration(pollCount*2) * time.Second,
				}).Debug("Still waiting for submission window to open...")
			}
		}
	}
}

// Close closes the client connection
func (vpa *ValidatorPriorityAssigner) Close() {
	if vpa.client != nil {
		vpa.client.Close()
	}
}

// CacheEpochPriorities caches all validator priorities for an epoch
func (pcc *PriorityCachingClient) CacheEpochPriorities(ctx context.Context, dataMarket string, epochID uint64) error {
	pcc.logger.WithFields(logrus.Fields{
		"epochID":    epochID,
		"dataMarket": dataMarket,
	}).Info("Caching VPA priorities for epoch")

	// Get priorities from VPA contract
	priorities, metadata, err := pcc.getHistoricalPrioritiesFromContract(ctx, dataMarket, epochID)
	if err != nil {
		return fmt.Errorf("failed to get historical priorities: %w", err)
	}

	// Prepare cached data
	cachedData := CachedPriorities{
		EpochID:      epochID,
		Metadata:     metadata,
		Priorities:   priorities,
		TopValidator: pcc.findTopValidator(priorities),
		CachedAt:     time.Now(),
	}

	// Cache in Redis
	return pcc.storeCachedPriorities(ctx, epochID, &cachedData)
}

// GetMyPriority gets this validator's priority with caching (cache-first)
// This wraps the base GetMyPriority() method with Redis caching
func (pcc *PriorityCachingClient) GetMyPriority(ctx context.Context, dataMarketAddr string, epochID uint64) (int, error) {
	// Get validator ID (cached, doesn't change at runtime)
	validatorID, err := pcc.getCachedValidatorID(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get validator ID: %w", err)
	}

	validatorIDStr := strconv.FormatUint(validatorID, 10)

	// Try cache first
	if priority, err := pcc.getValidatorPriorityFromCache(ctx, epochID, validatorIDStr); err == nil {
		// Priority 0 is ambiguous - could mean "not assigned yet" or "no priority"
		// Since priorities are assigned synchronously on epoch release, priority 0 likely means
		// the validator doesn't have priority OR there's a validator ID mapping mismatch.
		// Force contract re-query to ensure we have fresh data and correct mapping.
		if priority == 0 {
			pcc.logger.WithFields(logrus.Fields{
				"epochID":     epochID,
				"validatorID": validatorIDStr,
			}).Debug("Cached priority is 0, re-querying contract to verify (could be stale or mapping issue)")
		} else {
			pcc.logger.WithFields(logrus.Fields{
				"epochID":     epochID,
				"validatorID": validatorIDStr,
				"priority":    priority,
			}).Debug("VPA priority retrieved from cache")
			return priority, nil
		}
	}

	// Cache miss - call ProtocolState.getPriorities() to get all priorities for this epoch
	pcc.logger.WithFields(logrus.Fields{
		"epochID":     epochID,
		"validatorID": validatorIDStr,
	}).Debug("Cache miss, calling ProtocolState.getPriorities()")

	priority, err := pcc.getPriorityFromProtocolState(ctx, dataMarketAddr, epochID, validatorID)
	if err != nil {
		pcc.logger.WithError(err).WithFields(logrus.Fields{
			"epochID":     epochID,
			"validatorID": validatorIDStr,
			"dataMarket":  dataMarketAddr,
		}).Error("getPriorityFromProtocolState failed")
		return 0, err
	}

	// Log the priority we got before caching
	pcc.logger.WithFields(logrus.Fields{
		"epochID":     epochID,
		"validatorID": validatorIDStr,
		"priority":    priority,
		"dataMarket":  dataMarketAddr,
	}).Info("GetMyPriority: Retrieved priority from ProtocolState contract")

	// Cache the result for future use
	epochIDStr := strconv.FormatUint(epochID, 10)
	validatorKey := pcc.keyBuilder.VPAValidatorPriority(epochIDStr, validatorIDStr)
	if cacheErr := pcc.redisClient.Set(ctx, validatorKey, priority, pcc.cacheTTL).Err(); cacheErr != nil {
		pcc.logger.WithError(cacheErr).Warn("Failed to cache validator priority")
	} else {
		logLevel := logrus.DebugLevel
		if priority == 0 {
			// Log priority 0 at info level since it's unusual and might indicate an issue
			logLevel = logrus.InfoLevel
		}
		pcc.logger.WithFields(logrus.Fields{
			"epochID":     epochID,
			"validatorID": validatorIDStr,
			"priority":    priority,
			"cacheKey":    validatorKey,
		}).Log(logLevel, "Cached VPA priority")
	}

	return priority, nil
}

// GetValidatorPriority gets priority for a specific validator (cache-first)
func (pcc *PriorityCachingClient) GetValidatorPriority(ctx context.Context, dataMarket string, epochID uint64, validatorID string) (int, error) {
	// Try cache first
	if priority, err := pcc.getValidatorPriorityFromCache(ctx, epochID, validatorID); err == nil {
		return priority, nil
	}

	// Fallback to contract call
	pcc.logger.WithFields(logrus.Fields{
		"epochID":     epochID,
		"validatorID": validatorID,
	}).Warn("Cache miss, falling back to contract call")

	return pcc.getHistoricalPriorityFromContract(ctx, dataMarket, epochID, validatorID)
}

// IsTopPriority checks if validator has top priority for the epoch
func (pcc *PriorityCachingClient) IsTopPriority(ctx context.Context, dataMarket string, epochID uint64, validatorID string) (bool, error) {
	// Get top validator from cache
	topValidatorKey := pcc.keyBuilder.VPATopValidator(strconv.FormatUint(epochID, 10))
	topValidator, err := pcc.redisClient.Get(ctx, topValidatorKey).Result()
	if err == nil && topValidator == validatorID {
		return true, nil
	}

	// Fallback to checking all priorities
	priority, err := pcc.GetValidatorPriority(ctx, dataMarket, epochID, validatorID)
	if err != nil {
		return false, err
	}

	return priority == 1, nil // Priority 1 is top priority
}

// getHistoricalPrioritiesFromContract fetches all priorities from VPA contract
func (pcc *PriorityCachingClient) getHistoricalPrioritiesFromContract(_ context.Context, dataMarket string, epochID uint64) (map[string]int, PriorityMetadata, error) {
	// This is a placeholder - actual implementation would call VPA contract methods
	// like getHistoricalPriorities() and getHistoricalValidatorCount()

	// For now, return empty data
	priorities := make(map[string]int)
	metadata := PriorityMetadata{
		EpochID:        epochID,
		Seed:           "",
		Timestamp:      time.Now(),
		ValidatorCount: 0,
		DataMarket:     dataMarket,
	}

	return priorities, metadata, fmt.Errorf("contract calls not yet implemented")
}

// storeCachedPriorities stores cached priorities in Redis
func (pcc *PriorityCachingClient) storeCachedPriorities(ctx context.Context, epochID uint64, data *CachedPriorities) error {
	epochIDStr := strconv.FormatUint(epochID, 10)

	// Store full priorities object
	prioritiesJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal cached priorities: %w", err)
	}

	prioritiesKey := pcc.keyBuilder.VPAPriorities(epochIDStr)
	if err := pcc.redisClient.Set(ctx, prioritiesKey, prioritiesJSON, pcc.cacheTTL).Err(); err != nil {
		return fmt.Errorf("failed to cache priorities: %w", err)
	}

	// Store individual validator priorities for quick lookup
	for validatorID, priority := range data.Priorities {
		validatorKey := pcc.keyBuilder.VPAValidatorPriority(epochIDStr, validatorID)
		if err := pcc.redisClient.Set(ctx, validatorKey, priority, pcc.cacheTTL).Err(); err != nil {
			pcc.logger.WithError(err).Warn("Failed to cache validator priority")
		}
	}

	// Store top validator
	if data.TopValidator != "" {
		topValidatorKey := pcc.keyBuilder.VPATopValidator(epochIDStr)
		if err := pcc.redisClient.Set(ctx, topValidatorKey, data.TopValidator, pcc.cacheTTL).Err(); err != nil {
			pcc.logger.WithError(err).Warn("Failed to cache top validator")
		}
	}

	pcc.logger.WithFields(logrus.Fields{
		"epochID":        epochID,
		"validatorCount": len(data.Priorities),
		"topValidator":   data.TopValidator,
	}).Info("Successfully cached VPA priorities")

	return nil
}

// getValidatorPriorityFromCache gets validator priority from Redis cache
func (pcc *PriorityCachingClient) getValidatorPriorityFromCache(ctx context.Context, epochID uint64, validatorID string) (int, error) {
	epochIDStr := strconv.FormatUint(epochID, 10)
	validatorKey := pcc.keyBuilder.VPAValidatorPriority(epochIDStr, validatorID)

	priorityStr, err := pcc.redisClient.Get(ctx, validatorKey).Result()
	if err != nil {
		return -1, err // Cache miss
	}

	priority, err := strconv.Atoi(priorityStr)
	if err != nil {
		return -1, fmt.Errorf("invalid priority format: %w", err)
	}

	return priority, nil
}

// findTopValidator finds the validator with priority 1
func (pcc *PriorityCachingClient) findTopValidator(priorities map[string]int) string {
	for validatorID, priority := range priorities {
		if priority == 1 {
			return validatorID
		}
	}
	return ""
}

// getHistoricalPriorityFromContract fetches priority from VPA contract (placeholder)
func (pcc *PriorityCachingClient) getHistoricalPriorityFromContract(_ context.Context, _ string, _ uint64, _ string) (int, error) {
	// Placeholder for actual contract call implementation
	return -1, fmt.Errorf("contract call not yet implemented")
}
