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
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"

	"bunny/hops"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/gateway/grpcclient"
	"github.com/moby/buildkit/util/appcontext"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	buildContextName  string = "context"
	clientOptFilename string = "filename"
)

type CLIOpts struct {
	// If set just print the version and exit
	Version bool
	// The Containerfile to be used for building the unikernel container
	ContainerFile string
	// Choose the execution mode. If set, then bunny will not act as a
	// buidlkit frontend. Instead it will just print the LLB.
	PrintLLB bool
}

var version string

func usage() {

	fmt.Println("Usage of bunny")
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

func readFileFromLLB(ctx context.Context, c client.Client, filename string) ([]byte, error) {
	// Get the file from client's context
	fileSrc := llb.Local(buildContextName, llb.IncludePatterns([]string{filename}),
		llb.WithCustomName("Internal:Read-"+filename))
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
		return nil, fmt.Errorf("Failed to get reference of result for fetching %s: %w", clientOptFilename, err)
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
		return nil, fmt.Errorf("Failed te get reference build result: %v", err)
	}

	config := ocispecs.Image{
		Platform: ocispecs.Platform{
			Architecture: runtime.GOARCH,
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

	imageConfig, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal image config: %v", err)
	}
	res.AddMeta(exptypes.ExporterImageConfigKey, imageConfig)
	for annot, val := range annots {
		res.AddMeta(exptypes.AnnotationManifestKey(nil, annot), []byte(val))
	}
	res.SetRef(ref)

	return res, nil
}

func bunnyBuilder(ctx context.Context, c client.Client) (*client.Result, error) {
	// Get the Build options from buildkit
	buildOpts := c.BuildOpts().Opts

	// Get the file that contains the instructions
	bunnyFile := buildOpts[clientOptFilename]
	if bunnyFile == "" {
		return nil, fmt.Errorf("Could not find %s", clientOptFilename)
	}

	// Fetch and read contents of user-specified file in build context
	fileBytes, err := readFileFromLLB(ctx, c, bunnyFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch and read %s: %w", clientOptFilename, err)
	}

	// Parse packaging/building instructions
	packInst, err := hops.ParseFile(fileBytes, buildContextName)
	if err != nil {
		return nil, fmt.Errorf("Error parsing building instructions: %v", err)
	}

	// Create the LLB definition of packing the final image
	dt, err := hops.PackLLB(*packInst)
	if err != nil {
		return nil, fmt.Errorf("Could not create LLB definition: %v", err)
	}

	// Pass LLB to buildkit
	result, err := c.Solve(ctx, client.SolveRequest{
		Definition: dt.ToPB(),
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to resolve LLB: %v", err)
	}

	// Add annotations and Labels in output image
	result, err = annotateRes(packInst.Annots, result)
	if err != nil {
		return nil, fmt.Errorf("Failed to annotate final image: %v", err)
	}

	return result, nil
}

func main() {
	var cliOpts CLIOpts
	var packInst *hops.PackInstructions

	cliOpts = parseCLIOpts()

	if cliOpts.Version {
		fmt.Printf("bunny version %s\n", version)
		return
	}

	if !cliOpts.PrintLLB {
		// Run as buildkit frontend
		ctx := appcontext.Context()
		if err := grpcclient.RunFromEnvironment(ctx, bunnyBuilder); err != nil {
			fmt.Fprintf(os.Stderr, "Error: Could not connect to buildkit: %v\n", err)
			os.Exit(1)
		}

		return
	}

	// Normal local execution to print LLB
	if cliOpts.ContainerFile == "" {
		fmt.Fprintf(os.Stderr, "Error: No instructions file as input\n")
		fmt.Fprintf(os.Stderr, "Use -h or --help for more info\n")
		os.Exit(1)
	}

	CntrFileContent, err := os.ReadFile(cliOpts.ContainerFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not read %s: %v\n", cliOpts.ContainerFile, err)
		os.Exit(1)
	}

	// Parse file with packaging/building instructions
	packInst, err = hops.ParseFile(CntrFileContent, buildContextName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not parse building instructions: %v\n", err)
		os.Exit(1)
	}

	// Create the LLB definition of packing the final image
	dt, err := hops.PackLLB(*packInst)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not create LLB definition: %v\n", err)
		os.Exit(1)
	}

	// Print the LLB to give it as input in buildctl
	err = llb.WriteTo(dt, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Could not write LLB to stdout: %v\n", err)
		os.Exit(1)
	}
}
