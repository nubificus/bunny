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
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/distribution/reference"
	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"github.com/moby/buildkit/frontend/gateway/client"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

type ResultAndConfig struct {
	// The result
	Res *client.Result
	// The OCI config of the final image
	OCIConfig ocispecs.Image
}

func (rc *ResultAndConfig) GetBaseConfig(ctx context.Context, c client.Client, ref string, mon string) error {
	if ref == "" || ref == "scratch" {
		return nil
	}

	baseRef, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return fmt.Errorf("Failed to parse image name %s: %v", ref, err)
	}
	baseImageName := reference.TagNameOnly(baseRef).String()

	plat := ocispecs.Platform{
		Architecture: runtime.GOARCH,
	}
	if strings.HasPrefix(ref, unikraftHub) {
		// Define the platform to qemu/amd64 so we can pull unikraft images
		plat.OS = mon
	} else {
		plat.OS = "linux"
	}
	_, _, config, err := c.ResolveImageConfig(ctx, baseImageName,
		sourceresolver.Opt{
			LogName:  "resolving image metadata for " + baseImageName,
			Platform: &plat,
		})
	if err != nil {
		return fmt.Errorf("Failed to get image config from %s: %v", baseImageName, err)
	}

	err = json.Unmarshal(config, &rc.OCIConfig)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal image config of %ss: %v", baseImageName, err)
	}

	return nil
}

func (rc *ResultAndConfig) UpdateConfig(annots map[string]string, cmd []string) {
	plat := ocispecs.Platform{
		Architecture: runtime.GOARCH,
		OS:           "linux",
	}
	rfs := ocispecs.RootFS{
		Type: "layers",
	}

	// Overwrite platform and rootfs to remove unikraft specific platform
	// and initialize empty configs.
	rc.OCIConfig.Platform = plat
	rc.OCIConfig.RootFS = rfs
	// Overwrite Cmd and entrypoint based on the values of bunnyfile
	rc.OCIConfig.Config.Cmd = cmd
	rc.OCIConfig.Config.Entrypoint = []string{}

	if rc.OCIConfig.Config.Labels == nil {
		rc.OCIConfig.Config.Labels = make(map[string]string)
	}
	for k, v := range annots {
		rc.OCIConfig.Config.Labels[k] = v
	}
}

func (rc *ResultAndConfig) ApplyConfig(annots map[string]string) error {
	res := rc.Res
	ref, err := res.SingleRef()
	if err != nil {
		return fmt.Errorf("Failed te get reference build result: %v", err)
	}

	imageConfig, err := json.Marshal(rc.OCIConfig)
	if err != nil {
		return fmt.Errorf("Failed to marshal image config: %v", err)
	}
	res.AddMeta(exptypes.ExporterImageConfigKey, imageConfig)
	for annot, val := range annots {
		res.AddMeta(exptypes.AnnotationManifestKey(nil, annot), []byte(val))
	}
	res.SetRef(ref)

	rc.Res = res
	return nil
}
