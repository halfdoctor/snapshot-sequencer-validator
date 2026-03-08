package consensus

import (
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// VoteAggregator collects and aggregates votes from multiple validators
type VoteAggregator struct {
	epochID       uint64
	votes         map[string]*FinalizedBatch // sequencer_id -> batch
	voteThreshold float64                    // e.g., 0.66 for 2/3 majority
	mu            sync.RWMutex

	// Timing
	votingDeadline time.Time

	// Callbacks
	onConsensus func(*ConsensusResult)
}

// NewVoteAggregator creates a new vote aggregator for an epoch
func NewVoteAggregator(epochID uint64, threshold float64, votingDuration time.Duration) *VoteAggregator {
	return &VoteAggregator{
		epochID:        epochID,
		votes:          make(map[string]*FinalizedBatch),
		voteThreshold:  threshold,
		votingDeadline: time.Now().Add(votingDuration),
	}
}

// AddVote adds a vote from a validator
func (va *VoteAggregator) AddVote(batch *FinalizedBatch) error {
	va.mu.Lock()
	defer va.mu.Unlock()

	// Check if voting window is still open
	if time.Now().After(va.votingDeadline) {
		return fmt.Errorf("voting window closed for epoch %d", va.epochID)
	}

	// Verify epoch matches
	if batch.EpochId != va.epochID {
		return fmt.Errorf("batch epoch %d doesn't match aggregator epoch %d",
			batch.EpochId, va.epochID)
	}

	// Store vote
	va.votes[batch.SequencerId] = batch
	log.Printf("Added vote from %s for epoch %d (total votes: %d)",
		batch.SequencerId, va.epochID, len(va.votes))

	// Check if we have consensus
	if consensus := va.checkConsensus(); consensus != nil {
		if va.onConsensus != nil {
			go va.onConsensus(consensus)
		}
		return nil
	}

	return nil
}

// checkConsensus checks if we have reached consensus
func (va *VoteAggregator) checkConsensus() *ConsensusResult {
	// Group votes by merkle root (representing identical batches)
	rootGroups := make(map[string][]*FinalizedBatch)

	for _, batch := range va.votes {
		rootHex := hex.EncodeToString(batch.MerkleRoot)
		rootGroups[rootHex] = append(rootGroups[rootHex], batch)
	}

	// Check if any group has sufficient votes
	totalVotes := len(va.votes)
	requiredVotes := int(float64(totalVotes) * va.voteThreshold)

	for _, batches := range rootGroups {
		if len(batches) >= requiredVotes {
			// We have consensus!
			return va.buildConsensusResult(batches)
		}
	}

	return nil
}

// buildConsensusResult builds the final consensus result from agreeing batches
func (va *VoteAggregator) buildConsensusResult(batches []*FinalizedBatch) *ConsensusResult {
	// Use the first batch as reference (all should be identical)
	refBatch := batches[0]

	// Collect participating sequencers
	participants := make([]string, 0, len(batches))
	for _, batch := range batches {
		participants = append(participants, batch.SequencerId)
	}

	// Create participant bitmap (simplified - in production use proper bitmap)
	bitmap := va.createParticipantBitmap(participants)

	// Aggregate signatures (simplified - in production use actual BLS aggregation)
	aggregateSig := va.aggregateSignatures(batches)

	result := &ConsensusResult{
		EpochId:                 refBatch.EpochId,
		ProjectIds:              refBatch.ProjectIds,
		SnapshotCids:            refBatch.SnapshotCids,
		AggregateSignature:      aggregateSig,
		ParticipantBitmap:       bitmap,
		TotalValidators:         uint32(len(va.votes)),
		ParticipatingValidators: uint32(len(batches)),
		MerkleRoot:              refBatch.MerkleRoot,
	}

	log.Printf("Consensus reached for epoch %d with %d/%d validators",
		va.epochID, len(batches), len(va.votes))

	return result
}

// aggregateSignatures combines multiple BLS signatures
func (va *VoteAggregator) aggregateSignatures(batches []*FinalizedBatch) []byte {
	// Simplified implementation - in production use actual BLS library
	// For now, just concatenate and hash
	combined := ""
	for _, batch := range batches {
		combined += hex.EncodeToString(batch.BlsSignature)
	}

	// In production: use proper BLS aggregation
	// return bls.AggregateSignatures(signatures)

	// For testing: simple concatenation
	return []byte(combined[:64]) // Return first 64 chars as dummy aggregate
}

// createParticipantBitmap creates a bitmap of participating validators
func (va *VoteAggregator) createParticipantBitmap(participants []string) []byte {
	// Simplified bitmap - in production use proper bitmap with validator indices
	bitmap := make([]byte, (len(participants)+7)/8)
	for i := range participants {
		byteIndex := i / 8
		bitIndex := i % 8
		bitmap[byteIndex] |= 1 << bitIndex
	}
	return bitmap
}

// GetStatus returns the current aggregation status
func (va *VoteAggregator) GetStatus() map[string]interface{} {
	va.mu.RLock()
	defer va.mu.RUnlock()

	// Count votes by merkle root
	rootCounts := make(map[string]int)
	for _, batch := range va.votes {
		rootHex := hex.EncodeToString(batch.MerkleRoot)
		rootCounts[rootHex]++
	}

	// Find leading root
	maxCount := 0
	var leadingRoot string
	for root, count := range rootCounts {
		if count > maxCount {
			maxCount = count
			leadingRoot = root
		}
	}

	timeRemaining := time.Until(va.votingDeadline).Seconds()
	if timeRemaining < 0 {
		timeRemaining = 0
	}

	return map[string]interface{}{
		"epoch_id":       va.epochID,
		"total_votes":    len(va.votes),
		"required_votes": int(float64(len(va.votes)) * va.voteThreshold),
		"unique_roots":   len(rootCounts),
		"leading_root":   leadingRoot,
		"leading_count":  maxCount,
		"deadline":       va.votingDeadline.Format(time.RFC3339),
		"time_remaining": timeRemaining,
	}
}

// SetConsensusCallback sets the callback for when consensus is reached
func (va *VoteAggregator) SetConsensusCallback(cb func(*ConsensusResult)) {
	va.onConsensus = cb
}

// ConsensusResult struct (matches protobuf structure)
type ConsensusResult struct {
	EpochId                 uint64
	ProjectIds              []string
	SnapshotCids            []string
	AggregateSignature      []byte
	ParticipantBitmap       []byte
	TotalValidators         uint32
	ParticipatingValidators uint32
	MerkleRoot              []byte
}
