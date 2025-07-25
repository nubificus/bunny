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
}

type PackEntry struct {
	SourceState llb.State // the state where the files live
	SourceRef   string    // the reference of the state
	FilePath    string    // path to the file within the state
}

func handleKernel(_ Framework, buildContext string, mon string, k Kernel) (*PackEntry, error) {
	var entry *PackEntry
	entry.SourceRef = k.From
	if k.From == "local" {
		entry.SourceState = llb.Local(buildContext)
	} else {
		entry.SourceState = GetSourceState(k.From, mon)
	}
	entry.FilePath = k.Path
	
	return entry, nil
}

func handleRootfs(f Framework, buildContext string, mon string, r Rootfs) (*PackEntry, error) {
	var entry *PackEntry

	// Make sure that the specified rootfs type is supported
	// from the framework.
	if r.Type != "" {
		if !f.SupportsRootfsType(r.Type) {
			return nil, fmt.Errorf("Cannot build %s rootfs for %s",
				r.Type, f.Name())
		}
	}

	if r.From != "scratch" && r.From != "" {
		// We do not need to build the rootfs.
		// We will simply get it from somewhere else
		entry.SourceRef = r.From
		if r.From == "local" {
			entry.SourceState = llb.Local(buildContext)
		} else {
			entry.SourceState = GetSourceState(r.From, mon)
		}
		// TODO: Be aware of the case r.Path is empty, which means we have a
		// raw rootfs that we reuse.
		entry.FilePath = r.Path

		// TODO: Handle cases where we append files in a rootfs
		return entry, nil
	}
	// The from field of rootfs is scratch or empty, hence we need to create
	// a rootfs or just here is no rootfs entry.
	if len(r.Includes) != 0 {
		// If the user has not specified a type, then CreateRootfs
		// will build the default rootfs type for the specified framework.
		var err error
		entry.SourceRef = "scratch"
		entry.SourceState, err = f.CreateRootfs(buildContext)
		if err != nil {
			return nil, fmt.Errorf("Could not create rootfs: %v", err)
		}
		if f.GetRootfsType() != "raw" {
			entry.FilePath = DefaultRootfsPath
		} else {
			entry.FilePath = ""
		}
	}

	return entry, nil
}

func makeCopy(entry PackEntry, dst string) PackCopies {
	return PackCopies{
		SrcState: entry.SourceState,
		SrcPath:  entry.FilePath,
		DstPath:  dst,
	}
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
	}
	if h.Platform.Version != "" {
		instr.Annots["com.urunc.unikernel.unikernelVersion"] = h.Platform.Version
	}

	// Get the framework and call the respective function to create the
	// rootfs.
	switch h.Platform.Framework {
	case unikraftName:
		framework = NewUnikraft(h.Platform, h.Rootfs)
	default:
		framework = NewGeneric(h.Platform, h.Rootfs)
	}

	kernelEntry, err := handleKernel(framework, buildContext, h.Platform. Monitor, h.Kernel)
	if err != nil {
		return nil, fmt.Errorf("Error handling kernel entry: %v", err)
	}

	rootfsEntry, err := handleKernel(framework, buildContext, h.Platform. Monitor, h.Kernel)
	if err != nil {
		return nil, fmt.Errorf("Error handling rootfs entry: %v", err)
	}

	// Select which state (kernel or rootfs) will serve as the base for
	// the final image. The goal is to merge both with minimal file copies.
	// Typically, we prefer to use the state that already contains one or more
	// of the required files (i.e., when fetched remotely) to avoid unnecessary
	// copying.
	//
	// When both kernel and rootfs are from remote sources,
	// we default to using the kernel as the base to preserve its image configuration.
	//
	// However, if the rootfs is of type "raw", we instead use it as the base,
	// since doing so minimizes copies in that scenario.

	kernelCopy := false
	switch kernelEntry.SourceRef {
	case "":
		return nil, fmt.Errorf("Source of kernel State is empty")
	case "local":
		instr.Copies = append(instr.Copies,
			makeCopy(*kernelEntry, DefaultKernelPath))
		instr.Base = llb.Scratch()
		kernelCopy = true
	default:
		instr.Base = rootfsEntry.SourceState
	}

	rootfsCopy := false
	switch rootfsEntry.SourceRef {
	case "":
		// The rootfs entry is empty, no need to do anything
	case "local":
		instr.Copies = append(instr.Copies,
			makeCopy(*rootfsEntry, DefaultRootfsPath))
		rootfsCopy = true
	default:
		instr.Base = rootfsEntry.SourceState
	}

	if !rootfsCopy && !kernelCopy {
		instr.Copies = append(instr.Copies,
			makeCopy(*kernelEntry, DefaultKernelPath))
		kernelCopy = true
	}

	if kernelCopy {
		// We had to copy the kernel and hence the path will
		// always be DefaultKernelPath
		instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
	} else {
		// We did not have to copy the kernel
		instr.Annots["com.urunc.unikernel.binary"] = kernelEntry.FilePath
	}

	switch framework.GetRootfsType() {
	case "initrd":
		if rootfsCopy {
			instr.Annots["com.urunc.unikernel.initrd"] = DefaultRootfsPath
		} else {
			instr.Annots["com.urunc.unikernel.initrd"] = rootfsEntry.FilePath
		}
	case "raw":
		instr.Annots["com.urunc.unikernel.mountRootfs"] = "true"
	default:
		return nil, fmt.Errorf("Unexpected RootfsType value %s", framework.GetRootfsType())
	}

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
