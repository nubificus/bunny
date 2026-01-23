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
	wasiName = "wasi"
)

type WasiInfo struct {
	Version string
	Monitor string
	Rootfs  Rootfs
	App     App
}

func NewWasi(plat Platform, rfs Rootfs, app App) *WasiInfo {
	return &WasiInfo{
		Version: plat.Version,
		Monitor: plat.Monitor,
		Rootfs:  rfs,
		App:     app,
	}
}

func (i *WasiInfo) Name() string {
	return wasiName
}

func (i *WasiInfo) GetRootfsType() string {
	return ""
}

func (i *WasiInfo) SupportsRootfsType(rootfsType string) bool {
	return false
}

func (i *WasiInfo) SupportsFsType(string) bool {
	return false
}

func (i *WasiInfo) SupportsMonitor(mon string) bool {
	switch mon {
	case "wasmtime":
		return true
	case "wasmedge":
		return true
	default:
		return false
	}
}

func (i *WasiInfo) SupportsArch(_ string) bool {
	return true
}

func (i *WasiInfo) CreateRootfs(buildContext string) (llb.State, error) {
	if i.Rootfs.Type == "raw" {
		return FilesLLB(i.Rootfs.Includes, buildContext, llb.Scratch()), nil
	} else {
		return llb.Scratch(), fmt.Errorf("Unsupported rootfs type %s", i.Rootfs.Type)
	}
}

func (i *WasiInfo) UpdateRootfs(buildContext string) (llb.State, error) {
	base := llb.Image(i.Rootfs.From)
	if i.Rootfs.Type == "raw" {
		return FilesLLB(i.Rootfs.Includes, buildContext, base), nil
	} else {
		return llb.Scratch(), fmt.Errorf("Unsupported rootfs type %s", i.Rootfs.Type)
	}
}

func (i *WasiInfo) BuildKernel(_ string) llb.State {
	return llb.Scratch()
}

func (i *WasiInfo) BuildApp(buildContext string) llb.State {
	switch i.App.Language {
	case "rust":
		return buildRustWasm(i.App, buildContext)
	case "C":
		return buildCWasm(i.App, buildContext)
	default:
		return llb.Scratch()
	}
}
