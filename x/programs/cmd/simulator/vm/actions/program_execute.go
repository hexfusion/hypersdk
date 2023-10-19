// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"

	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/simulator/vm/storage"
)

var _ chain.Action = (*ProgramExecute)(nil)

type ProgramExecute struct {
	ProgramID string `json:"programId"`
	Function string `json:"programFunction"`
	MaxFee	uint64 `json:"maxFee"`
	Params []uint64 `json:"arguments"`

	runtime runtime.Runtime
}

func (*ProgramExecute) GetTypeID() uint8 {
	return programCreateID
}

func (t *ProgramExecute) StateKeys(rauth chain.Auth, id ids.ID) []string {
	return []string{string(storage.ProgramKey(id))}
}

func (*ProgramExecute) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.ProgramChunks}
}

func (*ProgramExecute) OutputsWarpMessage() bool {
	return false
}

func (t *ProgramExecute) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	_ chain.Auth,
	id ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	if len(t.ProgramID) == 0 {
		return false, 1, OutputValueZero, nil, nil
	}
	if len(t.Function) == 0 {
		return false, 1, OutputValueZero, nil, nil
	}

	// TODO: take fee out of balance?
	programID, err := ids.FromString(t.ProgramID)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}	

	programBytes, err := storage.GetProgram(ctx, mu, programID)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}
	
	err = t.runtime.Initialize(ctx, programBytes, t.MaxFee)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}
	defer t.runtime.Stop()

	resp, err := t.runtime.Call(ctx, t.Function, t.Params...)
	if err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}

	return true, resp[0], nil, nil, nil
}

func (*ProgramExecute) MaxComputeUnits(chain.Rules) uint64 {
	return ProgramExecuteComputeUnits
}

func (*ProgramExecute) Size() int {
	return ed25519.PublicKeyLen + consts.Uint64Len
}

func (t *ProgramExecute) Marshal(p *codec.Packer) {
	p.PackString(t.ProgramID)
	p.PackString(t.Function)
	p.PackUint64(t.MaxFee)
	p.PackUint64(uint64(len(t.Params)))
	for _, param := range t.Params {
		p.PackUint64(param)
	}
}

func UnmarshalProgramExecute(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var pe ProgramExecute
	pe.ProgramID = p.UnpackString(true)
	pe.Function = p.UnpackString(true)
	pe.MaxFee = p.UnpackUint64(true)
	paramLen := p.UnpackUint64(true)
	pe.Params = make([]uint64, paramLen)
	for i := uint64(0); i < paramLen; i++ {
		pe.Params[i] = p.UnpackUint64(true)
	}
	return &pe, p.Err()
}

func (*ProgramExecute) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
