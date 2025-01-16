package helper

import (
	"log/slog"

	"github.com/autonity/autonity/accounts/abi"
	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/crypto"
)

var (
	DeployerAddress                        = common.Address{}
	AutonityContractAddress                = crypto.CreateAddress(DeployerAddress, 0)
	AccountabilityContractAddress          = crypto.CreateAddress(DeployerAddress, 1)
	OracleContractAddress                  = crypto.CreateAddress(DeployerAddress, 2)
	ACUContractAddress                     = crypto.CreateAddress(DeployerAddress, 3)
	SupplyControlContractAddress           = crypto.CreateAddress(DeployerAddress, 4)
	StabilizationContractAddress           = crypto.CreateAddress(DeployerAddress, 5)
	UpgradeManagerContractAddress          = crypto.CreateAddress(DeployerAddress, 6)
	InflationControllerContractAddress     = crypto.CreateAddress(DeployerAddress, 7)
	StakeableVestingManagerContractAddress = crypto.CreateAddress(DeployerAddress, 8)
	NonStakeableVestingContractAddress     = crypto.CreateAddress(DeployerAddress, 9)
	OmissionAccountabilityContractAddress  = crypto.CreateAddress(DeployerAddress, 10)

	ContractAddresses = append([]common.Address{
		AutonityContractAddress,
		AccountabilityContractAddress,
		OracleContractAddress,
		ACUContractAddress,
		SupplyControlContractAddress,
		StabilizationContractAddress,
		UpgradeManagerContractAddress,
		InflationControllerContractAddress,
		StakeableVestingManagerContractAddress,
		NonStakeableVestingContractAddress,
		OmissionAccountabilityContractAddress,
	})
)

func PrintContractAddresses() {
	slog.Info("Contract addresses",
		"Deployer Address", DeployerAddress.Hex(),
		"AutonityContractAddress", AutonityContractAddress.Hex(),
		"AccountabilityContractAddress", AccountabilityContractAddress.Hex(),
		"OracleContractAddress", OracleContractAddress.Hex(),
		"ACUContractAddress", ACUContractAddress.Hex(),
		"SupplyControlContractAddress", SupplyControlContractAddress.Hex(),
		"StabilizationContractAddress", StabilizationContractAddress.Hex(),
		"UpgradeManagerContractAddress", UpgradeManagerContractAddress.Hex(),
		"InflationControllerContractAddress", InflationControllerContractAddress.Hex(),
		"StakeableVestingManagerContractAddress", StakeableVestingManagerContractAddress.Hex(),
		"NonStakeableVestingContractAddress", NonStakeableVestingContractAddress.Hex(),
		"OmissionAccountabilityContractAddress", OmissionAccountabilityContractAddress.Hex(),
	)
}

func addressToABI(address common.Address) abi.ABI {
	//TODO
	switch address {
	case AccountabilityContractAddress:
		return abi.ABI{}

	}
	return abi.ABI{}
}

func AddressToContractName(address common.Address) string {
	switch address {
	case AutonityContractAddress:
		return "Autonity"
	case AccountabilityContractAddress:
		return "Accountability"
	case OracleContractAddress:
		return "Oracle"
	case ACUContractAddress:
		return "ACU"
	case SupplyControlContractAddress:
		return "SupplyControl"
	case StabilizationContractAddress:
		return "Stabilization"
	case UpgradeManagerContractAddress:
		return "UpgradeManager"
	case InflationControllerContractAddress:
		return "InflationController"
	case StakeableVestingManagerContractAddress:
		return "StakeableVestingManager"
	case NonStakeableVestingContractAddress:
		return "NonStakeableVestingManager"
	case OmissionAccountabilityContractAddress:
		return "OmissionAccountability"
	}
	return "Unknown"
}

func AutCommitteeToCommittee(autCommittee []autonity.AutonityCommitteeMember) []types.CommitteeMember {
	committee := make([]types.CommitteeMember, 0)
	for _, member := range autCommittee {
		committee = append(committee, types.CommitteeMember{
			Address:           member.Addr,
			ConsensusKeyBytes: member.ConsensusKey,
			VotingPower:       member.VotingPower,
		})
	}
	return committee
}
func CommitteeToAutCommittee(committee []types.CommitteeMember) []autonity.AutonityCommitteeMember {
	autCommittee := make([]autonity.AutonityCommitteeMember, 0)
	for _, member := range committee {
		autCommittee = append(autCommittee, autonity.AutonityCommitteeMember{
			Addr:         member.Address,
			ConsensusKey: member.ConsensusKeyBytes,
			VotingPower:  member.VotingPower,
		})
	}
	return autCommittee
}
