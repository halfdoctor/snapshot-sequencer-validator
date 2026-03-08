package consensus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/powerloom/snapshot-sequencer-validator/pkgs/submissions"
)

// DummyBatchGenerator creates test finalized batches for Phase 3 testing
type DummyBatchGenerator struct {
	SequencerID string
	Projects    []string // List of possible project IDs (contract addresses)
}

// NewDummyBatchGenerator creates a new generator with default test projects
func NewDummyBatchGenerator(sequencerID string) *DummyBatchGenerator {
	return &DummyBatchGenerator{
		SequencerID: sequencerID,
		Projects: []string{
			"0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984", // UNI token
			"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // USDC
			"0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT
			"0x6B175474E89094C44Da98b954EedeAC495271d0F", // DAI
			"0x514910771AF9Ca656af840dff83E8264EcF986CA", // LINK
			"0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", // WBTC
			"0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9", // AAVE
			"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH
		},
	}
}

// GenerateDummyBatch creates a test finalized batch for the given epoch
func (g *DummyBatchGenerator) GenerateDummyBatch(epochID uint64) *FinalizedBatch {
	rand.Seed(time.Now().UnixNano())

	// Randomly select 3-6 active projects for this epoch
	numProjects := 3 + rand.Intn(4)
	selectedProjects := g.selectRandomProjects(numProjects)

	// Generate CIDs for each project
	projectIDs := make([]string, 0, numProjects)
	snapshotCIDs := make([]string, 0, numProjects)
	projectVotes := make(map[string]uint32)

	for _, project := range selectedProjects {
		projectIDs = append(projectIDs, project)
		cid := g.generateDummyCID(epochID, project)
		snapshotCIDs = append(snapshotCIDs, cid)
		// Simulate vote counts from snapshotters (5-10 votes per project)
		projectVotes[project] = uint32(5 + rand.Intn(6))
	}

	// Calculate merkle root from the arrays
	merkleRoot := g.calculateMerkleRoot(projectIDs, snapshotCIDs)

	// Generate dummy BLS signature (in production, use actual BLS signing)
	blsSignature := g.generateDummySignature(epochID, merkleRoot)

	return &FinalizedBatch{
		EpochId:      epochID,
		ProjectIds:   projectIDs,
		SnapshotCids: snapshotCIDs,
		MerkleRoot:   merkleRoot,
		BlsSignature: blsSignature,
		SequencerId:  g.SequencerID,
		Timestamp:    uint64(time.Now().Unix()),
		ProjectVotes: projectVotes,
	}
}

// selectRandomProjects randomly selects n projects from the available list
func (g *DummyBatchGenerator) selectRandomProjects(n int) []string {
	if n > len(g.Projects) {
		n = len(g.Projects)
	}

	// Shuffle and select first n
	shuffled := make([]string, len(g.Projects))
	copy(shuffled, g.Projects)

	for i := range shuffled {
		j := rand.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:n]
}

// generateDummyCID creates a deterministic but unique CID for testing
func (g *DummyBatchGenerator) generateDummyCID(epochID uint64, projectID string) string {
	data := fmt.Sprintf("epoch:%d:project:%s:sequencer:%s:time:%d",
		epochID, projectID, g.SequencerID, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))

	// Create IPFS-like CID format (Qm prefix for SHA256)
	// In production, use actual IPFS CID generation
	return "Qm" + hex.EncodeToString(hash[:])[:44]
}

// calculateMerkleRoot computes merkle root from project IDs and CIDs
func (g *DummyBatchGenerator) calculateMerkleRoot(projectIDs, snapshotCIDs []string) []byte {
	// Simple implementation - in production use proper merkle tree
	combined := ""
	for i := range projectIDs {
		combined += projectIDs[i] + ":" + snapshotCIDs[i] + ":"
	}
	hash := sha256.Sum256([]byte(combined))
	return hash[:]
}

// generateDummySignature creates a dummy BLS signature for testing
func (g *DummyBatchGenerator) generateDummySignature(epochID uint64, merkleRoot []byte) []byte {
	// In production, use actual BLS signing with private key
	data := fmt.Sprintf("epoch:%d:root:%s:signer:%s",
		epochID, hex.EncodeToString(merkleRoot), g.SequencerID)
	hash := sha256.Sum256([]byte(data))
	return hash[:]
}

// FinalizedBatch struct (will be generated from protobuf in production)
type FinalizedBatch struct {
	EpochId           uint64                                      `json:"EpochId"`
	ProjectIds        []string                                    `json:"ProjectIds"`
	SnapshotCids      []string                                    `json:"SnapshotCids"`
	MerkleRoot        []byte                                      `json:"MerkleRoot"`
	BlsSignature      []byte                                      `json:"BlsSignature"`
	SequencerId       string                                      `json:"SequencerId"`
	Timestamp         uint64                                      `json:"Timestamp"`
	ProjectVotes      map[string]uint32                           `json:"ProjectVotes"`
	SubmissionDetails map[string][]submissions.SubmissionMetadata `json:"submission_details"`         // project→submissions mapping with snapshotter attribution (enables challenges/proofs)
	ValidatorBatches  map[string]string                           `json:"ValidatorBatches,omitempty"` // Attribution tracking: validator_id → ipfs_cid of their individual Level 1 finalization
	BatchIPFSCID      string                                      `json:"batch_ipfs_cid,omitempty"`   // IPFS CID of this aggregated batch (Level 2)
	ValidatorCount    int                                         `json:"ValidatorCount,omitempty"`   // Number of validators who contributed
	DataMarket        string                                      `json:"DataMarket"`                 // Data market address in EIP-55 checksummed format (mandatory)
}
