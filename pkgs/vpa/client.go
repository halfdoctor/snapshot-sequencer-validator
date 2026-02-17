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
	Priorities   map[string]int   `json:"priorities"`   // nodeId (string) -> priority
	TopValidator string           `json:"topValidator"` // nodeId (string) of validator with priority 1
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
	protocolStateAddr common.Address // ProtocolState contract for getPriorities, getActiveValidators
	protocolStateABI  abi.ABI        // ProtocolState ABI
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
		errorMsg := err.Error()
		logrus.WithError(err).WithFields(logrus.Fields{
			"epoch":        epochID,
			"data_market":  dataMarketAddr,
			"vpa_contract": vpa.contractAddr.Hex(),
			"validator":    vpa.validator.Hex(),
			"error_msg":    errorMsg,
			"timestamp":    time.Now().Unix(),
		}).Warn("CanValidatorSubmit contract call failed - check window config and epochReleaseTime")
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

// GetEpochReleaseTime gets the epoch release timestamp from DataMarket contract via ProtocolState
// Uses ProtocolState.epochInfo(dataMarket, epochID) to avoid needing DataMarket ABI
func (vpa *ValidatorPriorityAssigner) GetEpochReleaseTime(ctx context.Context, dataMarketAddr string, epochID uint64) (uint64, error) {
	// This method is called from PriorityCachingClient which has protocolStateABI
	// For base ValidatorPriorityAssigner, we need to load ProtocolState ABI
	protocolStateABI, err := abiloader.LoadABI("PowerloomProtocolState.abi.json")
	if err != nil {
		return 0, fmt.Errorf("failed to load ProtocolState ABI: %w", err)
	}

	// Get ProtocolState address from VPA contract's protocolState() function
	protocolStateAddr, err := vpa.getProtocolStateAddress(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get ProtocolState address: %w", err)
	}

	dataMarketAddress := common.HexToAddress(dataMarketAddr)
	data, err := protocolStateABI.Pack("epochInfo", dataMarketAddress, big.NewInt(int64(epochID)))
	if err != nil {
		return 0, fmt.Errorf("failed to pack epochInfo call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &protocolStateAddr,
		Data: data,
	}
	result, err := vpa.client.CallContract(ctx, msg, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to call ProtocolState.epochInfo: %w", err)
	}

	var epochInfo struct {
		Timestamp   *big.Int
		Blocknumber *big.Int
		EpochEnd    *big.Int
	}
	err = protocolStateABI.UnpackIntoInterface(&epochInfo, "epochInfo", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack epochInfo result: %w", err)
	}

	if epochInfo.Timestamp == nil {
		return 0, fmt.Errorf("epochInfo timestamp is nil")
	}

	return epochInfo.Timestamp.Uint64(), nil
}

// GetSubmissionWindows gets submission window config from DataMarket contract
// Uses a minimal ABI to avoid requiring DataMarket.json file
func (vpa *ValidatorPriorityAssigner) GetSubmissionWindows(ctx context.Context, dataMarketAddr string) (uint64, uint64, uint64, error) {
	// Use a minimal ABI for just getSubmissionWindows() to avoid needing DataMarket.json
	// Note: matching contract return names exactly (including typo: preSubmisisonWindow)
	minimalABI := `[{
		"inputs": [],
		"name": "getSubmissionWindows",
		"outputs": [
			{"internalType": "uint256", "name": "preSubmisisonWindow", "type": "uint256"},
			{"internalType": "uint256", "name": "p1SubmissionWindow", "type": "uint256"},
			{"internalType": "uint256", "name": "pNSubmissionWindow", "type": "uint256"}
		],
		"stateMutability": "view",
		"type": "function"
	}]`

	dataMarketABI, err := abi.JSON(strings.NewReader(minimalABI))
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to parse minimal DataMarket ABI: %w", err)
	}

	data, err := dataMarketABI.Pack("getSubmissionWindows")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to pack getSubmissionWindows call: %w", err)
	}

	dataMarketAddress := common.HexToAddress(dataMarketAddr)
	msg := ethereum.CallMsg{
		To:   &dataMarketAddress,
		Data: data,
	}
	result, err := vpa.client.CallContract(ctx, msg, nil)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to call getSubmissionWindows: %w", err)
	}

	// Unpack directly - struct field names must match ABI output names exactly
	outputs, err := dataMarketABI.Unpack("getSubmissionWindows", result)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to unpack getSubmissionWindows result: %w", err)
	}

	if len(outputs) != 3 {
		return 0, 0, 0, fmt.Errorf("unexpected number of outputs: expected 3, got %d", len(outputs))
	}

	preSubmissionWindow, ok := outputs[0].(*big.Int)
	if !ok || preSubmissionWindow == nil {
		return 0, 0, 0, fmt.Errorf("invalid type for preSubmissionWindow: %T", outputs[0])
	}

	p1SubmissionWindow, ok := outputs[1].(*big.Int)
	if !ok || p1SubmissionWindow == nil {
		return 0, 0, 0, fmt.Errorf("invalid type for p1SubmissionWindow: %T", outputs[1])
	}

	pNSubmissionWindow, ok := outputs[2].(*big.Int)
	if !ok || pNSubmissionWindow == nil {
		return 0, 0, 0, fmt.Errorf("invalid type for pNSubmissionWindow: %T", outputs[2])
	}

	return preSubmissionWindow.Uint64(), p1SubmissionWindow.Uint64(), pNSubmissionWindow.Uint64(), nil
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

