// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// SnapshotterStateContractMetaData contains all meta data concerning the SnapshotterStateContract contract.
var SnapshotterStateContractMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"target\",\"type\":\"address\"}],\"name\":\"AddressEmptyCode\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"balance\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"needed\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"tokenId\",\"type\":\"uint256\"}],\"name\":\"ERC1155InsufficientBalance\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"approver\",\"type\":\"address\"}],\"name\":\"ERC1155InvalidApprover\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"idsLength\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"valuesLength\",\"type\":\"uint256\"}],\"name\":\"ERC1155InvalidArrayLength\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"ERC1155InvalidOperator\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"receiver\",\"type\":\"address\"}],\"name\":\"ERC1155InvalidReceiver\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"sender\",\"type\":\"address\"}],\"name\":\"ERC1155InvalidSender\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"ERC1155MissingApprovalForAll\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"implementation\",\"type\":\"address\"}],\"name\":\"ERC1967InvalidImplementation\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ERC1967NonPayable\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"EnforcedPause\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ExpectedPause\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"FailedCall\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"InvalidInitialization\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"NotInitializing\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"OwnableInvalidOwner\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"OwnableUnauthorizedAccount\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ReentrancyGuardReentrantCall\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"UUPSUnauthorizedCallContext\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"slot\",\"type\":\"bytes32\"}],\"name\":\"UUPSUnsupportedProxiableUUID\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"adminAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"allowed\",\"type\":\"bool\"}],\"name\":\"AdminsUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"approved\",\"type\":\"bool\"}],\"name\":\"ApprovalForAll\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"paramName\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"newValue\",\"type\":\"uint256\"}],\"name\":\"ConfigurationUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"Deposit\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"EmergencyWithdraw\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint64\",\"name\":\"version\",\"type\":\"uint64\"}],\"name\":\"Initialized\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"claimer\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nodeId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"LegacyNodeTokensClaimed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"newName\",\"type\":\"string\"}],\"name\":\"NameUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nodeId\",\"type\":\"uint256\"}],\"name\":\"NodeBurned\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nodeId\",\"type\":\"uint256\"}],\"name\":\"NodeMinted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferStarted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Paused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nodeId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"oldSnapshotter\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"newSnapshotter\",\"type\":\"address\"}],\"name\":\"SnapshotterAddressChanged\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newSnapshotterState\",\"type\":\"address\"}],\"name\":\"SnapshotterStateUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"claimer\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nodeId\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"SnapshotterTokensClaimed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256[]\",\"name\":\"ids\",\"type\":\"uint256[]\"},{\"indexed\":false,\"internalType\":\"uint256[]\",\"name\":\"values\",\"type\":\"uint256[]\"}],\"name\":\"TransferBatch\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"from\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"to\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"value\",\"type\":\"uint256\"}],\"name\":\"TransferSingle\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"value\",\"type\":\"string\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"URI\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"newUri\",\"type\":\"string\"}],\"name\":\"URIUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"Unpaused\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"implementation\",\"type\":\"address\"}],\"name\":\"Upgraded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"snapshotterAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"bool\",\"name\":\"allowed\",\"type\":\"bool\"}],\"name\":\"allSnapshottersUpdated\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"MAX_SUPPLY\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"UPGRADE_INTERFACE_VERSION\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"acceptOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_to\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"_amount\",\"type\":\"uint256\"},{\"internalType\":\"bool\",\"name\":\"_isKyced\",\"type\":\"bool\"}],\"name\":\"adminMintLegacyNodes\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"name\":\"allSnapshotters\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"nodeId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"snapshotterAddress\",\"type\":\"address\"}],\"name\":\"assignSnapshotterToNode\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"nodeId\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"snapshotterAddress\",\"type\":\"address\"}],\"name\":\"assignSnapshotterToNodeAdmin\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256[]\",\"name\":\"nodeIds\",\"type\":\"uint256[]\"},{\"internalType\":\"address[]\",\"name\":\"snapshotterAddresses\",\"type\":\"address[]\"}],\"name\":\"assignSnapshotterToNodeBulk\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256[]\",\"name\":\"nodeIds\",\"type\":\"uint256[]\"},{\"internalType\":\"address[]\",\"name\":\"snapshotterAddresses\",\"type\":\"address[]\"}],\"name\":\"assignSnapshotterToNodeBulkAdmin\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"balanceOf\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"accounts\",\"type\":\"address[]\"},{\"internalType\":\"uint256[]\",\"name\":\"ids\",\"type\":\"uint256[]\"}],\"name\":\"balanceOfBatch\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_nodeId\",\"type\":\"uint256\"}],\"name\":\"burnNode\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_nodeId\",\"type\":\"uint256\"}],\"name\":\"claimNodeTokens\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_nodeId\",\"type\":\"uint256\"}],\"name\":\"claimableLegacyNodeTokens\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_nodeId\",\"type\":\"uint256\"}],\"name\":\"claimableNodeTokens\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"_claimableNodeTokens\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_nodeId\",\"type\":\"uint256\"}],\"name\":\"completeKyc\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_legacyNodeCount\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_legacyNodeInitialClaimPercentage\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_legacyNodeCliff\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_legacyNodeValue\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_legacyNodeVestingDays\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_legacyNodeVestingStart\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_legacyTokensSentOnL1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"_legacyNodeNonKycedCooldown\",\"type\":\"uint256\"}],\"name\":\"configureLegacyNodes\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"emergencyWithdraw\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"enabledNodeCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"exists\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getAdmins\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"getAllUserNodeIds\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getLegacyInitialClaim\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"getNodesOwned\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getTotalSnapshotterCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"getUserBurnedNodeIds\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"getUserOwnedNodeIds\",\"outputs\":[{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"initialOwner\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"initialNodePrice\",\"type\":\"uint256\"},{\"internalType\":\"string\",\"name\":\"initialName\",\"type\":\"string\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"isApprovedForAll\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_nodeId\",\"type\":\"uint256\"}],\"name\":\"isNodeAvailable\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"isNodeBurned\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"lastSnapshotterChange\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"legacyNodeCliff\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"legacyNodeCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"legacyNodeInitialClaimPercentage\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"legacyNodeNonKycedCooldown\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"legacyNodeValue\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"legacyNodeVestingDays\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"legacyNodeVestingStart\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"legacyTokensSentOnL1\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"amount\",\"type\":\"uint256\"}],\"name\":\"mintNode\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"mintStartTime\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"name\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"nodeCount\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"nodeIdToOwner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"nodeIdToVestingInfo\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"initialClaim\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"tokensAfterInitialClaim\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"tokensClaimed\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"lastClaim\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"nodeInfo\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"snapshotterAddress\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"nodePrice\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"amountSentOnL1\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"mintedOn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"burnedOn\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"lastUpdated\",\"type\":\"uint256\"},{\"internalType\":\"bool\",\"name\":\"isLegacy\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"claimedTokens\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"active\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"isKyced\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"nodePrice\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"nodeId\",\"type\":\"uint256\"}],\"name\":\"nodeSnapshotterMapping\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pause\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"pendingOwner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"proxiableUUID\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256[]\",\"name\":\"\",\"type\":\"uint256[]\"},{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"name\":\"safeBatchTransferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"},{\"internalType\":\"bytes\",\"name\":\"\",\"type\":\"bytes\"}],\"name\":\"safeTransferFrom\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"},{\"internalType\":\"bool\",\"name\":\"approved\",\"type\":\"bool\"}],\"name\":\"setApprovalForAll\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_mintStartTime\",\"type\":\"uint256\"}],\"name\":\"setMintStartTime\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"_name\",\"type\":\"string\"}],\"name\":\"setName\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_snapshotterAddressChangeCooldown\",\"type\":\"uint256\"}],\"name\":\"setSnapshotterAddressChangeCooldown\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_snapshotterTokenClaimCooldown\",\"type\":\"uint256\"}],\"name\":\"setSnapshotterTokenClaimCooldown\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"newuri\",\"type\":\"string\"}],\"name\":\"setURI\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"snapshotterAddressChangeCooldown\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"snapshotterTokenClaimCooldown\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes4\",\"name\":\"interfaceId\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"id\",\"type\":\"uint256\"}],\"name\":\"totalSupply\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"unpause\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"_admins\",\"type\":\"address[]\"},{\"internalType\":\"bool[]\",\"name\":\"_status\",\"type\":\"bool[]\"}],\"name\":\"updateAdmins\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_maxSupply\",\"type\":\"uint256\"}],\"name\":\"updateMaxSupply\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"_nodePrice\",\"type\":\"uint256\"}],\"name\":\"updateNodePrice\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newImplementation\",\"type\":\"address\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"upgradeToAndCall\",\"outputs\":[],\"stateMutability\":\"payable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"uri\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"vestedLegacyNodeTokens\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"stateMutability\":\"payable\",\"type\":\"receive\"}]",
}

// SnapshotterStateContractABI is the input ABI used to generate the binding from.
// Deprecated: Use SnapshotterStateContractMetaData.ABI instead.
var SnapshotterStateContractABI = SnapshotterStateContractMetaData.ABI

// SnapshotterStateContract is an auto generated Go binding around an Ethereum contract.
type SnapshotterStateContract struct {
	SnapshotterStateContractCaller     // Read-only binding to the contract
	SnapshotterStateContractTransactor // Write-only binding to the contract
	SnapshotterStateContractFilterer   // Log filterer for contract events
}

// SnapshotterStateContractCaller is an auto generated read-only Go binding around an Ethereum contract.
type SnapshotterStateContractCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SnapshotterStateContractTransactor is an auto generated write-only Go binding around an Ethereum contract.
type SnapshotterStateContractTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SnapshotterStateContractFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type SnapshotterStateContractFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SnapshotterStateContractSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type SnapshotterStateContractSession struct {
	Contract     *SnapshotterStateContract // Generic contract binding to set the session for
	CallOpts     bind.CallOpts             // Call options to use throughout this session
	TransactOpts bind.TransactOpts         // Transaction auth options to use throughout this session
}

// SnapshotterStateContractCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type SnapshotterStateContractCallerSession struct {
	Contract *SnapshotterStateContractCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                   // Call options to use throughout this session
}

// SnapshotterStateContractTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type SnapshotterStateContractTransactorSession struct {
	Contract     *SnapshotterStateContractTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                   // Transaction auth options to use throughout this session
}

// SnapshotterStateContractRaw is an auto generated low-level Go binding around an Ethereum contract.
type SnapshotterStateContractRaw struct {
	Contract *SnapshotterStateContract // Generic contract binding to access the raw methods on
}

// SnapshotterStateContractCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type SnapshotterStateContractCallerRaw struct {
	Contract *SnapshotterStateContractCaller // Generic read-only contract binding to access the raw methods on
}

// SnapshotterStateContractTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type SnapshotterStateContractTransactorRaw struct {
	Contract *SnapshotterStateContractTransactor // Generic write-only contract binding to access the raw methods on
}

// NewSnapshotterStateContract creates a new instance of SnapshotterStateContract, bound to a specific deployed contract.
func NewSnapshotterStateContract(address common.Address, backend bind.ContractBackend) (*SnapshotterStateContract, error) {
	contract, err := bindSnapshotterStateContract(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContract{SnapshotterStateContractCaller: SnapshotterStateContractCaller{contract: contract}, SnapshotterStateContractTransactor: SnapshotterStateContractTransactor{contract: contract}, SnapshotterStateContractFilterer: SnapshotterStateContractFilterer{contract: contract}}, nil
}

// NewSnapshotterStateContractCaller creates a new read-only instance of SnapshotterStateContract, bound to a specific deployed contract.
func NewSnapshotterStateContractCaller(address common.Address, caller bind.ContractCaller) (*SnapshotterStateContractCaller, error) {
	contract, err := bindSnapshotterStateContract(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractCaller{contract: contract}, nil
}

// NewSnapshotterStateContractTransactor creates a new write-only instance of SnapshotterStateContract, bound to a specific deployed contract.
func NewSnapshotterStateContractTransactor(address common.Address, transactor bind.ContractTransactor) (*SnapshotterStateContractTransactor, error) {
	contract, err := bindSnapshotterStateContract(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractTransactor{contract: contract}, nil
}

// NewSnapshotterStateContractFilterer creates a new log filterer instance of SnapshotterStateContract, bound to a specific deployed contract.
func NewSnapshotterStateContractFilterer(address common.Address, filterer bind.ContractFilterer) (*SnapshotterStateContractFilterer, error) {
	contract, err := bindSnapshotterStateContract(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractFilterer{contract: contract}, nil
}

// bindSnapshotterStateContract binds a generic wrapper to an already deployed contract.
func bindSnapshotterStateContract(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := SnapshotterStateContractMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SnapshotterStateContract *SnapshotterStateContractRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SnapshotterStateContract.Contract.SnapshotterStateContractCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SnapshotterStateContract *SnapshotterStateContractRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SnapshotterStateContractTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SnapshotterStateContract *SnapshotterStateContractRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SnapshotterStateContractTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SnapshotterStateContract *SnapshotterStateContractCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SnapshotterStateContract.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SnapshotterStateContract *SnapshotterStateContractTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SnapshotterStateContract *SnapshotterStateContractTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.contract.Transact(opts, method, params...)
}

// MAXSUPPLY is a free data retrieval call binding the contract method 0x32cb6b0c.
//
// Solidity: function MAX_SUPPLY() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) MAXSUPPLY(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "MAX_SUPPLY")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MAXSUPPLY is a free data retrieval call binding the contract method 0x32cb6b0c.
//
// Solidity: function MAX_SUPPLY() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) MAXSUPPLY() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.MAXSUPPLY(&_SnapshotterStateContract.CallOpts)
}

// MAXSUPPLY is a free data retrieval call binding the contract method 0x32cb6b0c.
//
// Solidity: function MAX_SUPPLY() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) MAXSUPPLY() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.MAXSUPPLY(&_SnapshotterStateContract.CallOpts)
}

// UPGRADEINTERFACEVERSION is a free data retrieval call binding the contract method 0xad3cb1cc.
//
// Solidity: function UPGRADE_INTERFACE_VERSION() view returns(string)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) UPGRADEINTERFACEVERSION(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "UPGRADE_INTERFACE_VERSION")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// UPGRADEINTERFACEVERSION is a free data retrieval call binding the contract method 0xad3cb1cc.
//
// Solidity: function UPGRADE_INTERFACE_VERSION() view returns(string)
func (_SnapshotterStateContract *SnapshotterStateContractSession) UPGRADEINTERFACEVERSION() (string, error) {
	return _SnapshotterStateContract.Contract.UPGRADEINTERFACEVERSION(&_SnapshotterStateContract.CallOpts)
}

// UPGRADEINTERFACEVERSION is a free data retrieval call binding the contract method 0xad3cb1cc.
//
// Solidity: function UPGRADE_INTERFACE_VERSION() view returns(string)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) UPGRADEINTERFACEVERSION() (string, error) {
	return _SnapshotterStateContract.Contract.UPGRADEINTERFACEVERSION(&_SnapshotterStateContract.CallOpts)
}

// AllSnapshotters is a free data retrieval call binding the contract method 0x3d15d0f4.
//
// Solidity: function allSnapshotters(address ) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) AllSnapshotters(opts *bind.CallOpts, arg0 common.Address) (bool, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "allSnapshotters", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// AllSnapshotters is a free data retrieval call binding the contract method 0x3d15d0f4.
//
// Solidity: function allSnapshotters(address ) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractSession) AllSnapshotters(arg0 common.Address) (bool, error) {
	return _SnapshotterStateContract.Contract.AllSnapshotters(&_SnapshotterStateContract.CallOpts, arg0)
}

// AllSnapshotters is a free data retrieval call binding the contract method 0x3d15d0f4.
//
// Solidity: function allSnapshotters(address ) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) AllSnapshotters(arg0 common.Address) (bool, error) {
	return _SnapshotterStateContract.Contract.AllSnapshotters(&_SnapshotterStateContract.CallOpts, arg0)
}

