// Copyright (c) 2023-2025, Nubificus LTD
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

func newGeneric(plat Platform, rfs Rootfs) *GenericInfo {
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
	case "raw":
		return true
	default:
		return false
	}
}

func (i *GenericInfo) SupportsFsType(string) bool {
	return false
}

func (i *GenericInfo) SupportsMonitor(string) bool {
	return false
}

func (i *GenericInfo) SupportsArch(arch string) bool {
	switch arch {
	case "x86_64", "amd64":
		return true
	case "aarch64":
		return true
	default:
		return false
	}
}

func (i *GenericInfo) CreateRootfs(buildContext string) llb.State {
	local := llb.Local(buildContext)
	switch i.Rootfs.Type {
	case "initrd":
		contentState := FilesLLB(i.Rootfs.Includes, local, llb.Scratch())
		return initrdLLB(contentState)
	case "raw":
		return FilesLLB(i.Rootfs.Includes, local, llb.Scratch())
	default:
		// We should never reach this point
		return llb.Scratch()
	}
}

func (i *GenericInfo) BuildKernel(_ string) llb.State {
	return llb.Scratch()
}
