package protocolstate

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	rpchelper "github.com/powerloom/go-rpc-helper"
	log "github.com/sirupsen/logrus"

	"github.com/powerloom/snapshot-sequencer-validator/pkgs/protocolstate/contract"
)

// EventProcessor handles event-driven updates to slot cache using polling
type EventProcessor struct {
	ctx                      context.Context
	rpcHelper                *rpchelper.RPCHelper
	snapshotterStateContract *contract.SnapshotterStateContract
	snapshotterStateABI      abi.ABI
	slotManager              *SlotManager
	protocolStateAddr        common.Address
	snapshotterStateAddr     common.Address

	// Event tracking
	lastProcessedBlock uint64
	pollInterval       time.Duration

	// Event signatures (cached, exported for BlockPoller registration)
	snapshotterChangedSig common.Hash
	nodeMintedSig         common.Hash
	nodeBurnedSig         common.Hash
}

// EventSignatures returns the cached event signature hashes so callers (e.g.
// BlockPoller) can build FilterQuery entries without re-computing them.
func (ep *EventProcessor) EventSignatures() (snapshotterChanged, nodeMinted, nodeBurned common.Hash) {
	return ep.snapshotterChangedSig, ep.nodeMintedSig, ep.nodeBurnedSig
}

// ContractAddress returns the SnapshotterState contract address monitored by
// this processor, for use in BlockPoller consumer registration.
func (ep *EventProcessor) ContractAddress() common.Address {
	return ep.snapshotterStateAddr
}

