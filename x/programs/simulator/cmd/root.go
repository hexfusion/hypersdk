// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"encoding/json"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	avago_version "github.com/ava-labs/avalanchego/version"

	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/x/programs/simulator/vm/controller"
	"github.com/ava-labs/hypersdk/x/programs/simulator/vm/genesis"
)

const (
	dbFolder = ".simulator/db"
)

type simulator struct {
	log      logging.Logger
	logLevel string

	// vm used for the simulator
	vm *vm.VM
	// database used to store the vm state
	db      *state.SimpleMutable
	genesis *genesis.Genesis
}

func NewRootCmd() *cobra.Command {
	s := &simulator{}
	cmd := &cobra.Command{
		Use:   "simulator",
		Short: "HyperSDK program VM simulator",
		RunE: func(cmd *cobra.Command, args []string) error {
			return s.Init()
		},
	}

	// add subcommands
	cmd.AddCommand(
		newRunCmd(s.log, s.db),
		newProgramCmd(s.log, s.db),
		newKeyCmd(),
	)

	cobra.EnablePrefixMatching = true
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.DisableAutoGenTag = true
	cmd.SilenceErrors = true
	cmd.SetHelpCommand(&cobra.Command{Hidden: true})

	cmd.PersistentFlags().StringVar(&s.logLevel, "log-level", "info", "log level")

	cobra.OnFinalize(func() {
		if s.vm != nil {
			// ensure vm and databases are properly closed
			err := s.vm.Shutdown(cmd.Context())
			if err != nil {
				utils.Outf("{{red}}simulator vm closed with error:{{/}} %s\n", err)
			}
		}
	})

	return cmd
}
func (s *simulator) Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dbPath := path.Join(homeDir, dbFolder)

	// TODO: allow for user defined ids.
	nodeID := ids.GenerateTestNodeID()
	networkID := uint32(1)
	subnetID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()

	loggingConfig := logging.Config{}
	loggingConfig.LogLevel, err = logging.ToLevel(s.logLevel)
	logFactory := logging.NewFactory(loggingConfig)
	log, err := logFactory.Make(nodeID.String())
	if err != nil {
		logFactory.Close()
		return nil
	}

	sk, err := bls.NewSecretKey()
	if err != nil {
		return nil
	}

	pdb, _, err := pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		return nil
	}
	db, err := manager.NewManagerFromDBs([]*manager.VersionedDatabase{
		{
			Database: pdb,
			Version:  avago_version.CurrentDatabase,
		},
	})
	if err != nil {
		return nil
	}

	genesisBytes, err := json.Marshal(genesis.Default())
	if err != nil {
		return nil
	}

	snowCtx := &snow.Context{
		NetworkID:    networkID,
		SubnetID:     subnetID,
		ChainID:      chainID,
		NodeID:       nodeID,
		Log:          log,
		ChainDataDir: dbPath,
		Metrics:      metrics.NewOptionalGatherer(),
		PublicKey:    bls.PublicFromSecretKey(sk),
	}

	toEngine := make(chan common.Message, 1)
	vm := controller.New()
	err = vm.Initialize(
		context.TODO(),
		snowCtx,
		db,
		genesisBytes,
		nil,
		nil,
		toEngine,
		nil,
		nil,
	)
	if err != nil {
		return nil
	}
	s.vm = vm

	stateDB, err := s.vm.State()
	if err != nil {
		return err
	}
	s.db = state.NewSimpleMutable(stateDB)
	s.genesis = genesis.Default()

	return nil
}
