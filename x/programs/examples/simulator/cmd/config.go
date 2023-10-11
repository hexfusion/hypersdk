package cmd

import "github.com/ava-labs/hypersdk/x/programs/runtime"

func newConfig(action *Action, config *Config) (*runtime.Config, error) {
	return runtime.NewConfigBuilder(action.MaxFee).
		WithEnableTestingOnlyMode(true).
		WithBulkMemory(true).
		WithLimitMaxMemory(config.MaxMemoryPages * runtime.MemoryPageSize).
		Build()
}