// HandleBlockPollerLogs processes logs delivered by the BlockPoller. Each log
// is dispatched based on its event signature. This replaces the internal
// polling loop when the BlockPoller is in use.
func (ep *EventProcessor) HandleBlockPollerLogs(logs []types.Log, _ uint64) {
	for _, vLog := range logs {
		if len(vLog.Topics) == 0 {
			continue
		}
		sig := vLog.Topics[0]

		switch sig {
		case ep.snapshotterChangedSig:
			event, err := ep.parseSnapshotterAddressChanged(vLog)
			if err != nil {
				log.Errorf("Failed to parse SnapshotterAddressChanged event: %v", err)
				continue
			}
			if event != nil {
				ep.handleSnapshotterAddressChanged(event)
			}

		case ep.nodeMintedSig:
			event, err := ep.parseNodeMinted(vLog)
			if err != nil {
				log.Errorf("Failed to parse NodeMinted event: %v", err)
				continue
			}
			if event != nil {
				ep.handleNodeMinted(event)
			}

		case ep.nodeBurnedSig:
			event, err := ep.parseNodeBurned(vLog)
			if err != nil {
				log.Errorf("Failed to parse NodeBurned event: %v", err)
				continue
			}
			if event != nil {
				ep.handleNodeBurned(event)
			}
		}
	}
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(
	ctx context.Context,
	rpcHelper *rpchelper.RPCHelper,
	snapshotterStateContract *contract.SnapshotterStateContract,
	snapshotterStateABI abi.ABI,
	slotManager *SlotManager,
	protocolStateAddr common.Address,
	snapshotterStateAddr common.Address,
	pollInterval time.Duration,
) *EventProcessor {
	// Compute event signatures once at startup
	snapshotterChangedSig := snapshotterStateABI.Events["SnapshotterAddressChanged"].ID
	nodeMintedSig := snapshotterStateABI.Events["NodeMinted"].ID
	nodeBurnedSig := snapshotterStateABI.Events["NodeBurned"].ID

	// Get current block as starting point
	startBlock := uint64(0)
	if currentBlock, err := rpcHelper.BlockNumber(ctx); err == nil && currentBlock > 0 {
		startBlock = currentBlock - 1 // Start from previous block
	}

	return &EventProcessor{
		ctx:                      ctx,
		rpcHelper:                rpcHelper,
		snapshotterStateContract: snapshotterStateContract,
		snapshotterStateABI:      snapshotterStateABI,
		slotManager:              slotManager,
		protocolStateAddr:        protocolStateAddr,
		snapshotterStateAddr:     snapshotterStateAddr,
		lastProcessedBlock:       startBlock,
		pollInterval:             pollInterval,
		snapshotterChangedSig:    snapshotterChangedSig,
		nodeMintedSig:            nodeMintedSig,
		nodeBurnedSig:            nodeBurnedSig,
	}
}


// parseSnapshotterAddressChanged parses a log into SnapshotterAddressChanged event
func (ep *EventProcessor) parseSnapshotterAddressChanged(vLog types.Log) (*contract.SnapshotterStateContractSnapshotterAddressChanged, error) {
	event := new(contract.SnapshotterStateContractSnapshotterAddressChanged)

	// SnapshotterAddressChanged(uint256 nodeId, address oldSnapshotter, address newSnapshotter)
	// All parameters are non-indexed, so they're all in data
	// Data layout: nodeId (32 bytes) + oldSnapshotter (32 bytes) + newSnapshotter (32 bytes)
	if len(vLog.Data) < 96 {
		return nil, fmt.Errorf("invalid SnapshotterAddressChanged event data: expected 96 bytes, got %d", len(vLog.Data))
	}

	// Parse nodeId (first 32 bytes)
	event.NodeId = new(big.Int).SetBytes(vLog.Data[0:32])

	// Parse oldSnapshotter (next 32 bytes, but address is last 20 bytes)
	event.OldSnapshotter = common.BytesToAddress(vLog.Data[44:64])

	// Parse newSnapshotter (last 32 bytes, but address is last 20 bytes)
	event.NewSnapshotter = common.BytesToAddress(vLog.Data[76:96])

	event.Raw = vLog
	return event, nil
}

// parseNodeMinted parses a log into NodeMinted event
func (ep *EventProcessor) parseNodeMinted(vLog types.Log) (*contract.SnapshotterStateContractNodeMinted, error) {
	event := new(contract.SnapshotterStateContractNodeMinted)

	// NodeMinted(address indexed to, uint256 nodeId)
	// topics[0] = event signature
	// topics[1] = to (indexed)
	// data = nodeId (non-indexed, 32 bytes)
	if len(vLog.Topics) < 2 {
		return nil, fmt.Errorf("invalid NodeMinted event: expected at least 2 topics")
	}

	// Parse indexed 'to' address from topics[1]
	event.To = common.BytesToAddress(vLog.Topics[1].Bytes())

	// Parse non-indexed nodeId from data (32 bytes)
	if len(vLog.Data) < 32 {
		return nil, fmt.Errorf("invalid NodeMinted event data: expected 32 bytes, got %d", len(vLog.Data))
	}
	event.NodeId = new(big.Int).SetBytes(vLog.Data[0:32])

	event.Raw = vLog
	return event, nil
}

// parseNodeBurned parses a log into NodeBurned event
func (ep *EventProcessor) parseNodeBurned(vLog types.Log) (*contract.SnapshotterStateContractNodeBurned, error) {
	event := new(contract.SnapshotterStateContractNodeBurned)

	// NodeBurned(address indexed from, uint256 nodeId)
	// topics[0] = event signature
	// topics[1] = from (indexed)
	// data = nodeId (non-indexed, 32 bytes)
	if len(vLog.Topics) < 2 {
		return nil, fmt.Errorf("invalid NodeBurned event: expected at least 2 topics")
	}

	// Parse indexed 'from' address from topics[1]
	event.From = common.BytesToAddress(vLog.Topics[1].Bytes())

	// Parse non-indexed nodeId from data (32 bytes)
	if len(vLog.Data) < 32 {
		return nil, fmt.Errorf("invalid NodeBurned event data: expected 32 bytes, got %d", len(vLog.Data))
	}
	event.NodeId = new(big.Int).SetBytes(vLog.Data[0:32])

	event.Raw = vLog
	return event, nil
}

// handleSnapshotterAddressChanged updates slot info when snapshotter address changes
func (ep *EventProcessor) handleSnapshotterAddressChanged(event *contract.SnapshotterStateContractSnapshotterAddressChanged) {
	nodeID := event.NodeId.Uint64()
	log.Infof("SnapshotterAddressChanged event: nodeID=%d, oldSnapshotter=%s, newSnapshotter=%s",
		nodeID, event.OldSnapshotter.Hex(), event.NewSnapshotter.Hex())

	// Fetch updated slot info from contract
	if err := ep.slotManager.FetchSlot(ep.ctx, nodeID); err != nil {
		log.Errorf("Failed to update slot %d after SnapshotterAddressChanged event: %v", nodeID, err)
	} else {
		log.Infof("✅ Updated slot %d cache after SnapshotterAddressChanged event", nodeID)
	}
}

// handleNodeMinted updates slot info when a new node is minted
func (ep *EventProcessor) handleNodeMinted(event *contract.SnapshotterStateContractNodeMinted) {
	nodeID := event.NodeId.Uint64()
	log.Infof("NodeMinted event: nodeID=%d, to=%s", nodeID, event.To.Hex())

	// Fetch slot info from contract
	if err := ep.slotManager.FetchSlot(ep.ctx, nodeID); err != nil {
		log.Errorf("Failed to fetch slot %d after NodeMinted event: %v", nodeID, err)
	} else {
		log.Infof("✅ Cached slot %d after NodeMinted event", nodeID)
	}
}

// handleNodeBurned updates slot info when a node is burned
func (ep *EventProcessor) handleNodeBurned(event *contract.SnapshotterStateContractNodeBurned) {
	nodeID := event.NodeId.Uint64()
	log.Infof("NodeBurned event: nodeID=%d, from=%s", nodeID, event.From.Hex())

	// Fetch updated slot info from contract (will have Active=false)
	if err := ep.slotManager.FetchSlot(ep.ctx, nodeID); err != nil {
		log.Errorf("Failed to update slot %d after NodeBurned event: %v", nodeID, err)
	} else {
		log.Infof("✅ Updated slot %d cache after NodeBurned event", nodeID)
	}
}

// SyncHistoricalEvents syncs historical events from a starting block
// Note: This is a placeholder - full implementation would require RPC helper access
// For now, we rely on cold sync fallback to catch any missed events
func (ep *EventProcessor) SyncHistoricalEvents(startBlock uint64) error {
	log.Infof("Historical event sync not fully implemented - relying on cold sync fallback")
	// TODO: Implement historical event sync if needed
	// This would require access to RPC helper to get current block number
	return nil
}
