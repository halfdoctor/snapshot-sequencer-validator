package spam

import (
	"strings"
)

// PeerWhitelist manages whitelisted Peer IDs for spam protection bypass
type PeerWhitelist struct {
	fullNodePeerIDs    map[string]bool // Set of full node Peer IDs
	bulkServicePeerIDs map[string]bool // Set of bulk service Peer IDs
}

// NewPeerWhitelist creates a new PeerWhitelist instance
func NewPeerWhitelist(fullNodePeerIDs, bulkServicePeerIDs []string) *PeerWhitelist {
	return &PeerWhitelist{
		fullNodePeerIDs:    makeSet(fullNodePeerIDs),
		bulkServicePeerIDs: makeSet(bulkServicePeerIDs),
	}
}

// IsWhitelisted checks if a Peer ID is whitelisted (either full node or bulk service)
func (w *PeerWhitelist) IsWhitelisted(peerID string) bool {
	return w.fullNodePeerIDs[peerID] || w.bulkServicePeerIDs[peerID]
}

// IsFullNode checks if a Peer ID is a whitelisted full node
func (w *PeerWhitelist) IsFullNode(peerID string) bool {
	return w.fullNodePeerIDs[peerID]
}

// IsBulkService checks if a Peer ID is a whitelisted bulk service snapshotter
func (w *PeerWhitelist) IsBulkService(peerID string) bool {
	return w.bulkServicePeerIDs[peerID]
}

// makeSet creates a map set from a slice of strings
func makeSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			set[item] = true
		}
	}
	return set
}
