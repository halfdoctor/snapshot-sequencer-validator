package protocolstate

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// SlotInfo represents the cached slot information from SnapshotterState contract
// This structure matches the data stored by protocol-state-cacher in Redis
// Handles both legacy and new contract versions (new has libP2pAddress field)
type SlotInfo struct {
	SnapshotterAddress common.Address `json:"SnapshotterAddress"`
	LibP2pAddress      string         `json:"LibP2pAddress,omitempty"` // Only in new contract version
	NodePrice          *big.Int       `json:"NodePrice"`
	AmountSentOnL1     *big.Int       `json:"AmountSentOnL1"`
	MintedOn           *big.Int       `json:"MintedOn"`
	BurnedOn           *big.Int       `json:"BurnedOn"`
	LastUpdated        *big.Int       `json:"LastUpdated"`
	IsLegacy           bool           `json:"IsLegacy"`
	ClaimedTokens      bool           `json:"ClaimedTokens"`
	Active             bool           `json:"Active"`
	IsKyced            bool           `json:"IsKyced"`
}

// TODO: Other state variables cached by protocol-state-cacher (for future implementation):
// - TotalNodesCount - Total number of nodes minted
// - EpochsInADay (per data market) - Number of epochs in a day
// - EPOCH_SIZE (per data market) - Size of an epoch
// - SOURCE_CHAIN_BLOCK_TIME (per data market) - Block time of source chain
// - CurrentDay (per data market) - Current day counter
// - DailySnapshotQuota (per data market) - Daily snapshot quota table
// - CurrentEpochID (per data market) - Current epoch ID (event-driven via EpochReleased)
