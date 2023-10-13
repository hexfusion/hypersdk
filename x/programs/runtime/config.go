// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"fmt"
	"math"

	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/bytecodealliance/wasmtime-go/v13"
)

const (
	defaultMaxWasmStack                 = 256 * 1024 * 1024 // 256 MiB
	defaultWasmThreads                  = false
	defaultFuelMetering                 = true
	defaultWasmMultiMemory              = false
	defaultWasmMemory64                 = false
	defaultLimitMaxMemory               = 18 * 64 * 1024 // 18 pages
	defaultSIMD                         = false
	defaultCompilerStrategy             = wasmtime.StrategyCranelift
	defaultEpochInterruption            = true
	defaultNaNCanonicalization          = "true"
	defaultCraneliftOptLevel            = wasmtime.OptLevelSpeed
	defaultEnableReferenceTypes         = false
	defaultEnableBulkMemory             = false
	defaultProfilingStrategy            = wasmtime.ProfilingStrategyNone
	defaultMultiValue                   = false
	defaultEnableCraneliftDebugVerifier = false
	defaultEnableDebugInfo              = false
	defaultCompileStrategy              = CompileWasm

	defaultLimitMaxTableElements = 4096
	defaultLimitMaxTables        = 1
	defaultLimitMaxInstances     = 32
	defaultLimitMaxMemories      = 1
)

// NewConfig returns a new runtime configuration.
func NewConfig(meterMaxUnits uint64) *Config {
	return &Config{
		meterMaxUnits: meterMaxUnits,
	}
}

type Config struct {
	enableBulkMemory         bool
	enableWasmMultiValue     bool
	enableWasmReferenceTypes bool
	enableWasmSIMD           bool
	enableDefaultCache       bool
	enableTestingOnlyMode    bool

	maxWasmStack      int
	limitMaxMemory    int64
	profilingStrategy wasmtime.ProfilingStrategy
	compileStrategy   EngineCompileStrategy

	err           wrappers.Errs
	meterMaxUnits uint64
}

type config struct {
	engine *wasmtime.Config

	// store limits
	limitMaxMemory        int64
	limitMaxTableElements int64
	limitMaxTables        int64
	// number of instances of this module can be instantiated in parallel
	limitMaxInstances int64
	limitMaxMemories  int64

	testingOnlyMode bool

	meterMaxUnits   uint64
	compileStrategy EngineCompileStrategy
}

// ResetUnits resets the meters max units to 0. This is useful for initializing
// a secondary runtime.
func (c *Config) ResetUnits() *Config {
	c.meterMaxUnits = NoUnits
	return c
}

// WithCompileStrategy defines the EngineCompileStrategy.
// Default is “.
func (c *Config) WithCompileStrategy(strategy EngineCompileStrategy) *Config {
	c.compileStrategy = strategy
	return c
}

// WithMaxWasmStack defines the maximum amount of stack space available for
// executing WebAssembly code.
//
// Default is 256 MiB.
func (c *Config) WithMaxWasmStack(max int) *Config {
	c.maxWasmStack = max
	return c
}

// WithMultiValue enables modules that can return multiple values.
//
// ref. https://github.com/webassembly/multi-value
// Default is false.
func (c *Config) WithMultiValue(enable bool) *Config {
	c.enableWasmMultiValue = enable
	return c
}

// WithBulkMemory enables`memory.copy` instruction, tables and passive data.
//
// ref. https://github.com/WebAssembly/bulk-memory-operations
// Default is false.
func (c *Config) WithBulkMemory(enable bool) *Config {
	c.enableBulkMemory = enable
	return c
}

// WithReferenceTypes Enables the `externref` and `funcref` types as well as
// allowing a module to define multiple tables.
//
// ref. https://github.com/webassembly/reference-types
//
// Note: depends on bulk memory being enabled.
// Default is false.
func (c *Config) WithReferenceTypes(enable bool) *Config {
	c.enableWasmReferenceTypes = enable
	return c
}

// WithSIMD enables SIMD instructions including v128.
//
// ref. https://github.com/webassembly/simd
// Default is false.
func (c *Config) WithSIMD(enable bool) *Config {
	c.enableWasmSIMD = enable
	return c
}

// WithProfilingStrategy defines the profiling strategy used for defining the
// default profiler.
//
// Default is `wasmtime.ProfilingStrategyNone`.
func (c *Config) WithProfilingStrategy(strategy wasmtime.ProfilingStrategy) *Config {
	c.profilingStrategy = strategy
	return c
}

