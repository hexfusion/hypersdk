// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

func newConfig(step *Step, config *Config) *runtime.Config {
	return runtime.NewConfig(step.MaxFee).
		WithEnableTestingOnlyMode(true).
		// TODO: remove when non wasi-preview logging is supported
		// ONLY required for debug logs in testing only mode.
		WithBulkMemory(true).
		WithLimitMaxMemory(config.MaxMemoryPages * runtime.MemoryPageSize)
}
