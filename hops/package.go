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

	"github.com/moby/buildkit/client/llb"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
)

const (
	unikraftKernelPath string = "/unikraft/bin/kernel"
	unikraftHub        string = "unikraft.org"
	uruncJSONPath      string = "/urunc.json"
)

type PackInstructions struct {
	Base   string                     // The Base image to use
	Copies []instructions.CopyCommand // Copy commands
	Annots map[string]string          // Annotations
}

// ParseDockerFile reads a Dockerfile-like file and returns a Hops
// struct with the info from the file
func ParseDockerFile(fileBytes []byte) (*PackInstructions, error) {
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
			instr.Copies = append(instr.Copies, *c)
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
	for _, aCopy := range instr.Copies {
		base = copyIn(base, buildContext, aCopy.SourcePaths[0], aCopy.DestPath)
	}

	// Create the urunc.json file in the rootfs
	base = base.File(llb.Mkfile(uruncJSONPath, 0644, uruncJSONBytes))

	dt, err := base.Marshal(context.TODO(), llb.LinuxAmd64)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal LLB state: %v", err)
	}

	return dt, nil
}
