package testnet

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type DelegatorPrivKeys []*secp256k1.PrivKey

func NewDelegatorPrivKeys(n int) DelegatorPrivKeys {
	dpk := make(DelegatorPrivKeys, n)

	for i := range dpk {
		dpk[i] = secp256k1.GenPrivKey()
	}

	return dpk
}

func (dpk DelegatorPrivKeys) BaseAccounts() BaseAccounts {
	ba := make(BaseAccounts, len(dpk))

	for i, pk := range dpk {
		pubKey := pk.PubKey()

		const accountNumber = 0
		const sequenceNumber = 0

		ba[i] = authtypes.NewBaseAccount(
			pubKey.Address().Bytes(), pubKey, accountNumber, sequenceNumber,
		)
	}

	return ba
}

type BaseAccounts []*authtypes.BaseAccount

func (ba BaseAccounts) Balances(singleBalance sdk.Coins) (balances []banktypes.Balance, supply sdk.Coins) {
	balances = make([]banktypes.Balance, len(ba))

	for i, b := range ba {
		balances[i] = banktypes.Balance{
			Address: b.GetAddress().String(),
			Coins:   singleBalance,
		}

		supply = supply.Add(singleBalance...)
	}

	return balances, supply
}

func (ba BaseAccounts) Delegations(vals CometGenesisValidators) []stakingtypes.Delegation {
	if len(vals) != len(ba) {
		panic(fmt.Errorf("number of base accounts (%d) != number of validators (%d)", len(ba), len(vals)))
	}

	ds := make([]stakingtypes.Delegation, len(ba))
	for i, a := range ba {
		ds[i] = stakingtypes.NewDelegation(
			a.GetAddress(),
			vals[i].Address.Bytes(),
			sdkmath.LegacyOneDec(),
		)
	}

	return ds
}