// BalanceOf is a free data retrieval call binding the contract method 0x00fdd58e.
//
// Solidity: function balanceOf(address account, uint256 id) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) BalanceOf(opts *bind.CallOpts, account common.Address, id *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "balanceOf", account, id)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// BalanceOf is a free data retrieval call binding the contract method 0x00fdd58e.
//
// Solidity: function balanceOf(address account, uint256 id) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) BalanceOf(account common.Address, id *big.Int) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.BalanceOf(&_SnapshotterStateContract.CallOpts, account, id)
}

// BalanceOf is a free data retrieval call binding the contract method 0x00fdd58e.
//
// Solidity: function balanceOf(address account, uint256 id) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) BalanceOf(account common.Address, id *big.Int) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.BalanceOf(&_SnapshotterStateContract.CallOpts, account, id)
}

// BalanceOfBatch is a free data retrieval call binding the contract method 0x4e1273f4.
//
// Solidity: function balanceOfBatch(address[] accounts, uint256[] ids) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractCaller) BalanceOfBatch(opts *bind.CallOpts, accounts []common.Address, ids []*big.Int) ([]*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "balanceOfBatch", accounts, ids)

	if err != nil {
		return *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)

	return out0, err

}

// BalanceOfBatch is a free data retrieval call binding the contract method 0x4e1273f4.
//
// Solidity: function balanceOfBatch(address[] accounts, uint256[] ids) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractSession) BalanceOfBatch(accounts []common.Address, ids []*big.Int) ([]*big.Int, error) {
	return _SnapshotterStateContract.Contract.BalanceOfBatch(&_SnapshotterStateContract.CallOpts, accounts, ids)
}

// BalanceOfBatch is a free data retrieval call binding the contract method 0x4e1273f4.
//
// Solidity: function balanceOfBatch(address[] accounts, uint256[] ids) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) BalanceOfBatch(accounts []common.Address, ids []*big.Int) ([]*big.Int, error) {
	return _SnapshotterStateContract.Contract.BalanceOfBatch(&_SnapshotterStateContract.CallOpts, accounts, ids)
}

// ClaimableLegacyNodeTokens is a free data retrieval call binding the contract method 0x6e961585.
//
// Solidity: function claimableLegacyNodeTokens(uint256 _nodeId) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) ClaimableLegacyNodeTokens(opts *bind.CallOpts, _nodeId *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "claimableLegacyNodeTokens", _nodeId)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ClaimableLegacyNodeTokens is a free data retrieval call binding the contract method 0x6e961585.
//
// Solidity: function claimableLegacyNodeTokens(uint256 _nodeId) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) ClaimableLegacyNodeTokens(_nodeId *big.Int) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.ClaimableLegacyNodeTokens(&_SnapshotterStateContract.CallOpts, _nodeId)
}

// ClaimableLegacyNodeTokens is a free data retrieval call binding the contract method 0x6e961585.
//
// Solidity: function claimableLegacyNodeTokens(uint256 _nodeId) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) ClaimableLegacyNodeTokens(_nodeId *big.Int) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.ClaimableLegacyNodeTokens(&_SnapshotterStateContract.CallOpts, _nodeId)
}

// ClaimableNodeTokens is a free data retrieval call binding the contract method 0xb5719823.
//
// Solidity: function claimableNodeTokens(uint256 _nodeId) view returns(uint256 _claimableNodeTokens)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) ClaimableNodeTokens(opts *bind.CallOpts, _nodeId *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "claimableNodeTokens", _nodeId)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ClaimableNodeTokens is a free data retrieval call binding the contract method 0xb5719823.
//
// Solidity: function claimableNodeTokens(uint256 _nodeId) view returns(uint256 _claimableNodeTokens)
func (_SnapshotterStateContract *SnapshotterStateContractSession) ClaimableNodeTokens(_nodeId *big.Int) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.ClaimableNodeTokens(&_SnapshotterStateContract.CallOpts, _nodeId)
}

// ClaimableNodeTokens is a free data retrieval call binding the contract method 0xb5719823.
//
// Solidity: function claimableNodeTokens(uint256 _nodeId) view returns(uint256 _claimableNodeTokens)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) ClaimableNodeTokens(_nodeId *big.Int) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.ClaimableNodeTokens(&_SnapshotterStateContract.CallOpts, _nodeId)
}

// EnabledNodeCount is a free data retrieval call binding the contract method 0xce7b6afb.
//
// Solidity: function enabledNodeCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) EnabledNodeCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "enabledNodeCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// EnabledNodeCount is a free data retrieval call binding the contract method 0xce7b6afb.
//
// Solidity: function enabledNodeCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) EnabledNodeCount() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.EnabledNodeCount(&_SnapshotterStateContract.CallOpts)
}

// EnabledNodeCount is a free data retrieval call binding the contract method 0xce7b6afb.
//
// Solidity: function enabledNodeCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) EnabledNodeCount() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.EnabledNodeCount(&_SnapshotterStateContract.CallOpts)
}

// Exists is a free data retrieval call binding the contract method 0x4f558e79.
//
// Solidity: function exists(uint256 id) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) Exists(opts *bind.CallOpts, id *big.Int) (bool, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "exists", id)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Exists is a free data retrieval call binding the contract method 0x4f558e79.
//
// Solidity: function exists(uint256 id) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractSession) Exists(id *big.Int) (bool, error) {
	return _SnapshotterStateContract.Contract.Exists(&_SnapshotterStateContract.CallOpts, id)
}

// Exists is a free data retrieval call binding the contract method 0x4f558e79.
//
// Solidity: function exists(uint256 id) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) Exists(id *big.Int) (bool, error) {
	return _SnapshotterStateContract.Contract.Exists(&_SnapshotterStateContract.CallOpts, id)
}

// GetAdmins is a free data retrieval call binding the contract method 0x31ae450b.
//
// Solidity: function getAdmins() view returns(address[])
func (_SnapshotterStateContract *SnapshotterStateContractCaller) GetAdmins(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "getAdmins")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetAdmins is a free data retrieval call binding the contract method 0x31ae450b.
//
// Solidity: function getAdmins() view returns(address[])
func (_SnapshotterStateContract *SnapshotterStateContractSession) GetAdmins() ([]common.Address, error) {
	return _SnapshotterStateContract.Contract.GetAdmins(&_SnapshotterStateContract.CallOpts)
}

// GetAdmins is a free data retrieval call binding the contract method 0x31ae450b.
//
// Solidity: function getAdmins() view returns(address[])
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) GetAdmins() ([]common.Address, error) {
	return _SnapshotterStateContract.Contract.GetAdmins(&_SnapshotterStateContract.CallOpts)
}

// GetAllUserNodeIds is a free data retrieval call binding the contract method 0xc3739643.
//
// Solidity: function getAllUserNodeIds(address _address) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractCaller) GetAllUserNodeIds(opts *bind.CallOpts, _address common.Address) ([]*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "getAllUserNodeIds", _address)

	if err != nil {
		return *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)

	return out0, err

}

// GetAllUserNodeIds is a free data retrieval call binding the contract method 0xc3739643.
//
// Solidity: function getAllUserNodeIds(address _address) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractSession) GetAllUserNodeIds(_address common.Address) ([]*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetAllUserNodeIds(&_SnapshotterStateContract.CallOpts, _address)
}

// GetAllUserNodeIds is a free data retrieval call binding the contract method 0xc3739643.
//
// Solidity: function getAllUserNodeIds(address _address) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) GetAllUserNodeIds(_address common.Address) ([]*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetAllUserNodeIds(&_SnapshotterStateContract.CallOpts, _address)
}

// GetLegacyInitialClaim is a free data retrieval call binding the contract method 0x72133626.
//
// Solidity: function getLegacyInitialClaim() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) GetLegacyInitialClaim(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "getLegacyInitialClaim")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetLegacyInitialClaim is a free data retrieval call binding the contract method 0x72133626.
//
// Solidity: function getLegacyInitialClaim() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) GetLegacyInitialClaim() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetLegacyInitialClaim(&_SnapshotterStateContract.CallOpts)
}

// GetLegacyInitialClaim is a free data retrieval call binding the contract method 0x72133626.
//
// Solidity: function getLegacyInitialClaim() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) GetLegacyInitialClaim() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetLegacyInitialClaim(&_SnapshotterStateContract.CallOpts)
}

// GetNodesOwned is a free data retrieval call binding the contract method 0x2b754a12.
//
// Solidity: function getNodesOwned(address _address) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) GetNodesOwned(opts *bind.CallOpts, _address common.Address) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "getNodesOwned", _address)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetNodesOwned is a free data retrieval call binding the contract method 0x2b754a12.
//
// Solidity: function getNodesOwned(address _address) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) GetNodesOwned(_address common.Address) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetNodesOwned(&_SnapshotterStateContract.CallOpts, _address)
}

// GetNodesOwned is a free data retrieval call binding the contract method 0x2b754a12.
//
// Solidity: function getNodesOwned(address _address) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) GetNodesOwned(_address common.Address) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetNodesOwned(&_SnapshotterStateContract.CallOpts, _address)
}

// GetTotalSnapshotterCount is a free data retrieval call binding the contract method 0x92ae6f66.
//
// Solidity: function getTotalSnapshotterCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) GetTotalSnapshotterCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "getTotalSnapshotterCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetTotalSnapshotterCount is a free data retrieval call binding the contract method 0x92ae6f66.
//
// Solidity: function getTotalSnapshotterCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) GetTotalSnapshotterCount() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetTotalSnapshotterCount(&_SnapshotterStateContract.CallOpts)
}

// GetTotalSnapshotterCount is a free data retrieval call binding the contract method 0x92ae6f66.
//
// Solidity: function getTotalSnapshotterCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) GetTotalSnapshotterCount() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetTotalSnapshotterCount(&_SnapshotterStateContract.CallOpts)
}

// GetUserBurnedNodeIds is a free data retrieval call binding the contract method 0x177d6994.
//
// Solidity: function getUserBurnedNodeIds(address _address) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractCaller) GetUserBurnedNodeIds(opts *bind.CallOpts, _address common.Address) ([]*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "getUserBurnedNodeIds", _address)

	if err != nil {
		return *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)

	return out0, err

}

// GetUserBurnedNodeIds is a free data retrieval call binding the contract method 0x177d6994.
//
// Solidity: function getUserBurnedNodeIds(address _address) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractSession) GetUserBurnedNodeIds(_address common.Address) ([]*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetUserBurnedNodeIds(&_SnapshotterStateContract.CallOpts, _address)
}

// GetUserBurnedNodeIds is a free data retrieval call binding the contract method 0x177d6994.
//
// Solidity: function getUserBurnedNodeIds(address _address) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) GetUserBurnedNodeIds(_address common.Address) ([]*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetUserBurnedNodeIds(&_SnapshotterStateContract.CallOpts, _address)
}

// GetUserOwnedNodeIds is a free data retrieval call binding the contract method 0x8214e6d5.
//
// Solidity: function getUserOwnedNodeIds(address _address) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractCaller) GetUserOwnedNodeIds(opts *bind.CallOpts, _address common.Address) ([]*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "getUserOwnedNodeIds", _address)

	if err != nil {
		return *new([]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([]*big.Int)).(*[]*big.Int)

	return out0, err

}

// GetUserOwnedNodeIds is a free data retrieval call binding the contract method 0x8214e6d5.
//
// Solidity: function getUserOwnedNodeIds(address _address) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractSession) GetUserOwnedNodeIds(_address common.Address) ([]*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetUserOwnedNodeIds(&_SnapshotterStateContract.CallOpts, _address)
}

// GetUserOwnedNodeIds is a free data retrieval call binding the contract method 0x8214e6d5.
//
// Solidity: function getUserOwnedNodeIds(address _address) view returns(uint256[])
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) GetUserOwnedNodeIds(_address common.Address) ([]*big.Int, error) {
	return _SnapshotterStateContract.Contract.GetUserOwnedNodeIds(&_SnapshotterStateContract.CallOpts, _address)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address account, address operator) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) IsApprovedForAll(opts *bind.CallOpts, account common.Address, operator common.Address) (bool, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "isApprovedForAll", account, operator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address account, address operator) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractSession) IsApprovedForAll(account common.Address, operator common.Address) (bool, error) {
	return _SnapshotterStateContract.Contract.IsApprovedForAll(&_SnapshotterStateContract.CallOpts, account, operator)
}

// IsApprovedForAll is a free data retrieval call binding the contract method 0xe985e9c5.
//
// Solidity: function isApprovedForAll(address account, address operator) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) IsApprovedForAll(account common.Address, operator common.Address) (bool, error) {
	return _SnapshotterStateContract.Contract.IsApprovedForAll(&_SnapshotterStateContract.CallOpts, account, operator)
}

// IsNodeAvailable is a free data retrieval call binding the contract method 0x97a9ae27.
//
// Solidity: function isNodeAvailable(uint256 _nodeId) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) IsNodeAvailable(opts *bind.CallOpts, _nodeId *big.Int) (bool, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "isNodeAvailable", _nodeId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsNodeAvailable is a free data retrieval call binding the contract method 0x97a9ae27.
//
// Solidity: function isNodeAvailable(uint256 _nodeId) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractSession) IsNodeAvailable(_nodeId *big.Int) (bool, error) {
	return _SnapshotterStateContract.Contract.IsNodeAvailable(&_SnapshotterStateContract.CallOpts, _nodeId)
}

// IsNodeAvailable is a free data retrieval call binding the contract method 0x97a9ae27.
//
// Solidity: function isNodeAvailable(uint256 _nodeId) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) IsNodeAvailable(_nodeId *big.Int) (bool, error) {
	return _SnapshotterStateContract.Contract.IsNodeAvailable(&_SnapshotterStateContract.CallOpts, _nodeId)
}

// IsNodeBurned is a free data retrieval call binding the contract method 0xb84113cb.
//
// Solidity: function isNodeBurned(uint256 ) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) IsNodeBurned(opts *bind.CallOpts, arg0 *big.Int) (bool, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "isNodeBurned", arg0)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsNodeBurned is a free data retrieval call binding the contract method 0xb84113cb.
//
// Solidity: function isNodeBurned(uint256 ) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractSession) IsNodeBurned(arg0 *big.Int) (bool, error) {
	return _SnapshotterStateContract.Contract.IsNodeBurned(&_SnapshotterStateContract.CallOpts, arg0)
}

// IsNodeBurned is a free data retrieval call binding the contract method 0xb84113cb.
//
// Solidity: function isNodeBurned(uint256 ) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) IsNodeBurned(arg0 *big.Int) (bool, error) {
	return _SnapshotterStateContract.Contract.IsNodeBurned(&_SnapshotterStateContract.CallOpts, arg0)
}

// LastSnapshotterChange is a free data retrieval call binding the contract method 0x9bb94bab.
//
// Solidity: function lastSnapshotterChange(uint256 ) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) LastSnapshotterChange(opts *bind.CallOpts, arg0 *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "lastSnapshotterChange", arg0)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LastSnapshotterChange is a free data retrieval call binding the contract method 0x9bb94bab.
//
// Solidity: function lastSnapshotterChange(uint256 ) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) LastSnapshotterChange(arg0 *big.Int) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LastSnapshotterChange(&_SnapshotterStateContract.CallOpts, arg0)
}

// LastSnapshotterChange is a free data retrieval call binding the contract method 0x9bb94bab.
//
// Solidity: function lastSnapshotterChange(uint256 ) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) LastSnapshotterChange(arg0 *big.Int) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LastSnapshotterChange(&_SnapshotterStateContract.CallOpts, arg0)
}

