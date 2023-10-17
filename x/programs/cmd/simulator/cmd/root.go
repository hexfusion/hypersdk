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

	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/controller"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/genesis"
	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/rpc"
)

const (
	dbFolder = ".simulator/db"
)

type simulator struct {
	log      logging.Logger
	logLevel string

	// vm used for the simulator
	vm *vm.VM

	// client used to send JSON-RPC requests to the VM
	cli *rpc.JSONRPCClient
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
		newRunCmd(s.log, s.cli),
		newProgramCmd(s.log, s.vm.State()),
		newKeyCmd(),
	)

	cobra.EnablePrefixMatching = true
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.DisableAutoGenTag = true
	cmd.SilenceErrors = true
	cmd.SetHelpCommand(&cobra.Command{Hidden: true})

	cmd.PersistentFlags().StringVar(&s.logLevel, "log-level", "info", "log level")

	// ensure vm and databases are properly closed
	cobra.OnFinalize(func() {
		if s.vm != nil {
			err := s.vm.Shutdown(cmd.Context())
			if err != nil {
				utils.Outf("{{red}}vm closed with error:{{/}} %s\n", err)
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
	app := &appSender{}

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
		app,
	)
	if err != nil {
		return nil
	}

	s.handlers, err = vm.CreateHandlers(context.Background())
	if err != nil {
		return nil
	}


	return nil
}