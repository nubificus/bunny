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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"gopkg.in/yaml.v3"
)

const (
	DefaultBsdcpioImage  string = "harbor.nbfc.io/nubificus/bunny/libarchive:latest"
	DefaultInitrdContent string = "/initrd/"
	DefaultKernelPath    string = "/.boot/kernel"
	DefaultRootfsPath    string = "/.boot/rootfs"
	unikraftKernelPath   string = "/unikraft/bin/kernel"
	unikraftHub          string = "unikraft.org"
	uruncJSONPath        string = "/urunc.json"
	bunnyFileVersion     string = "0.1"
)

type Platform struct {
	Framework string `yaml:"framework"`
	Version   string `yaml:"version"`
	Monitor   string `yaml:"monitor"`
	Arch      string `yaml:"arch"`
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

type PackCopies struct { // A struct to represent a copy operation in the final image
	SrcState llb.State // The state where the file resides
	SrcPath  string    // The source path inside the SrcState where the file resides
	DstPath  string    // The destination path to copy the file inside the final image
}

type PackInstructions struct {
	Base   llb.State         // The Base image to use
	Copies []PackCopies      // A list of packCopies, rpresenting the files to copy inside the final image
	Annots map[string]string // Annotations
}

// ToPack converts Hops into PackInstructions
func (h Hops) ToPack(buildContext string) (*PackInstructions, error) {
	instr := new(PackInstructions)
	instr.Annots = make(map[string]string)
	instr.Annots["com.urunc.unikernel.useDMBlock"] = "false"

	if h.Kernel.From == "local" {
		var aCopy PackCopies

		instr.Base = llb.Scratch()
		instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath

		aCopy.SrcState = llb.Local(buildContext)
		aCopy.SrcPath = h.Kernel.Path
		aCopy.DstPath = DefaultKernelPath
		instr.Copies = append(instr.Copies, aCopy)
	} else {
		instr.Base = getBase(h.Kernel.From, h.Platform.Monitor)
		instr.Annots["com.urunc.unikernel.binary"] = h.Kernel.Path
	}

	if h.Platform.Framework == "unikraft" {
		var aCopy PackCopies
		// Make sure SrcPath is set to empty string, so we can check if
		// we got inside the if clauses and really set the SrcState and SrcPath
		aCopy.SrcPath = ""

		if h.Rootfs.From != "" {
			switch h.Rootfs.From {
			case "local":
				aCopy.SrcState = llb.Local(buildContext)
				aCopy.SrcPath = h.Rootfs.Path
			case "scratch":
				if len(h.Rootfs.Includes) > 0 {
					local := llb.Local(buildContext)
					contentState := FilesLLB(h.Rootfs.Includes, local, llb.Scratch())
					aCopy.SrcState = initrdLLB(contentState)
					aCopy.SrcPath = DefaultRootfsPath
				}
			default:
				aCopy.SrcState = llb.Image(h.Rootfs.From)
				aCopy.SrcPath = h.Rootfs.Path
			}
		} else {
			aCopy.SrcState = llb.Image(h.Rootfs.From)
			aCopy.SrcPath = h.Rootfs.Path
		}

		// Add the Copy onli if we got in one of the above if claueses.
		if aCopy.SrcPath != "" {
			aCopy.DstPath = DefaultRootfsPath
			instr.Copies = append(instr.Copies, aCopy)
			instr.Annots["com.urunc.unikernel.initrd"] = DefaultRootfsPath
		}
	} else {
		var aCopy PackCopies
		// Make sure SrcPath is set to empty string, so we can check if
		// we got inside the if clauses and really set the SrcState and SrcPath
		aCopy.SrcPath = ""
		switch h.Rootfs.From {
		case "local":
			aCopy.SrcState = llb.Local(buildContext)
			aCopy.SrcPath = h.Rootfs.Path
		case "scratch", "":
			if len(h.Rootfs.Includes) > 0 {
				if h.Rootfs.Type == "initrd" {
					local := llb.Local(buildContext)
					contentState := FilesLLB(h.Rootfs.Includes, local, llb.Scratch())
					aCopy.SrcState = initrdLLB(contentState)
					aCopy.SrcPath = DefaultRootfsPath
				} else if h.Rootfs.Type == "raw" {
					instr.Annots["com.urunc.unikernel.useDMBlock"] = "true"
					if h.Kernel.From != "local" {
						var kernelCopy PackCopies
						kernelCopy.SrcState = instr.Base
						kernelCopy.SrcPath = h.Kernel.Path
						kernelCopy.DstPath = DefaultKernelPath
						instr.Copies = append(instr.Copies, kernelCopy)
						instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
					}
					local := llb.Local(buildContext)
					instr.Base = FilesLLB(h.Rootfs.Includes, local, instr.Base)
				}
			}
		default:
			if h.Rootfs.Path != "" {
				aCopy.SrcState = llb.Image(h.Rootfs.From)
				aCopy.SrcPath = h.Rootfs.Path
			} else {
				if h.Rootfs.Type == "raw" {
					instr.Annots["com.urunc.unikernel.useDMBlock"] = "true"
				}
				if h.Kernel.From != "local" {
					var kernelCopy PackCopies
					kernelCopy.SrcState = instr.Base
					kernelCopy.SrcPath = h.Kernel.Path
					kernelCopy.DstPath = DefaultKernelPath
					instr.Copies = append(instr.Copies, kernelCopy)
					instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
				}
				instr.Base = getBase(h.Rootfs.From, "")
			}
		}

		// Add the Copy only if we got in one of the above if claueses.
		if aCopy.SrcPath != "" {
			aCopy.DstPath = DefaultRootfsPath
			instr.Copies = append(instr.Copies, aCopy)
			if h.Rootfs.Type == "initrd" {
				instr.Annots["com.urunc.unikernel.initrd"] = DefaultRootfsPath
			} else if h.Rootfs.Type == "block" {
				instr.Annots["com.urunc.unikernel.block"] = DefaultRootfsPath
				instr.Annots["com.urunc.unikernel.blkMntPoint"] = "/"
			}
		}
	}
	instr.Annots["com.urunc.unikernel.unikernelType"] = h.Platform.Framework
	instr.Annots["com.urunc.unikernel.cmdline"] = h.Cmd
	instr.Annots["com.urunc.unikernel.hypervisor"] = h.Platform.Monitor
	if h.Platform.Version != "" {
		instr.Annots["com.urunc.unikernel.unikernelVersion"] = h.Platform.Version
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

// ValidatePlatform checks if user input meets all conditions regarding the platforms
// field. The conditions are:
// 1) framework can not be empty or not set
// 2) monitor can not be empty or not set
func ValidatePlatform(plat Platform) error {
	if plat.Framework == "" {
		return fmt.Errorf("The framework field of platforms is necessary")
	}
	if plat.Monitor == "" {
		return fmt.Errorf("The monitor field of platforms is necessary")
	}

	return nil
}

// ValidateRootfs checks if user input meets all conditions regarding the rootfs
// field. The conditions are:
// 1) if from is empty then path should also be empty
// 2) if path is empty then from should also be empty
// 3) if from is not scratch or empty, include should not be set
// 4) An entry in include can not have the first part (before ":" empty
func ValidateRootfs(rootfs Rootfs) error {
	if (rootfs.From == "scratch" || rootfs.From == "") && rootfs.Path != "" {
		return fmt.Errorf("The from field of rootfs can not be empty or scratch, if path is set")
	}
	if rootfs.Path != "" && rootfs.Type == "raw" {
		return fmt.Errorf("The path field in rootfs can not be combined with a raw rootfs")
	}
	if rootfs.From == "local" && rootfs.Type == "raw" {
		return fmt.Errorf("If type of rootfs is raw, then from can not be local")
	}
	if len(rootfs.Includes) > 0 && rootfs.From != "scratch" {
		return fmt.Errorf("Adding files to an existing rootfs is not yet supported")
	}

	for _, file := range rootfs.Includes {
		parts := strings.Split(file, ":")
		if len(parts) < 1 || len(parts[0]) == 0 {
			return fmt.Errorf("Invalid syntax in rootf's include. AN entry can not have its first part empty")
		}
	}

	return nil
}

// ValidateKernel checks if user input meets all conditions regarding the kernel
// field. The conditions are:
// 1) from can not be empty or not set
// 2) path not be empty or not set
func ValidateKernel(kernel Kernel) error {
	if kernel.From == "" {
		return fmt.Errorf("The from field of kernel is necessary")
	}
	if kernel.Path == "" {
		return fmt.Errorf("The path field of kernel is necessary")
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

	// Set default value of from to scratch if include is specified.
	if bunnyHops.Rootfs.From == "" && len(bunnyHops.Rootfs.Includes) > 0 {
		bunnyHops.Rootfs.From = "scratch"
	}
	if (bunnyHops.Rootfs.From == "scratch" || bunnyHops.Rootfs.From == "") && bunnyHops.Rootfs.Type == "" {

		bunnyHops.Rootfs.Type = "raw"
	}
	err = ValidateRootfs(bunnyHops.Rootfs)
	if err != nil {
		return nil, err
	}

	return bunnyHops.ToPack(buildContext)
}

// ParseDockerFile reads a Dockerfile-like file and returns a Hops
// struct with the info from the file
func ParseDockerFile(fileBytes []byte, buildContext string) (*PackInstructions, error) {
	instr := new(PackInstructions)
	instr.Annots = make(map[string]string)
	instr.Base = llb.Scratch()
	BaseString := ""

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
			if BaseString != "" {
				return nil, fmt.Errorf("Multi-stage builds are not supported")
			}
			BaseString = c.BaseName
		case *instructions.CopyCommand:
			// Handle COPY
			var aCopy PackCopies

			aCopy.SrcState = llb.Local(buildContext)
			aCopy.SrcPath = c.SourcePaths[0]
			aCopy.DstPath = c.DestPath
			instr.Copies = append(instr.Copies, aCopy)
		case *instructions.LabelCommand:
			// Handle LABEL annotations
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
	instr.Base = getBase(BaseString, instr.Annots["com.urunc.unikernel.hypervisor"])

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
			}
			break
		}
	}

	return ParseBunnyFile(fileBytes, buildContext)
}

// Create a LLB State that simply copies all the files in the include list inside
// an empty image
func FilesLLB(fileList []string, fromState llb.State, toState llb.State) llb.State {
	for _, file := range fileList {
		var aCopy PackCopies

		parts := strings.Split(file, ":")
		aCopy.SrcState = fromState
		aCopy.SrcPath = parts[0]
		// If user did not define destination path, use the same as the source
		aCopy.DstPath = parts[0]
		if len(parts) != 1 && len(parts[1]) > 0 {
			aCopy.DstPath = parts[1]
		}
		toState = copyIn(toState, aCopy)
	}

	return toState
}

// Create a LLB State that creates an initrd based on the data from the HopsRootfs
// argument
func initrdLLB(content llb.State) llb.State {
	base := llb.Image(DefaultBsdcpioImage, llb.WithCustomName("Internal:Create initrd"))

	base = base.File(llb.Mkdir("/.boot/", 0755))
	base = base.File(llb.Mkdir("/tmp", 0755))
	base = base.Dir(DefaultInitrdContent).
		Run(llb.Shlexf("sh -c \"find . -depth -print | tac | bsdcpio -o --format newc > %s && find /.boot && find\"", DefaultRootfsPath), llb.AddMount(DefaultInitrdContent, content)).Root()
	return base
}

func copyIn(to llb.State, from PackCopies) llb.State {

	copyState := to.File(llb.Copy(from.SrcState, from.SrcPath, from.DstPath,
		&llb.CopyInfo{CreateDestPath: true}))

	return copyState
}

// Set the base image where we will pack the unikernel
func getBase(inputBase string, monitor string) llb.State {
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
		base = copyIn(base, aCopy)
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
