// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"crypto/rand"
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

var (
	programID ids.ID
)

type Program struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Actions     []Action `yaml:"actions"`
	CallerKey   string   `yaml:"caller_key"`
	Config      Config   `yaml:"config"`
}

type Config struct {
	// Maximum number of pages of memory that can be used.
	// Each page represents 64KiB of memory.
	MaxMemoryPages uint64 `yaml:"max_memory_pages"`
}

type Action struct {
	// Name of the action to perform. Valid values are: create, call.
	Name string `yaml:"name"`
	// Description of the action.
	Description string `yaml:"description"`
	// Maximum fee to pay for the action.
	MaxFee uint64 `yaml:"max_fee"`
	// Path to the program to deploy. Only used with deploy actions.
	ProgramPath string `yaml:"program_path,omitempty"`
	// ID of the program to call. Use `inherit` to use the program ID from the
	// most recent create action.
	ProgramID string `yaml:"program_id,omitempty"`
	// Used to override the program caller key.
	CallerKey string `yaml:"caller_key,omitempty"`
	// Name of the function to call.
	Function string `yaml:"function,omitempty"`
	// Parameters to pass to the function.
	Parameters []Parameter `yaml:"parameters,omitempty"`
}

type Parameter struct {
	Type  string      `yaml:"type"`
	Value interface{} `yaml:"value"`
}

func programCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "program",
		RunE: func(*cobra.Command, []string) error {
			return ErrMissingSubcommand
		},
	}

	cmd.PersistentPreRunE = func(*cobra.Command, []string) (err error) {
		db, err = getDB(dbPath)
		if err != nil {
			return err
		}
		utils.Outf("{{yellow}}database:{{/}} %s\n", dbPath)
		return nil
	}

	// add subcommands
	cmd.AddCommand(
		runCmd(),
	)
	return cmd
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a series of program actions",
		RunE:  runCmdFunc,
	}

	return cmd
}

func runCmdFunc(cmd *cobra.Command, args []string) error {
	configPath := args[0]
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var p Program
	err = yaml.Unmarshal(configBytes, &p)
	if err != nil {
		return err
	}

	utils.Outf("{{green}}simulating: {{/}} %v\n", p.Name)

	for _, action := range p.Actions {
		switch action.Name {
		case "deploy":
			if action.ProgramPath == "" {
				return fmt.Errorf("%w: %s", ErrProgramPathRequired, action.Name)
			}
			id, err := deployProgram(context.Background(), &action)
			if err != nil {
				return err
			}

			utils.Outf("{{green}}deploy transaction successful: {{/}} %v\n", id)
		case "call":
			id, err := callProgram(context.Background(), &action, &p.Config)
			if err != nil {
				return err
			}

			utils.Outf("{{green}}call transaction successful: {{/}} %v\n", id)
		default:
			return fmt.Errorf("%w: %s", ErrInvalidAction, action.Name)
		}
	}

	return nil
}

func deployProgram(ctx context.Context, action *Action) (string, error) {
	programBytes, err := os.ReadFile(action.ProgramPath)
	if err != nil {
		return "", err
	}
	// simulate create program transaction
	programID, err = generateRandomID()
	if err != nil {
		return "", err
	}
	// store the program to disk
	err = storage.SetProgram(ctx, db, programID, programBytes)
	if err != nil {
		return "", err
	}

	return programID.String(), nil
}

func callProgram(ctx context.Context, action *Action, config *Config) (string, error) {
	programID, err := ids.ToID([]byte(action.ProgramID))
	if err != nil {
		return "", err
	}

	// get program bytes from disk
	programBytes, ok, err := storage.GetProgram(ctx, db, programID)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrProgramNotFound, programID)
	}
	if err != nil {
		return "", err
	}

	// simulate call program transaction
	callID, err := generateRandomID()
	if err != nil {
		return "", err
	}

	cfg, err := newConfig(action, config)
	if err != nil {
		return "", err
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
	err = rt.Initialize(ctx, programBytes)
	if err != nil {
		return "", err
	}

	// get function params
	params, err := createParams(ctx, programID, rt.Memory(), db, action.Parameters)
	if err != nil {
		return "", err
	}

	rt.Call(ctx, action.Function, params...)

	// only commit to state if the call is successful
	err = db.Commit(ctx)
	if err != nil {
		return "", err
	}
	return callID.String(), nil
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
