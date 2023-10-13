// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
)

const (
	programPrefix = 0x0
	keyPrefix     = 0x1
)

//
// Program
//

func programKey(id ids.ID) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	k[0] = programPrefix
	copy(k[1:], id[:])
	return
}

// [programID] -> [programBytes]
func getProgram(
	ctx context.Context,
	db state.Immutable,
	programID ids.ID,
) (
	[]byte, // program bytes
	bool, // exists
	error,
) {
	k := programKey(programID)
	v, err := db.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// setProgram stores [program] at [programID]
func setProgram(
	ctx context.Context,
	mu state.Mutable,
	programID ids.ID,
	program []byte,
) error {
	k := programKey(programID)
	return mu.Insert(ctx, k, program)
}

//
// Keys
//

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
