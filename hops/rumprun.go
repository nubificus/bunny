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
	"path/filepath"

	"github.com/moby/buildkit/client/llb"
)

const (
	rumprunName              = "rumprun"
	defaultRumprunToolsImage = "harbor.nbfc.io/nubificus/bunny/rumprun/tools:latest"
)

type rumprunInfo struct {
	Monitor string
	Arch    string
	Rootfs  Rootfs
	App     App
}

func newRumprun(plat Platform, rfs Rootfs, app App) *rumprunInfo {
	var arch string
	if plat.Arch == "amd64" || plat.Arch == "x86" {
		arch = "x86_64"
	} else {
		arch = "aarch64"
	}
	return &rumprunInfo{
		Monitor: plat.Monitor,
		Arch:    arch,
		Rootfs:  rfs,
		App:     app,
	}
}

func (i *rumprunInfo) Name() string {
	return rumprunName
}

func (i *rumprunInfo) GetRootfsType() string {
	return i.Rootfs.Type
}

func (i *rumprunInfo) SupportsRootfsType(rootfsType string) bool {
	switch rootfsType {
	case "initrd":
		return true
	case "raw":
		return true
	default:
		return false
	}
}

func (i *rumprunInfo) SupportsFsType(string) bool {
	return false
}

func (i *rumprunInfo) SupportsMonitor(string) bool {
	return false
}

func (i *rumprunInfo) SupportsArch(arch string) bool {
	switch arch {
	case "x86_64", "amd64":
		return true
	case "aarch64":
		return true
	default:
		return false
	}
}

func (i *rumprunInfo) CreateRootfs(buildContext string) llb.State {
	local := llb.Local(buildContext)
	return FilesLLB(i.Rootfs.Includes, local, llb.Scratch())
}

func (i *rumprunInfo) BuildKernel(buildContext string) llb.State {
	content := llb.Git(i.App.From, i.App.Branch)
	outDir := "/.boot"
	workDir := "/workdir"
	toolSet := llb.Image(defaultRumprunToolsImage, llb.WithCustomName("Internal:Build rumprun unikernel"))
	var tuple string
	if i.Arch == "aarch64" {
		tuple = "aarch64-rumprun-netbsd"
	} else {
		tuple = "x86_64-rumprun-netbsd"
	}
	buildExec := toolSet.Dir(filepath.Join(workDir, i.App.Name)).
		AddEnv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/rumprun/rumprun-solo5/bin").
		AddEnv("RUMPRUN_TOOLCHAIN_TUPLE", tuple).
		Run(llb.Shlex("make"))
	buildExec.AddMount(workDir, content)
	var bakeCmd string
	if i.Monitor == "hvt" {
		bakeCmd = "rumprun-bake solo5_hvt " + DefaultKernelPath
	} else {
		bakeCmd = "rumprun-bake solo5_spt " + DefaultKernelPath
	}
	bakeState := buildExec.AddMount(filepath.Join(workDir, i.App.Name, "bin"), llb.Scratch())
	bakeExec := toolSet.Dir(workDir).
		AddEnv("PATH", "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/rumprun/rumprun-solo5/bin").
		Run(llb.Shlexf("find . -type f -perm -111 -exec %s {} \\; -quit", bakeCmd), llb.AddMount(workDir, bakeState, llb.Readonly))
	base := llb.Scratch().File(llb.Mkdir(outDir, 0755))
	return base.With(getArtifacts(bakeExec, outDir))
}