// LegacyNodeCliff is a free data retrieval call binding the contract method 0xc7996dee.
//
// Solidity: function legacyNodeCliff() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) LegacyNodeCliff(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "legacyNodeCliff")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LegacyNodeCliff is a free data retrieval call binding the contract method 0xc7996dee.
//
// Solidity: function legacyNodeCliff() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) LegacyNodeCliff() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeCliff(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeCliff is a free data retrieval call binding the contract method 0xc7996dee.
//
// Solidity: function legacyNodeCliff() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) LegacyNodeCliff() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeCliff(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeCount is a free data retrieval call binding the contract method 0x9c7e9945.
//
// Solidity: function legacyNodeCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) LegacyNodeCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "legacyNodeCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LegacyNodeCount is a free data retrieval call binding the contract method 0x9c7e9945.
//
// Solidity: function legacyNodeCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) LegacyNodeCount() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeCount(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeCount is a free data retrieval call binding the contract method 0x9c7e9945.
//
// Solidity: function legacyNodeCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) LegacyNodeCount() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeCount(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeInitialClaimPercentage is a free data retrieval call binding the contract method 0xab5820a2.
//
// Solidity: function legacyNodeInitialClaimPercentage() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) LegacyNodeInitialClaimPercentage(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "legacyNodeInitialClaimPercentage")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LegacyNodeInitialClaimPercentage is a free data retrieval call binding the contract method 0xab5820a2.
//
// Solidity: function legacyNodeInitialClaimPercentage() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) LegacyNodeInitialClaimPercentage() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeInitialClaimPercentage(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeInitialClaimPercentage is a free data retrieval call binding the contract method 0xab5820a2.
//
// Solidity: function legacyNodeInitialClaimPercentage() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) LegacyNodeInitialClaimPercentage() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeInitialClaimPercentage(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeNonKycedCooldown is a free data retrieval call binding the contract method 0x5901ba89.
//
// Solidity: function legacyNodeNonKycedCooldown() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) LegacyNodeNonKycedCooldown(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "legacyNodeNonKycedCooldown")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LegacyNodeNonKycedCooldown is a free data retrieval call binding the contract method 0x5901ba89.
//
// Solidity: function legacyNodeNonKycedCooldown() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) LegacyNodeNonKycedCooldown() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeNonKycedCooldown(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeNonKycedCooldown is a free data retrieval call binding the contract method 0x5901ba89.
//
// Solidity: function legacyNodeNonKycedCooldown() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) LegacyNodeNonKycedCooldown() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeNonKycedCooldown(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeValue is a free data retrieval call binding the contract method 0x3fd3623a.
//
// Solidity: function legacyNodeValue() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) LegacyNodeValue(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "legacyNodeValue")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LegacyNodeValue is a free data retrieval call binding the contract method 0x3fd3623a.
//
// Solidity: function legacyNodeValue() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) LegacyNodeValue() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeValue(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeValue is a free data retrieval call binding the contract method 0x3fd3623a.
//
// Solidity: function legacyNodeValue() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) LegacyNodeValue() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeValue(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeVestingDays is a free data retrieval call binding the contract method 0x02c00cc3.
//
// Solidity: function legacyNodeVestingDays() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) LegacyNodeVestingDays(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "legacyNodeVestingDays")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LegacyNodeVestingDays is a free data retrieval call binding the contract method 0x02c00cc3.
//
// Solidity: function legacyNodeVestingDays() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) LegacyNodeVestingDays() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeVestingDays(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeVestingDays is a free data retrieval call binding the contract method 0x02c00cc3.
//
// Solidity: function legacyNodeVestingDays() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) LegacyNodeVestingDays() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeVestingDays(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeVestingStart is a free data retrieval call binding the contract method 0x864e89f2.
//
// Solidity: function legacyNodeVestingStart() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) LegacyNodeVestingStart(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "legacyNodeVestingStart")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LegacyNodeVestingStart is a free data retrieval call binding the contract method 0x864e89f2.
//
// Solidity: function legacyNodeVestingStart() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) LegacyNodeVestingStart() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeVestingStart(&_SnapshotterStateContract.CallOpts)
}

// LegacyNodeVestingStart is a free data retrieval call binding the contract method 0x864e89f2.
//
// Solidity: function legacyNodeVestingStart() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) LegacyNodeVestingStart() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyNodeVestingStart(&_SnapshotterStateContract.CallOpts)
}

// LegacyTokensSentOnL1 is a free data retrieval call binding the contract method 0x1349c668.
//
// Solidity: function legacyTokensSentOnL1() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) LegacyTokensSentOnL1(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "legacyTokensSentOnL1")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// LegacyTokensSentOnL1 is a free data retrieval call binding the contract method 0x1349c668.
//
// Solidity: function legacyTokensSentOnL1() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) LegacyTokensSentOnL1() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyTokensSentOnL1(&_SnapshotterStateContract.CallOpts)
}

// LegacyTokensSentOnL1 is a free data retrieval call binding the contract method 0x1349c668.
//
// Solidity: function legacyTokensSentOnL1() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) LegacyTokensSentOnL1() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.LegacyTokensSentOnL1(&_SnapshotterStateContract.CallOpts)
}

// MintStartTime is a free data retrieval call binding the contract method 0x931e2e49.
//
// Solidity: function mintStartTime() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) MintStartTime(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "mintStartTime")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MintStartTime is a free data retrieval call binding the contract method 0x931e2e49.
//
// Solidity: function mintStartTime() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) MintStartTime() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.MintStartTime(&_SnapshotterStateContract.CallOpts)
}

// MintStartTime is a free data retrieval call binding the contract method 0x931e2e49.
//
// Solidity: function mintStartTime() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) MintStartTime() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.MintStartTime(&_SnapshotterStateContract.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) Name(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "name")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_SnapshotterStateContract *SnapshotterStateContractSession) Name() (string, error) {
	return _SnapshotterStateContract.Contract.Name(&_SnapshotterStateContract.CallOpts)
}

// Name is a free data retrieval call binding the contract method 0x06fdde03.
//
// Solidity: function name() view returns(string)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) Name() (string, error) {
	return _SnapshotterStateContract.Contract.Name(&_SnapshotterStateContract.CallOpts)
}

// NodeCount is a free data retrieval call binding the contract method 0x6da49b83.
//
// Solidity: function nodeCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) NodeCount(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "nodeCount")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NodeCount is a free data retrieval call binding the contract method 0x6da49b83.
//
// Solidity: function nodeCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) NodeCount() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.NodeCount(&_SnapshotterStateContract.CallOpts)
}

// NodeCount is a free data retrieval call binding the contract method 0x6da49b83.
//
// Solidity: function nodeCount() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) NodeCount() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.NodeCount(&_SnapshotterStateContract.CallOpts)
}

// NodeIdToOwner is a free data retrieval call binding the contract method 0xc274bd4f.
//
// Solidity: function nodeIdToOwner(uint256 ) view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) NodeIdToOwner(opts *bind.CallOpts, arg0 *big.Int) (common.Address, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "nodeIdToOwner", arg0)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// NodeIdToOwner is a free data retrieval call binding the contract method 0xc274bd4f.
//
// Solidity: function nodeIdToOwner(uint256 ) view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractSession) NodeIdToOwner(arg0 *big.Int) (common.Address, error) {
	return _SnapshotterStateContract.Contract.NodeIdToOwner(&_SnapshotterStateContract.CallOpts, arg0)
}

// NodeIdToOwner is a free data retrieval call binding the contract method 0xc274bd4f.
//
// Solidity: function nodeIdToOwner(uint256 ) view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) NodeIdToOwner(arg0 *big.Int) (common.Address, error) {
	return _SnapshotterStateContract.Contract.NodeIdToOwner(&_SnapshotterStateContract.CallOpts, arg0)
}

// NodeIdToVestingInfo is a free data retrieval call binding the contract method 0x63387621.
//
// Solidity: function nodeIdToVestingInfo(uint256 ) view returns(address owner, uint256 initialClaim, uint256 tokensAfterInitialClaim, uint256 tokensClaimed, uint256 lastClaim)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) NodeIdToVestingInfo(opts *bind.CallOpts, arg0 *big.Int) (struct {
	Owner                   common.Address
	InitialClaim            *big.Int
	TokensAfterInitialClaim *big.Int
	TokensClaimed           *big.Int
	LastClaim               *big.Int
}, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "nodeIdToVestingInfo", arg0)

	outstruct := new(struct {
		Owner                   common.Address
		InitialClaim            *big.Int
		TokensAfterInitialClaim *big.Int
		TokensClaimed           *big.Int
		LastClaim               *big.Int
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.Owner = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.InitialClaim = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.TokensAfterInitialClaim = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.TokensClaimed = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)
	outstruct.LastClaim = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)

	return *outstruct, err

}

// NodeIdToVestingInfo is a free data retrieval call binding the contract method 0x63387621.
//
// Solidity: function nodeIdToVestingInfo(uint256 ) view returns(address owner, uint256 initialClaim, uint256 tokensAfterInitialClaim, uint256 tokensClaimed, uint256 lastClaim)
func (_SnapshotterStateContract *SnapshotterStateContractSession) NodeIdToVestingInfo(arg0 *big.Int) (struct {
	Owner                   common.Address
	InitialClaim            *big.Int
	TokensAfterInitialClaim *big.Int
	TokensClaimed           *big.Int
	LastClaim               *big.Int
}, error) {
	return _SnapshotterStateContract.Contract.NodeIdToVestingInfo(&_SnapshotterStateContract.CallOpts, arg0)
}

// NodeIdToVestingInfo is a free data retrieval call binding the contract method 0x63387621.
//
// Solidity: function nodeIdToVestingInfo(uint256 ) view returns(address owner, uint256 initialClaim, uint256 tokensAfterInitialClaim, uint256 tokensClaimed, uint256 lastClaim)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) NodeIdToVestingInfo(arg0 *big.Int) (struct {
	Owner                   common.Address
	InitialClaim            *big.Int
	TokensAfterInitialClaim *big.Int
	TokensClaimed           *big.Int
	LastClaim               *big.Int
}, error) {
	return _SnapshotterStateContract.Contract.NodeIdToVestingInfo(&_SnapshotterStateContract.CallOpts, arg0)
}

// NodeInfo is a free data retrieval call binding the contract method 0xb02439ae.
//
// Solidity: function nodeInfo(uint256 ) view returns(address snapshotterAddress, uint256 nodePrice, uint256 amountSentOnL1, uint256 mintedOn, uint256 burnedOn, uint256 lastUpdated, bool isLegacy, bool claimedTokens, bool active, bool isKyced)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) NodeInfo(opts *bind.CallOpts, arg0 *big.Int) (struct {
	SnapshotterAddress common.Address
	NodePrice          *big.Int
	AmountSentOnL1     *big.Int
	MintedOn           *big.Int
	BurnedOn           *big.Int
	LastUpdated        *big.Int
	IsLegacy           bool
	ClaimedTokens      bool
	Active             bool
	IsKyced            bool
}, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "nodeInfo", arg0)

	outstruct := new(struct {
		SnapshotterAddress common.Address
		NodePrice          *big.Int
		AmountSentOnL1     *big.Int
		MintedOn           *big.Int
		BurnedOn           *big.Int
		LastUpdated        *big.Int
		IsLegacy           bool
		ClaimedTokens      bool
		Active             bool
		IsKyced            bool
	})
	if err != nil {
		return *outstruct, err
	}

	outstruct.SnapshotterAddress = *abi.ConvertType(out[0], new(common.Address)).(*common.Address)
	outstruct.NodePrice = *abi.ConvertType(out[1], new(*big.Int)).(**big.Int)
	outstruct.AmountSentOnL1 = *abi.ConvertType(out[2], new(*big.Int)).(**big.Int)
	outstruct.MintedOn = *abi.ConvertType(out[3], new(*big.Int)).(**big.Int)
	outstruct.BurnedOn = *abi.ConvertType(out[4], new(*big.Int)).(**big.Int)
	outstruct.LastUpdated = *abi.ConvertType(out[5], new(*big.Int)).(**big.Int)
	outstruct.IsLegacy = *abi.ConvertType(out[6], new(bool)).(*bool)
	outstruct.ClaimedTokens = *abi.ConvertType(out[7], new(bool)).(*bool)
	outstruct.Active = *abi.ConvertType(out[8], new(bool)).(*bool)
	outstruct.IsKyced = *abi.ConvertType(out[9], new(bool)).(*bool)

	return *outstruct, err

}

// NodeInfo is a free data retrieval call binding the contract method 0xb02439ae.
//
// Solidity: function nodeInfo(uint256 ) view returns(address snapshotterAddress, uint256 nodePrice, uint256 amountSentOnL1, uint256 mintedOn, uint256 burnedOn, uint256 lastUpdated, bool isLegacy, bool claimedTokens, bool active, bool isKyced)
func (_SnapshotterStateContract *SnapshotterStateContractSession) NodeInfo(arg0 *big.Int) (struct {
	SnapshotterAddress common.Address
	NodePrice          *big.Int
	AmountSentOnL1     *big.Int
	MintedOn           *big.Int
	BurnedOn           *big.Int
	LastUpdated        *big.Int
	IsLegacy           bool
	ClaimedTokens      bool
	Active             bool
	IsKyced            bool
}, error) {
	return _SnapshotterStateContract.Contract.NodeInfo(&_SnapshotterStateContract.CallOpts, arg0)
}

// NodeInfo is a free data retrieval call binding the contract method 0xb02439ae.
//
// Solidity: function nodeInfo(uint256 ) view returns(address snapshotterAddress, uint256 nodePrice, uint256 amountSentOnL1, uint256 mintedOn, uint256 burnedOn, uint256 lastUpdated, bool isLegacy, bool claimedTokens, bool active, bool isKyced)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) NodeInfo(arg0 *big.Int) (struct {
	SnapshotterAddress common.Address
	NodePrice          *big.Int
	AmountSentOnL1     *big.Int
	MintedOn           *big.Int
	BurnedOn           *big.Int
	LastUpdated        *big.Int
	IsLegacy           bool
	ClaimedTokens      bool
	Active             bool
	IsKyced            bool
}, error) {
	return _SnapshotterStateContract.Contract.NodeInfo(&_SnapshotterStateContract.CallOpts, arg0)
}

// NodePrice is a free data retrieval call binding the contract method 0xf1fec2b8.
//
// Solidity: function nodePrice() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) NodePrice(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "nodePrice")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// NodePrice is a free data retrieval call binding the contract method 0xf1fec2b8.
//
// Solidity: function nodePrice() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) NodePrice() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.NodePrice(&_SnapshotterStateContract.CallOpts)
}

// NodePrice is a free data retrieval call binding the contract method 0xf1fec2b8.
//
// Solidity: function nodePrice() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) NodePrice() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.NodePrice(&_SnapshotterStateContract.CallOpts)
}

// NodeSnapshotterMapping is a free data retrieval call binding the contract method 0xdfb0081e.
//
// Solidity: function nodeSnapshotterMapping(uint256 nodeId) view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) NodeSnapshotterMapping(opts *bind.CallOpts, nodeId *big.Int) (common.Address, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "nodeSnapshotterMapping", nodeId)

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// NodeSnapshotterMapping is a free data retrieval call binding the contract method 0xdfb0081e.
//
// Solidity: function nodeSnapshotterMapping(uint256 nodeId) view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractSession) NodeSnapshotterMapping(nodeId *big.Int) (common.Address, error) {
	return _SnapshotterStateContract.Contract.NodeSnapshotterMapping(&_SnapshotterStateContract.CallOpts, nodeId)
}