// getProtocolStateAddress gets the ProtocolState contract address from VPA contract
func (vpa *ValidatorPriorityAssigner) getProtocolStateAddress(ctx context.Context) (common.Address, error) {
	// Call VPA contract's protocolState() public variable
	data, err := vpa.abi.Pack("protocolState")
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to pack protocolState call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &vpa.contractAddr,
		Data: data,
	}

	result, err := vpa.client.CallContract(ctx, msg, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to call protocolState(): %w", err)
	}

	// Unpack the result (address)
	var protocolStateAddr common.Address
	err = vpa.abi.UnpackIntoInterface(&protocolStateAddr, "protocolState", result)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to unpack protocolState result: %w", err)
	}

	return protocolStateAddr, nil
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

// getCachedNodeId gets the validator nodeId with caching (only queries once).
// Returns the node ID from ValidatorState (stable per validator).
func (pcc *PriorityCachingClient) getCachedNodeId(ctx context.Context) (uint64, error) {
	pcc.nodeIdMutex.RLock()
	if pcc.cachedNodeId != nil {
		nodeId := *pcc.cachedNodeId
		pcc.nodeIdMutex.RUnlock()
		return nodeId, nil
	}
	pcc.nodeIdMutex.RUnlock()

	pcc.nodeIdMutex.Lock()
	defer pcc.nodeIdMutex.Unlock()

	if pcc.cachedNodeId != nil {
		return *pcc.cachedNodeId, nil
	}

	nodeId, err := pcc.ValidatorPriorityAssigner.getValidatorID(ctx)
	if err != nil {
		return 0, err
	}

	pcc.cachedNodeId = &nodeId
	pcc.logger.WithFields(logrus.Fields{
		"nodeId":        nodeId,
		"validatorAddr": pcc.validator.Hex(),
	}).Info("Cached validator nodeId (stable for runtime)")

	return nodeId, nil
}

