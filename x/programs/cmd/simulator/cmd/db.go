// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/state"
	"github.com/prometheus/client_golang/prometheus"
)

var defaultDBConfig = merkledb.Config{
	BranchFactor:              merkledb.BranchFactor16,
	EvictionBatchSize:         100,
	HistoryLength:             2048,
	ValueNodeCacheSize:        2 * units.GiB,
	IntermediateNodeCacheSize: 2 * units.GiB,
	Reg:                       prometheus.NewRegistry(),
	Tracer:                    trace.Noop,
}

type dBCloserFn func() error

func getStorage(ctx context.Context, dbPath string) (*state.SimpleMutable, dBCloserFn, error) {
	pdb, _, err := pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		return nil, nil, err
	}

	stateDB, err := merkledb.New(ctx, pdb, defaultDBConfig)
	if err != nil {
		return nil, nil, err
	}

	// ensure DBs are closed
	closer := func() error {
		err = stateDB.Close()
		if err != nil {
			return err
		}
		err = pdb.Close()
		if err != nil {
			return err
		}
		return nil
	}

	return state.NewSimpleMutable(stateDB), closer, nil
}
