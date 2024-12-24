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

package main

import (
	"encoding/base64"
	"encoding/json"
	"context"
	"flag"
	"os"
	"fmt"
	"bytes"
	"strings"
	"io/ioutil"

	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/gateway/client"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
)

const (
	unikraftKernelPath string = "/unikraft/bin/kernel"
	unikraftHub        string = "unikraft.org"
	packContextName    string = "context"
	clientOptFilename  string = "filename"
	uruncJSONPath      string = "/urunc.json"
)

type CLIOpts struct {
	// If set just print the version and exit
	Version        bool
	// The Containerfile to be used for building the unikernel container
	ContainerFile  string
	// Choose the execution mode. If set, then pun will not act as a
	// buidlkit frontend. Instead it will just print the LLB.
	PrintLLB       bool
}

type PackInstructions struct {
	Base   string			  // The Base image to use
	Copies []instructions.CopyCommand // Copy commands
	Annots map[string]string	  // Annotations
}

var version string

func usage() {

	fmt.Println("Usage of pun")
	fmt.Printf("%s [<args>]\n\n", os.Args[0])
	fmt.Println("Supported command line arguments")
	fmt.Println("\t-v, --version bool \t\tPrint the version and exit")
	fmt.Println("\t-f, --file filename \t\tPath to the Containerfile")
	fmt.Println("\t--LLB bool \t\t\tPrint the LLB instead of acting as a frontend")
}

func parseCLIOpts() CLIOpts {
	var opts CLIOpts

	flag.BoolVar(&opts.Version, "version", false, "Print the version and exit")
	flag.BoolVar(&opts.Version, "v", false, "Print the version and exit")
	flag.StringVar(&opts.ContainerFile, "file", "", "Path to the Containerfile")
	flag.StringVar(&opts.ContainerFile, "f", "", "Path to the Containerfile")
	flag.BoolVar(&opts.PrintLLB, "LLB", false, "Print the LLB, instead of acting as a frontend")

	flag.Usage = usage
	flag.Parse()

	return opts
}

func parseFile(fileBytes []byte) (*PackInstructions, error) {
	var instr *PackInstructions
	instr = new(PackInstructions)
	instr.Annots = make(map[string]string)

	r := bytes.NewReader(fileBytes)

	// Parse the Dockerfile
	parseRes, err := parser.Parse(r)
	if err != nil {
		fmt.Printf("Failed to parse file: %v\n", err)
		return nil, err
	}

	// Traverse Dockerfile commands
	for _, child := range parseRes.AST.Children {
		cmd, err := instructions.ParseInstruction(child)
		if err != nil {
			fmt.Printf("Failed to parse instruction %s: %v\n", child.Value, err)
			return nil, err
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
			fmt.Printf("UNsupported command%s\n", c.Name())
		default:
			fmt.Printf("%f is not a command type\n", c)
		}

	}

	return instr, nil
}

func copyIn(base llb.State, from string, src string, dst string) llb.State {
	var copyState llb.State
	var localSrc llb.State

	localSrc = llb.Local(packContextName)
	copyState = base.File(llb.Copy(localSrc, src, dst, &llb.CopyInfo{
				CreateDestPath: true,}))

	return copyState
}

func constructLLB(instr PackInstructions) (*llb.Definition, error) {
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
	} else {
		// Define the platform to qemu/amd64 so we cna pull unikraft images
		platform := ocispecs.Platform{
			OS:           "qemu",
			Architecture: "amd64",
		}
		base = llb.Image(instr.Base, llb.Platform(platform),)
	}

	// Perform any copies inside the image
	for _, aCopy := range instr.Copies {
		base = copyIn(base, packContextName, aCopy.SourcePaths[0], aCopy.DestPath)
	}

	// Create the urunc.json file in the rootfs
	base = base.File(llb.Mkfile(uruncJSONPath, 0644, uruncJSONBytes))

	dt, err := base.Marshal(context.TODO(), llb.LinuxAmd64)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal LLB state: %v", err)
	}

	return dt, nil
}

