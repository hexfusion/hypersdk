package cmd

import "errors"

var (
	ErrMissingSubcommand   = errors.New("missing subcommand")
	ErrProgramNotFound     = errors.New("program not found")
	ErrProgramPathRequired = errors.New("program path required for this action")
	ErrInvalidAction       = errors.New("invalid action")
	ErrMissingArgument     = errors.New("missing argument")
	ErrDuplicateKeyName    = errors.New("duplicate key name")
	ErrNamedKeyNotFound    = errors.New("named key not found")
)