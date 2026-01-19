package eventmonitor

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	rpchelper "github.com/powerloom/go-rpc-helper"
	abiloader "github.com/powerloom/snapshot-sequencer-validator/pkgs/abi"
	log "github.com/sirupsen/logrus"
)

// WindowConfig represents submission window configuration from contract
type WindowConfig struct {
	SnapshotCommitWindow      *big.Int
	SnapshotRevealWindow      *big.Int
	ValidatorVoteCommitWindow *big.Int
	ValidatorVoteRevealWindow *big.Int
	P1SubmissionWindow        *big.Int
	PNSubmissionWindow        *big.Int
	PreSubmissionWindow       *big.Int // Calculated: sum of commit/reveal windows
}

// TotalSubmissionPeriod calculates the total submission period for a given max priority
// LocalFinalizationWindow calculates when Level 1 local finalization should begin.
// This method handles two cases:
//
// Case 1: Snapshot Commit/Reveal windows enabled (non-zero)
//   - Level 1 finalization triggers when Snapshot Reveal window closes
//   - Validators need to see revealed snapshots before they can finalize locally
//   - Formula: snapshotCommitWindow + snapshotRevealWindow
//   - Note: Validator vote commit/reveal is a separate workflow and doesn't affect this timing
//
// Case 2: Snapshot Commit/Reveal windows disabled (both zero)
//   - Level 1 finalization triggers before P1 window closes to allow time for:
//     a) Level 1 local finalization to complete
//     b) Level 2 network-wide aggregation to complete
//     c) Priority 1 validator to commit on-chain during P1 window
//   - Formula: (PreSubmissionWindow + P1SubmissionWindow) - 10 seconds
//   - The 10-second buffer ensures finalization completes before P1 window opens
//
// After Level 1 finalization completes, validators commit their finalizations on-chain during
// their priority windows (P1, P2, P3, etc.) as defined in the contract.
func (wc *WindowConfig) LocalFinalizationWindow(fallbackDelay time.Duration) time.Duration {
	// Check if snapshot commit/reveal windows are enabled (non-zero)
	hasSnapshotCommitReveal := wc.SnapshotCommitWindow.Uint64() > 0 ||
		wc.SnapshotRevealWindow.Uint64() > 0

	if hasSnapshotCommitReveal {
		// Case 1: Snapshot Commit/Reveal enabled - trigger when snapshot reveal closes
		// Validators need revealed snapshots to begin local finalization
		totalSeconds := new(big.Int)
		totalSeconds.Add(wc.SnapshotCommitWindow, wc.SnapshotRevealWindow)
		return time.Duration(totalSeconds.Uint64()) * time.Second
	} else {
		// Case 2: Snapshot Commit/Reveal disabled - trigger at P1 window start minus 10 seconds
		// P1 window = PreSubmissionWindow + P1SubmissionWindow
		// Level 1 finalization must complete before P1 window opens for on-chain submission
		p1WindowSeconds := new(big.Int)
		p1WindowSeconds.Add(wc.PreSubmissionWindow, wc.P1SubmissionWindow)
		
		// Subtract 2/3 of the P1 window to allow for Level 1 finalization and Level 2 aggregation
		bufferSeconds := new(big.Int).Mul(p1WindowSeconds, big.NewInt(2))
		bufferSeconds.Div(bufferSeconds, big.NewInt(3))
		p1WindowSeconds.Sub(p1WindowSeconds, bufferSeconds)
		return time.Duration(p1WindowSeconds.Uint64()) * time.Second
	}
}

// TotalSubmissionPeriod calculates the total on-chain submission period (for reference only).
// This is when all priority windows for on-chain submission have closed.
// Formula: preSubmissionWindow + p1SubmissionWindow + (pNSubmissionWindow * maxPriority)
//
// Note: This is NOT used for triggering Level 1 finalization. Level 1 finalization triggers
// when LocalFinalizationWindow() expires (at P1 window closure).
func (wc *WindowConfig) TotalSubmissionPeriod(maxPriority int) time.Duration {
	if maxPriority < 1 {
		maxPriority = 1
	}

	// Calculate: preSubmissionWindow + p1SubmissionWindow + (pNSubmissionWindow * maxPriority)
	totalSeconds := new(big.Int)
	totalSeconds.Add(wc.PreSubmissionWindow, wc.P1SubmissionWindow)

	// Add pNSubmissionWindow * maxPriority (matches contract's Priority N end time calculation)
	if maxPriority > 1 {
		pNTotal := new(big.Int).Mul(wc.PNSubmissionWindow, big.NewInt(int64(maxPriority)))
		totalSeconds.Add(totalSeconds, pNTotal)
	}

	// Convert to time.Duration (assuming seconds)
	return time.Duration(totalSeconds.Uint64()) * time.Second
}

