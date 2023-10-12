// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	// keyPrefix that stores pub key -> private key mapping
	keyPrefix = 0x1
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
		RunE:  createKey,
		Args:  cobra.MinimumNArgs(1),
	}
}

func createKey(cmd *cobra.Command, args []string) error {
	name := args[0]
	err := newKey(cmd.Context(), db, name)
	if err != nil {
		return err
	}
	utils.Outf("{{green}}created new private key:{{/}} %s\n", name)

	return nil
}

func newKey(ctx context.Context, db *state.SimpleMutable, name string) error {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return err
	}

	_, ok, err := getPublicKey(ctx, db, name)
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

// gets the public key mapped to the given name.
func getPublicKey(ctx context.Context, db state.Immutable, name string) (ed25519.PublicKey, bool, error) {
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], []byte(name))
	v, err := db.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return ed25519.EmptyPublicKey, false, nil
	}
	if err != nil {
		return ed25519.EmptyPublicKey, false, err
	}
	return ed25519.PublicKey(v), true, nil
}

func setKey(ctx context.Context, db state.Mutable, privateKey ed25519.PrivateKey, name string) error {
	k := make([]byte, 1+ed25519.PublicKeyLen)
	k[0] = keyPrefix
	copy(k[1:], []byte(name))
	return db.Insert(ctx, k, privateKey[:])
}