func readFileFromLLB(ctx context.Context, c client.Client, filename string) ([]byte, error) {
	// Get the file from client's context
	fileSrc := llb.Local(packContextName, llb.IncludePatterns([]string {filename}),
				llb.WithCustomName("Internal:Read-" + filename))
	fileDef, err := fileSrc.Marshal(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal state for fetching %s: %w", clientOptFilename, err)
	}
	fileRes, err := c.Solve(ctx, client.SolveRequest{
		Definition: fileDef.ToPB(),
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to solve state for fetching %s: %w", clientOptFilename, err)
	}
	fileRef, err := fileRes.SingleRef()
	if err != nil {
		return nil, fmt.Errorf("Failed to get ref from solve resutl for fetching %s: %w", clientOptFilename, err)
	}

	// Read the content of the file
	fileBytes, err := fileRef.ReadFile(ctx, client.ReadRequest{
		Filename: filename,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to read %s: %w", clientOptFilename, err)
	}

	return fileBytes, nil
}

func annotateRes(annots map[string]string, res *client.Result) (*client.Result, error) {
	ref, err := res.SingleRef()
	if err != nil {
		return nil, fmt.Errorf("Failed te get reference of LLB solve result : %v",err)
	}

	config := ocispecs.Image{
		Platform: ocispecs.Platform{
			Architecture: "amd64",
			OS:           "linux",
		},
		RootFS: ocispecs.RootFS{
			Type: "layers",
		},
		Config: ocispecs.ImageConfig{
			WorkingDir: "/",
			Entrypoint: []string{"/hello2"},
			Labels:     annots,
		},
	}

	uruncJSONBytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal urunc json: %v", err)
	}
	res.AddMeta(exptypes.ExporterImageConfigKey, uruncJSONBytes)
	for annot, val := range annots {
		res.AddMeta(exptypes.AnnotationManifestKey(nil, annot), []byte(val))
	}
	res.SetRef(ref)

	return res, nil
}

func punBuilder(ctx context.Context, c client.Client) (*client.Result, error) {
	// Get the Build options from buildkit
	packOpts := c.BuildOpts().Opts

	// Get the file that contains the instructions
	packFile := packOpts[clientOptFilename]
	if packFile == "" {
		return nil, fmt.Errorf("%s: was not provided", clientOptFilename)
	}

	// Fetch and read contents of user-specified file in build context
	fileBytes, err := readFileFromLLB(ctx, c, packFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch and read %s: %w", clientOptFilename, err)
	}

	// Parse packing instructions
	packInst, err := parseFile(fileBytes)
	if err != nil {
		return nil, fmt.Errorf("Error parsing packing instructions", err)
	}

	// Create the LLB definiton
	dt, err := constructLLB(*packInst)
	if err != nil {
		return nil, fmt.Errorf("Failed to create LLB definition : %v\n", err)
	}

	// Pass LLB to buildkit
	result, err := c.Solve(ctx, client.SolveRequest{
		Definition: dt.ToPB(),
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve LLB: %v",err)
	}

	// Add annotations and Labels in output image
	result, err = annotateRes(packInst.Annots, result)
	if err != nil {
		return nil, fmt.Errorf("Failed to annotate final image: %v",err)
	}

	return result, nil
}

func main() {
	var cliOpts CLIOpts
	var packInst *PackInstructions

	cliOpts = parseCLIOpts()

	if cliOpts.Version {
		fmt.Printf("pun version %s\n", version)
		return
	}
	if !cliOpts.PrintLLB {
		// Run as buildkit frontend
		ctx := appcontext.Context()
		if err := grpcclient.RunFromEnvironment(ctx, punBuilder); err != nil {
			fmt.Printf("Could not start grpcclient: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Normal local execution to print LLB
	if cliOpts.ContainerFile == "" {
		fmt.Println("Please specify the Containerfile")
		fmt.Println("Use -h or --help for more info")
		os.Exit(1)
	}

	CntrFileContent, err := ioutil.ReadFile(cliOpts.ContainerFile)
	if err != nil {
		fmt.Printf("Failed to read %s: %v\n", cliOpts.ContainerFile, err)
		os.Exit(1)
	}

	// Parse file with packaging instructions
	packInst, err = parseFile(CntrFileContent)
	if err != nil {
		fmt.Println("Error parsing packing instructions", err)
		os.Exit(1)
	}

	// Create the LLB definition
	dt, err := constructLLB(*packInst)
	if err != nil {
		fmt.Printf("Failed to create LLB definition : %v\n", err)
		os.Exit(1)
	}

	// Print the LLB to give it as input in buildctl
	llb.WriteTo(dt, os.Stdout)
}