// NodeSnapshotterMapping is a free data retrieval call binding the contract method 0xdfb0081e.
//
// Solidity: function nodeSnapshotterMapping(uint256 nodeId) view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) NodeSnapshotterMapping(nodeId *big.Int) (common.Address, error) {
	return _SnapshotterStateContract.Contract.NodeSnapshotterMapping(&_SnapshotterStateContract.CallOpts, nodeId)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractSession) Owner() (common.Address, error) {
	return _SnapshotterStateContract.Contract.Owner(&_SnapshotterStateContract.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) Owner() (common.Address, error) {
	return _SnapshotterStateContract.Contract.Owner(&_SnapshotterStateContract.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "paused")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractSession) Paused() (bool, error) {
	return _SnapshotterStateContract.Contract.Paused(&_SnapshotterStateContract.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) Paused() (bool, error) {
	return _SnapshotterStateContract.Contract.Paused(&_SnapshotterStateContract.CallOpts)
}

// PendingOwner is a free data retrieval call binding the contract method 0xe30c3978.
//
// Solidity: function pendingOwner() view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) PendingOwner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "pendingOwner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// PendingOwner is a free data retrieval call binding the contract method 0xe30c3978.
//
// Solidity: function pendingOwner() view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractSession) PendingOwner() (common.Address, error) {
	return _SnapshotterStateContract.Contract.PendingOwner(&_SnapshotterStateContract.CallOpts)
}

// PendingOwner is a free data retrieval call binding the contract method 0xe30c3978.
//
// Solidity: function pendingOwner() view returns(address)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) PendingOwner() (common.Address, error) {
	return _SnapshotterStateContract.Contract.PendingOwner(&_SnapshotterStateContract.CallOpts)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) ProxiableUUID(opts *bind.CallOpts) ([32]byte, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "proxiableUUID")

	if err != nil {
		return *new([32]byte), err
	}

	out0 := *abi.ConvertType(out[0], new([32]byte)).(*[32]byte)

	return out0, err

}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_SnapshotterStateContract *SnapshotterStateContractSession) ProxiableUUID() ([32]byte, error) {
	return _SnapshotterStateContract.Contract.ProxiableUUID(&_SnapshotterStateContract.CallOpts)
}

// ProxiableUUID is a free data retrieval call binding the contract method 0x52d1902d.
//
// Solidity: function proxiableUUID() view returns(bytes32)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) ProxiableUUID() ([32]byte, error) {
	return _SnapshotterStateContract.Contract.ProxiableUUID(&_SnapshotterStateContract.CallOpts)
}

// SnapshotterAddressChangeCooldown is a free data retrieval call binding the contract method 0x2d5b493d.
//
// Solidity: function snapshotterAddressChangeCooldown() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) SnapshotterAddressChangeCooldown(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "snapshotterAddressChangeCooldown")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SnapshotterAddressChangeCooldown is a free data retrieval call binding the contract method 0x2d5b493d.
//
// Solidity: function snapshotterAddressChangeCooldown() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) SnapshotterAddressChangeCooldown() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.SnapshotterAddressChangeCooldown(&_SnapshotterStateContract.CallOpts)
}

// SnapshotterAddressChangeCooldown is a free data retrieval call binding the contract method 0x2d5b493d.
//
// Solidity: function snapshotterAddressChangeCooldown() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) SnapshotterAddressChangeCooldown() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.SnapshotterAddressChangeCooldown(&_SnapshotterStateContract.CallOpts)
}

// SnapshotterTokenClaimCooldown is a free data retrieval call binding the contract method 0x950595f2.
//
// Solidity: function snapshotterTokenClaimCooldown() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) SnapshotterTokenClaimCooldown(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "snapshotterTokenClaimCooldown")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// SnapshotterTokenClaimCooldown is a free data retrieval call binding the contract method 0x950595f2.
//
// Solidity: function snapshotterTokenClaimCooldown() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) SnapshotterTokenClaimCooldown() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.SnapshotterTokenClaimCooldown(&_SnapshotterStateContract.CallOpts)
}

// SnapshotterTokenClaimCooldown is a free data retrieval call binding the contract method 0x950595f2.
//
// Solidity: function snapshotterTokenClaimCooldown() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) SnapshotterTokenClaimCooldown() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.SnapshotterTokenClaimCooldown(&_SnapshotterStateContract.CallOpts)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) SupportsInterface(opts *bind.CallOpts, interfaceId [4]byte) (bool, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "supportsInterface", interfaceId)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _SnapshotterStateContract.Contract.SupportsInterface(&_SnapshotterStateContract.CallOpts, interfaceId)
}

// SupportsInterface is a free data retrieval call binding the contract method 0x01ffc9a7.
//
// Solidity: function supportsInterface(bytes4 interfaceId) view returns(bool)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) SupportsInterface(interfaceId [4]byte) (bool, error) {
	return _SnapshotterStateContract.Contract.SupportsInterface(&_SnapshotterStateContract.CallOpts, interfaceId)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) TotalSupply(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "totalSupply")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) TotalSupply() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.TotalSupply(&_SnapshotterStateContract.CallOpts)
}

// TotalSupply is a free data retrieval call binding the contract method 0x18160ddd.
//
// Solidity: function totalSupply() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) TotalSupply() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.TotalSupply(&_SnapshotterStateContract.CallOpts)
}

// TotalSupply0 is a free data retrieval call binding the contract method 0xbd85b039.
//
// Solidity: function totalSupply(uint256 id) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) TotalSupply0(opts *bind.CallOpts, id *big.Int) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "totalSupply0", id)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// TotalSupply0 is a free data retrieval call binding the contract method 0xbd85b039.
//
// Solidity: function totalSupply(uint256 id) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) TotalSupply0(id *big.Int) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.TotalSupply0(&_SnapshotterStateContract.CallOpts, id)
}

// TotalSupply0 is a free data retrieval call binding the contract method 0xbd85b039.
//
// Solidity: function totalSupply(uint256 id) view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) TotalSupply0(id *big.Int) (*big.Int, error) {
	return _SnapshotterStateContract.Contract.TotalSupply0(&_SnapshotterStateContract.CallOpts, id)
}

// Uri is a free data retrieval call binding the contract method 0x0e89341c.
//
// Solidity: function uri(uint256 ) view returns(string)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) Uri(opts *bind.CallOpts, arg0 *big.Int) (string, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "uri", arg0)

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Uri is a free data retrieval call binding the contract method 0x0e89341c.
//
// Solidity: function uri(uint256 ) view returns(string)
func (_SnapshotterStateContract *SnapshotterStateContractSession) Uri(arg0 *big.Int) (string, error) {
	return _SnapshotterStateContract.Contract.Uri(&_SnapshotterStateContract.CallOpts, arg0)
}

// Uri is a free data retrieval call binding the contract method 0x0e89341c.
//
// Solidity: function uri(uint256 ) view returns(string)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) Uri(arg0 *big.Int) (string, error) {
	return _SnapshotterStateContract.Contract.Uri(&_SnapshotterStateContract.CallOpts, arg0)
}

// VestedLegacyNodeTokens is a free data retrieval call binding the contract method 0x98f56a58.
//
// Solidity: function vestedLegacyNodeTokens() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCaller) VestedLegacyNodeTokens(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _SnapshotterStateContract.contract.Call(opts, &out, "vestedLegacyNodeTokens")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// VestedLegacyNodeTokens is a free data retrieval call binding the contract method 0x98f56a58.
//
// Solidity: function vestedLegacyNodeTokens() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractSession) VestedLegacyNodeTokens() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.VestedLegacyNodeTokens(&_SnapshotterStateContract.CallOpts)
}

// VestedLegacyNodeTokens is a free data retrieval call binding the contract method 0x98f56a58.
//
// Solidity: function vestedLegacyNodeTokens() view returns(uint256)
func (_SnapshotterStateContract *SnapshotterStateContractCallerSession) VestedLegacyNodeTokens() (*big.Int, error) {
	return _SnapshotterStateContract.Contract.VestedLegacyNodeTokens(&_SnapshotterStateContract.CallOpts)
}

// AcceptOwnership is a paid mutator transaction binding the contract method 0x79ba5097.
//
// Solidity: function acceptOwnership() returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) AcceptOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "acceptOwnership")
}

// AcceptOwnership is a paid mutator transaction binding the contract method 0x79ba5097.
//
// Solidity: function acceptOwnership() returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) AcceptOwnership() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AcceptOwnership(&_SnapshotterStateContract.TransactOpts)
}

// AcceptOwnership is a paid mutator transaction binding the contract method 0x79ba5097.
//
// Solidity: function acceptOwnership() returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) AcceptOwnership() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AcceptOwnership(&_SnapshotterStateContract.TransactOpts)
}

// AdminMintLegacyNodes is a paid mutator transaction binding the contract method 0x0e4376bb.
//
// Solidity: function adminMintLegacyNodes(address _to, uint256 _amount, bool _isKyced) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) AdminMintLegacyNodes(opts *bind.TransactOpts, _to common.Address, _amount *big.Int, _isKyced bool) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "adminMintLegacyNodes", _to, _amount, _isKyced)
}

// AdminMintLegacyNodes is a paid mutator transaction binding the contract method 0x0e4376bb.
//
// Solidity: function adminMintLegacyNodes(address _to, uint256 _amount, bool _isKyced) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) AdminMintLegacyNodes(_to common.Address, _amount *big.Int, _isKyced bool) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AdminMintLegacyNodes(&_SnapshotterStateContract.TransactOpts, _to, _amount, _isKyced)
}

// AdminMintLegacyNodes is a paid mutator transaction binding the contract method 0x0e4376bb.
//
// Solidity: function adminMintLegacyNodes(address _to, uint256 _amount, bool _isKyced) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) AdminMintLegacyNodes(_to common.Address, _amount *big.Int, _isKyced bool) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AdminMintLegacyNodes(&_SnapshotterStateContract.TransactOpts, _to, _amount, _isKyced)
}

// AssignSnapshotterToNode is a paid mutator transaction binding the contract method 0xc0b7da01.
//
// Solidity: function assignSnapshotterToNode(uint256 nodeId, address snapshotterAddress) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) AssignSnapshotterToNode(opts *bind.TransactOpts, nodeId *big.Int, snapshotterAddress common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "assignSnapshotterToNode", nodeId, snapshotterAddress)
}

// AssignSnapshotterToNode is a paid mutator transaction binding the contract method 0xc0b7da01.
//
// Solidity: function assignSnapshotterToNode(uint256 nodeId, address snapshotterAddress) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) AssignSnapshotterToNode(nodeId *big.Int, snapshotterAddress common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AssignSnapshotterToNode(&_SnapshotterStateContract.TransactOpts, nodeId, snapshotterAddress)
}

// AssignSnapshotterToNode is a paid mutator transaction binding the contract method 0xc0b7da01.
//
// Solidity: function assignSnapshotterToNode(uint256 nodeId, address snapshotterAddress) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) AssignSnapshotterToNode(nodeId *big.Int, snapshotterAddress common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AssignSnapshotterToNode(&_SnapshotterStateContract.TransactOpts, nodeId, snapshotterAddress)
}

// AssignSnapshotterToNodeAdmin is a paid mutator transaction binding the contract method 0x0f7e07f7.
//
// Solidity: function assignSnapshotterToNodeAdmin(uint256 nodeId, address snapshotterAddress) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) AssignSnapshotterToNodeAdmin(opts *bind.TransactOpts, nodeId *big.Int, snapshotterAddress common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "assignSnapshotterToNodeAdmin", nodeId, snapshotterAddress)
}

// AssignSnapshotterToNodeAdmin is a paid mutator transaction binding the contract method 0x0f7e07f7.
//
// Solidity: function assignSnapshotterToNodeAdmin(uint256 nodeId, address snapshotterAddress) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) AssignSnapshotterToNodeAdmin(nodeId *big.Int, snapshotterAddress common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AssignSnapshotterToNodeAdmin(&_SnapshotterStateContract.TransactOpts, nodeId, snapshotterAddress)
}

// AssignSnapshotterToNodeAdmin is a paid mutator transaction binding the contract method 0x0f7e07f7.
//
// Solidity: function assignSnapshotterToNodeAdmin(uint256 nodeId, address snapshotterAddress) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) AssignSnapshotterToNodeAdmin(nodeId *big.Int, snapshotterAddress common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AssignSnapshotterToNodeAdmin(&_SnapshotterStateContract.TransactOpts, nodeId, snapshotterAddress)
}

// AssignSnapshotterToNodeBulk is a paid mutator transaction binding the contract method 0xadfb178c.
//
// Solidity: function assignSnapshotterToNodeBulk(uint256[] nodeIds, address[] snapshotterAddresses) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) AssignSnapshotterToNodeBulk(opts *bind.TransactOpts, nodeIds []*big.Int, snapshotterAddresses []common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "assignSnapshotterToNodeBulk", nodeIds, snapshotterAddresses)
}

// AssignSnapshotterToNodeBulk is a paid mutator transaction binding the contract method 0xadfb178c.
//
// Solidity: function assignSnapshotterToNodeBulk(uint256[] nodeIds, address[] snapshotterAddresses) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) AssignSnapshotterToNodeBulk(nodeIds []*big.Int, snapshotterAddresses []common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AssignSnapshotterToNodeBulk(&_SnapshotterStateContract.TransactOpts, nodeIds, snapshotterAddresses)
}

// AssignSnapshotterToNodeBulk is a paid mutator transaction binding the contract method 0xadfb178c.
//
// Solidity: function assignSnapshotterToNodeBulk(uint256[] nodeIds, address[] snapshotterAddresses) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) AssignSnapshotterToNodeBulk(nodeIds []*big.Int, snapshotterAddresses []common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AssignSnapshotterToNodeBulk(&_SnapshotterStateContract.TransactOpts, nodeIds, snapshotterAddresses)
}

// AssignSnapshotterToNodeBulkAdmin is a paid mutator transaction binding the contract method 0x99844a34.
//
// Solidity: function assignSnapshotterToNodeBulkAdmin(uint256[] nodeIds, address[] snapshotterAddresses) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) AssignSnapshotterToNodeBulkAdmin(opts *bind.TransactOpts, nodeIds []*big.Int, snapshotterAddresses []common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "assignSnapshotterToNodeBulkAdmin", nodeIds, snapshotterAddresses)
}

// AssignSnapshotterToNodeBulkAdmin is a paid mutator transaction binding the contract method 0x99844a34.
//
// Solidity: function assignSnapshotterToNodeBulkAdmin(uint256[] nodeIds, address[] snapshotterAddresses) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) AssignSnapshotterToNodeBulkAdmin(nodeIds []*big.Int, snapshotterAddresses []common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AssignSnapshotterToNodeBulkAdmin(&_SnapshotterStateContract.TransactOpts, nodeIds, snapshotterAddresses)
}

// AssignSnapshotterToNodeBulkAdmin is a paid mutator transaction binding the contract method 0x99844a34.
//
// Solidity: function assignSnapshotterToNodeBulkAdmin(uint256[] nodeIds, address[] snapshotterAddresses) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) AssignSnapshotterToNodeBulkAdmin(nodeIds []*big.Int, snapshotterAddresses []common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.AssignSnapshotterToNodeBulkAdmin(&_SnapshotterStateContract.TransactOpts, nodeIds, snapshotterAddresses)
}

// BurnNode is a paid mutator transaction binding the contract method 0x5587850d.
//
// Solidity: function burnNode(uint256 _nodeId) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) BurnNode(opts *bind.TransactOpts, _nodeId *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "burnNode", _nodeId)
}

// BurnNode is a paid mutator transaction binding the contract method 0x5587850d.
//
// Solidity: function burnNode(uint256 _nodeId) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) BurnNode(_nodeId *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.BurnNode(&_SnapshotterStateContract.TransactOpts, _nodeId)
}

// BurnNode is a paid mutator transaction binding the contract method 0x5587850d.
//
// Solidity: function burnNode(uint256 _nodeId) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) BurnNode(_nodeId *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.BurnNode(&_SnapshotterStateContract.TransactOpts, _nodeId)
}

