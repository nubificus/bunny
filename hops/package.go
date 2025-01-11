// Copyright (c) 2023-2024, Nubificus LTD
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
	"encoding/base64"
	"encoding/json"
	"context"
	"fmt"
	"bytes"
	"strings"

	"gopkg.in/yaml.v3"
	"github.com/moby/buildkit/client/llb"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/hashicorp/go-version"
)

const (
	DefaultKernelPath  string = "/.boot/kernel"
	DefaultInitrdPath  string = "/.boot/initrd"
	unikraftKernelPath string = "/unikraft/bin/kernel"
	unikraftHub        string = "unikraft.org"
	uruncJSONPath      string = "/urunc.json"
	bunnyFileVersion   string = "0.1"
)

type HopsPlatform struct {
	Framework string `yaml:"framework"`
	Version   string `yaml:"version"`
	Monitor   string `yaml:"monitor"`
	Arch      string `yaml:"arch"`
}

type HopsRootfs struct {
	From string `yaml:"from"`
	Path string `yaml:"path"`
}

type HopsKernel struct {
	From   string `yaml:"from"`
	Path   string `yaml:"path"`
}

type Hops struct {
	Version  string       `yaml:"version"`
	Platform HopsPlatform `yaml:"platforms"`
	Rootfs   HopsRootfs   `yaml:"rootfs"`
	Kernel   HopsKernel   `yaml:"kernel"`
	Cmd      string       `yaml:"cmdline"`
}

type PackCopies struct {    // A struct to represent a copy operation in the final image
	SrcState llb.State  // The state where the file resides
	SrcPath  string     // The source path inside the SrcState where the file resides
	DstPath  string     // The destination path to copy the file inside the final image
}

type PackInstructions struct {
	Base   string            // The Base image to use
	Copies []PackCopies      // A list of packCopies, rpresenting the files to copy inside the final image
	Annots map[string]string // Annotations
}

// HopsToPack converts Hops into PackInstructions
func HopsToPack(hops Hops, buildContext string) (*PackInstructions, error) {
	var instr *PackInstructions
	instr = new(PackInstructions)
	instr.Annots = make(map[string]string)

	if hops.Kernel.From == "local" {
		var aCopy PackCopies

		instr.Base = "scratch"
		instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath

		aCopy.SrcState = llb.Local(buildContext)
		aCopy.SrcPath = hops.Kernel.Path
		aCopy.DstPath = DefaultKernelPath
		instr.Copies = append(instr.Copies, aCopy)
	} else {
		instr.Base = hops.Kernel.From
		instr.Annots["com.urunc.unikernel.binary"] = hops.Kernel.Path
	}

	if hops.Rootfs.From == "local" && hops.Platform.Framework == "unikraft" {
		var aCopy PackCopies

		aCopy.SrcState = llb.Local(buildContext)
		aCopy.SrcPath = hops.Rootfs.Path
		aCopy.DstPath = DefaultInitrdPath
		instr.Copies = append(instr.Copies, aCopy)
		instr.Annots["com.urunc.unikernel.initrd"] = DefaultInitrdPath
	}
	instr.Annots["com.urunc.unikernel.unikernelType"] = hops.Platform.Framework
	instr.Annots["com.urunc.unikernel.cmdline"] = hops.Cmd
	instr.Annots["com.urunc.unikernel.hypervisor"] = hops.Platform.Monitor
	if hops.Platform.Version != "" {
		instr.Annots["com.urunc.unikernel.unikernelVersion"] = hops.Platform.Version
	}

	return instr, nil
}

// CheckBunnyfileVersion checks if the version of the user's input file
// is compatible with the supported version.
func CheckBunnyfileVersion(userVersion string) error {
	if userVersion == "" {
		return fmt.Errorf("The version field is necessary")
	}
	hopsVersion, err := version.NewVersion(bunnyFileVersion)
	if err != nil {
		return fmt.Errorf("Internal error in current bunnyfile version %s: %v", bunnyFileVersion, err)
	}
	userFileVer, err := version.NewVersion(userVersion)
	if err != nil {
		return fmt.Errorf("Could not parse version in user bunnyfile %s: %v", userVersion, err)
	}
	if hopsVersion.LessThan(userFileVer) {
		return fmt.Errorf("Unsupported version %s. Please use %s or earlier", userVersion, bunnyFileVersion)
	}

	return nil
}

