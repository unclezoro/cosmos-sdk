package testnet

import (
	"encoding/json"

	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/codec"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type GenesisBuilder struct {
	amino *codec.LegacyAmino

	json map[string]json.RawMessage
}

func NewGenesisBuilder() *GenesisBuilder {
	return &GenesisBuilder{
		amino: codec.NewLegacyAmino(),

		json: make(map[string]json.RawMessage),
	}
}

func (b *GenesisBuilder) ChainID(id string) *GenesisBuilder {
	var err error
	b.json["chain_id"], err = json.Marshal(id)
	if err != nil {
		panic(err)
	}

	return b
}

func (b *GenesisBuilder) Consensus(params *cmttypes.ConsensusParams, vals CometGenesisValidators) *GenesisBuilder {
	if params == nil {
		params = cmttypes.DefaultConsensusParams()
	}
	var err error
	b.json[consensusparamtypes.ModuleName], err = (&genutiltypes.ConsensusGenesis{
		Params:     params,
		Validators: vals,
	}).MarshalJSON()
	if err != nil {
		panic(err)
	}

	return b
}

func (b *GenesisBuilder) DefaultStaking(vals StakingValidators, delegations []stakingtypes.Delegation) *GenesisBuilder {
	return b.Staking(stakingtypes.DefaultParams(), vals, delegations)
}

func (b *GenesisBuilder) Staking(
	params stakingtypes.Params,
	vals StakingValidators,
	delegations []stakingtypes.Delegation,
) *GenesisBuilder {
	var err error
	b.json[stakingtypes.ModuleName], err = b.amino.MarshalJSON(
		stakingtypes.NewGenesisState(params, vals, delegations),
	)
	if err != nil {
		panic(err)
	}

	return b
}

func (b *GenesisBuilder) Encode() []byte {
	j, err := b.amino.MarshalJSON(b.json)
	if err != nil {
		panic(err)
	}

	return j
}