// ClaimNodeTokens is a paid mutator transaction binding the contract method 0x7e5af75b.
//
// Solidity: function claimNodeTokens(uint256 _nodeId) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) ClaimNodeTokens(opts *bind.TransactOpts, _nodeId *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "claimNodeTokens", _nodeId)
}

// ClaimNodeTokens is a paid mutator transaction binding the contract method 0x7e5af75b.
//
// Solidity: function claimNodeTokens(uint256 _nodeId) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) ClaimNodeTokens(_nodeId *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.ClaimNodeTokens(&_SnapshotterStateContract.TransactOpts, _nodeId)
}

// ClaimNodeTokens is a paid mutator transaction binding the contract method 0x7e5af75b.
//
// Solidity: function claimNodeTokens(uint256 _nodeId) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) ClaimNodeTokens(_nodeId *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.ClaimNodeTokens(&_SnapshotterStateContract.TransactOpts, _nodeId)
}

// CompleteKyc is a paid mutator transaction binding the contract method 0x7b4ae3d7.
//
// Solidity: function completeKyc(uint256 _nodeId) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) CompleteKyc(opts *bind.TransactOpts, _nodeId *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "completeKyc", _nodeId)
}

// CompleteKyc is a paid mutator transaction binding the contract method 0x7b4ae3d7.
//
// Solidity: function completeKyc(uint256 _nodeId) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) CompleteKyc(_nodeId *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.CompleteKyc(&_SnapshotterStateContract.TransactOpts, _nodeId)
}

// CompleteKyc is a paid mutator transaction binding the contract method 0x7b4ae3d7.
//
// Solidity: function completeKyc(uint256 _nodeId) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) CompleteKyc(_nodeId *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.CompleteKyc(&_SnapshotterStateContract.TransactOpts, _nodeId)
}

// ConfigureLegacyNodes is a paid mutator transaction binding the contract method 0x7f8cc9e3.
//
// Solidity: function configureLegacyNodes(uint256 _legacyNodeCount, uint256 _legacyNodeInitialClaimPercentage, uint256 _legacyNodeCliff, uint256 _legacyNodeValue, uint256 _legacyNodeVestingDays, uint256 _legacyNodeVestingStart, uint256 _legacyTokensSentOnL1, uint256 _legacyNodeNonKycedCooldown) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) ConfigureLegacyNodes(opts *bind.TransactOpts, _legacyNodeCount *big.Int, _legacyNodeInitialClaimPercentage *big.Int, _legacyNodeCliff *big.Int, _legacyNodeValue *big.Int, _legacyNodeVestingDays *big.Int, _legacyNodeVestingStart *big.Int, _legacyTokensSentOnL1 *big.Int, _legacyNodeNonKycedCooldown *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "configureLegacyNodes", _legacyNodeCount, _legacyNodeInitialClaimPercentage, _legacyNodeCliff, _legacyNodeValue, _legacyNodeVestingDays, _legacyNodeVestingStart, _legacyTokensSentOnL1, _legacyNodeNonKycedCooldown)
}

// ConfigureLegacyNodes is a paid mutator transaction binding the contract method 0x7f8cc9e3.
//
// Solidity: function configureLegacyNodes(uint256 _legacyNodeCount, uint256 _legacyNodeInitialClaimPercentage, uint256 _legacyNodeCliff, uint256 _legacyNodeValue, uint256 _legacyNodeVestingDays, uint256 _legacyNodeVestingStart, uint256 _legacyTokensSentOnL1, uint256 _legacyNodeNonKycedCooldown) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) ConfigureLegacyNodes(_legacyNodeCount *big.Int, _legacyNodeInitialClaimPercentage *big.Int, _legacyNodeCliff *big.Int, _legacyNodeValue *big.Int, _legacyNodeVestingDays *big.Int, _legacyNodeVestingStart *big.Int, _legacyTokensSentOnL1 *big.Int, _legacyNodeNonKycedCooldown *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.ConfigureLegacyNodes(&_SnapshotterStateContract.TransactOpts, _legacyNodeCount, _legacyNodeInitialClaimPercentage, _legacyNodeCliff, _legacyNodeValue, _legacyNodeVestingDays, _legacyNodeVestingStart, _legacyTokensSentOnL1, _legacyNodeNonKycedCooldown)
}

// ConfigureLegacyNodes is a paid mutator transaction binding the contract method 0x7f8cc9e3.
//
// Solidity: function configureLegacyNodes(uint256 _legacyNodeCount, uint256 _legacyNodeInitialClaimPercentage, uint256 _legacyNodeCliff, uint256 _legacyNodeValue, uint256 _legacyNodeVestingDays, uint256 _legacyNodeVestingStart, uint256 _legacyTokensSentOnL1, uint256 _legacyNodeNonKycedCooldown) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) ConfigureLegacyNodes(_legacyNodeCount *big.Int, _legacyNodeInitialClaimPercentage *big.Int, _legacyNodeCliff *big.Int, _legacyNodeValue *big.Int, _legacyNodeVestingDays *big.Int, _legacyNodeVestingStart *big.Int, _legacyTokensSentOnL1 *big.Int, _legacyNodeNonKycedCooldown *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.ConfigureLegacyNodes(&_SnapshotterStateContract.TransactOpts, _legacyNodeCount, _legacyNodeInitialClaimPercentage, _legacyNodeCliff, _legacyNodeValue, _legacyNodeVestingDays, _legacyNodeVestingStart, _legacyTokensSentOnL1, _legacyNodeNonKycedCooldown)
}

// EmergencyWithdraw is a paid mutator transaction binding the contract method 0xdb2e21bc.
//
// Solidity: function emergencyWithdraw() returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) EmergencyWithdraw(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "emergencyWithdraw")
}

// EmergencyWithdraw is a paid mutator transaction binding the contract method 0xdb2e21bc.
//
// Solidity: function emergencyWithdraw() returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) EmergencyWithdraw() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.EmergencyWithdraw(&_SnapshotterStateContract.TransactOpts)
}

// EmergencyWithdraw is a paid mutator transaction binding the contract method 0xdb2e21bc.
//
// Solidity: function emergencyWithdraw() returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) EmergencyWithdraw() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.EmergencyWithdraw(&_SnapshotterStateContract.TransactOpts)
}

// Initialize is a paid mutator transaction binding the contract method 0x81d60956.
//
// Solidity: function initialize(address initialOwner, uint256 initialNodePrice, string initialName) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) Initialize(opts *bind.TransactOpts, initialOwner common.Address, initialNodePrice *big.Int, initialName string) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "initialize", initialOwner, initialNodePrice, initialName)
}

// Initialize is a paid mutator transaction binding the contract method 0x81d60956.
//
// Solidity: function initialize(address initialOwner, uint256 initialNodePrice, string initialName) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) Initialize(initialOwner common.Address, initialNodePrice *big.Int, initialName string) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.Initialize(&_SnapshotterStateContract.TransactOpts, initialOwner, initialNodePrice, initialName)
}

// Initialize is a paid mutator transaction binding the contract method 0x81d60956.
//
// Solidity: function initialize(address initialOwner, uint256 initialNodePrice, string initialName) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) Initialize(initialOwner common.Address, initialNodePrice *big.Int, initialName string) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.Initialize(&_SnapshotterStateContract.TransactOpts, initialOwner, initialNodePrice, initialName)
}

// MintNode is a paid mutator transaction binding the contract method 0x0debc023.
//
// Solidity: function mintNode(uint256 amount) payable returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) MintNode(opts *bind.TransactOpts, amount *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "mintNode", amount)
}

// MintNode is a paid mutator transaction binding the contract method 0x0debc023.
//
// Solidity: function mintNode(uint256 amount) payable returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) MintNode(amount *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.MintNode(&_SnapshotterStateContract.TransactOpts, amount)
}

// MintNode is a paid mutator transaction binding the contract method 0x0debc023.
//
// Solidity: function mintNode(uint256 amount) payable returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) MintNode(amount *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.MintNode(&_SnapshotterStateContract.TransactOpts, amount)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) Pause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "pause")
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) Pause() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.Pause(&_SnapshotterStateContract.TransactOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) Pause() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.Pause(&_SnapshotterStateContract.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) RenounceOwnership() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.RenounceOwnership(&_SnapshotterStateContract.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.RenounceOwnership(&_SnapshotterStateContract.TransactOpts)
}

// SafeBatchTransferFrom is a paid mutator transaction binding the contract method 0x2eb2c2d6.
//
// Solidity: function safeBatchTransferFrom(address , address , uint256[] , uint256[] , bytes ) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) SafeBatchTransferFrom(opts *bind.TransactOpts, arg0 common.Address, arg1 common.Address, arg2 []*big.Int, arg3 []*big.Int, arg4 []byte) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "safeBatchTransferFrom", arg0, arg1, arg2, arg3, arg4)
}

// SafeBatchTransferFrom is a paid mutator transaction binding the contract method 0x2eb2c2d6.
//
// Solidity: function safeBatchTransferFrom(address , address , uint256[] , uint256[] , bytes ) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) SafeBatchTransferFrom(arg0 common.Address, arg1 common.Address, arg2 []*big.Int, arg3 []*big.Int, arg4 []byte) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SafeBatchTransferFrom(&_SnapshotterStateContract.TransactOpts, arg0, arg1, arg2, arg3, arg4)
}

// SafeBatchTransferFrom is a paid mutator transaction binding the contract method 0x2eb2c2d6.
//
// Solidity: function safeBatchTransferFrom(address , address , uint256[] , uint256[] , bytes ) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) SafeBatchTransferFrom(arg0 common.Address, arg1 common.Address, arg2 []*big.Int, arg3 []*big.Int, arg4 []byte) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SafeBatchTransferFrom(&_SnapshotterStateContract.TransactOpts, arg0, arg1, arg2, arg3, arg4)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0xf242432a.
//
// Solidity: function safeTransferFrom(address , address , uint256 , uint256 , bytes ) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) SafeTransferFrom(opts *bind.TransactOpts, arg0 common.Address, arg1 common.Address, arg2 *big.Int, arg3 *big.Int, arg4 []byte) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "safeTransferFrom", arg0, arg1, arg2, arg3, arg4)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0xf242432a.
//
// Solidity: function safeTransferFrom(address , address , uint256 , uint256 , bytes ) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) SafeTransferFrom(arg0 common.Address, arg1 common.Address, arg2 *big.Int, arg3 *big.Int, arg4 []byte) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SafeTransferFrom(&_SnapshotterStateContract.TransactOpts, arg0, arg1, arg2, arg3, arg4)
}

// SafeTransferFrom is a paid mutator transaction binding the contract method 0xf242432a.
//
// Solidity: function safeTransferFrom(address , address , uint256 , uint256 , bytes ) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) SafeTransferFrom(arg0 common.Address, arg1 common.Address, arg2 *big.Int, arg3 *big.Int, arg4 []byte) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SafeTransferFrom(&_SnapshotterStateContract.TransactOpts, arg0, arg1, arg2, arg3, arg4)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) SetApprovalForAll(opts *bind.TransactOpts, operator common.Address, approved bool) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "setApprovalForAll", operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetApprovalForAll(&_SnapshotterStateContract.TransactOpts, operator, approved)
}

// SetApprovalForAll is a paid mutator transaction binding the contract method 0xa22cb465.
//
// Solidity: function setApprovalForAll(address operator, bool approved) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) SetApprovalForAll(operator common.Address, approved bool) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetApprovalForAll(&_SnapshotterStateContract.TransactOpts, operator, approved)
}

// SetMintStartTime is a paid mutator transaction binding the contract method 0xd5b3621b.
//
// Solidity: function setMintStartTime(uint256 _mintStartTime) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) SetMintStartTime(opts *bind.TransactOpts, _mintStartTime *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "setMintStartTime", _mintStartTime)
}

// SetMintStartTime is a paid mutator transaction binding the contract method 0xd5b3621b.
//
// Solidity: function setMintStartTime(uint256 _mintStartTime) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) SetMintStartTime(_mintStartTime *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetMintStartTime(&_SnapshotterStateContract.TransactOpts, _mintStartTime)
}

// SetMintStartTime is a paid mutator transaction binding the contract method 0xd5b3621b.
//
// Solidity: function setMintStartTime(uint256 _mintStartTime) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) SetMintStartTime(_mintStartTime *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetMintStartTime(&_SnapshotterStateContract.TransactOpts, _mintStartTime)
}

// SetName is a paid mutator transaction binding the contract method 0xc47f0027.
//
// Solidity: function setName(string _name) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) SetName(opts *bind.TransactOpts, _name string) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "setName", _name)
}

// SetName is a paid mutator transaction binding the contract method 0xc47f0027.
//
// Solidity: function setName(string _name) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) SetName(_name string) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetName(&_SnapshotterStateContract.TransactOpts, _name)
}

// SetName is a paid mutator transaction binding the contract method 0xc47f0027.
//
// Solidity: function setName(string _name) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) SetName(_name string) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetName(&_SnapshotterStateContract.TransactOpts, _name)
}

// SetSnapshotterAddressChangeCooldown is a paid mutator transaction binding the contract method 0x9eebcf76.
//
// Solidity: function setSnapshotterAddressChangeCooldown(uint256 _snapshotterAddressChangeCooldown) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) SetSnapshotterAddressChangeCooldown(opts *bind.TransactOpts, _snapshotterAddressChangeCooldown *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "setSnapshotterAddressChangeCooldown", _snapshotterAddressChangeCooldown)
}

// SetSnapshotterAddressChangeCooldown is a paid mutator transaction binding the contract method 0x9eebcf76.
//
// Solidity: function setSnapshotterAddressChangeCooldown(uint256 _snapshotterAddressChangeCooldown) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) SetSnapshotterAddressChangeCooldown(_snapshotterAddressChangeCooldown *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetSnapshotterAddressChangeCooldown(&_SnapshotterStateContract.TransactOpts, _snapshotterAddressChangeCooldown)
}

// SetSnapshotterAddressChangeCooldown is a paid mutator transaction binding the contract method 0x9eebcf76.
//
// Solidity: function setSnapshotterAddressChangeCooldown(uint256 _snapshotterAddressChangeCooldown) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) SetSnapshotterAddressChangeCooldown(_snapshotterAddressChangeCooldown *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetSnapshotterAddressChangeCooldown(&_SnapshotterStateContract.TransactOpts, _snapshotterAddressChangeCooldown)
}

// SetSnapshotterTokenClaimCooldown is a paid mutator transaction binding the contract method 0x97b66148.
//
// Solidity: function setSnapshotterTokenClaimCooldown(uint256 _snapshotterTokenClaimCooldown) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) SetSnapshotterTokenClaimCooldown(opts *bind.TransactOpts, _snapshotterTokenClaimCooldown *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "setSnapshotterTokenClaimCooldown", _snapshotterTokenClaimCooldown)
}

// SetSnapshotterTokenClaimCooldown is a paid mutator transaction binding the contract method 0x97b66148.
//
// Solidity: function setSnapshotterTokenClaimCooldown(uint256 _snapshotterTokenClaimCooldown) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) SetSnapshotterTokenClaimCooldown(_snapshotterTokenClaimCooldown *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetSnapshotterTokenClaimCooldown(&_SnapshotterStateContract.TransactOpts, _snapshotterTokenClaimCooldown)
}

// SetSnapshotterTokenClaimCooldown is a paid mutator transaction binding the contract method 0x97b66148.
//
// Solidity: function setSnapshotterTokenClaimCooldown(uint256 _snapshotterTokenClaimCooldown) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) SetSnapshotterTokenClaimCooldown(_snapshotterTokenClaimCooldown *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetSnapshotterTokenClaimCooldown(&_SnapshotterStateContract.TransactOpts, _snapshotterTokenClaimCooldown)
}

// SetURI is a paid mutator transaction binding the contract method 0x02fe5305.
//
// Solidity: function setURI(string newuri) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) SetURI(opts *bind.TransactOpts, newuri string) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "setURI", newuri)
}

