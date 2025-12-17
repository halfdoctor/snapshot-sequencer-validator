package protocolstate

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"

	"github.com/powerloom/snapshot-sequencer-validator/pkgs/protocolstate/contract"
)

// EventProcessor handles event-driven updates to slot cache
type EventProcessor struct {
	ctx                      context.Context
	snapshotterStateContract *contract.SnapshotterStateContract
	slotManager             *SlotManager
	protocolStateAddr       common.Address
	snapshotterStateAddr    common.Address
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(
	ctx context.Context,
	snapshotterStateContract *contract.SnapshotterStateContract,
	slotManager *SlotManager,
	protocolStateAddr common.Address,
	snapshotterStateAddr common.Address,
) *EventProcessor {
	return &EventProcessor{
		ctx:                      ctx,
		snapshotterStateContract: snapshotterStateContract,
		slotManager:             slotManager,
		protocolStateAddr:       protocolStateAddr,
		snapshotterStateAddr:     snapshotterStateAddr,
	}
}

// WatchEvents watches SnapshotterState contract events and updates cache reactively
func (ep *EventProcessor) WatchEvents() error {
	log.Info("Starting event-driven slot cache updates...")

	// Watch SnapshotterAddressChanged events
	snapshotterChangedSink := make(chan *contract.SnapshotterStateContractSnapshotterAddressChanged)
	snapshotterChangedSub, err := ep.snapshotterStateContract.WatchSnapshotterAddressChanged(
		&bind.WatchOpts{Context: ep.ctx},
		snapshotterChangedSink,
	)
	if err != nil {
		return fmt.Errorf("failed to watch SnapshotterAddressChanged events: %w", err)
	}
	defer snapshotterChangedSub.Unsubscribe()

	// Watch NodeMinted events
	nodeMintedSink := make(chan *contract.SnapshotterStateContractNodeMinted)
	nodeMintedSub, err := ep.snapshotterStateContract.WatchNodeMinted(
		&bind.WatchOpts{Context: ep.ctx},
		nodeMintedSink,
		nil, // no indexed parameters filter
	)
	if err != nil {
		return fmt.Errorf("failed to watch NodeMinted events: %w", err)
	}
	defer nodeMintedSub.Unsubscribe()

	// Watch NodeBurned events
	nodeBurnedSink := make(chan *contract.SnapshotterStateContractNodeBurned)
	nodeBurnedSub, err := ep.snapshotterStateContract.WatchNodeBurned(
		&bind.WatchOpts{Context: ep.ctx},
		nodeBurnedSink,
		nil, // no indexed parameters filter
	)
	if err != nil {
		return fmt.Errorf("failed to watch NodeBurned events: %w", err)
	}
	defer nodeBurnedSub.Unsubscribe()

	log.Info("✅ Event watchers started for SnapshotterAddressChanged, NodeMinted, and NodeBurned")

	// Process events
	for {
		select {
		case event := <-snapshotterChangedSink:
			if event != nil {
				ep.handleSnapshotterAddressChanged(event)
			}

		case event := <-nodeMintedSink:
			if event != nil {
				ep.handleNodeMinted(event)
			}

		case event := <-nodeBurnedSink:
			if event != nil {
				ep.handleNodeBurned(event)
			}

		case err := <-snapshotterChangedSub.Err():
			log.Errorf("SnapshotterAddressChanged subscription error: %v", err)
			// Reconnect logic could be added here
			return fmt.Errorf("event subscription error: %w", err)

		case err := <-nodeMintedSub.Err():
			log.Errorf("NodeMinted subscription error: %v", err)
			return fmt.Errorf("event subscription error: %w", err)

		case err := <-nodeBurnedSub.Err():
			log.Errorf("NodeBurned subscription error: %v", err)
			return fmt.Errorf("event subscription error: %w", err)

		case <-ep.ctx.Done():
			log.Info("Event processor stopped (context cancelled)")
			return nil
		}
	}
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

