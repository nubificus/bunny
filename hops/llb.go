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
	"runtime"
	"strings"

	"github.com/moby/buildkit/client/llb"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	defaultBsdcpioImage string = "harbor.nbfc.io/nubificus/bunny/libarchive:latest"
)

// Create a LLB State that simply copies all the files in the include list inside
// an empty image
func FilesLLB(fileList []string, fromState llb.State, toState llb.State) (llb.State, error) {
	retState := llb.Scratch()
	for i, file := range fileList {
		var aCopy PackCopies

		parts := strings.Split(file, ":")
		aCopy.SrcState = fromState
		aCopy.SrcPath = parts[0]
		// If user did not define destination path, use the same as the source
		aCopy.DstPath = parts[0]
		if len(parts) < 1 || len(parts) > 2 || len(parts[0]) == 0 {
			return llb.Scratch(), fmt.Errorf("Invalid format of the file list to copy")
		}
		if len(parts) == 2 && len(parts[1]) > 0 {
			aCopy.DstPath = parts[1]
		}
		if i == 0 {
			retState = CopyLLB(toState, aCopy)
		} else {
			retState = CopyLLB(retState, aCopy)
		}
	}

	return retState, nil
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

// Set the base image where we will pack the unikernel
func BaseLLB(inputBase string, monitor string) llb.State {
	if monitor == "firecracker" {
		monitor = "fc"
	}
	if inputBase == "scratch" {
		return llb.Scratch()
	}
	if strings.HasPrefix(inputBase, unikraftHub) {
		// Define the platform to qemu/amd64 so we can pull unikraft images
		platform := ocispecs.Platform{
			OS:           monitor,
			Architecture: runtime.GOARCH,
		}
		return llb.Image(inputBase, llb.Platform(platform))
	}
	return llb.Image(inputBase)
}
