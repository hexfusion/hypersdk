// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
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
	r.log.Debug("simulation",
		zap.String("plan", r.plan.Name),
	)

	for i, step := range r.plan.Steps {
		r.log.Debug("simulation",
			zap.Int("step", i),
			zap.String("description", step.Description),
		)

		switch step.Endpoint {
		case KeyEndpoint:
			utils.Outf("{{red}} key:{{/}} %d\n", i)
			resp := Response{
				ID: i,
			}
			keyName, ok := step.Params[0].Value.(string)
			if !ok {
				resp.Error = fmt.Sprintf("%w: %s", ErrFailedParamTypeCast, step.Params[0].Type)
				return nil
			}

			err := keyCreateFunc(ctx, r.db, keyName)
			if errors.Is(err, ErrDuplicateKeyName) {
				r.log.Debug("key already exists")
			} else if err != nil {
				resp.Error = err.Error()
				return nil
			}
			resp.Result = Result{
				Msg: fmt.Sprintf("created key %s", keyName),
			}
			resp.Print()
		case ExecuteEndpoint, ReadOnlyEndpoint: // for now the logic is the same for both
			utils.Outf("{{red}} endpoint:{{/}} %s\n", step.Endpoint)
			utils.Outf("{{red}} method:{{/}} %s\n", step.Method)

			switch step.Method {
			case ProgramCreate:
				utils.Outf("{{red program create")
				resp := Response{
					ID: i,
				}
				defer resp.Print()
				// get program path from params
				programPath, ok := step.Params[0].Value.(string)
				if !ok {
					resp.Error = fmt.Sprintf("%x: %s", ErrFailedParamTypeCast.Error(), step.Params[0].Type)
					return nil
				}
				id, err := programCreateFunc(ctx, r.db, programPath)
				if err != nil {
					resp.Error = err.Error()
					return nil
				}
				// create a mapping from the step id to the program id for use
				// during inline program executions.
				r.programMap[fmt.Sprintf("step_%d", i)] = id
				resp.Result = Result{
					ID: id.String(),
				}
				resp.Print()
			default:
				utils.Outf("{{red}} program call default{{/}} %s\n", step.Method)
				resp := Response{
					ID: i,
				}
				defer resp.Print()
				if len(step.Params) < 2 {
					resp.Error = fmt.Sprintf("%s: %s", ErrInvalidStep.Error(), "execute requires at least 2 params")
					return nil
				}

				// get program ID from params
				if step.Params[0].Type != ID {
					resp.Error = fmt.Sprintf("%s: %s", ErrInvalidParamType.Error(), step.Params[0].Type)
					return nil
				}
				idStr, ok := step.Params[0].Value.(string)
				if !ok {
					resp.Error = fmt.Sprintf("%s: %s", ErrFailedParamTypeCast.Error(), step.Params[0].Type)
					return nil
				}
				programID, err := r.getProgramID(idStr)
				if err != nil {
					resp.Error = err.Error()
					return nil
				}

				// maxUnits from params
				if step.Params[1].Type != Uint64 {
					resp.Error = fmt.Sprintf("%s: %s", ErrInvalidParamType.Error(), step.Params[1].Type)
					return nil
				}
				maxUnits, err := intToUint64(step.Params[1].Value)
				if err != nil {
					resp.Error = fmt.Sprintf("failed to convert max_unit to uint64: %s", err.Error())
					return nil
				}

				id, result, err := programExecuteFunc(ctx, r.db, programID, step.Params, step.Method, maxUnits)
				if err != nil {
					resp.Error = err.Error()
					return nil
				}

				if step.Method == ProgramExecute {
					resp.Result = Result{
						ID:      id.String(),
						Balance: result[0],
					}
				} else {
					resp.Result = Result{
						Response: result,
					}
				}
				resp.Print()
			}

		default:
			return fmt.Errorf("%w: %s", ErrInvalidEndpoint, step.Endpoint)
		}
	}
	return nil
}

// getProgramID checks the program map for the synthetic identifier `step_N`
// where N is the step the id was created from execution.
func (r *runCmd) getProgramID(idStr string) (ids.ID, error) {
	if r.programMap[idStr] != ids.Empty {
		programID, ok := r.programMap[idStr]
		if ok {
			return programID, nil
		}
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
