package testnet

import (
	"encoding/json"
	"fmt"

	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type GenesisBuilder struct {
	amino *codec.LegacyAmino

	chainID string

	json map[string]json.RawMessage

	gentxs []sdk.Tx
}

func NewGenesisBuilder() *GenesisBuilder {
	return &GenesisBuilder{
		amino: codec.NewLegacyAmino(),

		json: map[string]json.RawMessage{
			"app_state": json.RawMessage("{}"),
		},
	}
}

func (b *GenesisBuilder) GenTx(privVal cmted25519.PrivKey, val cmttypes.GenesisValidator, amount sdk.Coin) *GenesisBuilder {
	if b.chainID == "" {
		panic(fmt.Errorf("(*GenesisBuilder).GenTx called before (*GenesisBuilder).ChainID"))
	}

	pubKey, err := cryptocodec.FromCmtPubKeyInterface(val.PubKey)
	if err != nil {
		panic(err)
	}

	// Produce the create validator message.
	msg, err := stakingtypes.NewMsgCreateValidator(
		val.Address.Bytes(),
		pubKey,
		amount,
		stakingtypes.Description{
			Moniker: "TODO",
		},
		stakingtypes.CommissionRates{
			Rate:          sdk.MustNewDecFromStr("0.1"),
			MaxRate:       sdk.MustNewDecFromStr("0.2"),
			MaxChangeRate: sdk.MustNewDecFromStr("0.01"),
		},
		sdk.OneInt(),
	)
	if err != nil {
		panic(err)
	}

	if err := msg.ValidateBasic(); err != nil {
		panic(err)
	}

	// Configure the tx builder.
	ir := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(ir)
	stakingtypes.RegisterInterfaces(ir)
	pCodec := codec.NewProtoCodec(ir)

	txConf := authtx.NewTxConfig(pCodec, tx.DefaultSignModes)

	txb := txConf.NewTxBuilder()
	if err := txb.SetMsgs(msg); err != nil {
		panic(err)
	}

	const signMode = signing.SignMode_SIGN_MODE_DIRECT

	// Generate bytes to be signed.
	bytesToSign, err := txConf.SignModeHandler().GetSignBytes(
		signing.SignMode_SIGN_MODE_DIRECT,
		authsigning.SignerData{
			ChainID: b.chainID,
			PubKey:  pubKey,
			Address: sdk.ValAddress(val.Address).String(), // TODO: this relies on global bech32 config.

			// No account or sequence number for gentx.
		},
		txb.GetTx(),
	)
	if err != nil {
		panic(err)
	}

	// Produce the signature.
	signed, err := privVal.Sign(bytesToSign)
	if err != nil {
		panic(err)
	}

	// Set the signature on the builder.
	if err := txb.SetSignatures(
		signing.SignatureV2{
			PubKey: pubKey,
			Data: &signing.SingleSignatureData{
				SignMode:  signMode,
				Signature: signed,
			},
		},
	); err != nil {
		panic(err)
	}

	b.gentxs = append(b.gentxs, txb.GetTx())

	return b
}

func (b *GenesisBuilder) ChainID(id string) *GenesisBuilder {
	b.chainID = id

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

func (b *GenesisBuilder) StakingWithDefaultParams(vals StakingValidators, delegations []stakingtypes.Delegation) *GenesisBuilder {
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

func (b *GenesisBuilder) BankingWithDefaultParams(
	balances []banktypes.Balance,
	totalSupply sdk.Coins,
	denomMetadata []banktypes.Metadata,
	sendEnabled []banktypes.SendEnabled,
) *GenesisBuilder {
	return b.Banking(
		banktypes.DefaultParams(),
		balances,
		totalSupply,
		denomMetadata,
		sendEnabled,
	)
}

func (b *GenesisBuilder) Banking(
	params banktypes.Params,
	balances []banktypes.Balance,
	totalSupply sdk.Coins,
	denomMetadata []banktypes.Metadata,
	sendEnabled []banktypes.SendEnabled,
) *GenesisBuilder {
	var err error
	b.json[banktypes.ModuleName], err = b.amino.MarshalJSON(
		banktypes.NewGenesisState(
			params,
			balances,
			totalSupply,
			denomMetadata,
			sendEnabled,
		),
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
