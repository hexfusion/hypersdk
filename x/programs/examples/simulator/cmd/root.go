// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	defaultDBPath = "simulator.db"
	dbPath        string
	db            *state.SimpleMutable
	dbCloser      dBCloserFn
	log           logging.Logger
	rootCmd       = &cobra.Command{
		Use:   "simulator",
		Short: "HyperSDK Program simulator",
	}
)

func init() {
	cobra.EnablePrefixMatching = true
	rootCmd.AddCommand(
		programCmd(),
		keyCmd(),
	)

	rootCmd.PersistentFlags().StringVar(
		&dbPath,
		"database",
		defaultDBPath,
		"path to database (will create if missing)",
	)

	rootCmd.PersistentPreRunE = func(*cobra.Command, []string) (err error) {
		db, dbCloser, err = getDB(context.Background(), dbPath)
		if err != nil {
			return err
		}

		utils.Outf("{{yellow}}database:{{/}} %s\n", dbPath)
		return nil
	}
}

func Execute() error {
	defer func() {
		err := dbCloser()
		if err != nil {
			utils.Outf("{{red}}database closed with error:{{/}} %s\n", err)
		}
	}()

	return rootCmd.Execute()
}
