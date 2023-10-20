// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
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

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/controller"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/genesis"
)

const (
	dbFolder = ".simulator/db"
)

type simulator struct {
	log      logging.Logger
	logLevel string

	vm *vm.VM
	db      *state.SimpleMutable
	genesis *genesis.Genesis
}

func NewRootCmd() *cobra.Command {
	s := &simulator{}
	cmd := &cobra.Command{
		Use:   "simulator",
		Short: "HyperSDK program VM simulator",
	}

	cobra.EnablePrefixMatching = true
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.DisableAutoGenTag = true
	cmd.SilenceErrors = true
	cmd.SetHelpCommand(&cobra.Command{Hidden: true})
	cmd.PersistentFlags().StringVar(&s.logLevel, "log-level", "info", "log level")

	err := s.Init()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// add subcommands
	cmd.AddCommand(
		newRunCmd(s.log, s.db),
		newProgramCmd(s.log, s.db),
		newKeyCmd(s.log, s.db),
	)

	// ensure vm and databases are properly closed on simulator exit
	cobra.OnFinalize(func() {
		if s.vm != nil {
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

	// setup logger
	loggingConfig := logging.Config{}
	loggingConfig.LogLevel, err = logging.ToLevel(s.logLevel)
	if err != nil {
		return err
	}
	logFactory := logging.NewFactory(loggingConfig)
	s.log, err = logFactory.Make(nodeID.String())
	if err != nil {
		logFactory.Close()
		return nil
	}

	sk, err := bls.NewSecretKey()
	if err != nil {
		return nil
	}

	// setup pebble and db manager
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
		Log:          s.log,
		ChainDataDir: dbPath,
		Metrics:      metrics.NewOptionalGatherer(),
		PublicKey:    bls.PublicFromSecretKey(sk),
	}

	toEngine := make(chan common.Message, 1)

	// initialize the simulator VM
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
		return err
	}
	s.vm = vm
	// force the vm to be ready because it has no peers.
	s.vm.ForceReady()

	stateDB, err := s.vm.State()
	if err != nil {
		return err
	}
	s.db = state.NewSimpleMutable(stateDB)
	s.genesis = genesis.Default()

	return nil
}