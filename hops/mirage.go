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
	"fmt"

	"github.com/moby/buildkit/client/llb"
)

const (
	mirageName              = "mirage"
	defaultMirageToolsImage = "harbor.nbfc.io/nubificus/bunny/mirage/tools:latest"
)

type mirageInfo struct {
	Monitor string
	Arch    string
	Rootfs  Rootfs
	App     App
}

func NewMirage(plat Platform, rfs Rootfs, app App) *mirageInfo {
	var arch string
	if plat.Arch == "amd64" || plat.Arch == "x86" {
		arch = "x86_64"
	} else {
		arch = "aarch64"
	}
	return &mirageInfo{
		Monitor: plat.Monitor,
		Arch:    arch,
		Rootfs:  rfs,
		App:     app,
	}
}

func (i *mirageInfo) Name() string {
	return mirageName
}

func (i *mirageInfo) GetRootfsType() string {
	return i.Rootfs.Type
}

func (i *mirageInfo) SupportsRootfsType(rootfsType string) bool {
	switch rootfsType {
	case "initrd":
		return false
	case "block":
		return true
	case "raw":
		return true
	default:
		return false
	}
}

func (i *mirageInfo) SupportsFsType(string) bool {
	return false
}

func (i *mirageInfo) SupportsMonitor(string) bool {
	return false
}

func (i *mirageInfo) SupportsArch(arch string) bool {
	switch arch {
	case "x86_64", "amd64":
		return true
	case "aarch64":
		return true
	default:
		return false
	}
}

func (i *mirageInfo) CreateRootfs(buildContext string) (llb.State, error) {
	return llb.Scratch(), fmt.Errorf("Can not create rootfs for Mirage")
}

func (i *mirageInfo) UpdateRootfs(buildContext string) (llb.State, error) {
	return llb.Scratch(), fmt.Errorf("Can not update rootfs for Mirage")
}

func (i *mirageInfo) BuildKernel(buildContext string) llb.State {
	var content llb.State
	if i.App.From == "local" {
		content = llb.Local(buildContext)
	} else {
		content = llb.Git(i.App.From, i.App.Branch)
	}
	outDir := "/.boot"
	workDir := "/workdir"
	toolSet := llb.Image(defaultMirageToolsImage, llb.WithCustomName("Internal:Build Mirage unikernel"))
	workState, _ := FilesLLB([]string{"/:"+"/home/opam" + workDir}, content, toolSet, 1000)
	var envMode string
	if i.Monitor == "qemu" {
		envMode = "virtio"
	} else {
		envMode = i.Monitor
	}

	confExec := workState.Dir("/home/opam"+workDir).
		User("opam").
		AddEnv("CAML_LD_LIBRARY_PATH", "/home/opam/.opam/5.3/lib/stublibs:/home/opam/.opam/5.3/lib/ocaml/stublibs:/home/opam/.opam/5.3/lib/ocaml").
		AddEnv("OCAML_TOPLEVEL_PATH", "/home/opam/.opam/5.3/lib/toplevel").
		AddEnv("OPAMYES", "1").
		AddEnv("OPAMPRECISETRACKING", "1").
		AddEnv("OPAMERRLOGLEN", "0").
		AddEnv("OPAM_SWITCH_PREFIX", "/home/opam/.opam/5.3").
		AddEnv("OPAMCONFIRMLEVEL", "unsafe-yes").
		AddEnv("PATH", "/home/opam/.opam/5.3/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		AddEnv("MODE", envMode).
		Run(llb.Shlexf("mirage configure -t %s", envMode)).
		Root()

	prepExec := confExec.Dir("/home/opam"+workDir).
		User("opam").
		AddEnv("MODE", envMode).
		AddEnv("MIRAGE_EXTRA_REPOS", "opam-overlays:https://github.com/dune-universe/opam-overlays.git#395cbc4acc1f4524853728c5885f32f1cfff281b,mirage-opam-overlays:https://github.com/dune-universe/mirage-opam-overlays.git#797cb363df3ff763c43c8fbec5cd44de2878757e").
		AddEnv("CAML_LD_LIBRARY_PATH", "/home/opam/.opam/5.3/lib/stublibs:/home/opam/.opam/5.3/lib/ocaml/stublibs:/home/opam/.opam/5.3/lib/ocaml").
		AddEnv("OCAML_TOPLEVEL_PATH", "/home/opam/.opam/5.3/lib/toplevel").
		AddEnv("OPAMYES", "1").
		AddEnv("OPAMPRECISETRACKING", "1").
		AddEnv("OPAMERRLOGLEN", "0").
		AddEnv("OPAM_SWITCH_PREFIX", "/home/opam/.opam/5.3").
		AddEnv("OPAMCONFIRMLEVEL", "unsafe-yes").
		AddEnv("PATH", "/home/opam/.opam/5.3/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		Run(llb.Shlexf("make lock")).
		Run(llb.Shlexf("make depends")).
		Run(llb.Shlexf("make pull")).
		Root()

	buildExec := prepExec.Dir("/home/opam"+workDir).
		User("opam").
		AddEnv("MODE", envMode).
		AddEnv("MIRAGE_EXTRA_REPOS", "opam-overlays:https://github.com/dune-universe/opam-overlays.git#395cbc4acc1f4524853728c5885f32f1cfff281b,mirage-opam-overlays:https://github.com/dune-universe/mirage-opam-overlays.git#797cb363df3ff763c43c8fbec5cd44de2878757e").
		AddEnv("CAML_LD_LIBRARY_PATH", "/home/opam/.opam/5.3/lib/stublibs:/home/opam/.opam/5.3/lib/ocaml/stublibs:/home/opam/.opam/5.3/lib/ocaml").
		AddEnv("OCAML_TOPLEVEL_PATH", "/home/opam/.opam/5.3/lib/toplevel").
		AddEnv("OPAMYES", "1").
		AddEnv("OPAMPRECISETRACKING", "1").
		AddEnv("OPAMERRLOGLEN", "0").
		AddEnv("OPAM_SWITCH_PREFIX", "/home/opam/.opam/5.3").
		AddEnv("OPAMCONFIRMLEVEL", "unsafe-yes").
		AddEnv("PATH", "/home/opam/.opam/5.3/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		AddEnv("DUNE_CACHE", "enabled").
		AddEnv("DUNE_CACHE_TRANSPORT", "direct").
		Run(llb.Shlexf("make build")).
		Root()

	outExec := buildExec.Dir("/home/opam"+workDir).
		User("root").
		Run(llb.Shlexf("find dist -type f -perm -111 -exec cp {} /.boot/kernel \\; -quit"))

	base := llb.Scratch().File(llb.Mkdir(outDir, 0755))
	return base.With(getArtifacts(outExec, outDir))
}
