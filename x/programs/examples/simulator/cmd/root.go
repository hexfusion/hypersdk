package cmd

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	defaultDBPath = "simulator.db"
	dbPath        string
	db            *state.SimpleMutable
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
		db, err = getDB(dbPath)
		if err != nil {
			return err
		}

		utils.Outf("{{yellow}}database:{{/}} %s\n", dbPath)
		return nil
	}
}

func Execute() error {
	return rootCmd.Execute()
}
