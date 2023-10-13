// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
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
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const (
	inheritIDKey = "inherit"
)

var (
	parentID ids.ID
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
	MaxMemoryPages uint64 `yaml:"max_memory_pages" json:"maxMemoryPages,omitempty"`
}

type Step struct {
	// Name of the step to perform. Valid values are: create, call.
	Name string `json,yaml:"name"`
	// Description of the step.
	Description string `json,yaml:"description"`
	// Maximum fee to pay for the step.
	MaxFee uint64 `yaml:"max_fee" json:"maxFee"`
	// Path to the program to deploy. Only used with deploy steps.
	ProgramPath string `yaml:"program_path,omitempty" json:"programPath,omitempty"`
	// Name of the key to create program. Only used with create_key steps.
	KeyName string `yaml:"key_name,omitempty" json:"keyName,omitempty"`
	// ID of the program to call. Use `inherit` to use the program ID from the
	// most recent create step.
	ProgramID string `yaml:"program_id,omitempty" json:"programID,omitempty"`
	// Used to override the program caller key.
	CallerKey string `yaml:"caller_key,omitempty" json:"callerKey,omitempty"`
	// Name of the function to call.
	Function string `json,yaml:"function,omitempty"`
	// Params to pass to the function.
	Params []Parameter `json,yaml:"params,omitempty"`
	// Define assertions against the result of this step.
	Require Require `json,yaml:"require,omitempty"`
}

type Require struct {
	// Assertions against the result of the step.
	Result Assertion `json,yaml:"result,omitempty"`
	// Assertions against the fee balance after the step.
	Balance Assertion `json,yaml:"balance,omitempty"`
}

type Assertion struct {
	// Operator is the comparison operator to use.
	Operator string `json,yaml:"operator"`
	// Operand is the value to compare against the result of the step.
	Operand uint64 `json,yaml:"operand"`
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
		deployCmd(),
	)
	return cmd
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run [path]",
		Short: "Run a series of program steps from a run file",
		RunE:  runSteps,
		Args:  cobra.MinimumNArgs(1),
	}

	return cmd
}

func deployCmd() *cobra.Command {
	var keyName string
	cmd := &cobra.Command{
		Use:   "deploy [path] --key [key name]",
		Short: "Deploy a HyperSDK program",
		RunE: func(cmd *cobra.Command, args []string) error {
			exists, err := hasKey(cmd.Context(), db, keyName)
			if !exists {
				return fmt.Errorf("%w: %s", ErrNamedKeyNotFound, keyName)
			}
			if err != nil {
				return err
			}

			programPath := args[0]
			programID, err := deployProgram(cmd.Context(), programPath)
			if err != nil {
				return err
			}

			// only commit to state if the call is successful
			err = db.Commit(cmd.Context())
			if err != nil {
				return err
			}

			utils.Outf("{{green}}deploy transaction successful: {{/}}%s\n", programID.String())
			return nil
		},
		Args: cobra.MinimumNArgs(1),
	}

	cmd.PersistentFlags().StringVarP(&keyName, "key", "k", "", "name of the key to use to deploy the program")
	cmd.MarkPersistentFlagRequired("key")

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

	utils.Outf("{{green}}simulating: {{/}}%s\n\n", p.Name)

	for i, step := range p.Steps {
		utils.Outf("{{yellow}}step: {{/}}%d\n", i)
		if step.Description != "" {
			utils.Outf("{{yellow}}description: {{/}}%s\n", step.Description)
		}
		switch step.Name {
		case "create_key":
			if step.KeyName == "" {
				return fmt.Errorf("%w: %s", ErrKeyNameRequired, step.Name)
			}
			err := newKey(cmd.Context(), db, step.KeyName)
			if errors.Is(err, ErrDuplicateKeyName) {
				utils.Outf("{{yellow}}key already exists: {{/}}%s\n", step.KeyName)
			} else if err != nil {
				return err
			}
			utils.Outf("{{green}}key creation successful: {{/}}%s\n\n", step.KeyName)
		case "deploy":
			if step.ProgramPath == "" {
				return fmt.Errorf("%w: %s", ErrProgramPathRequired, step.Name)
			}
			var err error
			parentID, err = deployProgram(cmd.Context(), step.ProgramPath)
			if err != nil {
				return err
			}
			utils.Outf("{{green}}deploy transaction successful: {{/}}%s\n\n", parentID.String())
		case "call":
			id, resp, balance, err := callProgram(cmd.Context(), &step, &p.Config)
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
			return fmt.Errorf("%w: %s", ErrInvalidStep, step.Name)
		}
	}

	return nil
}

func deployProgram(ctx context.Context, path string) (ids.ID, error) {
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

func callProgram(ctx context.Context, step *Step, config *Config) (ids.ID, uint64, uint64, error) {
	// get program ID from deploy step if set to inherit
	var programID ids.ID
	if step.ProgramID == inheritIDKey {
		programID = parentID
	} else {
		stepID, err := ids.FromString(step.ProgramID)
		if err != nil {
			return ids.Empty, 0, 0, err
		}
		programID = stepID
	}

	// get program bytes from disk
	programBytes, ok, err := getProgram(ctx, db, programID)
	if !ok {
		return ids.Empty, 0, 0, fmt.Errorf("%w: %s", ErrProgramNotFound, programID.String())
	}
	if err != nil {
		return ids.Empty, 0, 0, err
	}

	// simulate call program transaction
	callID, err := generateRandomID()
	if err != nil {
		return ids.Empty, 0, 0, err
	}

	cfg := newConfig(step, config)
	// TODO: handle custom imports
	supported := runtime.NewSupportedImports()
	supported.Register("state", func() runtime.Import {
		return pstate.New(log, db)
	})
	supported.Register("program", func() runtime.Import {
		return program.New(log, db, cfg)
	})

	rt := runtime.New(log, cfg, supported.Imports())
	defer rt.Stop()
	err = rt.Initialize(ctx, programBytes)
	if err != nil {
		return ids.Empty, 0, 0, err
	}

	// get function params
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
			val, ok := param.Value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
			ptr, err := runtime.WriteBytes(memory, []byte(val))
			if err != nil {
				return nil, err
			}
			params = append(params, ptr)
		case "bool":
			val := param.Value.(bool)
			params = append(params, boolToUint64(val))
		case "id":
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
		case "key":
			val, ok := param.Value.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %s", ErrFailedParamTypeCast, param.Type)
			}
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
		case "uint64":
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

// validateAssertion validates the assertion against the actual value. Returns true if the assertion is nil.
func validateAssertion(actual uint64, assertion *Assertion) bool {
	operator := assertion.Operator
	operand := assertion.Operand

	switch Operator(operator) {
	case GreaterThan:
		if actual > operand {
			return true
		}
	case LessThan:
		if actual < operand {
			return true
		}
	case GreaterThanOrEqual:
		if actual >= operand {
			return true
		}
	case LessThanOrEqual:
		if actual <= operand {
			return true
		}
	case EqualTo:
		if actual == operand {
			return true
		}
	}

	return false
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
