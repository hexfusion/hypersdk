// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/runtime"

	"github.com/ava-labs/hypersdk/x/programs/simulator/vm/rpc"
	"github.com/ava-labs/hypersdk/x/programs/simulator/vm/storage"
)

const ()

var (
	parentID ids.ID
)

type runCmd struct {
	plan *Plan
	log  logging.Logger
	db   *state.SimpleMutable

	// tracks program IDs created during this simulation
	programMap  map[string]ids.ID
	stdinReader io.Reader
}

func newRunCmd(log logging.Logger, db *state.SimpleMutable) *cobra.Command {
	r := &runCmd{
		log:        log,
		db:         db,
		programMap: make(map[string]ids.ID),
	}
	cmd := &cobra.Command{
		Use:   "run [path]",
		Short: "Run a HyperSDK program simulation plan",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// if the first argument is "-" read from stdin
			r.stdinReader = cmd.InOrStdin()
			err := r.Init(args)
			if err != nil {
				return err
			}
			err = r.Verify()
			if err != nil {
				return err
			}
			return r.Run(cmd.Context())
		},
	}

	return cmd
}

func (r *runCmd) Init(args []string) (err error) {
	var planBytes []byte
	if args[0] == "-" {
		// read simulation plan from stdin
		reader := bufio.NewReader(os.Stdin)
		planStr, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		planBytes = []byte(planStr)
	} else {
		// read simulation plan from arg[0]
		planBytes, err = os.ReadFile(args[0])
		if err != nil {
			return err
		}
	}

	r.plan, err = unmarshalPlan(planBytes)
	if err != nil {
		return err
	}

	if r.plan.Steps == nil {
		return fmt.Errorf("%w: %s", ErrInvalidPlan, "no steps found")
	}

	if r.plan.Steps[0].Params == nil {
		return fmt.Errorf("%w: %s", ErrInvalidStep, "no params found")
	}

	return nil
}

func (r *runCmd) Verify() error {
	return nil
}

func (r *runCmd) Run(ctx context.Context) error {
	utils.Outf("{{green}}simulating: {{/}}%s\n\n", o.plan.Name)

	for i, step := range r.plan.Steps {
		utils.Outf("{{yellow}}step: {{/}}%d\n", i)
		utils.Outf("{{yellow}}description: {{/}}%s\n", step.Description)

		switch step.Endpoint {
		case KeyEndpoint:
			keyName, ok := step.Params[0].Value.(string)
			if !ok {
				return fmt.Errorf("%w: %s", ErrFailedParamTypeCast, step.Params[0].Type)
			}

			err := createKey(ctx, r.db, keyName)
			if errors.Is(err, ErrDuplicateKeyName) {
				r.log.Debug("key already exists")
			} else if err != nil {
				return err
			}
			utils.Outf("{{green}}key creation successful: {{/}}%s\n\n", keyName)
		case ExecuteEndpoint:
			switch step.Method {
			case ProgramCreate:
				// get program path from params
				programPath, ok := step.Params[0].Value.(string)
				if !ok {
					return fmt.Errorf("%w: %s", ErrFailedParamTypeCast, step.Params[0].Type)
				}
				id, err := programCreateFunc(ctx, r.db, programPath)
				if err != nil {
					return err
				}
				// create a mapping from the step id to the program id for use
				// during inline program executions.
				r.programMap[fmt.Sprintf("step_%d", i)] = id
			case ProgramExecute:
				if step.Params[0].Type != ID {
					return fmt.Errorf("%w: %s", ErrInvalidParamType, step.Params[0].Type)
				}
				idStr, ok := step.Params[0].Value.(string)
				if !ok {
					return fmt.Errorf("%w: %s", ErrFailedParamTypeCast, step.Params[0].Type)
				}
				// get program ID from params
				programID, err := r.getProgramID(idStr)
				if err != nil {
					return err
				}

				programExecuteFunc(ctx, r.db, programID, &step)
			}
			utils.Outf("{{green}}%s transaction successful: {{/}}%s\n\n", parentID.String())
		case Call:
			// get program ID from step
			programID, err := r.getProgramID(&step)
			if err != nil {
				return err
			}

			supported := runtime.NewSupportedImports()
			supported.Register("state", func() runtime.Import {
				return pstate.New(r.log, r.db)
			})
			supported.Register("program", func() runtime.Import {
				return program.New(r.log, r.db, cfg)
			})

			// create and initialize runtime
			rt := runtime.New(log, cfg, supported.Imports())
			defer rt.Stop()
			err = rt.Initialize(ctx, programBytes)
			if err != nil {
				return ids.Empty, 0, 0, err
			}

			// get program from db
			programBytes, err := getProgram(ctx, r.db, programID)
			if err != nil {
				return err
			}

			id, resp, balance, err := callProgram(ctx, programID, programBytes, &step.Params)
			if err != nil {
				return err
			}

			// simulate call program transaction
			txID, err := generateRandomID()
			if err != nil {
				return err
			}

			utils.Outf("{{yellow}}function: {{/}}%s\n", step.Function)
			utils.Outf("{{yellow}}params: {{/}}%v\n", step.Params)
			utils.Outf("{{yellow}}max fee: {{/}}%v\n", step.MaxFee)
			if step.Require.Result != (Assertion{}) {
				if !validateAssertion(resp, &step.Require.Result) {
					return fmt.Errorf("%w: %d %s %d", ErrResultAssertionFailed, resp, step.Require.Result.Operator, step.Require.Result.Operand)
				}
			}
			utils.Outf("{{yellow}}fee balance: {{/}}%d\n", balance)
			if step.Require.Balance != (Assertion{}) {
				if !validateAssertion(balance, &step.Require.Balance) {
					return fmt.Errorf("%w: %d %s %d", ErrBalanceAssertionFailed, balance, step.Require.Balance.Operator, step.Require.Balance.Operand)
				}
			}
			utils.Outf("{{blue}}response: {{/}}%d\n", resp)
			utils.Outf("{{green}}call transaction successful: {{/}}%s\n\n", id.String())
		default:
			return fmt.Errorf("%w: %s", ErrInvalidStep, step.Task)
		}
	}

	return nil
}

// getProgramID checks the program map for the synthetic identifier `step_N`
// where N is the step the id was created from execution.
func (r *runCmd) getProgramID(idStr string) (ids.ID, error) {
	if r.programMap[idStr] != ids.Empty {
		programID, _ := r.programMap[idStr]
		return programID, nil
	}

	return ids.FromString(idStr)
}

// generateRandomID creates a unique ID.
// Note: ids.GenerateID() is not used because the IDs are not unique and will
// collide.
func generateRandomID() (ids.ID, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return ids.Empty, err
	}
	id, err := ids.ToID(key)
	if err != nil {
		return ids.Empty, err
	}

	return id, nil
}


