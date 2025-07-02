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
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/moby/buildkit/client/llb"
)

const (
	DefaultKernelPath string = "/.boot/kernel"
	DefaultRootfsPath string = "/.boot/rootfs"
	unikraftHub       string = "unikraft.org"
	uruncJSONPath     string = "/urunc.json"
)

type Platform struct {
	Framework string `yaml:"framework"`
	Version   string `yaml:"version"`
	Monitor   string `yaml:"monitor"`
	Arch      string `yaml:"architecture"`
}

type Rootfs struct {
	From     string   `yaml:"from"`
	Path     string   `yaml:"path"`
	Type     string   `yaml:"type"`
	Includes []string `yaml:"include"`
}

type Kernel struct {
	From string `yaml:"from"`
	Path string `yaml:"path"`
}

type Hops struct {
	Version  string   `yaml:"version"`
	Platform Platform `yaml:"platforms"`
	Rootfs   Rootfs   `yaml:"rootfs"`
	Kernel   Kernel   `yaml:"kernel"`
	Cmd      string   `yaml:"cmdline"`
	Envs     []string `yaml:"envs"`
}

// A struct to represent a copy operation in the final image
type PackCopies struct {
	// The state where the file resides
	SrcState llb.State
	// The source path inside the SrcState where the file resides
	SrcPath string
	// The destination path to copy the file inside the final image
	DstPath string
}

type PackInstructions struct {
	// The Base image to use
	Base llb.State
	// The files to copy inside the final image
	Copies []PackCopies
	// Annotations
	Annots map[string]string
	// Environment variables
	EnvVars []string
}

