// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const (
	inheritIDKey = "inherit"
)

var (
	programID ids.ID
)

type Program struct {
	// Name of the program.
	Name string `yaml:"name"`
	// Description of the program.
	Description string `yaml:"description"`
	// Steps to perform against the program.
	Steps []Step `yaml:"steps"`
	// Key of the caller to use for all steps.
	CallerKey string `yaml:"caller_key"`
	// Runtime configuration.
	Config Config `yaml:"config"`
}

type Config struct {
	// Maximum number of pages of memory that can be used.
	// Each page represents 64KiB of memory.
	MaxMemoryPages uint64 `yaml:"max_memory_pages"`
}

type Step struct {
	// Name of the action to perform. Valid values are: create, call.
	Name string `json,yaml:"name"`
	// Description of the action.
	Description string `json,yaml:"description"`
	// Maximum fee to pay for the action.
	MaxFee uint64 `yaml:"max_fee" json:"maxFee"`
	// Path to the program to deploy. Only used with deploy actions.
	ProgramPath string `yaml:"program_path,omitempty" json:"programPath,omitempty"`
	// ID of the program to call. Use `inherit` to use the program ID from the
	// most recent create action.
	ProgramID string `yaml:"program_id,omitempty" json:"programID,omitempty"`
	// Used to override the program caller key.
	CallerKey string `yaml:"caller_key,omitempty" json:"callerKey,omitempty"`
	// Name of the function to call.
	Function string `json,yaml:"function,omitempty"`
	// Params to pass to the function.
	Params []Parameter `json,yaml:"params,omitempty"`
	// Define assertions against the result of this step.
	Require Require `json,yaml:"requires,omitempty"`
}

type Require struct {
	// Assertions against the result of the step.
	Result Assertion `json,yaml:"result,omitempty"`
	// Assertions against the fee balance after the step.
	Balance Assertion `json,yaml:"result,omitempty"`
	// Assertions against the error returned by the step.
	WantError bool `json,yaml:"want_error"`
}

type Assertion struct {
	// Operator is the comparison operator to use.
	Operator string `json,yaml:"operator"`
	// Operand is the value to compare against the result of the step.
	Operand int `json,yaml:"operand"`
}

type Operator string

const (
	GreaterThan        Operator = ">"
	LessThan           Operator = "<"
	GreaterThanOrEqual Operator = ">="
	LessThanOrEqual    Operator = "<="
	EqualTo            Operator = "=="
	NotEqualTo         Operator = "!="
)

type Parameter struct {
	Type  string      `yaml:"type"`
	Value interface{} `yaml:"value"`
}

func newProgramCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "program",
		Short: "Manage programs",
	}

	// add subcommands
	cmd.AddCommand(
		runCmd(),
	)
	return cmd
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [path]",
		Short: "Run a series of program steps from config file",
		RunE:  runSteps,
		Args:  cobra.MinimumNArgs(1),
	}

	return cmd
}

func runSteps(cmd *cobra.Command, args []string) error {
	configPath := args[0]

	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var p Program
	switch {
	case isJSON(string(configBytes)):
		if err := json.Unmarshal(configBytes, &p); err != nil {
			return err
		}
	case isYAML(string(configBytes)):
		if err := yaml.Unmarshal(configBytes, &p); err != nil {
			return err
		}
	default:
		return ErrInvalidConfigFormat
	}

	utils.Outf("{{green}}simulating: {{/}}%s\n", p.Name)

	for _, action := range p.Steps {
		switch action.Name {
		case "deploy":
			if action.ProgramPath == "" {
				return fmt.Errorf("%w: %s", ErrProgramPathRequired, action.Name)
			}
			var err error
			programID, err = deployProgram(cmd.Context(), &action)
			if err != nil {
				return err
			}

			utils.Outf("{{green}}deploy transaction successful: {{/}} %v\n\n", programID.String())
		case "call":
			utils.Outf("{{yellow}}max fee:{{/}} %v\n", action.MaxFee)
			resp, id, err := callProgram(cmd.Context(), &action, &p.Config)
			if err != nil {
				return err
			}

			utils.Outf("{{green}}call transaction successful: {{/}} %s\n", id.String())
			utils.Outf("{{blue}}response: {{/}}%d\n\n", resp)
		default:
			return fmt.Errorf("%w: %s", ErrInvalidStep, action.Name)
		}
	}

	return nil
}

