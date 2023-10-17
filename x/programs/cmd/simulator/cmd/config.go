// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"encoding/json"

	"github.com/ava-labs/hypersdk/cli"
	"gopkg.in/yaml.v2"
)

type Simulation struct {
	// Name of the simulation.
	Name string `json,yaml:"name"`
	// Description of the simulation.
	Description string `json,yaml:"description"`
	// Steps to performed during simulation.
	Steps []Step `json,yaml:"steps"`
	// Key of the caller to use for all steps.
	CallerKey string `yaml:"caller_key" json:"callerKey"`
	// Runtime configuration.
	Config Config `json,yaml:"config"`
}

type Config struct {
	// Maximum number of pages of memory that can be used.
	// Each page represents 64KiB of memory.
	MaxMemoryPages uint64 `yaml:"max_memory_pages" json:"maxMemoryPages,omitempty"`
}

type Step struct {
	// Task this step performs (required).
	Task Task `json,yaml:"name"`
	// Name of the step (required).
	Name string `json,yaml:"name"`
	// Description of the step (required).
	Description string `json,yaml:"description"`
	// Maximum fee to pay for the step.
	MaxFee uint64 `yaml:"max_fee" json:"maxFee"`
	// Path to the program to create. Only used with the create_program task.
	CallerKey string `yaml:"caller_key,omitempty" json:"callerKey,omitempty"`
	// Name of the function to call.
	// Define required assertions against this step.
	Require Require `json,yaml:"require,omitempty"`
}

type Request struct {
	Method string      `json,yaml:"method"`
	Params []Parameter `json,yaml:"params"`
}

type Response struct {
	Result []byte `json,yaml:"result"`
}

type Call struct {
	// Name of the function to call.
}

type Task string

const (
	CreateProgramTask Task = "create_program"
	CreateKey         Task = "create_key"
	CallTask          Task = "call"
	ActionTask        Task = "action"
)

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
	Name  string      `json,yaml:"name,omitempty"`
	Type  Type        `json,yaml:"type"`
	Value interface{} `json,yaml:"value"`
}

type Type string

const (
	String Type = "string"
	Bool   Type = "bool"
	ID     Type = "id"
	Key    Type = "key"
	Uint64 Type = "uint64"
)

// validateAssertion validates the assertion against the actual value.
func validateAssertion(actual uint64, assertion *Assertion) bool {
	operand := assertion.Operand

	switch Operator(assertion.Operator) {
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
	case NotEqualTo:
		if actual != operand {
			return true
		}
	}

	return false
}

// validateStep validates the simulation step configuration format.
// func validateStep(step *Step) error {
// 	if step.Task == "" {
// 		return fmt.Errorf("%w: %s", ErrConfigMissingRequired, "task")
// 	}
// 	if step.Description == "" {
// 		return fmt.Errorf("%w: %s", ErrConfigMissingRequired, "description")
// 	}

// 	switch step.Task {
// 	case DeployProgram:
// 		if step.ProgramPath == "" {
// 			return fmt.Errorf("%w: %s: program path", ErrConfigMissingRequired, step.Task)
// 		}
// 	case CreateKey:
// 		if step.KeyName == "" {
// 			return fmt.Errorf("%w: %s: key name", ErrConfigMissingRequired, step.Task)
// 		}
// 	case Call:
// 		if step.ProgramID == "" {
// 			return fmt.Errorf("%w: %s: program id", ErrConfigMissingRequired, step.Task)
// 		}
// 		if step.Function == "" {
// 			return fmt.Errorf("%w: %s: function", ErrConfigMissingRequired, step.Task)
// 		}
// 	default:
// 		return fmt.Errorf("%w: %s", ErrInvalidTask, step.Task)
// 	}

// 	return nil
// }

func This() {
	cli.GenerateTransaction()
}

func unmarshalSimulation(bytes []byte) (*Simulation, error) {
	var s Simulation
	switch {
	case isJSON(string(bytes)):
		if err := json.Unmarshal(bytes, &s); err != nil {
			return nil, err
		}
	case isYAML(string(bytes)):
		if err := yaml.Unmarshal(bytes, &s); err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidConfigFormat
	}

	return &s, nil
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