// ToPack converts Hops into PackInstructions
func ToPack(h *Hops, buildContext string) (*PackInstructions, error) {
	var framework Framework
	instr := &PackInstructions{
		Annots: map[string]string{
			"com.urunc.unikernel.mountRootfs":   "false",
			"com.urunc.unikernel.unikernelType": h.Platform.Framework,
			"com.urunc.unikernel.cmdline":       h.Cmd,
			"com.urunc.unikernel.hypervisor":    h.Platform.Monitor,
		},
		EnvVars: h.Envs,
	}
	if h.Platform.Version != "" {
		instr.Annots["com.urunc.unikernel.unikernelVersion"] = h.Platform.Version
	}

	if h.Kernel.From == "local" {
		var kernelCopy PackCopies

		instr.Base = llb.Scratch()
		instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath

		kernelCopy.SrcState = llb.Local(buildContext)
		kernelCopy.SrcPath = h.Kernel.Path
		kernelCopy.DstPath = DefaultKernelPath
		instr.Copies = append(instr.Copies, kernelCopy)
	} else {
		instr.Base = BaseLLB(h.Kernel.From, h.Platform.Monitor)
		instr.Annots["com.urunc.unikernel.binary"] = h.Kernel.Path
	}

	// Get the framework and call the respective function to create the
	// rootfs.
	switch h.Platform.Framework {
	case unikraftName:
		framework = NewUnikraft(h.Platform, h.Rootfs)
	default:
		framework = NewGeneric(h.Platform, h.Rootfs)
	}

	// Make sure that the specified rootfs type is supported
	// from the framework.
	if h.Rootfs.Type != "" {
		if !framework.SupportsRootfsType(h.Rootfs.Type) {
			return nil, fmt.Errorf("Cannot build %s rootfs for %s",
				h.Rootfs.Type, h.Platform.Framework)
		}
	}

	if len(h.Rootfs.Includes) == 0 {
		// If the path field in rootfs is set and there are no entries
		// in the include field, then we do not need to create or change
		// any rootfs, but simply use the specified file as a rootfs.
		if h.Rootfs.Path != "" {
			var rootfsCopy PackCopies

			switch h.Rootfs.From {
			case "local":
				rootfsCopy.SrcState = llb.Local(buildContext)
			case "scratch":
				return nil, fmt.Errorf("invalid combination of from and path fields in rootfs: path can not be set when from is scratch")
			default:
				rootfsCopy.SrcState = llb.Image(h.Rootfs.From)
			}
			rootfsCopy.SrcPath = h.Rootfs.Path
			rootfsCopy.DstPath = DefaultRootfsPath
			instr.Copies = append(instr.Copies, rootfsCopy)
			if framework.GetRootfsType() == "initrd" {
				instr.Annots["com.urunc.unikernel.initrd"] = DefaultRootfsPath
			}

			return instr, nil
		}

		// If the path field in rootfs is not set and there are np
		// entries in the include field, then there are two scenarios:
		// 1) The rootfs is empty and we do not have to do anything
		// 2) The rootfs is a raw type rootfs
		// The value that will guide us is the From field
		if h.Rootfs.From != "scratch" {
			// We have a raw rootfs
			if !framework.SupportsRootfsType("raw") {
				return nil, fmt.Errorf("%s does not support raw rootfs type", framework.Name())
			}
			instr.Annots["com.urunc.unikernel.mountRootfs"] = "true"
			// Switch the base to the rootfs's From image
			// and copy the kernel inside it.
			if h.Kernel.From != "local" {
				var kernelCopy PackCopies
				kernelCopy.SrcState = instr.Base
				kernelCopy.SrcPath = h.Kernel.Path
				kernelCopy.DstPath = DefaultKernelPath
				instr.Copies = append(instr.Copies, kernelCopy)
				instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
			}
			instr.Base = BaseLLB(h.Rootfs.From, "")
		}

		// If the from and include field of rootfs is empty, we do
		// not need to do anything for the rootfs
		return instr, nil
	}

	// The include field of rootfs is not empty, hence the user wants to
	// create or append the rootfs. Currently only creation is supported.
	// TODO: Support update of an existing rootfs.

	// If the user has not specified a type, then CreateRootfs will build
	// the default rootfs type for the specified framework.
	rootfsState, err := framework.CreateRootfs(buildContext)
	if err != nil {
		return nil, err
	}
	switch framework.GetRootfsType() {
	case "initrd":
		instr.Annots["com.urunc.unikernel.initrd"] = DefaultRootfsPath
	case "raw":
		instr.Annots["com.urunc.unikernel.mountRootfs"] = "true"
	default:
		return nil, fmt.Errorf("Unexpected RootfsType value from framework")
	}

	// Switch the base to the rootfs's From image
	// and copy the kernel inside it.
	if h.Kernel.From != "local" {
		var kernelCopy PackCopies
		kernelCopy.SrcState = instr.Base
		kernelCopy.SrcPath = h.Kernel.Path
		kernelCopy.DstPath = DefaultKernelPath
		instr.Copies = append(instr.Copies, kernelCopy)
		instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
	}
	instr.Base = rootfsState

	return instr, nil
}

// PackLLB gets a PackInstructions struct and transforms it to an LLB definition
func PackLLB(instr PackInstructions) (*llb.Definition, error) {
	var base llb.State
	uruncJSON := make(map[string]string)
	base = instr.Base

	// Create urunc.json file, since annotations do not reach urunc
	for annot, val := range instr.Annots {
		encoded := base64.StdEncoding.EncodeToString([]byte(val))
		uruncJSON[annot] = string(encoded)
	}
	uruncJSONBytes, err := json.Marshal(uruncJSON)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal urunc json: %v", err)
	}

	// Perform any copies inside the image
	for _, aCopy := range instr.Copies {
		base = CopyLLB(base, aCopy)
	}

	// Create the urunc.json file in the rootfs
	base = base.File(llb.Mkfile(uruncJSONPath, 0644, uruncJSONBytes))

	var dt *llb.Definition
	switch runtime.GOARCH {
	case "amd64":
		dt, err = base.Marshal(context.TODO(), llb.LinuxAmd64)
	case "arm":
		dt, err = base.Marshal(context.TODO(), llb.LinuxArm)
	case "arm64":
		dt, err = base.Marshal(context.TODO(), llb.LinuxArm64)
	default:
		return nil, fmt.Errorf("Unsupported architecture: %s", runtime.GOARCH)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal LLB state: %v", err)
	}

	return dt, nil
}
