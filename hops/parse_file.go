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
	"bytes"
	"fmt"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"gopkg.in/yaml.v3"
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

// ParseContainerfile reads a Dockerfile-like file and returns a Hops
// struct with the info from the file
func ParseContainerfile(fileBytes []byte, buildContext string) (*PackInstructions, error) {
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
		case *instructions.CmdCommand:
			instr.Config.Cmd = c.CmdLine
		case *instructions.EntrypointCommand:
			instr.Config.Entrypoint = c.CmdLine
		case *instructions.EnvCommand:
			for _, kvp := range c.Env {
				eVar := kvp.Key + "=" + kvp.Value
				instr.Config.EnVars = append(instr.Config.EnVars, eVar)
			}
		case instructions.Command:
			// Catch all other commands
			return nil, fmt.Errorf("Unsupported command: %s", c.Name())
		default:
			return nil, fmt.Errorf("Not a command type: %s", c)
		}

	}
	instr.Base = GetSourceState(BaseString, instr.Annots["com.urunc.unikernel.hypervisor"])
	// TODO This check also takes place in GetSourceState, so we should merge them
	if BaseString != "scratch" && BaseString != "" {
		instr.Config.BaseRef = BaseString
		instr.Config.Monitor = instr.Annots["com.urunc.unikernel.hypervisor"]
	}

	// TODO: Remove this in next release.
	// Keep backwards compatibility and if the CMD in Containerfile
	// is not set, then the cmdline annotations will be used for the cmd of
	// the container image.
	if len(instr.Config.Cmd) == 0 && instr.Annots["com.urunc.unikernel.cmdline"] != "" {
		instr.Config.Cmd = strings.Split(instr.Annots["com.urunc.unikernel.cmdline"], " ")
	}

	return instr, nil
}

// ParseFile identifies the format of the given file and either calls
// ParseContainerfile or ParseBunnyfile
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
				return ParseContainerfile(fileBytes, buildContext)
			}
			break
		}
	}

	hops, err := ParseBunnyfile(fileBytes)
	if err != nil {
		return nil, err
	}
	return ToPack(hops, buildContext)
}