// WithLimitMaxMemory defines the maximum number of pages of memory that can be used.
// Each page represents 64KiB of memory.
//
// Default is 16 pages.
func (c *Config) WithLimitMaxMemory(max uint64) *Config {
	if max > math.MaxInt64 {
		c.err.Add(fmt.Errorf("max memory %d is greater than max int64 %d", max, math.MaxInt64))
	} else {
		c.limitMaxMemory = int64(max)
	}
	return c
}

// WithDefaultCache enables the default caching strategy.
//
// Default is false.
func (c *Config) WithDefaultCache(enabled bool) *Config {
	c.enableDefaultCache = enabled
	return c
}

// WithEnableTestingOnlyMode enables test mode which provides access to
// useful debugging information. This should not be set for a live
// system as it has both performance and security considerations.
//
// Note: This requires Rust programs to be compiled with the  Wasm to be
// compiled with the wasm32-wasi target.
//
// Default is false.
func (c *Config) WithEnableTestingOnlyMode(enabled bool) *Config {
	c.enableTestingOnlyMode = enabled
	return c
}

func (b *Config) build() (config, error) {
	cfg := defaultWasmtimeConfig()

	if b.err.Errored() {
		return config{}, b.err.Err
	}

	if b.enableDefaultCache {
		err := cfg.CacheConfigLoadDefault()
		if err != nil {
			return config{}, err
		}
	}

	if b.limitMaxMemory == 0 {
		b.limitMaxMemory = defaultLimitMaxMemory
	}

	//lint:ignore S1002 explicit check for default value
	if b.enableBulkMemory != defaultEnableBulkMemory {
		cfg.SetWasmBulkMemory(b.enableBulkMemory)
	}

	//lint:ignore S1002 explicit check for default value
	if b.enableWasmMultiValue != defaultMultiValue {
		cfg.SetWasmMultiValue(b.enableWasmMultiValue)
	}

	//lint:ignore S1002 explicit check for default value
	if b.enableWasmReferenceTypes != defaultEnableReferenceTypes { //nolint:ignore S1002
		cfg.SetWasmReferenceTypes(b.enableWasmReferenceTypes)
	}

	//lint:ignore S1002 explicit check for default value
	if b.enableWasmSIMD != defaultSIMD {
		cfg.SetWasmSIMD(b.enableWasmSIMD)
	}

	if b.profilingStrategy != defaultProfilingStrategy {
		cfg.SetProfiler(b.profilingStrategy)
	}

	if b.maxWasmStack > 0 {
		cfg.SetMaxWasmStack(b.maxWasmStack)
	}

	return config{
		// engine config
		engine: cfg,

		// limits
		limitMaxTableElements: defaultLimitMaxTableElements,
		limitMaxMemory:        b.limitMaxMemory,
		limitMaxTables:        defaultLimitMaxTables,
		limitMaxInstances:     defaultLimitMaxInstances,
		limitMaxMemories:      defaultLimitMaxMemories,

		// runtime config
		compileStrategy: b.compileStrategy,
		meterMaxUnits:   b.meterMaxUnits,
		testingOnlyMode: b.enableTestingOnlyMode,
	}, nil
}

func defaultWasmtimeConfig() *wasmtime.Config {
	cfg := wasmtime.NewConfig()

	// non configurable defaults
	cfg.SetCraneliftOptLevel(defaultCraneliftOptLevel)
	cfg.SetConsumeFuel(defaultFuelMetering)
	cfg.SetWasmThreads(defaultWasmThreads)
	cfg.SetWasmMultiMemory(defaultWasmMultiMemory)
	cfg.SetWasmMemory64(defaultWasmMemory64)
	cfg.SetStrategy(defaultCompilerStrategy)
	cfg.SetEpochInterruption(defaultEpochInterruption)
	cfg.SetCraneliftFlag("enable_nan_canonicalization", defaultNaNCanonicalization)

	// TODO: expose these knobs for developers
	cfg.SetCraneliftDebugVerifier(defaultEnableCraneliftDebugVerifier)
	cfg.SetDebugInfo(defaultEnableDebugInfo)

	// configurable defaults
	cfg.SetWasmSIMD(defaultSIMD)
	cfg.SetMaxWasmStack(defaultMaxWasmStack)
	cfg.SetWasmBulkMemory(defaultEnableBulkMemory)
	cfg.SetWasmReferenceTypes(defaultEnableReferenceTypes)
	cfg.SetWasmMultiValue(defaultMultiValue)
	cfg.SetProfiler(defaultProfilingStrategy)
	return cfg
}