// getActiveValidatorsFromProtocolState calls ProtocolState.getActiveValidators(dataMarket, epochId).
// Returns the node IDs for that epoch in order; validatorIndex from getPriorities is position in this array.
// Must be used to map validatorIndex -> nodeId (index alone is meaningless after burns).
func (pcc *PriorityCachingClient) getActiveValidatorsFromProtocolState(ctx context.Context, dataMarketAddr string, epochID uint64) ([]uint64, error) {
	data, err := pcc.protocolStateABI.Pack("getActiveValidators",
		common.HexToAddress(dataMarketAddr),
		big.NewInt(int64(epochID)))
	if err != nil {
		return nil, fmt.Errorf("failed to pack getActiveValidators call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &pcc.protocolStateAddr,
		Data: data,
	}
	result, err := pcc.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call ProtocolState.getActiveValidators: %w", err)
	}

	var bigArr []*big.Int
	err = pcc.protocolStateABI.UnpackIntoInterface(&bigArr, "getActiveValidators", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack getActiveValidators result: %w", err)
	}

	out := make([]uint64, len(bigArr))
	for i, b := range bigArr {
		if b == nil {
			return nil, fmt.Errorf("getActiveValidators[%d] is nil", i)
		}
		out[i] = b.Uint64()
	}
	return out, nil
}

