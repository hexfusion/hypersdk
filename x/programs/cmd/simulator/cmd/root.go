// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	dbFolder = ".simulator/db"
	db       *state.SimpleMutable
	dbCloser dBCloserFn

	log logging.Logger
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "simulator",
		Short: "HyperSDK program VM simulator",
		PersistentPreRunE: func(*cobra.Command, []string) (err error) {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			// Open storage
			dbPath := path.Join(homeDir, dbFolder)
			db, dbCloser, err = getDB(context.Background(), dbPath)
			if err != nil {
				return err
			}

			log = logging.NewLogger(
				"",
				logging.NewWrappedCore(
					logging.Info,
					os.Stderr,
					logging.Plain.ConsoleEncoder(),
				))

			utils.Outf("{{yellow}}database:{{/}} %s\n", dbPath)
			return nil
		},
	}

	// add subcommands
	cmd.AddCommand(
		newProgramCmd(),
		newKeyCmd(),
	)

	cobra.EnablePrefixMatching = true
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.DisableAutoGenTag = true
	cmd.SilenceErrors = true
	cmd.SetHelpCommand(&cobra.Command{Hidden: true})

	// ensure database is properly shutdown
	cobra.OnFinalize(func() {
		if db != nil {
			err := dbCloser()
			if err != nil {
				utils.Outf("{{red}}database closed with error:{{/}} %s\n", err)
			}
		}
	})

	return cmd
}
