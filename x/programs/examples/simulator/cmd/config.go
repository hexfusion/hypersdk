// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

func newConfig(action *Action, config *Config) (*runtime.Config, error) {
	return runtime.NewConfigBuilder(action.MaxFee).
		WithEnableTestingOnlyMode(true).
		WithBulkMemory(true).
		WithLimitMaxMemory(config.MaxMemoryPages * runtime.MemoryPageSize).
		Build()
}
