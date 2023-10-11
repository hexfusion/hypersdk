package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/state"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	maxKeyValuesLimit      = 2048
	defaultRequestKeyLimit = maxKeyValuesLimit
)

var defaultDBConfig = merkledb.Config{
	EvictionBatchSize:         100,
	HistoryLength:             defaultRequestKeyLimit,
	ValueNodeCacheSize:        defaultRequestKeyLimit,
	IntermediateNodeCacheSize: defaultRequestKeyLimit,
	Reg:                       prometheus.NewRegistry(),
	Tracer:                    trace.Noop,
}

func getDB(dbPath string) (*state.SimpleMutable, error) {
	pdb, _, err := pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		return nil, err
	}

	stateDB, err := merkledb.New(context.Background(), pdb, defaultDBConfig)
	if err != nil {
		return nil, err
	}

	return state.NewSimpleMutable(stateDB), nil
}
