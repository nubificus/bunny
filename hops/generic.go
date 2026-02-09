// Copyright (c) 2023-2026, Nubificus LTD
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hops

import (
	"fmt"

	"github.com/moby/buildkit/client/llb"
)

const (
	genericName = "generic"
)

type GenericInfo struct {
	Version string
	Monitor string
	Arch    string
	Rootfs  Rootfs
}

func NewGeneric(plat Platform, rfs Rootfs) *GenericInfo {
	if rfs.Type == "" {
		rfs.Type = "raw"
	}
	return &GenericInfo{
		Version: plat.Version,
		Monitor: plat.Monitor,
		Arch:    plat.Arch,
		Rootfs:  rfs,
	}
}

func (i *GenericInfo) Name() string {
	return genericName
}

func (i *GenericInfo) GetRootfsType() string {
	return i.Rootfs.Type
}

func (i *GenericInfo) SupportsRootfsType(rootfsType string) bool {
	switch rootfsType {
	case "initrd":
		return true
	case "block":
		return true
	case "raw":
		return true
	default:
		return false
	}
}

func (i *GenericInfo) SupportsFsType(string) bool {
	return true
}

func (i *GenericInfo) SupportsMonitor(string) bool {
	return true
}

func (i *GenericInfo) SupportsArch(_ string) bool {
	return true
}

func (i *GenericInfo) CreateRootfs(buildContext string) (llb.State, error) {
	local := llb.Local(buildContext)
	switch i.Rootfs.Type {
	case "initrd":
		contentState := FilesLLB(i.Rootfs.Includes, local, llb.Scratch())
		return InitrdLLB(contentState), nil
	case "raw":
		return FilesLLB(i.Rootfs.Includes, local, llb.Scratch()), nil
	default:
		// We should never reach this point
		return llb.Scratch(), fmt.Errorf("Unsupported rootfs type %s", i.Rootfs.Type)
	}
}

func (i *GenericInfo) UpdateRootfs(buildContext string) (llb.State, error) {
	local := llb.Local(buildContext)
	base := llb.Image(i.Rootfs.From)
	switch i.Rootfs.Type {
	case "initrd":
		return llb.Scratch(), fmt.Errorf("Can not update an initrd rootfs")
	case "raw":
		return FilesLLB(i.Rootfs.Includes, local, base), nil
	default:
		// We should never reach this point
		return llb.Scratch(), fmt.Errorf("Unsupported rootfs type %s", i.Rootfs.Type)
	}
}

func (i *GenericInfo) BuildKernel(_ string) llb.State {
	return llb.Scratch()
}
