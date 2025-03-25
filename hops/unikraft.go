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
	unikraftName = "unikraft"
)

type UnikraftInfo struct {
	Version string
	Monitor string
	Arch    string
	Rootfs  Rootfs
}

func newUnikraft(plat Platform, rfs Rootfs) *UnikraftInfo {
	if rfs.Type == "" {
		rfs.Type = "initrd"
	}
	return &UnikraftInfo{
		Version: plat.Version,
		Monitor: plat.Monitor,
		Arch:    plat.Arch,
		Rootfs:  rfs,
	}
}

func (i *UnikraftInfo) Name() string {
	return unikraftName
}

func (i *UnikraftInfo) GetRootfsType() string {
	return i.Rootfs.Type
}

func (i *UnikraftInfo) SupportsRootfsType(rootfsType string) bool {
	switch rootfsType {
	case "initrd":
		return true
	default:
		return false
	}
}

func (i *UnikraftInfo) SupportsFsType(string) bool {
	return false
}

func (i *UnikraftInfo) SupportsMonitor(string) bool {
	return false
}

func (i *UnikraftInfo) SupportsArch(arch string) bool {
	switch arch {
	case "x86_64", "amd64":
		return true
	case "aarch64":
		return true
	default:
		return false
	}
}

func (i *UnikraftInfo) CreateRootfs(buildContext string) llb.State {
	// TODO: Add support for any other possible supported rootfs types
	// Currently, by default, we will build a initrd type.
	local := llb.Local(buildContext)
	contentState := FilesLLB(i.Rootfs.Includes, local, llb.Scratch())
	return InitrdLLB(contentState)
}

func (i *UnikraftInfo) BuildKernel(_ string) llb.State {
	return llb.Scratch()
}
