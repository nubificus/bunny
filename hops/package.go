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
	DefaultUnikraftBaseImage  string = "harbor.nbfc.io/nubificus/bunny/unikraft/base:latest"
	DefaultMewzBaseImage      string = "harbor.nbfc.io/nubificus/bunny/mewz/base:latest"
	DefaultBsdcpioImage       string = "harbor.nbfc.io/nubificus/bunny/libarchive:latest"
	DefaultRustBuildImage     string = "harbor.nbfc.io/nubificus/bunny/rust/wasm:latest"
	DefaultCBuildImage        string = "harbor.nbfc.io/nubificus/bunny/c/wasm:latest"
	DefaultBuildDir           string = "/build/"
	DefaultInitrdContent      string = "/initrd/"
	DefaultKernelPath         string = "/.boot/kernel"
	DefaultRootfsPath         string = "/.boot/rootfs"
	unikraftKernelPath        string = "/unikraft/bin/kernel"
	unikraftHub               string = "unikraft.org"
	uruncJSONPath             string = "/urunc.json"
	bunnyFileVersion          string = "0.1"
)

type HopsPlatform struct {
	Framework string `yaml:"framework"`
	Version   string `yaml:"version"`
	Monitor   string `yaml:"monitor"`
	Arch      string `yaml:"arch"`
}

type HopsRootfs struct {
	From     string   `yaml:"from"`
	Path     string   `yaml:"path"`
	Type     string   `yaml:"type"`
	Includes []string `yaml:"include"`
}

type HopsKernel struct {
	From   string `yaml:"from"`
	Path   string `yaml:"path"`
}

type HopsApp struct {
	Name     string `yaml:"name"`
	Source   string `yaml:"source"`
	Path     string `yaml:"path"`
	Language string `yaml:"language"`
	Target   string `yaml:"target"`
}

type Hops struct {
	Version  string       `yaml:"version"`
	Platform HopsPlatform `yaml:"platforms"`
	Rootfs   HopsRootfs   `yaml:"rootfs"`
	Kernel   HopsKernel   `yaml:"kernel"`
	App      HopsApp      `yaml:"app"`
	Cmd      string       `yaml:"cmdline"`
}

type PackCopies struct {    // A struct to represent a copy operation in the final image
	SrcState llb.State  // The state where the file resides
	SrcPath  string     // The source path inside the SrcState where the file resides
	DstPath  string     // The destination path to copy the file inside the final image
}

type PackInstructions struct {
	Base   llb.State         // The Base image to use
	Copies []PackCopies      // A list of packCopies, rpresenting the files to copy inside the final image
	Annots map[string]string // Annotations
}