// WindowConfigFetcher fetches submission window configuration from ProtocolState contract
type WindowConfigFetcher struct {
	rpcHelper         *rpchelper.RPCHelper
	protocolStateAddr common.Address
	protocolStateABI  *abi.ABI
	cache             map[string]*cachedConfig
	cacheMutex        sync.RWMutex
	cacheTTL          time.Duration
}

type cachedConfig struct {
	config    *WindowConfig
	expiresAt time.Time
}

// NewWindowConfigFetcher creates a new window config fetcher
func NewWindowConfigFetcher(rpcHelper *rpchelper.RPCHelper, protocolStateAddr string, cacheTTL time.Duration) (*WindowConfigFetcher, error) {
	// Load ProtocolState ABI
	protocolStateABI, err := abiloader.LoadABI("PowerloomProtocolState.abi.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load ProtocolState ABI: %w", err)
	}

	return &WindowConfigFetcher{
		rpcHelper:         rpcHelper,
		protocolStateAddr: common.HexToAddress(protocolStateAddr),
		protocolStateABI:  &protocolStateABI,
		cache:             make(map[string]*cachedConfig),
		cacheTTL:          cacheTTL,
	}, nil
}

// FetchWindowConfig fetches window configuration for a data market from ProtocolState contract
// Uses caching to avoid excessive RPC calls
func (f *WindowConfigFetcher) FetchWindowConfig(ctx context.Context, dataMarketAddr string) (*WindowConfig, error) {
	// Check cache first
	f.cacheMutex.RLock()
	if cached, exists := f.cache[dataMarketAddr]; exists {
		if time.Now().Before(cached.expiresAt) {
			f.cacheMutex.RUnlock()
			log.WithFields(log.Fields{
				"data_market": dataMarketAddr,
				"p1_window":   cached.config.P1SubmissionWindow.Uint64(),
				"pN_window":   cached.config.PNSubmissionWindow.Uint64(),
			}).Debug("Using cached window config")
			return cached.config, nil
		}
		// Cache expired, remove it
		delete(f.cache, dataMarketAddr)
	}
	f.cacheMutex.RUnlock()

	// Fetch from contract
	config, err := f.fetchFromContract(ctx, dataMarketAddr)
	if err != nil {
		return nil, err
	}

	// Update cache
	f.cacheMutex.Lock()
	f.cache[dataMarketAddr] = &cachedConfig{
		config:    config,
		expiresAt: time.Now().Add(f.cacheTTL),
	}
	f.cacheMutex.Unlock()

	log.WithFields(log.Fields{
		"data_market":           dataMarketAddr,
		"p1_submission_window":  config.P1SubmissionWindow.Uint64(),
		"pN_submission_window":  config.PNSubmissionWindow.Uint64(),
		"pre_submission_window": config.PreSubmissionWindow.Uint64(),
	}).Info("✅ Fetched window config from contract")

	return config, nil
}

