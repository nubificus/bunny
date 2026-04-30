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
	"context"
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/dockerfile2llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	"gopkg.in/yaml.v3"
)

const (
	defaultUrunitImage string = "harbor.nbfc.io/nubificus/urunit:latest"
	defaultUrunitPath  string = "/urunit"
	defaultKernelImage string = "harbor.nbfc.io/nubificus/urunc/linux-kernel-qemu:latest"
)

func (f *FileToInclude) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		parts := strings.SplitN(node.Value, ":", 2)
		if len(parts[0]) == 0 {
			return fmt.Errorf("invalid file mapping %q, src is empty", node.Value)
		}

		f.From = "local"
		f.Src = parts[0]
		f.Dst = parts[0]
		if len(parts) == 2 && len(parts[1]) != 0 {
			f.Dst = parts[1]
		}

		return nil
	case yaml.MappingNode:
		type auxInclude FileToInclude
		var tmp auxInclude

		err := node.Decode(&tmp)
		if err != nil {
			return err
		}
		if len(tmp.Src) == 0 {
			return fmt.Errorf("invalid file mapping at line %d, column %d: source is empty (from=%q, destination=%q)", node.Line, node.Column, tmp.From, tmp.Dst)
		}
		if len(tmp.Dst) == 0 {
			return fmt.Errorf("invalid file mapping at line %d, column %d: destination is empty (from=%q, source=%q)", node.Line, node.Column, tmp.From, tmp.Src)
		}

		f.From = tmp.From
		if len(tmp.From) == 0 {
			f.From = "local"
		}
		f.Src = tmp.Src
		f.Dst = tmp.Dst
		return nil
	default:
		return fmt.Errorf("invalid Include file format")
	}
}

// ParseBunnyfile reads a yaml file which contains instructions for
// bunny.
func ParseBunnyfile(fileBytes []byte) (*Hops, error) {
	bunnyHops := &Hops{}

	err := yaml.Unmarshal(fileBytes, bunnyHops)
	if err != nil {
		return nil, err
	}

	err = CheckBunnyfileVersion(bunnyHops.Version)
	if err != nil {
		return nil, err
	}

	err = ValidatePlatform(bunnyHops.Platform)
	if err != nil {
		return nil, err
	}

	err = ValidateKernel(bunnyHops.Kernel)
	if err != nil {
		return nil, err
	}

	// Set default value of from to scratch
	// Make sure that any reference to Rootfs.From can not be an empty string
	if bunnyHops.Rootfs.From == "" {
		bunnyHops.Rootfs.From = "scratch"
	}
	err = ValidateRootfs(bunnyHops.Rootfs)
	if err != nil {
		return nil, err
	}

	// TODO: Remove this in next release.
	// Keep backwards compatibility and if cmd is empty, then
	// use cmdline. Otherwise, the Cmdline is ignored.
	if len(bunnyHops.Cmd) == 0 && bunnyHops.Cmdline != "" {
		bunnyHops.Cmd = strings.Split(bunnyHops.Cmdline, " ")
	}

	return bunnyHops, nil
}

// ParseFile tries to first parse the given file using dockerfile2LLB.
// If that fails, then it attempts to read it using the bunnyfile format.
func ParseFile(ctx context.Context, fileBytes []byte, buildContext string, c client.Client) (*PackInstructions, error) {
	// Try to parse the file with dockerfile2LLB
	state, img, _, _, derr := dockerfile2llb.Dockerfile2LLB(ctx, fileBytes, dockerfile2llb.ConvertOpt{
		MetaResolver: c,
	})
	if derr != nil {
		// Could not parse Containerfile-like syntax file.
		// Try bunnyfile syntax.
		hops, err := ParseBunnyfile(fileBytes)
		if err != nil {
			// TODO: Handle which error should be shown
			// For contianerfile or bunnyfile?
			return nil, fmt.Errorf("failed to parse inout file: %w", err)
		}

		packInst, err := ToPack(hops, buildContext)
		if err != nil {
			return nil, fmt.Errorf("failed to convert hops to pack instructions: %w", err)
		}

		// Get the OCI Image config of the base Image if there is any
		baseConfig, err := getBaseConfig(ctx, c, packInst.BaseRef, packInst.Annots["com.urunc.unikernel.hypervisor"])
		if err != nil {
			return nil, fmt.Errorf("Failed to get OCI config of base image %s: %w", packInst.BaseRef, err)
		}

		if len(packInst.Img.Config.Cmd) > 0 {
			baseConfig.Cmd = packInst.Img.Config.Cmd
		}
		if len(packInst.Img.Config.Entrypoint) > 0 {
			baseConfig.Entrypoint = packInst.Img.Config.Entrypoint
		}
		baseConfig.Env = append(baseConfig.Env, packInst.Img.Config.Env...)
		packInst.Img.Config = baseConfig

		// Get the OCI Image config of the base Image if there is any
		packInst.Img = updateImage(packInst.Img, packInst.Annots)

		return packInst, nil
	}

	instr := new(PackInstructions)
	instr.Base = *state
	instr.Img = img.Image
	instr.Img.Config = img.Config.ImageConfig
	instr.Annots = make(map[string]string)
	for k, v := range img.Config.Labels {
		instr.Annots[k] = v
	}

	// Set default annotations if they are not set
	if instr.Annots["com.urunc.unikernel.unikernelType"] == "" {
		instr.Annots["com.urunc.unikernel.unikernelType"] = "linux"
	}
	if instr.Annots["com.urunc.unikernel.unikernelType"] == "linux" &&
		instr.Annots["bunny.urunit"] != "false" {
		var aCopy PackCopies

		aCopy.SrcState = llb.Image(defaultUrunitImage)
		aCopy.SrcPath = defaultUrunitPath
		aCopy.DstPath = defaultUrunitPath
		instr.Copies = append(instr.Copies, aCopy)
		instr.Img.Config.Entrypoint = append([]string{"/urunit"}, img.Config.ImageConfig.Entrypoint...)
	}
	if instr.Annots["com.urunc.unikernel.hypervisor"] == "" {
		instr.Annots["com.urunc.unikernel.hypervisor"] = "qemu"
	}
	if instr.Annots["com.urunc.unikernel.mountRootfs"] == "" &&
		instr.Annots["com.urunc.unikernel.initrd"] == "" &&
		instr.Annots["com.urunc.unikernel.blkMntPoint"] != "/" {
		instr.Annots["com.urunc.unikernel.mountRootfs"] = "true"
	}
	if instr.Annots["com.urunc.unikernel.binary"] == "" {
		var aCopy PackCopies

		aCopy.SrcState = llb.Image(defaultKernelImage)
		aCopy.SrcPath = DefaultKernelPath
		aCopy.DstPath = DefaultKernelPath
		instr.Copies = append(instr.Copies, aCopy)

		instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
	}

	return instr, nil
}