// HopsToPack converts Hops into PackInstructions
func HopsToPack(hops Hops, buildContext string) (*PackInstructions, error) {
	var instr *PackInstructions
	instr = new(PackInstructions)
	instr.Annots = make(map[string]string)
	instr.Annots["com.urunc.unikernel.useDMBlock"] = "false"

	if hops.Kernel.From != "" {
		if hops.Kernel.From == "local" {
			var aCopy PackCopies

			instr.Base = llb.Scratch()
			instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath

			aCopy.SrcState = llb.Local(buildContext)
			aCopy.SrcPath = hops.Kernel.Path
			aCopy.DstPath = DefaultKernelPath
			instr.Copies = append(instr.Copies, aCopy)
		} else {
			instr.Base = getBase(hops.Kernel.From)
			instr.Annots["com.urunc.unikernel.binary"] = hops.Kernel.Path
		}
	}

	if hops.Platform.Framework == "unikraft" {
		var aCopy PackCopies
		builtApp := llb.Scratch()

		if hops.App.Target == "wasm" {
			aCopy.SrcState = wamrUnikraftLLB(hops.App, hops.Platform.Version, buildContext)
			aCopy.SrcPath = DefaultKernelPath
			aCopy.DstPath = DefaultKernelPath
			instr.Copies = append(instr.Copies, aCopy)
			instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath

			if hops.App.Language != "c" {
				return nil, fmt.Errorf("Currently only C is supported for WASM")
			} else {
				builtApp = buildCWasm(hops.App, buildContext)
			}
		}

		// Make sure SrcPath is set to empty string, so we can check if
		// we got inside the if clauses and really set the SrcState and SrcPath
		aCopy.SrcPath = ""
		if hops.Rootfs.From != "" {
			if hops.Rootfs.From == "local" {
				aCopy.SrcState = llb.Local(buildContext)
				aCopy.SrcPath = hops.Rootfs.Path
			} else if hops.Rootfs.From == "scratch" {
				if len(hops.Rootfs.Includes) > 0 {
					local := llb.Local(buildContext)
					contentState := FilesLLB(hops.Rootfs.Includes, local, builtApp)
					aCopy.SrcState = initrdLLB(contentState)
					aCopy.SrcPath = DefaultRootfsPath
				}
			} else {
				aCopy.SrcState = llb.Image(hops.Rootfs.From)
				aCopy.SrcPath = hops.Rootfs.Path
			}
		} else {
			if hops.App.Target == "wasm" {
				aCopy.SrcState = initrdLLB(builtApp)
				aCopy.SrcPath = DefaultRootfsPath
			}
		}

		// Add the Copy onli if we got in one of the above if claueses.
		if aCopy.SrcPath != "" {
			aCopy.DstPath = DefaultRootfsPath
			instr.Copies = append(instr.Copies, aCopy)
			instr.Annots["com.urunc.unikernel.initrd"] = DefaultRootfsPath
		}
	} else if hops.Platform.Framework == "mewz" {
		var aCopy PackCopies

		if hops.App.Target == "wasm" {
			if hops.App.Language != "rust" {
				return nil, fmt.Errorf("Currently only Rust is supported for Mewz")
			} else {
				builtApp := buildRustWasm(hops.App, buildContext)
				aCopy.SrcState = mewzLLB(hops.App, builtApp)
				aCopy.SrcPath = DefaultKernelPath
				aCopy.DstPath = DefaultKernelPath
				instr.Copies = append(instr.Copies, aCopy)
				instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
			}
		}
	} else {
		var aCopy PackCopies
		// Make sure SrcPath is set to empty string, so we can check if
		// we got inside the if clauses and really set the SrcState and SrcPath
		aCopy.SrcPath = ""

		if hops.Rootfs.From == "local" {
			aCopy.SrcState = llb.Local(buildContext)
			aCopy.SrcPath = hops.Rootfs.Path
		} else if (hops.Rootfs.From == "scratch" || hops.Rootfs.From == "") {
			if len(hops.Rootfs.Includes) > 0 {
				if hops.Rootfs.Type == "initrd" {
					local := llb.Local(buildContext)
					contentState := FilesLLB(hops.Rootfs.Includes, local, llb.Scratch())
					aCopy.SrcState = initrdLLB(contentState)
					aCopy.SrcPath = DefaultRootfsPath
				} else if hops.Rootfs.Type == "raw" {
					instr.Annots["com.urunc.unikernel.useDMBlock"] = "true"
					if hops.Kernel.From != "local" {
						var kernelCopy PackCopies
						kernelCopy.SrcState = instr.Base
						kernelCopy.SrcPath = hops.Kernel.Path
						kernelCopy.DstPath = DefaultKernelPath
						instr.Copies = append(instr.Copies, kernelCopy)
						instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
					}
					local := llb.Local(buildContext)
					instr.Base = FilesLLB(hops.Rootfs.Includes, local, instr.Base)
				}
			}
		} else {
			if hops.Rootfs.Path != "" {
				aCopy.SrcState = llb.Image(hops.Rootfs.From)
				aCopy.SrcPath = hops.Rootfs.Path
			} else {
				if hops.Rootfs.Type == "raw" {
					instr.Annots["com.urunc.unikernel.useDMBlock"] = "true"
				}
				if hops.Kernel.From != "local" {
					var kernelCopy PackCopies
					kernelCopy.SrcState = instr.Base
					kernelCopy.SrcPath = hops.Kernel.Path
					kernelCopy.DstPath = DefaultKernelPath
					instr.Copies = append(instr.Copies, kernelCopy)
					instr.Annots["com.urunc.unikernel.binary"] = DefaultKernelPath
				}
				instr.Base = getBase(hops.Rootfs.From)
			}
		}

		// Add the Copy only if we got in one of the above if claueses.
		if aCopy.SrcPath != "" {
			aCopy.DstPath = DefaultRootfsPath
			instr.Copies = append(instr.Copies, aCopy)
			if hops.Rootfs.Type == "initrd" {
				instr.Annots["com.urunc.unikernel.initrd"] = DefaultRootfsPath
			} else if hops.Rootfs.Type == "block" {
				instr.Annots["com.urunc.unikernel.block"] = DefaultRootfsPath
				instr.Annots["com.urunc.unikernel.blkMntPoint"] = "/"
			}
		}
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

// ValidatePlatform checks if user input meets all conditions regarding the platforms
// field. The conditions are:
// 1) framework can not be empty or not set
// 2) monitor can not be empty or not set
func ValidatePlatform(plat HopsPlatform) error {
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
func ValidateRootfs(rootfs HopsRootfs) error {
	if (rootfs.From == "scratch" || rootfs.From == "") && rootfs.Path != "" {
		return fmt.Errorf("The from field of rootfs can not be empty or scratch, if path is set")
	}
	if rootfs.Path != "" && rootfs.Type == "raw" {
		return fmt.Errorf("The path field in rootfs can not be combined with a raw rootfs")
	}
	if rootfs.From == "local" && rootfs.Type == "raw"  {
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
func ValidateKernel(kernel HopsKernel) error {
	if kernel.From == "" {
		return fmt.Errorf("The from field of kernel is necessary")
	}
	if kernel.Path == "" {
		return fmt.Errorf("The path field of kernel is necessary")
	}

	return nil
}

// ValidateApp checks if user input meets all conditions regarding the app
// field. The conditions are:
// 1) source can not be empty or not set
// 2) language can not be empty or not set
func ValidateApp(app HopsApp) error {
	if app.Source != "local" {
		return fmt.Errorf("Currently only local is a valid value for app's source")
	}
	if app.Language == "" {
		return fmt.Errorf("The language field of app is necessary")
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

	if bunnyHops.App.Source == "" {
		err = ValidateKernel(bunnyHops.Kernel)
		if err != nil {
			return nil, fmt.Errorf("If app field is not set, then kernel field should be set: ", err)
		}
	}

	if bunnyHops.Kernel.From == "" {
		err = ValidateApp(bunnyHops.App)
		if err != nil {
			return nil, fmt.Errorf("If kernel field is not set, then app field should be set: ", err)
		}
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

	return HopsToPack(bunnyHops, buildContext)
}

// ParseDockerFile reads a Dockerfile-like file and returns a Hops
// struct with the info from the file
func ParseDockerFile(fileBytes []byte, buildContext string) (*PackInstructions, error) {
	var instr *PackInstructions
	instr = new(PackInstructions)
	instr.Annots = make(map[string]string)
	instr.Base = llb.Scratch()
	setStage := false

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
			if setStage {
				return nil, fmt.Errorf("Multi-stage builds are not supported")
			}
			setStage = true
			instr.Base = getBase(c.BaseName)
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

	// Simply check if the first non-empty/non-comment line starts With
	// FROM or version. If it starts with FROM, we assume a Dockerfile-like
	// syntax. Otherwise, we assume a bunnyfile syntax
	for _, line := range lines {
		if len(bytes.TrimSpace(line)) > 0 {
			if strings.HasPrefix(string(line), "#") {
				continue
			} else if strings.HasPrefix(string(line), "FROM") {
				return ParseDockerFile(fileBytes, buildContext)
			} else if strings.HasPrefix(string(line), "version") {
				return ParseBunnyFile(fileBytes, buildContext)
			} else {
				break
			}
		}
	}

	return nil, fmt.Errorf("Invalid format of file")
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

// Create a LLB State that builds a Mewz WAMR unikernel
func mewzLLB(appInfo HopsApp, wasmAppState llb.State) llb.State {
	base := llb.Image(DefaultMewzBaseImage, llb.WithCustomName("Internal:Build Mewz unikernel"))

	base = base.File(llb.Mkdir("/.boot/", 0755))
	base = base.Dir("/mewz").AddEnv("PATH", "/usr/bin/zig:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		Run(llb.Shlexf("wasker %s/%s.wasm", DefaultBuildDir, appInfo.Name), llb.AddMount(DefaultBuildDir, wasmAppState),).
		Run(llb.Shlex("zig build -Dapp-obj=./wasm.o")).
		Run(llb.Shlex("zig build -Dapp-obj=./wasm.o")).
		//Run(llb.Shlex("bash -c \"printf \"\\x03\\x00\" | dd of=zig-out/bin/mewz.elf  bs=1 seek=18 count=2 conv=notrunc\"")).
		Run(llb.Shlex("bash trela.sh")).
		Run(llb.Shlexf("cp zig-out/bin/mewz.elf %s", DefaultKernelPath)).
		Root()

	return base
}

// Create a LLB State that builds a Unikraft WAMR unikernel
func wamrUnikraftLLB(appInfo HopsApp, version string, buildContext string) llb.State {

	if version == "" {
		version = "0.17.0"
	}

	base := llb.Image(DefaultUnikraftBaseImage, llb.WithCustomName("Internal:Build Unikraft unikernel"))

	base = base.File(llb.Mkdir(DefaultBuildDir, 0755))
	base = base.File(llb.Mkdir("/.boot/", 0755))
	base = base.Dir(DefaultBuildDir).
		Run(llb.Shlex("git clone https://github.com/unikraft/app-wamr.git")).Root()
	base = base.Dir(DefaultBuildDir + "app-wamr").AddEnv("PWD", DefaultBuildDir + "app-wamr").
		//Run(llb.Shlex("git clone https://github.com/unikraft/app-wamr.git")).
		//Run(llb.Shlex("cd app-wamr")).
		Run(llb.Shlex("mkdir -p workdir/libs")).
		Run(llb.Shlexf("git clone https://github.com/unikraft/unikraft.git -b RELEASE-%s workdir/unikraft", version)).
		Run(llb.Shlexf("git clone https://github.com/unikraft/lib-lwip.git -b RELEASE-%s workdir/libs/lwip", version)).
		Run(llb.Shlexf("git clone https://github.com/unikraft/lib-musl.git -b RELEASE-%s workdir/libs/musl", version)).
		Run(llb.Shlex("git clone https://github.com/unikraft/lib-wamr.git workdir/libs/wamr")).
		Run(llb.Shlex("cp defconfigs/qemu-x86_64-initrd .config")).
		Run(llb.Shlex("ls workdir")).
		Run(llb.Shlex("ls workdir")).
		Run(llb.Shlex("ls workdir")).
		Run(llb.Shlex("make olddefconfig")).
		Run(llb.Shlex("make")).
		Run(llb.Shlexf("cp workdir/build/wamr_qemu-x86_64 %s", DefaultKernelPath)).
		Root()

	return base
}

// Create a LLB State that builds a C program targetting wasm
func buildCWasm(appInfo HopsApp, buildContext string) llb.State {
	base := llb.Image(DefaultCBuildImage, llb.WithCustomName("Internal:Build C in WASM"))

	local := llb.Local(buildContext)
	base = base.File(llb.Mkdir("/target_dir", 0755))
	base = base.Dir(DefaultBuildDir).
		Run(llb.Shlexf("clang-8 --target=wasm32 -O3 -Wl,--initial-memory=131072,--allow-undefined,--export=main,--no-threads,--strip-all,--no-entry -nostdlib -o /target_dir/%s.wasm %s.c", appInfo.Name, appInfo.Name), llb.AddMount(DefaultBuildDir, local),).Root()
	binCopy := []string{"/target_dir/"+appInfo.Name+".wasm:/"+appInfo.Name+".wasm"}
	return FilesLLB(binCopy, base, llb.Scratch())
}

// Create a LLB State that builds a Rust program targetting wasm
func buildRustWasm(appInfo HopsApp, buildContext string) llb.State {
	base := llb.Image(DefaultRustBuildImage, llb.WithCustomName("Internal:Build Rust in WASM"))

	local := llb.Local(buildContext)
	base = base.File(llb.Mkdir("/target_dir", 0755))
	base = base.Dir(DefaultBuildDir).
		AddEnv("PATH", "/usr/local/cargo/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		AddEnv("RUSTUP_HOME", "/usr/local/rustup").
		AddEnv("CARGO_HOME", "/usr/local/cargo").
		AddEnv("RUST_VERSION", "1.84.0").
		Run(llb.Shlex("find .")).
		Run(llb.Shlex("cargo build --target wasm32-wasip1  --target-dir=/target_dir"), llb.AddMount(DefaultBuildDir, local),).Root()
	binCopy := []string{"/target_dir/wasm32-wasip1/debug/"+appInfo.Name+".wasm:/"+appInfo.Name+".wasm"}
	return FilesLLB(binCopy, base, llb.Scratch())
}

// Create a LLB State that creates an initrd based on the data from the HopsRootfs
// argument
func initrdLLB(content llb.State) llb.State {
	base := llb.Image(DefaultBsdcpioImage, llb.WithCustomName("Internal:Create initrd"))

	base = base.File(llb.Mkdir("/.boot/", 0755))
	base = base.File(llb.Mkdir("/tmp", 0755))
	base = base.Dir(DefaultInitrdContent).
		Run(llb.Shlexf("sh -c \"find . -depth -print | tac | bsdcpio -o --format newc > %s && find /.boot && find\"", DefaultRootfsPath), llb.AddMount(DefaultInitrdContent, content),).Root()
	return base
}

func copyIn(to llb.State, from PackCopies) llb.State {

	copyState := to.File(llb.Copy(from.SrcState, from.SrcPath, from.DstPath,
				&llb.CopyInfo{CreateDestPath: true,}))

	return copyState
}

// Set the base image where we will pack the unikernel
func getBase(inputBase string) llb.State {
	var retBase llb.State

	if inputBase == "scratch" {
		retBase = llb.Scratch()
	} else if strings.HasPrefix(inputBase, unikraftHub) {
		// Define the platform to qemu/amd64 so we cna pull unikraft images
		platform := ocispecs.Platform{
			OS:           "qemu",
			Architecture: "amd64",
		}
		retBase = llb.Image(inputBase, llb.Platform(platform),)
	} else {
		retBase = llb.Image(inputBase)
	}

	return retBase
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

	dt, err := base.Marshal(context.TODO(), llb.LinuxAmd64)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal LLB state: %v", err)
	}

	return dt, nil
}