// getPriorityFromProtocolState calls ProtocolState.getPriorities() and getActiveValidators,
// then looks up priority by nodeId. validatorIndex in getPriorities is position-in-epoch-set, not nodeId.
func (pcc *PriorityCachingClient) getPriorityFromProtocolState(ctx context.Context, dataMarketAddr string, epochID uint64, nodeId uint64) (int, error) {
	activeValidators, err := pcc.getActiveValidatorsFromProtocolState(ctx, dataMarketAddr, epochID)
	if err != nil {
		pcc.logger.WithError(err).WithFields(logrus.Fields{
			"epochID":        epochID,
			"nodeId":         nodeId,
			"dataMarketAddr": dataMarketAddr,
		}).Error("getActiveValidatorsFromProtocolState failed")
		return 0, fmt.Errorf("failed to get active validators: %w", err)
	}
	if len(activeValidators) == 0 {
		return 0, nil
	}

	// Find index i where activeValidators[i] == nodeId
	var positionInSet uint64
	found := false
	for i, nid := range activeValidators {
		if nid == nodeId {
			positionInSet = uint64(i)
			found = true
			break
		}
	}
	if !found {
		pcc.logger.WithFields(logrus.Fields{
			"epochID": epochID,
			"nodeId":  nodeId,
		}).Warn("nodeId not found in epoch's active validators - returning priority 0")
		return 0, nil
	}

	// Call ProtocolState.getPriorities(dataMarket, epochId)
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
			"nodeId":            nodeId,
			"dataMarketAddr":    dataMarketAddr,
			"protocolStateAddr": pcc.protocolStateAddr.Hex(),
		}).Error("ProtocolState.getPriorities() call reverted - check contract addresses and parameters")
		return 0, fmt.Errorf("failed to call ProtocolState.getPriorities: %w", err)
	}

	var priorities []struct {
		ValidatorIndex *big.Int `json:"validatorIndex"`
		Priority       *big.Int `json:"priority"`
	}
	err = pcc.protocolStateABI.UnpackIntoInterface(&priorities, "getPriorities", result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack getPriorities result: %w", err)
	}

	// Find priority where ValidatorIndex (position in epoch set) matches our position
	positionBig := big.NewInt(int64(positionInSet))
	for _, p := range priorities {
		if p.ValidatorIndex != nil && p.ValidatorIndex.Cmp(positionBig) == 0 {
			if p.Priority != nil {
				priority := int(p.Priority.Int64())
				pcc.logger.WithFields(logrus.Fields{
					"epochID":  epochID,
					"nodeId":   nodeId,
					"priority": priority,
				}).Info("Found priority for validator")
				return priority, nil
			}
		}
	}

	pcc.logger.WithFields(logrus.Fields{
		"epochID":         epochID,
		"nodeId":         nodeId,
		"prioritiesCount": len(priorities),
	}).Warn("Validator in epoch set but no priority assigned - returning priority 0")
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
// For priority > 1, calculates when the window should open and waits until then instead of polling
func (vpa *ValidatorPriorityAssigner) WaitForSubmissionWindow(ctx context.Context, dataMarketAddr string, epochID uint64, priority int) error {
	// First check: if window is already open, return immediately
	canSubmit, err := vpa.CanValidatorSubmit(ctx, dataMarketAddr, epochID)
	if err != nil {
		errorMsg := err.Error()
		// "Submission window closed" means window has already passed (fatal)
		// "Submission window not open" means window hasn't opened yet (recoverable)
		if strings.Contains(errorMsg, "Submission window closed") {
			// Try to get current block to see what timestamp the contract sees
			header, headerErr := vpa.client.HeaderByNumber(ctx, nil)
			blockTimestamp := int64(0)
			if headerErr == nil && header != nil {
				blockTimestamp = int64(header.Time)
			}

			logrus.WithError(err).WithFields(logrus.Fields{
				"epoch":           epochID,
				"data_market":     dataMarketAddr,
				"vpa_contract":    vpa.contractAddr.Hex(),
				"block_timestamp": blockTimestamp,
				"error_msg":       errorMsg,
			}).Error("❌ Submission window has already closed - check if block.timestamp matches expected window timing")
			return fmt.Errorf("submission window closed: %w", err)
		}
		// "Submission window not open" or generic "execution reverted" - might open later
		// For priority > 1, we'll calculate wait time below; for priority 1, we'll poll
		if strings.Contains(errorMsg, "Submission window not open") || strings.Contains(errorMsg, "execution reverted") {
			if priority > 1 {
				logrus.WithFields(logrus.Fields{
					"epoch":        epochID,
					"priority":     priority,
					"data_market":  dataMarketAddr,
					"vpa_contract": vpa.contractAddr.Hex(),
					"error":        errorMsg,
				}).Debug("Submission window not yet open, will calculate wait time...")
			} else {
				logrus.WithFields(logrus.Fields{
					"epoch":        epochID,
					"data_market":  dataMarketAddr,
					"vpa_contract": vpa.contractAddr.Hex(),
					"error":        errorMsg,
				}).Debug("Submission window not yet open, will start polling...")
			}
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

	// For priority > 1, calculate exact wait time from contract config
	if priority > 1 {
		// Get epoch release time and window config from contract
		epochReleaseTime, err := vpa.GetEpochReleaseTime(ctx, dataMarketAddr, epochID)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"epoch":       epochID,
				"priority":    priority,
				"data_market": dataMarketAddr,
			}).Warn("Failed to get epochReleaseTime, falling back to polling")
			// Fall through to polling logic below
		} else {
			preSubmissionWindow, p1SubmissionWindow, pNSubmissionWindow, err := vpa.GetSubmissionWindows(ctx, dataMarketAddr)
			if err != nil {
				logrus.WithError(err).WithFields(logrus.Fields{
					"epoch":       epochID,
					"priority":    priority,
					"data_market": dataMarketAddr,
				}).Warn("Failed to get submission windows, falling back to polling")
				// Fall through to polling logic below
			} else {
				// Calculate when this priority's window opens
				// Priority 2: epochReleaseTime + preSubmissionWindow + p1SubmissionWindow
				// Priority 3: epochReleaseTime + preSubmissionWindow + p1SubmissionWindow + pNSubmissionWindow
				// Priority N: epochReleaseTime + preSubmissionWindow + p1SubmissionWindow + (pNSubmissionWindow * (priority - 1))
				windowStartTime := epochReleaseTime + preSubmissionWindow + p1SubmissionWindow + (pNSubmissionWindow * uint64(priority-1))

				localTime := time.Now().Unix()
				waitSeconds := int64(windowStartTime) - localTime

				if waitSeconds <= 0 {
					// Window should already be open, check immediately
					canSubmit, err := vpa.CanValidatorSubmit(ctx, dataMarketAddr, epochID)
					if err == nil && canSubmit {
						logrus.WithFields(logrus.Fields{
							"epoch":       epochID,
							"priority":    priority,
							"data_market": dataMarketAddr,
						}).Info("✅ Submission window is open")
						return nil
					}
					// Window closed or error - return
					if err != nil && strings.Contains(err.Error(), "Submission window closed") {
						return fmt.Errorf("submission window closed: %w", err)
					}
					return fmt.Errorf("submission window not open")
				}

				logrus.WithFields(logrus.Fields{
					"epoch":             epochID,
					"priority":          priority,
					"data_market":       dataMarketAddr,
					"wait_seconds":      waitSeconds,
					"window_start_time": windowStartTime,
					"local_time":        localTime,
				}).Info("⏳ Priority > 1: Waiting for lower priority windows to close...")

				// Wait until window should open
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(waitSeconds) * time.Second):
					// Retry a few times after waiting - block.timestamp may not match local time exactly
					maxRetries := 5
					retryInterval := 2 * time.Second
					for retry := 0; retry < maxRetries; retry++ {
						canSubmit, err := vpa.CanValidatorSubmit(ctx, dataMarketAddr, epochID)
						if err != nil {
							errorMsg := err.Error()
							if strings.Contains(errorMsg, "Submission window closed") {
								logrus.WithError(err).WithFields(logrus.Fields{
									"epoch":       epochID,
									"priority":    priority,
									"data_market": dataMarketAddr,
								}).Error("❌ Submission window has already closed")
								return fmt.Errorf("submission window closed: %w", err)
							}
							if retry < maxRetries-1 {
								logrus.WithFields(logrus.Fields{
									"epoch":          epochID,
									"priority":       priority,
									"data_market":    dataMarketAddr,
									"retry":          retry + 1,
									"max_retries":    maxRetries,
									"retry_interval": retryInterval,
								}).Debug("After priority specific wait: window not open yet, retrying...")
								select {
								case <-ctx.Done():
									return ctx.Err()
								case <-time.After(retryInterval):
									continue
								}
							}
							logrus.WithError(err).WithFields(logrus.Fields{
								"epoch":       epochID,
								"priority":    priority,
								"data_market": dataMarketAddr,
								"retries":     maxRetries,
							}).Warn("⚠️ Submission window still not open after waiting and retries")
							return fmt.Errorf("submission window not open after wait: %w", err)
						}
						if canSubmit {
							logrus.WithFields(logrus.Fields{
								"epoch":       epochID,
								"priority":    priority,
								"data_market": dataMarketAddr,
								"retries":     retry,
							}).Info("✅ Submission window is now open")
							return nil
						}
						if retry < maxRetries-1 {
							select {
							case <-ctx.Done():
								return ctx.Err()
							case <-time.After(retryInterval):
								continue
							}
						}
					}
					return fmt.Errorf("submission window not open after %d retries", maxRetries)
				}
			}
		}
	}

	// Priority 1: Poll aggressively since window opens immediately after preSubmissionWindow
	// With per block data markets, we need fast detection
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	logrus.WithFields(logrus.Fields{
		"epoch":       epochID,
		"priority":    priority,
		"data_market": dataMarketAddr,
	}).Info("⏳ Waiting for submission window to open (priority 1: polling every 500ms)...")

	pollCount := 0
	maxPollLogInterval := 10
	maxConsecutiveErrors := 10 // Reduced for priority 1

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			pollCount++
			canSubmit, err := vpa.CanValidatorSubmit(ctx, dataMarketAddr, epochID)
			if err != nil {
				errorMsg := err.Error()
				if strings.Contains(errorMsg, "Submission window closed") {
					logrus.WithError(err).WithFields(logrus.Fields{
						"epoch":       epochID,
						"priority":    priority,
						"data_market": dataMarketAddr,
					}).Error("❌ Submission window has already closed")
					return fmt.Errorf("submission window closed: %w", err)
				}
				if pollCount >= maxConsecutiveErrors {
					logrus.WithError(err).WithFields(logrus.Fields{
						"epoch":    epochID,
						"priority": priority,
						"polls":    pollCount,
					}).Warn("⚠️ Max polls reached, stopping")
					return fmt.Errorf("submission window not open after %d polls: %w", pollCount, err)
				}
				if pollCount%maxPollLogInterval == 0 {
					logrus.WithFields(logrus.Fields{
						"epoch":    epochID,
						"priority": priority,
						"polls":    pollCount,
					}).Debug("Still waiting for submission window to open...")
				}
				continue
			}
			if canSubmit {
				logrus.WithFields(logrus.Fields{
					"epoch":    epochID,
					"priority": priority,
					"polls":    pollCount,
				}).Info("✅ Submission window is now open")
				return nil
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
	nodeId, err := pcc.getCachedNodeId(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get validator node ID: %w", err)
	}

	validatorIDStr := strconv.FormatUint(nodeId, 10)

	// Try cache first
	if priority, err := pcc.getValidatorPriorityFromCache(ctx, epochID, validatorIDStr); err == nil {
		// Priority 0 is ambiguous - could mean "not assigned yet" or "no priority"
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

	// Cache miss - call getPriorityFromProtocolState (uses getPriorities + getActiveValidators)
	pcc.logger.WithFields(logrus.Fields{
		"epochID":     epochID,
		"validatorID": validatorIDStr,
	}).Debug("Cache miss, calling ProtocolState.getPriorities() and getActiveValidators()")

	priority, err := pcc.getPriorityFromProtocolState(ctx, dataMarketAddr, epochID, nodeId)
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
	// Store as string to ensure consistent serialization
	priorityStr := strconv.Itoa(priority)
	if cacheErr := pcc.redisClient.Set(ctx, validatorKey, priorityStr, pcc.cacheTTL).Err(); cacheErr != nil {
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
			"ttl":         pcc.cacheTTL,
		}).Log(logLevel, "Cached VPA priority")
	}

	return priority, nil
}

