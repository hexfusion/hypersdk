// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

//go:embed testdata/counter.wasm
var counterProgramBytes []byte

// go test -v -timeout 30s -run ^TestCounterProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestCounterProgram(t *testing.T) {
	require := require.New(t)
	db := newTestDB()
	maxUnits := uint64(50000)

	cfg := runtime.NewConfig(maxUnits).
		WithLimitMaxMemory(18 * runtime.MemoryPageSize) // 18 pages

	// define supported imports
	supported := runtime.NewSupportedImports()
	supported.Register("state", func() runtime.Import {
		return pstate.New(log, db)
	})
	supported.Register("program", func() runtime.Import {
		return program.New(log, db, cfg)
	})

	program := NewCounter(log, counterProgramBytes, db, cfg, supported.Imports())
	require.NoError(program.Run(context.Background()))
}