// SetURI is a paid mutator transaction binding the contract method 0x02fe5305.
//
// Solidity: function setURI(string newuri) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) SetURI(newuri string) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetURI(&_SnapshotterStateContract.TransactOpts, newuri)
}

// SetURI is a paid mutator transaction binding the contract method 0x02fe5305.
//
// Solidity: function setURI(string newuri) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) SetURI(newuri string) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.SetURI(&_SnapshotterStateContract.TransactOpts, newuri)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.TransferOwnership(&_SnapshotterStateContract.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.TransferOwnership(&_SnapshotterStateContract.TransactOpts, newOwner)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) Unpause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "unpause")
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) Unpause() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.Unpause(&_SnapshotterStateContract.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) Unpause() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.Unpause(&_SnapshotterStateContract.TransactOpts)
}

// UpdateAdmins is a paid mutator transaction binding the contract method 0x76fed816.
//
// Solidity: function updateAdmins(address[] _admins, bool[] _status) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) UpdateAdmins(opts *bind.TransactOpts, _admins []common.Address, _status []bool) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "updateAdmins", _admins, _status)
}

// UpdateAdmins is a paid mutator transaction binding the contract method 0x76fed816.
//
// Solidity: function updateAdmins(address[] _admins, bool[] _status) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) UpdateAdmins(_admins []common.Address, _status []bool) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.UpdateAdmins(&_SnapshotterStateContract.TransactOpts, _admins, _status)
}

// UpdateAdmins is a paid mutator transaction binding the contract method 0x76fed816.
//
// Solidity: function updateAdmins(address[] _admins, bool[] _status) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) UpdateAdmins(_admins []common.Address, _status []bool) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.UpdateAdmins(&_SnapshotterStateContract.TransactOpts, _admins, _status)
}

// UpdateMaxSupply is a paid mutator transaction binding the contract method 0xf103b433.
//
// Solidity: function updateMaxSupply(uint256 _maxSupply) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) UpdateMaxSupply(opts *bind.TransactOpts, _maxSupply *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "updateMaxSupply", _maxSupply)
}

// UpdateMaxSupply is a paid mutator transaction binding the contract method 0xf103b433.
//
// Solidity: function updateMaxSupply(uint256 _maxSupply) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) UpdateMaxSupply(_maxSupply *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.UpdateMaxSupply(&_SnapshotterStateContract.TransactOpts, _maxSupply)
}

// UpdateMaxSupply is a paid mutator transaction binding the contract method 0xf103b433.
//
// Solidity: function updateMaxSupply(uint256 _maxSupply) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) UpdateMaxSupply(_maxSupply *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.UpdateMaxSupply(&_SnapshotterStateContract.TransactOpts, _maxSupply)
}

// UpdateNodePrice is a paid mutator transaction binding the contract method 0xda5f051c.
//
// Solidity: function updateNodePrice(uint256 _nodePrice) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) UpdateNodePrice(opts *bind.TransactOpts, _nodePrice *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "updateNodePrice", _nodePrice)
}

// UpdateNodePrice is a paid mutator transaction binding the contract method 0xda5f051c.
//
// Solidity: function updateNodePrice(uint256 _nodePrice) returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) UpdateNodePrice(_nodePrice *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.UpdateNodePrice(&_SnapshotterStateContract.TransactOpts, _nodePrice)
}

// UpdateNodePrice is a paid mutator transaction binding the contract method 0xda5f051c.
//
// Solidity: function updateNodePrice(uint256 _nodePrice) returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) UpdateNodePrice(_nodePrice *big.Int) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.UpdateNodePrice(&_SnapshotterStateContract.TransactOpts, _nodePrice)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) UpgradeToAndCall(opts *bind.TransactOpts, newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.Transact(opts, "upgradeToAndCall", newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.UpgradeToAndCall(&_SnapshotterStateContract.TransactOpts, newImplementation, data)
}

// UpgradeToAndCall is a paid mutator transaction binding the contract method 0x4f1ef286.
//
// Solidity: function upgradeToAndCall(address newImplementation, bytes data) payable returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) UpgradeToAndCall(newImplementation common.Address, data []byte) (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.UpgradeToAndCall(&_SnapshotterStateContract.TransactOpts, newImplementation, data)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactor) Receive(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SnapshotterStateContract.contract.RawTransact(opts, nil) // calldata is disallowed for receive function
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_SnapshotterStateContract *SnapshotterStateContractSession) Receive() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.Receive(&_SnapshotterStateContract.TransactOpts)
}

// Receive is a paid mutator transaction binding the contract receive function.
//
// Solidity: receive() payable returns()
func (_SnapshotterStateContract *SnapshotterStateContractTransactorSession) Receive() (*types.Transaction, error) {
	return _SnapshotterStateContract.Contract.Receive(&_SnapshotterStateContract.TransactOpts)
}

// SnapshotterStateContractAdminsUpdatedIterator is returned from FilterAdminsUpdated and is used to iterate over the raw logs and unpacked data for AdminsUpdated events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractAdminsUpdatedIterator struct {
	Event *SnapshotterStateContractAdminsUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractAdminsUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractAdminsUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractAdminsUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractAdminsUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractAdminsUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractAdminsUpdated represents a AdminsUpdated event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractAdminsUpdated struct {
	AdminAddress common.Address
	Allowed      bool
	Raw          types.Log // Blockchain specific contextual infos
}

// FilterAdminsUpdated is a free log retrieval operation binding the contract event 0x915a9250b000555737056ef1e5c2447ba962b934fb8e16b5e3a24db239c2dcf1.
//
// Solidity: event AdminsUpdated(address adminAddress, bool allowed)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterAdminsUpdated(opts *bind.FilterOpts) (*SnapshotterStateContractAdminsUpdatedIterator, error) {

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "AdminsUpdated")
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractAdminsUpdatedIterator{contract: _SnapshotterStateContract.contract, event: "AdminsUpdated", logs: logs, sub: sub}, nil
}

// WatchAdminsUpdated is a free log subscription operation binding the contract event 0x915a9250b000555737056ef1e5c2447ba962b934fb8e16b5e3a24db239c2dcf1.
//
// Solidity: event AdminsUpdated(address adminAddress, bool allowed)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchAdminsUpdated(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractAdminsUpdated) (event.Subscription, error) {

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "AdminsUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractAdminsUpdated)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "AdminsUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAdminsUpdated is a log parse operation binding the contract event 0x915a9250b000555737056ef1e5c2447ba962b934fb8e16b5e3a24db239c2dcf1.
//
// Solidity: event AdminsUpdated(address adminAddress, bool allowed)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseAdminsUpdated(log types.Log) (*SnapshotterStateContractAdminsUpdated, error) {
	event := new(SnapshotterStateContractAdminsUpdated)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "AdminsUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractApprovalForAllIterator is returned from FilterApprovalForAll and is used to iterate over the raw logs and unpacked data for ApprovalForAll events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractApprovalForAllIterator struct {
	Event *SnapshotterStateContractApprovalForAll // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractApprovalForAllIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractApprovalForAll)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractApprovalForAll)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractApprovalForAllIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractApprovalForAllIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractApprovalForAll represents a ApprovalForAll event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractApprovalForAll struct {
	Account  common.Address
	Operator common.Address
	Approved bool
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterApprovalForAll is a free log retrieval operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed account, address indexed operator, bool approved)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterApprovalForAll(opts *bind.FilterOpts, account []common.Address, operator []common.Address) (*SnapshotterStateContractApprovalForAllIterator, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "ApprovalForAll", accountRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractApprovalForAllIterator{contract: _SnapshotterStateContract.contract, event: "ApprovalForAll", logs: logs, sub: sub}, nil
}

// WatchApprovalForAll is a free log subscription operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed account, address indexed operator, bool approved)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchApprovalForAll(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractApprovalForAll, account []common.Address, operator []common.Address) (event.Subscription, error) {

	var accountRule []interface{}
	for _, accountItem := range account {
		accountRule = append(accountRule, accountItem)
	}
	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "ApprovalForAll", accountRule, operatorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractApprovalForAll)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseApprovalForAll is a log parse operation binding the contract event 0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31.
//
// Solidity: event ApprovalForAll(address indexed account, address indexed operator, bool approved)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseApprovalForAll(log types.Log) (*SnapshotterStateContractApprovalForAll, error) {
	event := new(SnapshotterStateContractApprovalForAll)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "ApprovalForAll", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractConfigurationUpdatedIterator is returned from FilterConfigurationUpdated and is used to iterate over the raw logs and unpacked data for ConfigurationUpdated events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractConfigurationUpdatedIterator struct {
	Event *SnapshotterStateContractConfigurationUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractConfigurationUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractConfigurationUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractConfigurationUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractConfigurationUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractConfigurationUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractConfigurationUpdated represents a ConfigurationUpdated event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractConfigurationUpdated struct {
	ParamName string
	NewValue  *big.Int
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterConfigurationUpdated is a free log retrieval operation binding the contract event 0x319a4565da908d26083a1132a8a05b7630d680ea0b63943a59e84146cb97464e.
//
// Solidity: event ConfigurationUpdated(string paramName, uint256 newValue)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterConfigurationUpdated(opts *bind.FilterOpts) (*SnapshotterStateContractConfigurationUpdatedIterator, error) {

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "ConfigurationUpdated")
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractConfigurationUpdatedIterator{contract: _SnapshotterStateContract.contract, event: "ConfigurationUpdated", logs: logs, sub: sub}, nil
}

// WatchConfigurationUpdated is a free log subscription operation binding the contract event 0x319a4565da908d26083a1132a8a05b7630d680ea0b63943a59e84146cb97464e.
//
// Solidity: event ConfigurationUpdated(string paramName, uint256 newValue)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchConfigurationUpdated(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractConfigurationUpdated) (event.Subscription, error) {

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "ConfigurationUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractConfigurationUpdated)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "ConfigurationUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseConfigurationUpdated is a log parse operation binding the contract event 0x319a4565da908d26083a1132a8a05b7630d680ea0b63943a59e84146cb97464e.
//
// Solidity: event ConfigurationUpdated(string paramName, uint256 newValue)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseConfigurationUpdated(log types.Log) (*SnapshotterStateContractConfigurationUpdated, error) {
	event := new(SnapshotterStateContractConfigurationUpdated)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "ConfigurationUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractDepositIterator is returned from FilterDeposit and is used to iterate over the raw logs and unpacked data for Deposit events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractDepositIterator struct {
	Event *SnapshotterStateContractDeposit // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractDepositIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractDeposit)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractDeposit)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractDepositIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractDepositIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractDeposit represents a Deposit event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractDeposit struct {
	From   common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterDeposit is a free log retrieval operation binding the contract event 0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c.
//
// Solidity: event Deposit(address indexed from, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterDeposit(opts *bind.FilterOpts, from []common.Address) (*SnapshotterStateContractDepositIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "Deposit", fromRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractDepositIterator{contract: _SnapshotterStateContract.contract, event: "Deposit", logs: logs, sub: sub}, nil
}

// WatchDeposit is a free log subscription operation binding the contract event 0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c.
//
// Solidity: event Deposit(address indexed from, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchDeposit(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractDeposit, from []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "Deposit", fromRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractDeposit)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "Deposit", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseDeposit is a log parse operation binding the contract event 0xe1fffcc4923d04b559f4d29a8bfc6cda04eb5b0d3c460751c2402c5c5cc9109c.
//
// Solidity: event Deposit(address indexed from, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseDeposit(log types.Log) (*SnapshotterStateContractDeposit, error) {
	event := new(SnapshotterStateContractDeposit)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "Deposit", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractEmergencyWithdrawIterator is returned from FilterEmergencyWithdraw and is used to iterate over the raw logs and unpacked data for EmergencyWithdraw events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractEmergencyWithdrawIterator struct {
	Event *SnapshotterStateContractEmergencyWithdraw // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractEmergencyWithdrawIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractEmergencyWithdraw)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractEmergencyWithdraw)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractEmergencyWithdrawIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractEmergencyWithdrawIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractEmergencyWithdraw represents a EmergencyWithdraw event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractEmergencyWithdraw struct {
	Owner  common.Address
	Amount *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterEmergencyWithdraw is a free log retrieval operation binding the contract event 0x5fafa99d0643513820be26656b45130b01e1c03062e1266bf36f88cbd3bd9695.
//
// Solidity: event EmergencyWithdraw(address indexed owner, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterEmergencyWithdraw(opts *bind.FilterOpts, owner []common.Address) (*SnapshotterStateContractEmergencyWithdrawIterator, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "EmergencyWithdraw", ownerRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractEmergencyWithdrawIterator{contract: _SnapshotterStateContract.contract, event: "EmergencyWithdraw", logs: logs, sub: sub}, nil
}