// GetValidatorPriority gets priority for a specific validator (cache-first).
// validatorID must be nodeId string (matching cache keys and getHistoricalPriority contract).
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

// IsTopPriority checks if validator has top priority for the epoch.
// validatorID must be nodeId string.
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

// getHistoricalPrioritiesFromContract fetches all priorities from VPA contract.
// Uses getActiveValidators to map validatorIndex (position in epoch set) -> nodeId.
// Returns map with nodeId as keys (matching cache keys and getHistoricalPriority contract call).
func (pcc *PriorityCachingClient) getHistoricalPrioritiesFromContract(ctx context.Context, dataMarket string, epochID uint64) (map[string]int, PriorityMetadata, error) {
	activeValidators, err := pcc.getActiveValidatorsFromProtocolState(ctx, dataMarket, epochID)
	if err != nil {
		pcc.logger.WithError(err).WithFields(logrus.Fields{
			"epochID":    epochID,
			"dataMarket": dataMarket,
		}).Error("getActiveValidatorsFromProtocolState failed")
		return nil, PriorityMetadata{}, fmt.Errorf("failed to get active validators: %w", err)
	}

	data, err := pcc.protocolStateABI.Pack("getPriorities",
		common.HexToAddress(dataMarket),
		big.NewInt(int64(epochID)))
	if err != nil {
		return nil, PriorityMetadata{}, fmt.Errorf("failed to pack getPriorities call: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &pcc.protocolStateAddr,
		Data: data,
	}
	result, err := pcc.client.CallContract(ctx, msg, nil)
	if err != nil {
		pcc.logger.WithError(err).WithFields(logrus.Fields{
			"epochID":           epochID,
			"dataMarket":        dataMarket,
			"protocolStateAddr": pcc.protocolStateAddr.Hex(),
		}).Error("ProtocolState.getPriorities() call failed")
		return nil, PriorityMetadata{}, fmt.Errorf("failed to call ProtocolState.getPriorities: %w", err)
	}

	var prioritiesArray []struct {
		ValidatorIndex *big.Int `json:"validatorIndex"`
		Priority       *big.Int `json:"priority"`
	}
	err = pcc.protocolStateABI.UnpackIntoInterface(&prioritiesArray, "getPriorities", result)
	if err != nil {
		return nil, PriorityMetadata{}, fmt.Errorf("failed to unpack getPriorities result: %w", err)
	}

	// Map validatorIndex (position in epoch set) -> nodeId via activeValidators
	priorities := make(map[string]int)
	for _, p := range prioritiesArray {
		if p.ValidatorIndex != nil && p.Priority != nil {
			idx := p.ValidatorIndex.Uint64()
			if idx >= uint64(len(activeValidators)) {
				pcc.logger.WithFields(logrus.Fields{
					"epochID":       epochID,
					"validatorIndex": idx,
					"activeCount":   len(activeValidators),
				}).Warn("validatorIndex out of bounds for activeValidators - skipping")
				continue
			}
			nodeId := activeValidators[idx]
			nodeIdStr := strconv.FormatUint(nodeId, 10)
			priorities[nodeIdStr] = int(p.Priority.Int64())
		}
	}

	// Build metadata - seed is not available from getPriorities(), use empty string
	// Validator count is the length of priorities array
	metadata := PriorityMetadata{
		EpochID:        epochID,
		Seed:           "", // Seed is only available from PrioritiesAssigned events, not from getPriorities()
		Timestamp:      time.Now(),
		ValidatorCount: len(prioritiesArray),
		DataMarket:     dataMarket,
	}

	pcc.logger.WithFields(logrus.Fields{
		"epochID":        epochID,
		"dataMarket":     dataMarket,
		"validatorCount": len(prioritiesArray),
		"priorities":     len(priorities),
	}).Info("Successfully fetched historical priorities from contract")

	return priorities, metadata, nil
}

// storeCachedPriorities stores cached priorities in Redis.
// data.Priorities map keys are nodeIds from getActiveValidators (from getHistoricalPrioritiesFromContract).
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

	// Store individual validator priorities for quick lookup (keys are nodeIds)
	for validatorID, priority := range data.Priorities {
		validatorKey := pcc.keyBuilder.VPAValidatorPriority(epochIDStr, validatorID)
		priorityStr := strconv.Itoa(priority)
		if err := pcc.redisClient.Set(ctx, validatorKey, priorityStr, pcc.cacheTTL).Err(); err != nil {
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

// getValidatorPriorityFromCache gets validator priority from Redis cache.
// validatorID must be nodeId string (matching cache keys from storeCachedPriorities).
func (pcc *PriorityCachingClient) getValidatorPriorityFromCache(ctx context.Context, epochID uint64, validatorID string) (int, error) {
	epochIDStr := strconv.FormatUint(epochID, 10)
	validatorKey := pcc.keyBuilder.VPAValidatorPriority(epochIDStr, validatorID)

	priorityStr, err := pcc.redisClient.Get(ctx, validatorKey).Result()
	if err != nil {
		if err == redis.Nil {
			pcc.logger.WithFields(logrus.Fields{
				"epochID":     epochID,
				"validatorID": validatorID,
				"cacheKey":    validatorKey,
			}).Debug("Cache miss - key not found in Redis")
		} else {
			pcc.logger.WithError(err).WithFields(logrus.Fields{
				"epochID":     epochID,
				"validatorID": validatorID,
				"cacheKey":    validatorKey,
			}).Warn("Cache lookup failed")
		}
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

// getHistoricalPriorityFromContract fetches priority from VPA contract (placeholder).
// validatorID must be nodeId string. When implemented, call VPA.getHistoricalPriority(dataMarket, epochID, nodeId).
func (pcc *PriorityCachingClient) getHistoricalPriorityFromContract(_ context.Context, _ string, _ uint64, validatorID string) (int, error) {
	return -1, fmt.Errorf("contract call not yet implemented")
}
