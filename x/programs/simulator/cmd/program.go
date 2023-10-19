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

	"github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/simulator/vm/actions"
	"github.com/ava-labs/hypersdk/x/programs/simulator/vm/storage"
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
	db      *state.SimpleMutable
	keyName string
	path    string
	id      ids.ID
}

func newProgramCreateCmd(log logging.Logger, db *state.SimpleMutable) *cobra.Command {
	p := &programCreate{
		db: db,
	}
	cmd := &cobra.Command{
		Use:   "create --path [path] --key [key name]",
		Short: "Create a HyperSDK program transaction",
		Args: cobra.MinimumNArgs(1),
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
	}

	cmd.PersistentFlags().StringVarP(&p.keyName, "key", "k", p.keyName, "name of the key to use to deploy the program")
	cmd.MarkPersistentFlagRequired("key")
	return cmd
}

func (p *programCreate) Init(args []string) error {
	return nil
}

func (p *programCreate) Verify(ctx context.Context) error {
	exists, err := hasKey(ctx, p.db, p.keyName)
	if !exists {
		return fmt.Errorf("%w: %s", ErrNamedKeyNotFound, p.keyName)
	}

	return err
}

func (p *programCreate) Run(ctx context.Context) (err error) {
	p.id, err = programCreateFunc(ctx, p.db, p.path)
	if err != nil {
		return err
	}
	return nil
}

// createProgram simulates a create program transaction and stores the program to disk.
func programCreateFunc(ctx context.Context, db *state.SimpleMutable, path string) (ids.ID, error) {
	programBytes, err := os.ReadFile(path)
	if err != nil {
		return ids.Empty, err
	}

	// simulate create program transaction
	programID, err := generateRandomID()
	if err != nil {
		return ids.Empty, err
	}

	programCreateAction := actions.ProgramCreate{
		Program: programBytes,
	}

	// execute the action
	success, _, _, _, err := programCreateAction.Execute(ctx, nil, db, 0, nil, parentID, false)
	if !success {
		return ids.Empty, fmt.Errorf("program creation failed: %s", err)
	}
	if err != nil {
		return ids.Empty, err
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return ids.Empty, err
	}

	return programID, nil
}

func programExecuteFunc(ctx context.Context, log logging.Logger, db *state.SimpleMutable,  cfg *runtime.Config, programID ids.ID, params []Parameter) (ids.ID, uint64, uint64, error) {
	supported := runtime.NewSupportedImports()
	supported.Register("state", func() runtime.Import {
		return pstate.New(log, db)
	})
	supported.Register("program", func() runtime.Import {
		return program.New(log, db, cfg)
	})

	// create and initialize runtime
	rt := runtime.New(log, cfg, supported.Imports())
	programBytes, err := storage.GetProgram(ctx, db, programID)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}
	
	defer rt.Stop()
	err = rt.Initialize(ctx, programBytes)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}

	// create params from simulation step
	rparams, err := createParams(ctx, rt.Memory(), db, params)
	if err != nil {
		return ids.Empty, 0, 0, err
	}

	// simulate create program transaction
	programTxID, err := generateRandomID()
	if err != nil {
		return ids.Empty, 0, 0, err
	}

	programEecuteAction := actions.ProgramExecute{
		ProgramID: programID.String(),
		Function:  function,
		Params:    rparams,
		MaxFee:    maxFee,
	}

	// execute the action
	success, _, _, _, err := program.Execute(ctx, nil, db, 0, nil, parentID, false)
	if !success {
		return ids.Empty, fmt.Errorf("program creation failed: %s", err)
	}
	if err != nil {
		return ids.Empty, 0, 0, err
	}

	// store program to disk only on success
	err = db.Commit(ctx)
	if err != nil {
		return ids.Empty, 0, 0, err
	}	


	return callID, resp[0], rt.Meter().GetBalance(), nil
}


func createParams(ctx context.Context, memory runtime.Memory, db state.Immutable, p []Parameter) ([]uint64, error) {
	// first param should always the program ID
	params := []uint64{}
	for _, param := range p {
		// Cast the param value to the correct type and for non integer types
		// write the bytes to the guest programs memory.
		switch param.Type {
		case String:
			val, ok := param.Value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			ptr, err := runtime.WriteBytes(memory, []byte(val))
			if err != nil {
				return nil, err
			}
			params = append(params, ptr)
		case Bool:
			val, ok := param.Value.(bool)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			params = append(params, boolToUint64(val))
		case ID:
			val, ok := param.Value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			id, err := ids.FromString(val)
			if err != nil {
				return nil, err
			}
			ptr, err := runtime.WriteBytes(memory, id[:])
			if err != nil {
				return nil, err
			}
			params = append(params, ptr)
		case KeyEd25519:
			val, ok := param.Value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			// get named public key from db
			key, ok, err := storage.GetPublicKey(ctx, db, val)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrNamedKeyNotFound, val)
			}
			if err != nil {
				return nil, err
			}
			ptr, err := runtime.WriteBytes(memory, key[:])
			if err != nil {
				return nil, err
			}
			params = append(params, ptr)
		case Uint64:
			switch v := param.Value.(type) {
			case float64:
				// json unmarshal converts all numbers to float64
				params = append(params, uint64(v))
			case int:
				params = append(params, uint64(v))
			default:
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
		default:
			return nil, fmt.Errorf("%w: %s", ErrInvalidParamType, param.Type)
		}
	}

	return params, nil
}