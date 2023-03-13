package testnet

import (
	"fmt"
	"os"
	"path/filepath"

	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cosmos/cosmos-sdk/x/genutil"
)

// Comet wraps an in-process cometbft instance to support a validator.
type Comet struct{}

// CometConfig is the minimal set of configuration to run a Comet instance in-process.
type CometConfig struct {
	Cfg *cmtcfg.Config

	NodeKey *p2p.NodeKey
}

// NewDiskConfig initializes files on disk to support the given comet config.
// The rootDir is created and set as the root of the comet config.
func NewDiskConfig(rootDir string, cfg *cmtcfg.Config) (*CometConfig, error) {
	cfg.SetRoot(rootDir)

	// The config directory must exist, or initializing validator files will fail.
	if err := os.MkdirAll(filepath.Join(rootDir, "config"), 0o750); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	nodeKey, err := p2p.LoadOrGenNodeKey(cfg.NodeKeyFile())
	if err != nil {
		return nil, fmt.Errorf("failed to generate node key: %w", err)
	}

	_, _, err = genutil.InitializeNodeValidatorFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize node validator files: %w", err)
	}

	return &CometConfig{
		Cfg: cfg,

		NodeKey: nodeKey,
	}, nil
}
