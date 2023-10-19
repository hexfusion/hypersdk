// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

func newKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage private keys",
	}
	cmd.AddCommand(
		newCreateKeyCmd(),
	)
	return cmd
}

func newCreateKeyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create [name]",
		Short: "Creates a new named private key and stores it in the database",
		RunE:  func(cmd *cobra.Command, args []string) error {
			
		}
		Args:  cobra.MinimumNArgs(1),
	}
}

func createKeys(cmd *cobra.Command, args []string) error {
	name := args[0]
	err := newKey(cmd.Context(), db, name)
	if err != nil {
		return err
	}
	utils.Outf("{{green}}created new private key:{{/}} %s\n", name)

	return nil
}

func createKey(ctx context.Context, db *state.SimpleMutable, name string) error {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return err
	}
	ok, err := hasKey(ctx, db, name)
	if ok {
		return fmt.Errorf("%w: %s", ErrDuplicateKeyName, name)
	}
	if err != nil {
		return err
	}
	err = setKey(context.Background(), db, priv, name)
	if err != nil {
		return err
	}

	return db.Commit(ctx)
}

func hasKey(ctx context.Context, db state.Immutable, name string) (bool, error) {
	_, ok, err := getPublicKey(ctx, db, name)
	return ok, err
}