// ParseBunnyFile reads a yaml file which contains instructions for
// bunny.
func ParseBunnyFile(fileBytes []byte, buildContext string) (*PackInstructions, error) {
	var bunnyHops Hops

	err := yaml.Unmarshal(fileBytes, &bunnyHops)
	if err != nil {
		return nil, err
	}

	err =  CheckBunnyfileVersion(bunnyHops.Version)
	if err != nil {
		return nil, err
	}

	if bunnyHops.Platform.Framework == "" {
		return nil, fmt.Errorf("The framework field of platforms is necessary")
	}

	if bunnyHops.Platform.Monitor == "" {
		return nil, fmt.Errorf("The monitor field of platforms is necessary")
	}

	if bunnyHops.Kernel.From == "" {
		return nil, fmt.Errorf("The from field of kernel is necessary")
	}

	if bunnyHops.Kernel.Path == "" {
		return nil, fmt.Errorf("The path field of kernel is necessary")
	}

	return HopsToPack(bunnyHops, buildContext)
}

// ParseDockerFile reads a Dockerfile-like file and returns a Hops
// struct with the info from the file
func ParseDockerFile(fileBytes []byte, buildContext string) (*PackInstructions, error) {
	var instr *PackInstructions
	instr = new(PackInstructions)
	instr.Annots = make(map[string]string)

	r := bytes.NewReader(fileBytes)

	// Parse the Dockerfile
	parseRes, err := parser.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse data as Dockerfile: %v", err)
	}

	// Traverse Dockerfile commands
	for _, child := range parseRes.AST.Children {
		cmd, err := instructions.ParseInstruction(child)
		if err != nil {
			return nil, fmt.Errorf("Line %d: %v", child.StartLine, err)
		}
		switch c := cmd.(type) {
		case *instructions.Stage:
			// Handle FROM
			if instr.Base != "" {
				return nil, fmt.Errorf("Multi-stage builds are not supported")
			}
			instr.Base = c.BaseName
		case *instructions.CopyCommand:
			// Handle COPY
			var aCopy PackCopies

			aCopy.SrcState = llb.Local(buildContext)
			aCopy.SrcPath = c.SourcePaths[0]
			aCopy.DstPath = c.DestPath
			instr.Copies = append(instr.Copies, aCopy)
		case *instructions.LabelCommand:
			// Handle LABLE annotations
			for _, kvp := range c.Labels {
				annotKey := strings.Trim(kvp.Key, "\"")
				instr.Annots[annotKey] = strings.Trim(kvp.Value, "\"")
			}
		case instructions.Command:
			// Catch all other commands
			return nil, fmt.Errorf("Unsupported command: %s", c.Name())
		default:
			return nil, fmt.Errorf("Not a command type: %s", c)
		}

	}

	return instr, nil
}

// ParseFile identifies the format of the given file and either calls
// ParseDockerFile or ParseBunnyFile
func ParseFile(fileBytes []byte, buildContext string) (*PackInstructions, error) {
	lines := bytes.Split(fileBytes, []byte("\n"))

	// First line is always the #syntax
	if len(lines) <= 1 {
		return nil, fmt.Errorf("Invalid format of file")
	}

	// Simply check if the first non-empty line starts with FROM
	// If it starts we assume a Dockerfile
	// otherwise a bunnyfile
	for _, line := range lines[1:] {
		if len(bytes.TrimSpace(line)) > 0 {
			if strings.HasPrefix(string(line), "FROM") {
				return ParseDockerFile(fileBytes, buildContext)
			} else {
				break
			}
		}
	}

	return ParseBunnyFile(fileBytes, buildContext)
}

func copyIn(to llb.State, from PackCopies) llb.State {

	copyState := to.File(llb.Copy(from.SrcState, from.SrcPath, from.DstPath,
				&llb.CopyInfo{CreateDestPath: true,}))

	return copyState
}

// PackLLB gets a PackInstructions struct and transforms it to an LLB definition
func PackLLB(instr PackInstructions) (*llb.Definition, error) {
	var base llb.State
	uruncJSON := make(map[string]string)

	// Create urunc.json file, since annotations do not reach urunc
	for annot, val := range instr.Annots {
		encoded := base64.StdEncoding.EncodeToString([]byte(val))
		uruncJSON[annot] = string(encoded)
	}
	uruncJSONBytes, err := json.Marshal(uruncJSON)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal urunc json: %v", err)
	}

	// Set the base image where we will pack the unikernel
	if instr.Base == "scratch" {
		base = llb.Scratch()
	} else if strings.HasPrefix(instr.Base, unikraftHub) {
		// Define the platform to qemu/amd64 so we cna pull unikraft images
		platform := ocispecs.Platform{
			OS:           "qemu",
			Architecture: "amd64",
		}
		base = llb.Image(instr.Base, llb.Platform(platform),)
	} else {
		base = llb.Image(instr.Base)
	}

	// Perform any copies inside the image
	for _, aCopy := range instr.Copies {
		base = copyIn(base, aCopy)
	}

	// Create the urunc.json file in the rootfs
	base = base.File(llb.Mkfile(uruncJSONPath, 0644, uruncJSONBytes))

	dt, err := base.Marshal(context.TODO(), llb.LinuxAmd64)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal LLB state: %v", err)
	}

	return dt, nil
}
