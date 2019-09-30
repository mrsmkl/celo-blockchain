// Copyright 2017 The Celo Authors
// This file is part of the celo library.
//
// The celo library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The celo library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the celo library. If not, see <http://www.gnu.org/licenses/>.

package blockchain_parameters

import (
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/contract_comm"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

const (
	blockchainParametersABIString = `[{
			"constant": true,
			"inputs": [],
			"name": "getMinimumClientVersion",
			"outputs": [
			  {
				"name": "major",
				"type": "uint256"
			  },
			  {
				"name": "minor",
				"type": "uint256"
			  },
			  {
				"name": "patch",
				"type": "uint256"
			  }
			],
			"payable": false,
			"stateMutability": "view",
			"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "gasForNonGoldCurrencies",
		"outputs": [
		  {
			"name": "",
			"type": "uint256"
		  }
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	  }]`
)

const defaultGasAmount = 2000000

var (
	blockchainParametersABI, _ = abi.JSON(strings.NewReader(blockchainParametersABIString))
)

func CheckMinimumVersion(header *types.Header, state vm.StateDB) error {
	version := [3]*big.Int{big.NewInt(0), big.NewInt(0), big.NewInt(0)}
	var err error
	_, err = contract_comm.MakeStaticCall(
		params.BlockchainParametersRegistryId,
		blockchainParametersABI,
		"getMinimumClientVersion",
		[]interface{}{},
		&version,
		defaultGasAmount,
		header,
		state,
	)

	if err != nil {
		log.Warn("Error checking client version", "err", err, "contract id", params.BlockchainParametersRegistryId)
		return nil
	}

	if params.VersionMajor < version[0].Uint64() ||
		params.VersionMajor == version[0].Uint64() && params.VersionMinor < version[1].Uint64() ||
		params.VersionMajor == version[0].Uint64() && params.VersionMinor == version[1].Uint64() && params.VersionPatch < version[2].Uint64() {
		log.Crit("Client version older than required", "current", params.Version, "required", version)
	}

	return nil
}

func GetCurrencyGasCost(header *types.Header, state vm.StateDB, gasCurrency *common.Address) uint64 {
	var gas *big.Int
	var err error
	if gasCurrency == nil {
		return 0
	}
	_, err = contract_comm.MakeStaticCall(
		params.BlockchainParametersRegistryId,
		blockchainParametersABI,
		"gasForNonGoldCurrencies",
		[]interface{}{},
		&gas,
		defaultGasAmount,
		header,
		state,
	)
	if err != nil {
		log.Info("Default gas", "gas", params.AdditionalGasForNonGoldCurrencies)
		return params.AdditionalGasForNonGoldCurrencies
	}
	log.Info("Reading gas", "gas", gas)
	return gas.Uint64()
}
