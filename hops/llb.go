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
	"runtime"
	"strings"

	"github.com/moby/buildkit/client/llb"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultBsdcpioImage string = "harbor.nbfc.io/nubificus/bunny/libarchive:latest"
	DefaultC2WASMBuildImage string = "harbor.nbfc.io/nubificus/bunny/tool/wasm/c:latest"
	DefaultRust2WASMBuildImage string = "harbor.nbfc.io/nubificus/bunny/tool/wasm/rust:latest"
	DefaultBuildDir string = "/build"
)

// Create a LLB State that simply copies all the files in the include list inside
// an empty image
func FilesLLB(fileList []FileToInclude, buildContext string, toState llb.State) llb.State {
	retState := llb.Scratch()
	local := llb.Local(buildContext)
	for i, file := range fileList {
		var aCopy PackCopies

		fromState := local
		if file.From != "" && file.From != "local" {
			fromState = llb.Image(file.From)
		}
		aCopy.SrcState = fromState
		aCopy.SrcPath = file.Src
		aCopy.DstPath = file.Dst
		if i == 0 {
			retState = CopyLLB(toState, aCopy)
		} else {
			retState = CopyLLB(retState, aCopy)
		}
	}

	return retState
}

// Create a LLB State that constructs a cpio file with the data in the content
// State
func InitrdLLB(content llb.State) llb.State {
	outDir := "/.boot"
	workDir := "/workdir"
	toolSet := llb.Image(defaultBsdcpioImage, llb.WithCustomName("Internal:Create initrd")).
		File(llb.Mkdir("/tmp", 0755))
	cpioExec := toolSet.Dir(workDir).
		Run(llb.Shlexf("sh -c \"find . -depth -print | tac | bsdcpio -o --format newc > %s\"", DefaultRootfsPath), llb.AddMount(workDir, content, llb.Readonly))
	base := llb.Scratch().File(llb.Mkdir(outDir, 0755))
	return base.With(getArtifacts(cpioExec, outDir))
}

func getArtifacts(exec llb.ExecState, outDir string) llb.StateOption {
	return func(target llb.State) llb.State {
		return exec.AddMount(outDir, target, llb.SourcePath(outDir))
	}
}

func CopyLLB(to llb.State, from PackCopies) llb.State {

	copyState := to.File(llb.Copy(from.SrcState, from.SrcPath, from.DstPath,
		&llb.CopyInfo{CreateDestPath: true}))

	return copyState
}

// Set the source llb state from the sourceRef image and also set
// the appropriate platform for unikraft images.
func GetSourceState(sourceRef string, monitor string) llb.State {
	if monitor == "firecracker" {
		monitor = "fc"
	}
	if sourceRef == "scratch" {
		return llb.Scratch()
	}
	if strings.HasPrefix(sourceRef, unikraftHub) {
		// Define the platform to qemu/amd64 so we can pull unikraft images
		platform := ocispecs.Platform{
			OS:           monitor,
			Architecture: runtime.GOARCH,
		}
		return llb.Image(sourceRef, llb.Platform(platform))
	}

	return llb.Image(sourceRef)
}

func buildRustWasm(appInfo App, buildContext string) llb.State {
	tools := llb.Image(DefaultRust2WASMBuildImage, llb.WithCustomName("Internal:Build Rust in WASM"))
	outDir := "/.boot"
	var sourceState llb.State

	if appInfo.From == "local" {
		sourceState = llb.Local(buildContext)
	} else {
		sourceState = llb.Image(appInfo.From)
	}

	build := tools.Dir(DefaultBuildDir).
		AddEnv("PATH", "/root/.cargo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		Run(llb.Shlex("cargo build --target wasm32-wasip2 --release")).
		AddMount(DefaultBuildDir, sourceState)

	module := tools.Dir(DefaultBuildDir).
	Run(llb.Shlex("sh -c 'find target/wasm32-wasip2/release/ -depth -maxdepth 1 -type f -print0 | xargs -0 file | grep WebAssembly | cut -d: -f1 | xargs -I{} cp \"{}\" /.boot/app'"), llb.AddMount(DefaultBuildDir, build, llb.Readonly))

	outApp := llb.Scratch().File(llb.Mkdir(outDir, 0755))
	return outApp.With(getArtifacts(module, outDir))
}

func buildCWasm(appInfo App, buildContext string) llb.State {
	tools := llb.Image(DefaultC2WASMBuildImage, llb.WithCustomName("Internal:Build C app to WASM"))
	outDir := "/.boot"
	var sourceState llb.State

	if appInfo.From == "local" {
		sourceState = llb.Local(buildContext)
	} else {
		sourceState = llb.Image(appInfo.From)
	}

	build := tools.Dir(DefaultBuildDir).
		AddEnv("PATH", "/opt/wasi-sdk/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		AddEnv("CC", "clang").
		AddEnv("CFLAGS", "--target=wasm32-wasi --sysroot=/opt/wasi-sdk/share/wasi-sysroot").
		Run(llb.Shlex("make")).
		AddMount(DefaultBuildDir, sourceState)

	module := tools.Dir(DefaultBuildDir).
		Run(llb.Shlex("sh -c 'find . -type f -print0 | xargs -0 file | grep WebAssembly | cut -d: -f1 | xargs -I{} cp \"{}\" /.boot/app'"), llb.AddMount(DefaultBuildDir, build, llb.Readonly))

	outApp := llb.Scratch().File(llb.Mkdir(outDir, 0755))
	return outApp.With(getArtifacts(module, outDir))
}
