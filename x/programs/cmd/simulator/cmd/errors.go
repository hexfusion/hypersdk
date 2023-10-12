// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import "errors"

var (
	ErrMissingSubcommand      = errors.New("missing subcommand")
	ErrProgramNotFound        = errors.New("program not found")
	ErrProgramPathRequired    = errors.New("program path required for this step")
	ErrInvalidStep            = errors.New("invalid step")
	ErrDuplicateKeyName       = errors.New("duplicate key name")
	ErrNamedKeyNotFound       = errors.New("named key not found")
	ErrInvalidConfigFormat    = errors.New("invalid config format")
	ErrResultAssertionFailed  = errors.New("assertion failed expected result")
	ErrBalanceAssertionFailed = errors.New("assertion failed expected balance")
	ErrInvalidParamType       = errors.New("invalid param type")
	ErrFailedParamTypeCast    = errors.New("failed to cast param type")
	ErrKeyNameRequired        = errors.New("key name required for this step")
)