package protocolstate

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	rpchelper "github.com/powerloom/go-rpc-helper"
	"github.com/powerloom/snapshot-sequencer-validator/pkgs/eventmonitor"
)

// GetSnapshotterStateAddress retrieves the SnapshotterState contract address from ProtocolState contract
// This follows the same pattern as FetchVPAAddress in pkgs/vpa/fetcher.go
func GetSnapshotterStateAddress(ctx context.Context, rpcHelper *rpchelper.RPCHelper, protocolStateContract string, contractABIPath string) (common.Address, error) {
	if protocolStateContract == "" {
		return common.Address{}, fmt.Errorf("ProtocolState contract address is required")
	}

	// Load ProtocolState ABI using eventmonitor loader (same pattern as eventmonitor)
	protocolStateABI, err := eventmonitor.LoadContractABI(contractABIPath)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to load ProtocolState ABI: %w", err)
	}

	// Create contract backend from RPC helper
	contractBackend := rpcHelper.NewContractBackend()

	// Create contract binding
	parsedABI := protocolStateABI.GetABI()
	protocolStateAddress := common.HexToAddress(protocolStateContract)
	contract := bind.NewBoundContract(protocolStateAddress, parsedABI, contractBackend, contractBackend, contractBackend)

	// Call snapshotterState() method
	var results []interface{}
	opts := &bind.CallOpts{Context: ctx}
	err = contract.Call(opts, &results, "snapshotterState")
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to call snapshotterState(): %w", err)
	}

	if len(results) == 0 {
		return common.Address{}, fmt.Errorf("snapshotterState() returned no results")
	}

	snapshotterStateAddr, ok := results[0].(common.Address)
	if !ok {
		return common.Address{}, fmt.Errorf("snapshotterState() returned invalid address type")
	}

	if snapshotterStateAddr == (common.Address{}) {
		return common.Address{}, fmt.Errorf("snapshotterState() returned zero address")
	}

	return snapshotterStateAddr, nil
}

