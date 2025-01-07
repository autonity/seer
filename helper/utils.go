package helper

import (
	"log/slog"

	"github.com/autonity/autonity/accounts/abi"
	"github.com/autonity/autonity/common"
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