// fetchFromContract calls getDataMarketSubmissionWindowConfig on ProtocolState contract
func (f *WindowConfigFetcher) fetchFromContract(ctx context.Context, dataMarketAddr string) (*WindowConfig, error) {
	dataMarket := common.HexToAddress(dataMarketAddr)

	log.WithFields(log.Fields{
		"protocol_state_contract": f.protocolStateAddr.Hex(),
		"data_market":             dataMarketAddr,
	}).Debug("Calling getDataMarketSubmissionWindowConfig on ProtocolState contract")

	// Pack the function call: getDataMarketSubmissionWindowConfig(address)
	packedData, err := f.protocolStateABI.Pack("getDataMarketSubmissionWindowConfig", dataMarket)
	if err != nil {
		return nil, fmt.Errorf("failed to pack getDataMarketSubmissionWindowConfig call: %w", err)
	}

	// Call the contract
	callMsg := ethereum.CallMsg{
		To:   &f.protocolStateAddr,
		Data: packedData,
	}

	result, err := f.rpcHelper.CallContract(ctx, callMsg, nil)
	if err != nil {
		// Check if error contains "execution reverted" - this usually means the data market contract
		// doesn't have getSubmissionWindowConfig() method or the data market is not registered
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "execution reverted") || strings.Contains(errMsg, "revert") {
			log.WithError(err).WithFields(log.Fields{
				"protocol_state_contract": f.protocolStateAddr.Hex(),
				"data_market":             dataMarketAddr,
			}).Warn("⚠️  Data market contract does not support getSubmissionWindowConfig() - likely a legacy data market")
			return nil, fmt.Errorf("data market %s does not support getSubmissionWindowConfig (execution reverted): %w", dataMarketAddr, err)
		}
		log.WithError(err).WithFields(log.Fields{
			"protocol_state_contract": f.protocolStateAddr.Hex(),
			"data_market":             dataMarketAddr,
		}).Error("❌ Failed to call getDataMarketSubmissionWindowConfig")
		return nil, fmt.Errorf("failed to call getDataMarketSubmissionWindowConfig on contract %s: %w", f.protocolStateAddr.Hex(), err)
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("empty result from getDataMarketSubmissionWindowConfig")
	}

	// Unpack the result: (uint256, uint256, uint256, uint256, uint256, uint256)
	var (
		snapshotCommit      *big.Int
		snapshotReveal      *big.Int
		validatorVoteCommit *big.Int
		validatorVoteReveal *big.Int
		p1SubmissionWindow  *big.Int
		pNSubmissionWindow  *big.Int
	)

	// Unpack the result tuple
	outputs, err := f.protocolStateABI.Unpack("getDataMarketSubmissionWindowConfig", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack getDataMarketSubmissionWindowConfig result: %w", err)
	}

	if len(outputs) != 6 {
		return nil, fmt.Errorf("unexpected number of outputs: expected 6, got %d", len(outputs))
	}

	// Extract values from outputs (they come as *big.Int)
	var ok bool
	if snapshotCommit, ok = outputs[0].(*big.Int); !ok {
		return nil, fmt.Errorf("invalid type for snapshotCommit: %T", outputs[0])
	}
	if snapshotReveal, ok = outputs[1].(*big.Int); !ok {
		return nil, fmt.Errorf("invalid type for snapshotReveal: %T", outputs[1])
	}
	if validatorVoteCommit, ok = outputs[2].(*big.Int); !ok {
		return nil, fmt.Errorf("invalid type for validatorVoteCommit: %T", outputs[2])
	}
	if validatorVoteReveal, ok = outputs[3].(*big.Int); !ok {
		return nil, fmt.Errorf("invalid type for validatorVoteReveal: %T", outputs[3])
	}
	if p1SubmissionWindow, ok = outputs[4].(*big.Int); !ok {
		return nil, fmt.Errorf("invalid type for p1SubmissionWindow: %T", outputs[4])
	}
	if pNSubmissionWindow, ok = outputs[5].(*big.Int); !ok {
		return nil, fmt.Errorf("invalid type for pNSubmissionWindow: %T", outputs[5])
	}

	// Calculate preSubmissionWindow = sum of commit/reveal windows
	preSubmissionWindow := new(big.Int)
	preSubmissionWindow.Add(preSubmissionWindow, snapshotCommit)
	preSubmissionWindow.Add(preSubmissionWindow, snapshotReveal)
	preSubmissionWindow.Add(preSubmissionWindow, validatorVoteCommit)
	preSubmissionWindow.Add(preSubmissionWindow, validatorVoteReveal)

	return &WindowConfig{
		SnapshotCommitWindow:      snapshotCommit,
		SnapshotRevealWindow:      snapshotReveal,
		ValidatorVoteCommitWindow: validatorVoteCommit,
		ValidatorVoteRevealWindow: validatorVoteReveal,
		P1SubmissionWindow:        p1SubmissionWindow,
		PNSubmissionWindow:        pNSubmissionWindow,
		PreSubmissionWindow:       preSubmissionWindow,
	}, nil
}