// WatchEmergencyWithdraw is a free log subscription operation binding the contract event 0x5fafa99d0643513820be26656b45130b01e1c03062e1266bf36f88cbd3bd9695.
//
// Solidity: event EmergencyWithdraw(address indexed owner, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchEmergencyWithdraw(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractEmergencyWithdraw, owner []common.Address) (event.Subscription, error) {

	var ownerRule []interface{}
	for _, ownerItem := range owner {
		ownerRule = append(ownerRule, ownerItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "EmergencyWithdraw", ownerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractEmergencyWithdraw)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "EmergencyWithdraw", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseEmergencyWithdraw is a log parse operation binding the contract event 0x5fafa99d0643513820be26656b45130b01e1c03062e1266bf36f88cbd3bd9695.
//
// Solidity: event EmergencyWithdraw(address indexed owner, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseEmergencyWithdraw(log types.Log) (*SnapshotterStateContractEmergencyWithdraw, error) {
	event := new(SnapshotterStateContractEmergencyWithdraw)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "EmergencyWithdraw", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractInitializedIterator is returned from FilterInitialized and is used to iterate over the raw logs and unpacked data for Initialized events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractInitializedIterator struct {
	Event *SnapshotterStateContractInitialized // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractInitializedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractInitialized)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractInitialized)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractInitializedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractInitializedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractInitialized represents a Initialized event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractInitialized struct {
	Version uint64
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterInitialized is a free log retrieval operation binding the contract event 0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2.
//
// Solidity: event Initialized(uint64 version)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterInitialized(opts *bind.FilterOpts) (*SnapshotterStateContractInitializedIterator, error) {

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractInitializedIterator{contract: _SnapshotterStateContract.contract, event: "Initialized", logs: logs, sub: sub}, nil
}

// WatchInitialized is a free log subscription operation binding the contract event 0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2.
//
// Solidity: event Initialized(uint64 version)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchInitialized(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractInitialized) (event.Subscription, error) {

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "Initialized")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractInitialized)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "Initialized", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseInitialized is a log parse operation binding the contract event 0xc7f505b2f371ae2175ee4913f4499e1f2633a7b5936321eed1cdaeb6115181d2.
//
// Solidity: event Initialized(uint64 version)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseInitialized(log types.Log) (*SnapshotterStateContractInitialized, error) {
	event := new(SnapshotterStateContractInitialized)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "Initialized", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractLegacyNodeTokensClaimedIterator is returned from FilterLegacyNodeTokensClaimed and is used to iterate over the raw logs and unpacked data for LegacyNodeTokensClaimed events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractLegacyNodeTokensClaimedIterator struct {
	Event *SnapshotterStateContractLegacyNodeTokensClaimed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractLegacyNodeTokensClaimedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractLegacyNodeTokensClaimed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractLegacyNodeTokensClaimed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractLegacyNodeTokensClaimedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractLegacyNodeTokensClaimedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractLegacyNodeTokensClaimed represents a LegacyNodeTokensClaimed event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractLegacyNodeTokensClaimed struct {
	Claimer common.Address
	NodeId  *big.Int
	Amount  *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterLegacyNodeTokensClaimed is a free log retrieval operation binding the contract event 0xbb169a719b6920904a0656cdde21c131ffc48ca120ba404972cc0900e9cf394a.
//
// Solidity: event LegacyNodeTokensClaimed(address indexed claimer, uint256 nodeId, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterLegacyNodeTokensClaimed(opts *bind.FilterOpts, claimer []common.Address) (*SnapshotterStateContractLegacyNodeTokensClaimedIterator, error) {

	var claimerRule []interface{}
	for _, claimerItem := range claimer {
		claimerRule = append(claimerRule, claimerItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "LegacyNodeTokensClaimed", claimerRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractLegacyNodeTokensClaimedIterator{contract: _SnapshotterStateContract.contract, event: "LegacyNodeTokensClaimed", logs: logs, sub: sub}, nil
}

// WatchLegacyNodeTokensClaimed is a free log subscription operation binding the contract event 0xbb169a719b6920904a0656cdde21c131ffc48ca120ba404972cc0900e9cf394a.
//
// Solidity: event LegacyNodeTokensClaimed(address indexed claimer, uint256 nodeId, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchLegacyNodeTokensClaimed(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractLegacyNodeTokensClaimed, claimer []common.Address) (event.Subscription, error) {

	var claimerRule []interface{}
	for _, claimerItem := range claimer {
		claimerRule = append(claimerRule, claimerItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "LegacyNodeTokensClaimed", claimerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractLegacyNodeTokensClaimed)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "LegacyNodeTokensClaimed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseLegacyNodeTokensClaimed is a log parse operation binding the contract event 0xbb169a719b6920904a0656cdde21c131ffc48ca120ba404972cc0900e9cf394a.
//
// Solidity: event LegacyNodeTokensClaimed(address indexed claimer, uint256 nodeId, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseLegacyNodeTokensClaimed(log types.Log) (*SnapshotterStateContractLegacyNodeTokensClaimed, error) {
	event := new(SnapshotterStateContractLegacyNodeTokensClaimed)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "LegacyNodeTokensClaimed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractNameUpdatedIterator is returned from FilterNameUpdated and is used to iterate over the raw logs and unpacked data for NameUpdated events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractNameUpdatedIterator struct {
	Event *SnapshotterStateContractNameUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractNameUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractNameUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractNameUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractNameUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractNameUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractNameUpdated represents a NameUpdated event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractNameUpdated struct {
	NewName string
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterNameUpdated is a free log retrieval operation binding the contract event 0x9f7688a97f1ac51fe03bac18af18d6810f9f11f0db08c59b1938a9ac825ef744.
//
// Solidity: event NameUpdated(string newName)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterNameUpdated(opts *bind.FilterOpts) (*SnapshotterStateContractNameUpdatedIterator, error) {

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "NameUpdated")
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractNameUpdatedIterator{contract: _SnapshotterStateContract.contract, event: "NameUpdated", logs: logs, sub: sub}, nil
}

// WatchNameUpdated is a free log subscription operation binding the contract event 0x9f7688a97f1ac51fe03bac18af18d6810f9f11f0db08c59b1938a9ac825ef744.
//
// Solidity: event NameUpdated(string newName)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchNameUpdated(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractNameUpdated) (event.Subscription, error) {

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "NameUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractNameUpdated)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "NameUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseNameUpdated is a log parse operation binding the contract event 0x9f7688a97f1ac51fe03bac18af18d6810f9f11f0db08c59b1938a9ac825ef744.
//
// Solidity: event NameUpdated(string newName)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseNameUpdated(log types.Log) (*SnapshotterStateContractNameUpdated, error) {
	event := new(SnapshotterStateContractNameUpdated)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "NameUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractNodeBurnedIterator is returned from FilterNodeBurned and is used to iterate over the raw logs and unpacked data for NodeBurned events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractNodeBurnedIterator struct {
	Event *SnapshotterStateContractNodeBurned // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractNodeBurnedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractNodeBurned)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractNodeBurned)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractNodeBurnedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractNodeBurnedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractNodeBurned represents a NodeBurned event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractNodeBurned struct {
	From   common.Address
	NodeId *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterNodeBurned is a free log retrieval operation binding the contract event 0x2e40bf07b4680e3ff698c86415bf54d1dd08f89e2749efa0cc7f6585f5b2ab5c.
//
// Solidity: event NodeBurned(address indexed from, uint256 nodeId)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterNodeBurned(opts *bind.FilterOpts, from []common.Address) (*SnapshotterStateContractNodeBurnedIterator, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "NodeBurned", fromRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractNodeBurnedIterator{contract: _SnapshotterStateContract.contract, event: "NodeBurned", logs: logs, sub: sub}, nil
}

// WatchNodeBurned is a free log subscription operation binding the contract event 0x2e40bf07b4680e3ff698c86415bf54d1dd08f89e2749efa0cc7f6585f5b2ab5c.
//
// Solidity: event NodeBurned(address indexed from, uint256 nodeId)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchNodeBurned(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractNodeBurned, from []common.Address) (event.Subscription, error) {

	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "NodeBurned", fromRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractNodeBurned)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "NodeBurned", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseNodeBurned is a log parse operation binding the contract event 0x2e40bf07b4680e3ff698c86415bf54d1dd08f89e2749efa0cc7f6585f5b2ab5c.
//
// Solidity: event NodeBurned(address indexed from, uint256 nodeId)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseNodeBurned(log types.Log) (*SnapshotterStateContractNodeBurned, error) {
	event := new(SnapshotterStateContractNodeBurned)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "NodeBurned", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractNodeMintedIterator is returned from FilterNodeMinted and is used to iterate over the raw logs and unpacked data for NodeMinted events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractNodeMintedIterator struct {
	Event *SnapshotterStateContractNodeMinted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractNodeMintedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractNodeMinted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractNodeMinted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractNodeMintedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractNodeMintedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractNodeMinted represents a NodeMinted event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractNodeMinted struct {
	To     common.Address
	NodeId *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterNodeMinted is a free log retrieval operation binding the contract event 0xc35b4331d732ba6463b92bdbe977050a404a102668b70973bba297190b8551cd.
//
// Solidity: event NodeMinted(address indexed to, uint256 nodeId)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterNodeMinted(opts *bind.FilterOpts, to []common.Address) (*SnapshotterStateContractNodeMintedIterator, error) {

	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "NodeMinted", toRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractNodeMintedIterator{contract: _SnapshotterStateContract.contract, event: "NodeMinted", logs: logs, sub: sub}, nil
}

// WatchNodeMinted is a free log subscription operation binding the contract event 0xc35b4331d732ba6463b92bdbe977050a404a102668b70973bba297190b8551cd.
//
// Solidity: event NodeMinted(address indexed to, uint256 nodeId)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchNodeMinted(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractNodeMinted, to []common.Address) (event.Subscription, error) {

	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "NodeMinted", toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractNodeMinted)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "NodeMinted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseNodeMinted is a log parse operation binding the contract event 0xc35b4331d732ba6463b92bdbe977050a404a102668b70973bba297190b8551cd.
//
// Solidity: event NodeMinted(address indexed to, uint256 nodeId)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseNodeMinted(log types.Log) (*SnapshotterStateContractNodeMinted, error) {
	event := new(SnapshotterStateContractNodeMinted)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "NodeMinted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractOwnershipTransferStartedIterator is returned from FilterOwnershipTransferStarted and is used to iterate over the raw logs and unpacked data for OwnershipTransferStarted events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractOwnershipTransferStartedIterator struct {
	Event *SnapshotterStateContractOwnershipTransferStarted // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractOwnershipTransferStartedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractOwnershipTransferStarted)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractOwnershipTransferStarted)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractOwnershipTransferStartedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractOwnershipTransferStartedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractOwnershipTransferStarted represents a OwnershipTransferStarted event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractOwnershipTransferStarted struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferStarted is a free log retrieval operation binding the contract event 0x38d16b8cac22d99fc7c124b9cd0de2d3fa1faef420bfe791d8c362d765e22700.
//
// Solidity: event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterOwnershipTransferStarted(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*SnapshotterStateContractOwnershipTransferStartedIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "OwnershipTransferStarted", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractOwnershipTransferStartedIterator{contract: _SnapshotterStateContract.contract, event: "OwnershipTransferStarted", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferStarted is a free log subscription operation binding the contract event 0x38d16b8cac22d99fc7c124b9cd0de2d3fa1faef420bfe791d8c362d765e22700.
//
// Solidity: event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchOwnershipTransferStarted(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractOwnershipTransferStarted, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "OwnershipTransferStarted", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractOwnershipTransferStarted)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "OwnershipTransferStarted", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferStarted is a log parse operation binding the contract event 0x38d16b8cac22d99fc7c124b9cd0de2d3fa1faef420bfe791d8c362d765e22700.
//
// Solidity: event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseOwnershipTransferStarted(log types.Log) (*SnapshotterStateContractOwnershipTransferStarted, error) {
	event := new(SnapshotterStateContractOwnershipTransferStarted)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "OwnershipTransferStarted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractOwnershipTransferredIterator struct {
	Event *SnapshotterStateContractOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractOwnershipTransferred represents a OwnershipTransferred event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*SnapshotterStateContractOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractOwnershipTransferredIterator{contract: _SnapshotterStateContract.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractOwnershipTransferred)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseOwnershipTransferred(log types.Log) (*SnapshotterStateContractOwnershipTransferred, error) {
	event := new(SnapshotterStateContractOwnershipTransferred)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractPausedIterator is returned from FilterPaused and is used to iterate over the raw logs and unpacked data for Paused events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractPausedIterator struct {
	Event *SnapshotterStateContractPaused // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractPausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractPaused)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractPaused)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractPausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractPausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractPaused represents a Paused event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractPaused struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterPaused is a free log retrieval operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterPaused(opts *bind.FilterOpts) (*SnapshotterStateContractPausedIterator, error) {

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "Paused")
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractPausedIterator{contract: _SnapshotterStateContract.contract, event: "Paused", logs: logs, sub: sub}, nil
}

// WatchPaused is a free log subscription operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchPaused(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractPaused) (event.Subscription, error) {

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "Paused")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractPaused)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "Paused", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParsePaused is a log parse operation binding the contract event 0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258.
//
// Solidity: event Paused(address account)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParsePaused(log types.Log) (*SnapshotterStateContractPaused, error) {
	event := new(SnapshotterStateContractPaused)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "Paused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractSnapshotterAddressChangedIterator is returned from FilterSnapshotterAddressChanged and is used to iterate over the raw logs and unpacked data for SnapshotterAddressChanged events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractSnapshotterAddressChangedIterator struct {
	Event *SnapshotterStateContractSnapshotterAddressChanged // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractSnapshotterAddressChangedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractSnapshotterAddressChanged)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractSnapshotterAddressChanged)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractSnapshotterAddressChangedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractSnapshotterAddressChangedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractSnapshotterAddressChanged represents a SnapshotterAddressChanged event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractSnapshotterAddressChanged struct {
	NodeId         *big.Int
	OldSnapshotter common.Address
	NewSnapshotter common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterSnapshotterAddressChanged is a free log retrieval operation binding the contract event 0xf144ab1c632c4ddc8e89ce0dc5d941b4a7d76cc4a8b5f3189deeee0c23071c39.
//
// Solidity: event SnapshotterAddressChanged(uint256 nodeId, address oldSnapshotter, address newSnapshotter)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterSnapshotterAddressChanged(opts *bind.FilterOpts) (*SnapshotterStateContractSnapshotterAddressChangedIterator, error) {

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "SnapshotterAddressChanged")
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractSnapshotterAddressChangedIterator{contract: _SnapshotterStateContract.contract, event: "SnapshotterAddressChanged", logs: logs, sub: sub}, nil
}

