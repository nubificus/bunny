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

type PackInstructions struct {
	Base   string            // The Base image to use
	Copies map[string]string // Mappings of files top copy, source as key and destination as value
	Annots map[string]string // Annotations
}

// HopsToPack converts Hops into PackInstructions
func HopsToPack(hops Hops) (*PackInstructions, error) {
	var instr *PackInstructions
	instr = new(PackInstructions)
	instr.Copies = make(map[string]string)
	instr.Annots = make(map[string]string)

	if hops.Kernel.From == "local" {
		instr.Base = "scratch"
		instr.Copies[hops.Kernel.Path] = DefaultKernelPath
		instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
	} else {
		instr.Base = hops.Kernel.From
		instr.Annots["com.urunc.unikernel.binary"] = hops.Kernel.Path
	}

	if hops.Rootfs.From == "local" && hops.Platform.Framework == "unikraft" {
		instr.Copies[hops.Rootfs.Path] = DefaultInitrdPath
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
func ParseBunnyFile(fileBytes []byte) (*PackInstructions, error) {
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

	return HopsToPack(bunnyHops)
}

// ParseDockerFile reads a Dockerfile-like file and returns a Hops
// struct with the info from the file
func ParseDockerFile(fileBytes []byte) (*PackInstructions, error) {
	var instr *PackInstructions
	instr = new(PackInstructions)
	instr.Copies = make(map[string]string)
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
			//instr.Copies = append(instr.Copies, *c)
			instr.Copies[c.SourcePaths[0]] = c.DestPath
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
func ParseFile(fileBytes []byte) (*PackInstructions, error) {
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
				return ParseDockerFile(fileBytes)
			} else {
				break
			}
		}
	}

	return ParseBunnyFile(fileBytes)
}

func copyIn(base llb.State, from string, src string, dst string) llb.State {
	var copyState llb.State
	var localSrc llb.State

	localSrc = llb.Local(from)
	copyState = base.File(llb.Copy(localSrc, src, dst, &llb.CopyInfo{
				CreateDestPath: true,}))

	return copyState
}

// PackLLB gets a PackInstructions struct and transforms it to an LLB definition
func PackLLB(instr PackInstructions, buildContext string) (*llb.Definition, error) {
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
	for src, dst := range instr.Copies {
		base = copyIn(base, buildContext, src, dst)
	}

	// Create the urunc.json file in the rootfs
	base = base.File(llb.Mkfile(uruncJSONPath, 0644, uruncJSONBytes))

	dt, err := base.Marshal(context.TODO(), llb.LinuxAmd64)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal LLB state: %v", err)
	}

	return dt, nil
}