func deployProgram(ctx context.Context, step *Step) (ids.ID, error) {
	programBytes, err := os.ReadFile(step.ProgramPath)
	if err != nil {
		return ids.Empty, err
	}
	// simulate create program transaction
	programID, err = generateRandomID()
	if err != nil {
		return ids.Empty, err
	}
	// store the program to disk
	err = storage.SetProgram(ctx, db, programID, programBytes)
	if err != nil {
		return ids.Empty, err
	}

	return programID, nil
}

func callProgram(ctx context.Context, step *Step, config *Config) (uint64, ids.ID, error) {
	// get program ID from deploy action if set to inherit
	var programIDBytes = make([]byte, 32)
	if step.ProgramID == inheritIDKey {
		copy(programIDBytes, programID[:])
	} else {
		copy(programIDBytes, []byte(step.ProgramID))
	}

	programID, err := ids.ToID(programIDBytes)
	if err != nil {
		return 0, ids.Empty, err
	}

	// get program bytes from disk
	programBytes, ok, err := storage.GetProgram(ctx, db, programID)
	if !ok {
		return 0, ids.Empty, fmt.Errorf("%w: %s", ErrProgramNotFound, programID)
	}
	if err != nil {
		return 0, ids.Empty, err
	}

	// simulate call program transaction
	callID, err := generateRandomID()
	if err != nil {
		return 0, ids.Empty, err
	}

	cfg, err := newConfig(step, config)
	if err != nil {
		return 0, ids.Empty, err
	}

	// TODO: handle custom imports
	supported := runtime.NewSupportedImports()
	supported.Register("state", func() runtime.Import {
		return pstate.New(log, db)
	})
	supported.Register("program", func() runtime.Import {
		return program.New(log, db)
	})

	rt := runtime.New(log, cfg, supported.Imports())
	defer rt.Stop()
	err = rt.Initialize(ctx, programBytes)
	if err != nil {
		return 0, ids.Empty, err
	}

	// get function params
	params, err := createParams(ctx, programID, rt.Memory(), db, step.Params)
	if err != nil {
		return 0, ids.Empty, err
	}

	resp, err := rt.Call(ctx, step.Function, params...)
	if err != nil {
		return 0, ids.Empty, err
	}

	// only commit to state if the call is successful
	err = db.Commit(ctx)
	if err != nil {
		return 0, ids.Empty, err
	}
	utils.Outf("{{yellow}}fee balance: {{/}}%d\n", rt.Meter().GetBalance())
	return resp[0], callID, nil
}

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

// ParseStringParams parses the string params into uint64 which can be passed to the wasm program
func createParams(ctx context.Context, programID ids.ID, memory runtime.Memory, db state.Immutable, p []Parameter) ([]uint64, error) {
	programIDPtr, err := runtime.WriteBytes(memory, programID[:])
	if err != nil {
		return nil, err
	}

	params := []uint64{programIDPtr}
	for _, param := range p {
		switch strings.ToLower(param.Type) {
		case "string":
			val := param.Value.(string)
			ptr, err := runtime.WriteBytes(memory, []byte(val))
			if err != nil {
				return nil, err
			}
			params = append(params, ptr)
		case "bool":
			val := param.Value.(bool)
			params = append(params, boolToUint64(val))
		case "id":
			val := param.Value.(string)
			id, err := ids.ToID([]byte(val))
			if err != nil {
				return nil, err
			}
			ptr, err := runtime.WriteBytes(memory, id[:])
			if err != nil {
				return nil, err
			}
			params = append(params, ptr)
		case "key":
			val := param.Value.(string)
			// get named public key from db
			key, ok, err := getPublicKey(ctx, db, val)
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
		}
	}

	return params, nil
}

func boolToUint64(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}

func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func isYAML(s string) bool {
	var y map[string]interface{}
	return yaml.Unmarshal([]byte(s), &y) == nil
}