// WatchSnapshotterAddressChanged is a free log subscription operation binding the contract event 0xf144ab1c632c4ddc8e89ce0dc5d941b4a7d76cc4a8b5f3189deeee0c23071c39.
//
// Solidity: event SnapshotterAddressChanged(uint256 nodeId, address oldSnapshotter, address newSnapshotter)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchSnapshotterAddressChanged(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractSnapshotterAddressChanged) (event.Subscription, error) {

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "SnapshotterAddressChanged")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractSnapshotterAddressChanged)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "SnapshotterAddressChanged", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSnapshotterAddressChanged is a log parse operation binding the contract event 0xf144ab1c632c4ddc8e89ce0dc5d941b4a7d76cc4a8b5f3189deeee0c23071c39.
//
// Solidity: event SnapshotterAddressChanged(uint256 nodeId, address oldSnapshotter, address newSnapshotter)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseSnapshotterAddressChanged(log types.Log) (*SnapshotterStateContractSnapshotterAddressChanged, error) {
	event := new(SnapshotterStateContractSnapshotterAddressChanged)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "SnapshotterAddressChanged", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractSnapshotterStateUpdatedIterator is returned from FilterSnapshotterStateUpdated and is used to iterate over the raw logs and unpacked data for SnapshotterStateUpdated events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractSnapshotterStateUpdatedIterator struct {
	Event *SnapshotterStateContractSnapshotterStateUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractSnapshotterStateUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractSnapshotterStateUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractSnapshotterStateUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractSnapshotterStateUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractSnapshotterStateUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractSnapshotterStateUpdated represents a SnapshotterStateUpdated event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractSnapshotterStateUpdated struct {
	NewSnapshotterState common.Address
	Raw                 types.Log // Blockchain specific contextual infos
}

// FilterSnapshotterStateUpdated is a free log retrieval operation binding the contract event 0x531a371204593f579d529597cf9ccfda38680784da19a112b6e5fd32555a6fa0.
//
// Solidity: event SnapshotterStateUpdated(address indexed newSnapshotterState)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterSnapshotterStateUpdated(opts *bind.FilterOpts, newSnapshotterState []common.Address) (*SnapshotterStateContractSnapshotterStateUpdatedIterator, error) {

	var newSnapshotterStateRule []interface{}
	for _, newSnapshotterStateItem := range newSnapshotterState {
		newSnapshotterStateRule = append(newSnapshotterStateRule, newSnapshotterStateItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "SnapshotterStateUpdated", newSnapshotterStateRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractSnapshotterStateUpdatedIterator{contract: _SnapshotterStateContract.contract, event: "SnapshotterStateUpdated", logs: logs, sub: sub}, nil
}

// WatchSnapshotterStateUpdated is a free log subscription operation binding the contract event 0x531a371204593f579d529597cf9ccfda38680784da19a112b6e5fd32555a6fa0.
//
// Solidity: event SnapshotterStateUpdated(address indexed newSnapshotterState)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchSnapshotterStateUpdated(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractSnapshotterStateUpdated, newSnapshotterState []common.Address) (event.Subscription, error) {

	var newSnapshotterStateRule []interface{}
	for _, newSnapshotterStateItem := range newSnapshotterState {
		newSnapshotterStateRule = append(newSnapshotterStateRule, newSnapshotterStateItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "SnapshotterStateUpdated", newSnapshotterStateRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractSnapshotterStateUpdated)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "SnapshotterStateUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSnapshotterStateUpdated is a log parse operation binding the contract event 0x531a371204593f579d529597cf9ccfda38680784da19a112b6e5fd32555a6fa0.
//
// Solidity: event SnapshotterStateUpdated(address indexed newSnapshotterState)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseSnapshotterStateUpdated(log types.Log) (*SnapshotterStateContractSnapshotterStateUpdated, error) {
	event := new(SnapshotterStateContractSnapshotterStateUpdated)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "SnapshotterStateUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractSnapshotterTokensClaimedIterator is returned from FilterSnapshotterTokensClaimed and is used to iterate over the raw logs and unpacked data for SnapshotterTokensClaimed events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractSnapshotterTokensClaimedIterator struct {
	Event *SnapshotterStateContractSnapshotterTokensClaimed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractSnapshotterTokensClaimedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractSnapshotterTokensClaimed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractSnapshotterTokensClaimed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractSnapshotterTokensClaimedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractSnapshotterTokensClaimedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractSnapshotterTokensClaimed represents a SnapshotterTokensClaimed event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractSnapshotterTokensClaimed struct {
	Claimer common.Address
	NodeId  *big.Int
	Amount  *big.Int
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterSnapshotterTokensClaimed is a free log retrieval operation binding the contract event 0xeea3ac209cc25ead5cb60dc143c82a740ec3e179fbb59d2be92e986aceaf0737.
//
// Solidity: event SnapshotterTokensClaimed(address indexed claimer, uint256 nodeId, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterSnapshotterTokensClaimed(opts *bind.FilterOpts, claimer []common.Address) (*SnapshotterStateContractSnapshotterTokensClaimedIterator, error) {

	var claimerRule []interface{}
	for _, claimerItem := range claimer {
		claimerRule = append(claimerRule, claimerItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "SnapshotterTokensClaimed", claimerRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractSnapshotterTokensClaimedIterator{contract: _SnapshotterStateContract.contract, event: "SnapshotterTokensClaimed", logs: logs, sub: sub}, nil
}

// WatchSnapshotterTokensClaimed is a free log subscription operation binding the contract event 0xeea3ac209cc25ead5cb60dc143c82a740ec3e179fbb59d2be92e986aceaf0737.
//
// Solidity: event SnapshotterTokensClaimed(address indexed claimer, uint256 nodeId, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchSnapshotterTokensClaimed(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractSnapshotterTokensClaimed, claimer []common.Address) (event.Subscription, error) {

	var claimerRule []interface{}
	for _, claimerItem := range claimer {
		claimerRule = append(claimerRule, claimerItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "SnapshotterTokensClaimed", claimerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractSnapshotterTokensClaimed)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "SnapshotterTokensClaimed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSnapshotterTokensClaimed is a log parse operation binding the contract event 0xeea3ac209cc25ead5cb60dc143c82a740ec3e179fbb59d2be92e986aceaf0737.
//
// Solidity: event SnapshotterTokensClaimed(address indexed claimer, uint256 nodeId, uint256 amount)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseSnapshotterTokensClaimed(log types.Log) (*SnapshotterStateContractSnapshotterTokensClaimed, error) {
	event := new(SnapshotterStateContractSnapshotterTokensClaimed)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "SnapshotterTokensClaimed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractTransferBatchIterator is returned from FilterTransferBatch and is used to iterate over the raw logs and unpacked data for TransferBatch events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractTransferBatchIterator struct {
	Event *SnapshotterStateContractTransferBatch // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractTransferBatchIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractTransferBatch)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractTransferBatch)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractTransferBatchIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractTransferBatchIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractTransferBatch represents a TransferBatch event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractTransferBatch struct {
	Operator common.Address
	From     common.Address
	To       common.Address
	Ids      []*big.Int
	Values   []*big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterTransferBatch is a free log retrieval operation binding the contract event 0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb.
//
// Solidity: event TransferBatch(address indexed operator, address indexed from, address indexed to, uint256[] ids, uint256[] values)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterTransferBatch(opts *bind.FilterOpts, operator []common.Address, from []common.Address, to []common.Address) (*SnapshotterStateContractTransferBatchIterator, error) {

	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "TransferBatch", operatorRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractTransferBatchIterator{contract: _SnapshotterStateContract.contract, event: "TransferBatch", logs: logs, sub: sub}, nil
}

// WatchTransferBatch is a free log subscription operation binding the contract event 0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb.
//
// Solidity: event TransferBatch(address indexed operator, address indexed from, address indexed to, uint256[] ids, uint256[] values)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchTransferBatch(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractTransferBatch, operator []common.Address, from []common.Address, to []common.Address) (event.Subscription, error) {

	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "TransferBatch", operatorRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractTransferBatch)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "TransferBatch", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTransferBatch is a log parse operation binding the contract event 0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb.
//
// Solidity: event TransferBatch(address indexed operator, address indexed from, address indexed to, uint256[] ids, uint256[] values)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseTransferBatch(log types.Log) (*SnapshotterStateContractTransferBatch, error) {
	event := new(SnapshotterStateContractTransferBatch)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "TransferBatch", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractTransferSingleIterator is returned from FilterTransferSingle and is used to iterate over the raw logs and unpacked data for TransferSingle events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractTransferSingleIterator struct {
	Event *SnapshotterStateContractTransferSingle // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractTransferSingleIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractTransferSingle)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractTransferSingle)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractTransferSingleIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractTransferSingleIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractTransferSingle represents a TransferSingle event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractTransferSingle struct {
	Operator common.Address
	From     common.Address
	To       common.Address
	Id       *big.Int
	Value    *big.Int
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterTransferSingle is a free log retrieval operation binding the contract event 0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62.
//
// Solidity: event TransferSingle(address indexed operator, address indexed from, address indexed to, uint256 id, uint256 value)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterTransferSingle(opts *bind.FilterOpts, operator []common.Address, from []common.Address, to []common.Address) (*SnapshotterStateContractTransferSingleIterator, error) {

	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "TransferSingle", operatorRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractTransferSingleIterator{contract: _SnapshotterStateContract.contract, event: "TransferSingle", logs: logs, sub: sub}, nil
}

// WatchTransferSingle is a free log subscription operation binding the contract event 0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62.
//
// Solidity: event TransferSingle(address indexed operator, address indexed from, address indexed to, uint256 id, uint256 value)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchTransferSingle(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractTransferSingle, operator []common.Address, from []common.Address, to []common.Address) (event.Subscription, error) {

	var operatorRule []interface{}
	for _, operatorItem := range operator {
		operatorRule = append(operatorRule, operatorItem)
	}
	var fromRule []interface{}
	for _, fromItem := range from {
		fromRule = append(fromRule, fromItem)
	}
	var toRule []interface{}
	for _, toItem := range to {
		toRule = append(toRule, toItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "TransferSingle", operatorRule, fromRule, toRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractTransferSingle)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "TransferSingle", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseTransferSingle is a log parse operation binding the contract event 0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62.
//
// Solidity: event TransferSingle(address indexed operator, address indexed from, address indexed to, uint256 id, uint256 value)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseTransferSingle(log types.Log) (*SnapshotterStateContractTransferSingle, error) {
	event := new(SnapshotterStateContractTransferSingle)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "TransferSingle", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractURIIterator is returned from FilterURI and is used to iterate over the raw logs and unpacked data for URI events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractURIIterator struct {
	Event *SnapshotterStateContractURI // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractURIIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractURI)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractURI)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractURIIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractURIIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractURI represents a URI event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractURI struct {
	Value string
	Id    *big.Int
	Raw   types.Log // Blockchain specific contextual infos
}

// FilterURI is a free log retrieval operation binding the contract event 0x6bb7ff708619ba0610cba295a58592e0451dee2622938c8755667688daf3529b.
//
// Solidity: event URI(string value, uint256 indexed id)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterURI(opts *bind.FilterOpts, id []*big.Int) (*SnapshotterStateContractURIIterator, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "URI", idRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractURIIterator{contract: _SnapshotterStateContract.contract, event: "URI", logs: logs, sub: sub}, nil
}

// WatchURI is a free log subscription operation binding the contract event 0x6bb7ff708619ba0610cba295a58592e0451dee2622938c8755667688daf3529b.
//
// Solidity: event URI(string value, uint256 indexed id)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchURI(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractURI, id []*big.Int) (event.Subscription, error) {

	var idRule []interface{}
	for _, idItem := range id {
		idRule = append(idRule, idItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "URI", idRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractURI)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "URI", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseURI is a log parse operation binding the contract event 0x6bb7ff708619ba0610cba295a58592e0451dee2622938c8755667688daf3529b.
//
// Solidity: event URI(string value, uint256 indexed id)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseURI(log types.Log) (*SnapshotterStateContractURI, error) {
	event := new(SnapshotterStateContractURI)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "URI", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractURIUpdatedIterator is returned from FilterURIUpdated and is used to iterate over the raw logs and unpacked data for URIUpdated events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractURIUpdatedIterator struct {
	Event *SnapshotterStateContractURIUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractURIUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractURIUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractURIUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractURIUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractURIUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractURIUpdated represents a URIUpdated event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractURIUpdated struct {
	NewUri string
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterURIUpdated is a free log retrieval operation binding the contract event 0xe3afa94108b5f5e82e5f6e539d161ff4b5402a85f696c67b9768ec3ae54ce366.
//
// Solidity: event URIUpdated(string newUri)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterURIUpdated(opts *bind.FilterOpts) (*SnapshotterStateContractURIUpdatedIterator, error) {

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "URIUpdated")
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractURIUpdatedIterator{contract: _SnapshotterStateContract.contract, event: "URIUpdated", logs: logs, sub: sub}, nil
}

// WatchURIUpdated is a free log subscription operation binding the contract event 0xe3afa94108b5f5e82e5f6e539d161ff4b5402a85f696c67b9768ec3ae54ce366.
//
// Solidity: event URIUpdated(string newUri)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchURIUpdated(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractURIUpdated) (event.Subscription, error) {

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "URIUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractURIUpdated)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "URIUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseURIUpdated is a log parse operation binding the contract event 0xe3afa94108b5f5e82e5f6e539d161ff4b5402a85f696c67b9768ec3ae54ce366.
//
// Solidity: event URIUpdated(string newUri)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseURIUpdated(log types.Log) (*SnapshotterStateContractURIUpdated, error) {
	event := new(SnapshotterStateContractURIUpdated)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "URIUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractUnpausedIterator is returned from FilterUnpaused and is used to iterate over the raw logs and unpacked data for Unpaused events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractUnpausedIterator struct {
	Event *SnapshotterStateContractUnpaused // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractUnpausedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractUnpaused)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractUnpaused)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractUnpausedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractUnpausedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractUnpaused represents a Unpaused event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractUnpaused struct {
	Account common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterUnpaused is a free log retrieval operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterUnpaused(opts *bind.FilterOpts) (*SnapshotterStateContractUnpausedIterator, error) {

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "Unpaused")
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractUnpausedIterator{contract: _SnapshotterStateContract.contract, event: "Unpaused", logs: logs, sub: sub}, nil
}

// WatchUnpaused is a free log subscription operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchUnpaused(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractUnpaused) (event.Subscription, error) {

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "Unpaused")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractUnpaused)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "Unpaused", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseUnpaused is a log parse operation binding the contract event 0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa.
//
// Solidity: event Unpaused(address account)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseUnpaused(log types.Log) (*SnapshotterStateContractUnpaused, error) {
	event := new(SnapshotterStateContractUnpaused)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "Unpaused", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractUpgradedIterator is returned from FilterUpgraded and is used to iterate over the raw logs and unpacked data for Upgraded events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractUpgradedIterator struct {
	Event *SnapshotterStateContractUpgraded // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractUpgradedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractUpgraded)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractUpgraded)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractUpgradedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractUpgradedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractUpgraded represents a Upgraded event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractUpgraded struct {
	Implementation common.Address
	Raw            types.Log // Blockchain specific contextual infos
}

// FilterUpgraded is a free log retrieval operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterUpgraded(opts *bind.FilterOpts, implementation []common.Address) (*SnapshotterStateContractUpgradedIterator, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractUpgradedIterator{contract: _SnapshotterStateContract.contract, event: "Upgraded", logs: logs, sub: sub}, nil
}

// WatchUpgraded is a free log subscription operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchUpgraded(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractUpgraded, implementation []common.Address) (event.Subscription, error) {

	var implementationRule []interface{}
	for _, implementationItem := range implementation {
		implementationRule = append(implementationRule, implementationItem)
	}

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "Upgraded", implementationRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractUpgraded)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "Upgraded", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseUpgraded is a log parse operation binding the contract event 0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b.
//
// Solidity: event Upgraded(address indexed implementation)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseUpgraded(log types.Log) (*SnapshotterStateContractUpgraded, error) {
	event := new(SnapshotterStateContractUpgraded)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "Upgraded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SnapshotterStateContractAllSnapshottersUpdatedIterator is returned from FilterAllSnapshottersUpdated and is used to iterate over the raw logs and unpacked data for AllSnapshottersUpdated events raised by the SnapshotterStateContract contract.
type SnapshotterStateContractAllSnapshottersUpdatedIterator struct {
	Event *SnapshotterStateContractAllSnapshottersUpdated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SnapshotterStateContractAllSnapshottersUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SnapshotterStateContractAllSnapshottersUpdated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SnapshotterStateContractAllSnapshottersUpdated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SnapshotterStateContractAllSnapshottersUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SnapshotterStateContractAllSnapshottersUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SnapshotterStateContractAllSnapshottersUpdated represents a AllSnapshottersUpdated event raised by the SnapshotterStateContract contract.
type SnapshotterStateContractAllSnapshottersUpdated struct {
	SnapshotterAddress common.Address
	Allowed            bool
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterAllSnapshottersUpdated is a free log retrieval operation binding the contract event 0x743e47fbcd2e3a64a2ab8f5dcdeb4c17f892c17c9f6ab58d3a5c235953d60058.
//
// Solidity: event allSnapshottersUpdated(address snapshotterAddress, bool allowed)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) FilterAllSnapshottersUpdated(opts *bind.FilterOpts) (*SnapshotterStateContractAllSnapshottersUpdatedIterator, error) {

	logs, sub, err := _SnapshotterStateContract.contract.FilterLogs(opts, "allSnapshottersUpdated")
	if err != nil {
		return nil, err
	}
	return &SnapshotterStateContractAllSnapshottersUpdatedIterator{contract: _SnapshotterStateContract.contract, event: "allSnapshottersUpdated", logs: logs, sub: sub}, nil
}

// WatchAllSnapshottersUpdated is a free log subscription operation binding the contract event 0x743e47fbcd2e3a64a2ab8f5dcdeb4c17f892c17c9f6ab58d3a5c235953d60058.
//
// Solidity: event allSnapshottersUpdated(address snapshotterAddress, bool allowed)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) WatchAllSnapshottersUpdated(opts *bind.WatchOpts, sink chan<- *SnapshotterStateContractAllSnapshottersUpdated) (event.Subscription, error) {

	logs, sub, err := _SnapshotterStateContract.contract.WatchLogs(opts, "allSnapshottersUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SnapshotterStateContractAllSnapshottersUpdated)
				if err := _SnapshotterStateContract.contract.UnpackLog(event, "allSnapshottersUpdated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseAllSnapshottersUpdated is a log parse operation binding the contract event 0x743e47fbcd2e3a64a2ab8f5dcdeb4c17f892c17c9f6ab58d3a5c235953d60058.
//
// Solidity: event allSnapshottersUpdated(address snapshotterAddress, bool allowed)
func (_SnapshotterStateContract *SnapshotterStateContractFilterer) ParseAllSnapshottersUpdated(log types.Log) (*SnapshotterStateContractAllSnapshottersUpdated, error) {
	event := new(SnapshotterStateContractAllSnapshottersUpdated)
	if err := _SnapshotterStateContract.contract.UnpackLog(event, "allSnapshottersUpdated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
