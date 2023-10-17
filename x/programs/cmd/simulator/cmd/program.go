// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

func newProgramCmd(log logging.Logger, db *state.SimpleMutable) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "program",
		Short: "Manage HyperSDK programs",
	}

	// add subcommands
	cmd.AddCommand(
		newProgramCreateCmd(log, db),
	)
	return cmd
}

type programCreate struct {
	db       *state.SimpleMutable
	dbCloser dBCloserFn
	keyName  string
	path     string
	id       ids.ID
}

func newProgramCreateCmd(log logging.Logger, db *state.SimpleMutable) *cobra.Command {
	p := &programCreate{}
	cmd := &cobra.Command{
		Use:   "create [path] --key [key name]",
		Short: "Create a HyperSDK program transaction",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := p.Init(args)
			if err != nil {
				return err
			}
			err = p.Verify(cmd.Context())
			if err != nil {
				return err
			}
			err = p.Run(cmd.Context())
			if err != nil {
				return err
			}

			utils.Outf("{{green}}deploy transaction successful: {{/}}%s\n", p.id.String())
			return nil
		},
		Args: cobra.MinimumNArgs(1),
	}

	cmd.PersistentFlags().StringVarP(&p.keyName, "key", "k", "", "name of the key to use to deploy the program")
	cmd.MarkPersistentFlagRequired("key")

	return cmd
}

func (p *programCreate) Init(args []string) error {
	p.path = args[0]
	return nil
}

func (p *programCreate) Verify(ctx context.Context) error {
	// TODO: use key
	exists, err := hasKey(ctx, p.db, p.keyName)
	if !exists {
		return fmt.Errorf("%w: %s", ErrNamedKeyNotFound, p.keyName)
	}

	return err
}

func (p *programCreate) Run(ctx context.Context) (err error) {
	defer p.dbCloser()
	p.id, err = createProgram(ctx, p.path)
	if err != nil {
		return err
	}

	// only commit to state if the call is successful
	return p.db.Commit(ctx)
}

// createProgram simulates a create program transaction and stores the program to disk.
func createProgram(ctx context.Context, db *state.SimpleMutable, path string) (ids.ID, error) {
	programBytes, err := os.ReadFile(path)
	if err != nil {
		return ids.Empty, err
	}
	// simulate create program transaction
	programID, err := generateRandomID()
	if err != nil {
		return ids.Empty, err
	}
	// store the program to disk
	err = setProgram(ctx, db, programID, programBytes)
	if err != nil {
		return ids.Empty, err
	}

	return programID, nil
}

func callProgram(ctx context.Context, programID ids.ID, stepParams []Parameter) (ids.ID, uint64, uint64, error) {

	cfg := runtime.NewConfig(step.MaxFee).
		WithEnableTestingOnlyMode(true).
		// TODO: remove when non wasi-preview logging is supported
		// ONLY required for debug logs in testing only mode.
		WithBulkMemory(true).
		WithLimitMaxMemory(config.MaxMemoryPages * runtime.MemoryPageSize)

	// create params from simulation step
	params, err := createParams(ctx, programID, rt.Memory(), db, step.Params)
	if err != nil {
		return ids.Empty, 0, 0, err
	}

	resp, err := rt.Call(ctx, step.Function, params...)
	if err != nil {
		return ids.Empty, 0, 0, err
	}

	// only commit to state if the call is successful
	err = db.Commit(ctx)
	if err != nil {
		return ids.Empty, 0, 0, err
	}

	return callID, resp[0], rt.Meter().GetBalance(), nil
}
