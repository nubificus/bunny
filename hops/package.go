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
	Copies []PackCopies      // The files to copy inside the final image
	Annots map[string]string // Annotations
}

var Version string

// ToPack converts Hops into PackInstructions
func ToPack(h Hops, buildContext string) (*PackInstructions, error) {
	var framework Framework
	instr := &PackInstructions{
		Annots: map[string]string{
			"com.urunc.unikernel.useDMBlock":    "false",
			"com.urunc.unikernel.unikernelType": h.Platform.Framework,
			"com.urunc.unikernel.cmdline":       h.Cmd,
			"com.urunc.unikernel.hypervisor":    h.Platform.Monitor,
		},
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
		instr.Base = getBase(h.Kernel.From, h.Platform.Monitor)
		instr.Annots["com.urunc.unikernel.binary"] = h.Kernel.Path
	}

	// Get the framework and call the respective function to create the
	// rootfs.
	switch h.Platform.Framework {
	case unikraftName:
		framework = newUnikraft(h.Platform, h.Rootfs)
	default:
		framework = newGeneric(h.Platform, h.Rootfs)
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
			instr.Annots["com.urunc.unikernel.useDMBlock"] = "true"
			// Switch the base to the rootfs's From image
			// and copy the kernel inside it.
			var kernelCopy PackCopies
			kernelCopy.SrcState = instr.Base
			kernelCopy.SrcPath = h.Kernel.Path
			kernelCopy.DstPath = DefaultKernelPath
			instr.Copies = append(instr.Copies, kernelCopy)
			instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
			instr.Base = getBase(h.Rootfs.From, "")
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
	rootfsState := framework.CreateRootfs(buildContext)
	if framework.GetRootfsType() == "initrd" {
		instr.Annots["com.urunc.unikernel.initrd"] = DefaultRootfsPath
	}

	// Switch the base to the rootfs's From image
	// and copy the kernel inside it.
	var kernelCopy PackCopies
	kernelCopy.SrcState = instr.Base
	kernelCopy.SrcPath = h.Kernel.Path
	kernelCopy.DstPath = DefaultKernelPath
	instr.Copies = append(instr.Copies, kernelCopy)
	instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
	instr.Base = rootfsState

	return instr, nil
}

// CheckBunnyfileVersion checks if the version of the user's input file
// is compatible with the supported version.
func CheckBunnyfileVersion(fileVersion string) error {
	if fileVersion == "" {
		return fmt.Errorf("The version field is necessary")
	}
	// TODO: Replace tempVersion with Version, when we reach v0.1
	tempVersion := "0.1"
	hopsVersion, err := version.NewVersion(tempVersion)
	if err != nil {
		return fmt.Errorf("Internal error in current bunnyfile version %s: %v", tempVersion, err)
	}
	userFileVer, err := version.NewVersion(fileVersion)
	if err != nil {
		return fmt.Errorf("Could not parse version in user bunnyfile %s: %v", fileVersion, err)
	}
	if hopsVersion.LessThan(userFileVer) {
		return fmt.Errorf("Unsupported version %s. Please use %s or earlier", fileVersion, tempVersion)
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
// 1) if from is empty/scratch then path should also be empty
// 2) if path is empty then from should also be empty
// 3) if from is not scratch or empty, include should not be set
// 4) An entry in include can not have the first part (before ":" empty
func ValidateRootfs(rootfs Rootfs) error {
	if (rootfs.From == "scratch") && rootfs.Path != "" {
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
			return fmt.Errorf("Invalid syntax in rootf's include. An entry can not have its first part empty")
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

	// Set default value of from to scratch
	// Make sure that any reference to Rootfs.From can not be an empty string
	if bunnyHops.Rootfs.From == "" {
		bunnyHops.Rootfs.From = "scratch"
	}
	err = ValidateRootfs(bunnyHops.Rootfs)
	if err != nil {
		return nil, err
	}

	return ToPack(bunnyHops, buildContext)
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

// Create a LLB State that constructs a cpio file with the data in the content
// State
func InitrdLLB(content llb.State) llb.State {
	outDir := "/.boot"
	workDir := "/workdir"
	toolSet := llb.Image(DefaultBsdcpioImage, llb.WithCustomName("Internal:Create initrd")).
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
