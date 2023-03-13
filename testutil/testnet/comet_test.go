package testnet_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/proxy"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	servercmtlog "github.com/cosmos/cosmos-sdk/server/log"
	"github.com/cosmos/cosmos-sdk/testutil/mock"
	"github.com/cosmos/cosmos-sdk/testutil/testnet"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

type GenesisState struct {
	Validators []stakingtypes.Validator

	Delegations []stakingtypes.Delegation

	GenesisAccounts []authtypes.GenesisAccount

	Balances []banktypes.Balance

	CometValidators []cmttypes.GenesisValidator
}

func newGenesisState(t *testing.T, count int) GenesisState {
	t.Helper()

	stakingVals := make([]stakingtypes.Validator, count)
	genesisAccounts := make([]authtypes.GenesisAccount, count)
	delegations := make([]stakingtypes.Delegation, count)
	balances := make([]banktypes.Balance, count)
	cmtGenesisValidators := make([]cmttypes.GenesisValidator, count)
	for i := 0; i < count; i++ {
		privVal := mock.NewPV()
		pubKey, err := privVal.GetPubKey()
		require.NoError(t, err)

		const votingPower = 1
		cmtVal := cmttypes.NewValidator(pubKey, votingPower)
		cmtGenesisValidators[i] = cmttypes.GenesisValidator{
			Address: cmtVal.Address,
			PubKey:  cmtVal.PubKey,
			Power:   cmtVal.VotingPower,
			Name:    fmt.Sprintf("val-%d", i),
		}

		pk, err := cryptocodec.FromCmtPubKeyInterface(cmtVal.PubKey)
		require.NoError(t, err)

		pkAny, err := codectypes.NewAnyWithValue(pk)
		require.NoError(t, err)

		stakingVals[i] = stakingtypes.Validator{
			OperatorAddress:   sdk.ValAddress(cmtVal.Address).String(),
			ConsensusPubkey:   pkAny,
			Status:            stakingtypes.Bonded,
			Tokens:            sdk.DefaultPowerReduction,
			DelegatorShares:   sdkmath.LegacyOneDec(),
			MinSelfDelegation: sdkmath.ZeroInt(),

			// more fields uncopied from testutil/sims/app_helpers.go:220
		}

		delegatorPrivKey := secp256k1.GenPrivKey()
		genesisAccounts[i] = authtypes.NewBaseAccount(
			delegatorPrivKey.PubKey().Address().Bytes(),
			delegatorPrivKey.PubKey(),
			0,
			0,
		)
		balances[i] = banktypes.Balance{
			Address: genesisAccounts[i].GetAddress().String(),
			Coins:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100_000_000_000_000))),
		}
		delegations[i] = stakingtypes.NewDelegation(
			genesisAccounts[i].GetAddress(),
			cmtVal.Address.Bytes(), // Maybe wrong source? Something other than cmtVal?
			sdkmath.LegacyOneDec(),
		)
	}
	return GenesisState{
		Validators:      stakingVals,
		GenesisAccounts: genesisAccounts,
		Balances:        balances,

		CometValidators: cmtGenesisValidators,
	}
}

func (s GenesisState) GenesisJSON(chainID string) ([]byte, error) {
	jConsParams, err := (&genutiltypes.ConsensusGenesis{
		Params:     cmttypes.DefaultConsensusParams(),
		Validators: s.CometValidators,
	}).MarshalJSON()
	if err != nil {
		return nil, err
	}

	amino := codec.NewLegacyAmino()
	jStakingGenesis, err := amino.MarshalJSON(
		stakingtypes.NewGenesisState(
			stakingtypes.DefaultParams(),
			s.Validators,
			s.Delegations,
		),
	)
	if err != nil {
		return nil, err
	}

	totalSupply := sdk.NewCoins()
	for _, b := range s.Balances {
		totalSupply = totalSupply.Add(b.Coins...)
	}

	for range s.Delegations {
		// TODO: is there a better way to multiply instead of looping here?
		totalSupply = totalSupply.Add(sdk.NewCoin(sdk.DefaultBondDenom, sdk.DefaultPowerReduction))
	}

	jBankGenesis, err := amino.MarshalJSON(
		banktypes.NewGenesisState(
			banktypes.DefaultGenesisState().Params,
			s.Balances,
			totalSupply,
			nil, // []banktypes.Metadata
			nil, // []banktypes.SendEnabled
		),
	)
	if err != nil {
		return nil, err
	}

	return amino.MarshalJSON(map[string]json.RawMessage{
		"chain_id":                     json.RawMessage(strconv.Quote(chainID)),
		consensusparamtypes.ModuleName: jConsParams,
		stakingtypes.ModuleName:        jStakingGenesis,
		banktypes.ModuleName:           jBankGenesis,
	})
}

func TestComet_SingleInstance(t *testing.T) {
	dir := t.TempDir()

	cfg, err := testnet.NewDiskConfig(dir, cmtcfg.DefaultConfig())
	require.NoError(t, err)

	gs := newGenesisState(t, 2)
	jGenesis, err := gs.GenesisJSON("testchain")
	require.NoError(t, err)

	err = os.WriteFile(cfg.Cfg.GenesisFile(), jGenesis, 0600)
	require.NoError(t, err)

	appGenesisProvider := func() (*cmttypes.GenesisDoc, error) {
		appGenesis, err := genutiltypes.AppGenesisFromFile(cfg.Cfg.GenesisFile())
		if err != nil {
			return nil, err
		}

		return appGenesis.ToGenesisDoc()
	}

	logger := log.NewTestLogger(t)

	app := newMinimalBaseApp(logger)

	n, err := node.NewNode(
		cfg.Cfg,
		privval.LoadOrGenFilePV(cfg.Cfg.PrivValidatorKeyFile(), cfg.Cfg.PrivValidatorStateFile()),
		cfg.NodeKey,
		proxy.NewLocalClientCreator(app),
		appGenesisProvider,
		node.DefaultDBProvider,
		node.DefaultMetricsProvider(cfg.Cfg.Instrumentation),
		servercmtlog.CometZeroLogWrapper{Logger: logger.With("rootmodule", "comet_node")},
	)
	if err != nil {
		t.Fatal(err)
	}

	defer n.Stop()

	time.Sleep(time.Second)
}

func newMinimalBaseApp(logger log.Logger) *baseapp.BaseApp {
	ir := codectypes.NewInterfaceRegistry()
	pCodec := codec.NewProtoCodec(ir)

	txConfig := tx.NewTxConfig(pCodec, tx.DefaultSignModes)

	db := dbm.NewMemDB()
	consKey := storetypes.NewKVStoreKey(consensusparamtypes.StoreKey)
	consParamKeeper := consensusparamkeeper.NewKeeper(
		pCodec,
		consKey,
		authtypes.NewModuleAddress(govtypes.ModuleName).String(),
	)

	app := baseapp.NewBaseApp(
		"minimal_for_test",
		logger.With("rootmodule", "baseapp"),
		db,
		txConfig.TxDecoder(),
	)
	app.SetParamStore(&consParamKeeper)

	app.MountKVStores(map[string]*storetypes.KVStoreKey{
		consensusparamtypes.StoreKey: consKey,
	})

	cms := app.CommitMultiStore()
	if err := cms.LoadLatestVersion(); err != nil {
		panic(err)
	}

	return app
}
