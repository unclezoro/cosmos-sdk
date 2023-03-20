package testnet

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	cmttypes "github.com/cometbft/cometbft/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type ValidatorPrivKeys []cmted25519.PrivKey

func NewValidatorPrivKeys(n int) ValidatorPrivKeys {
	vpk := make(ValidatorPrivKeys, n)

	for i := range vpk {
		vpk[i] = cmted25519.GenPrivKey()
	}

	return vpk
}

func (vpk ValidatorPrivKeys) CometGenesisValidators() CometGenesisValidators {
	cgv := make(CometGenesisValidators, len(vpk))

	for i, pk := range vpk {
		pubKey := pk.PubKey()

		const votingPower = 1
		cmtVal := cmttypes.NewValidator(pubKey, votingPower)

		cgv[i] = cmttypes.GenesisValidator{
			Address: cmtVal.Address,
			PubKey:  cmtVal.PubKey,
			Power:   cmtVal.VotingPower,
			Name:    fmt.Sprintf("val-%d", i),
		}
	}

	return cgv
}

type CometGenesisValidators []cmttypes.GenesisValidator

func (cgv CometGenesisValidators) StakingValidators() (vals StakingValidators, supply sdk.Coins) {
	vals = make(StakingValidators, len(cgv))
	for i, v := range cgv {
		pk, err := cryptocodec.FromCmtPubKeyInterface(v.PubKey)
		if err != nil {
			panic(fmt.Errorf("failed to extract comet pub key: %w", err))
		}

		pkAny, err := codectypes.NewAnyWithValue(pk)
		if err != nil {
			panic(fmt.Errorf("failed to wrap pub key in any type: %w", err))
		}

		vals[i] = stakingtypes.Validator{
			OperatorAddress:   sdk.ValAddress(v.Address).String(), // TODO: this relies on global bech32 config.
			ConsensusPubkey:   pkAny,
			Status:            stakingtypes.Bonded,
			Tokens:            sdk.DefaultPowerReduction,
			DelegatorShares:   sdkmath.LegacyOneDec(),
			MinSelfDelegation: sdkmath.ZeroInt(),

			// more fields uncopied from testutil/sims/app_helpers.go:220
		}

		supply = supply.Add(sdk.NewCoin(sdk.DefaultBondDenom, sdk.DefaultPowerReduction))
	}

	return vals, supply
}

type StakingValidators []stakingtypes.Validator

func (sv StakingValidators) BondedPoolBalance() banktypes.Balance {
	var coins sdk.Coins

	for _, v := range sv {
		coins = coins.Add(sdk.NewCoin(sdk.DefaultBondDenom, v.Tokens))
	}

	return banktypes.Balance{
		Address: authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String(),
		Coins:   coins,
	}
}
