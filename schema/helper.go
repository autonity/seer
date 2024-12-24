package schema

import (
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

func addressToABI(address common.Address) abi.ABI {
	//TODO
	switch address {
	case AccountabilityContractAddress:
		return abi.ABI{}

	}
	return abi.ABI{}
}
