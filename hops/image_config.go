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

func getBaseConfig(ctx context.Context, c client.Client, ref string, mon string) (ocispecs.ImageConfig, error) {
	if ref == "" || ref == "scratch" {
		return ocispecs.ImageConfig{}, nil
	}

	baseRef, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return ocispecs.ImageConfig{}, fmt.Errorf("failed to parse image name %s: %v", ref, err)
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
			LogName: "resolving image metadata for " + baseImageName,
			ImageOpt: &sourceresolver.ResolveImageOpt{
				Platform: &plat,
			},
		})
	if err != nil {
		return ocispecs.ImageConfig{}, fmt.Errorf("failed to get image config from %s: %v", baseImageName, err)
	}

	var cfg ocispecs.ImageConfig
	err = json.Unmarshal(config, &cfg)
	if err != nil {
		return ocispecs.ImageConfig{}, fmt.Errorf("failed to unmarshal image config of %s: %v", baseImageName, err)
	}

	return cfg, nil
}

func updateImage(img ocispecs.Image, annots map[string]string) ocispecs.Image {
	img.Platform = ocispecs.Platform{
		Architecture: runtime.GOARCH,
		OS:           "linux",
	}

	if img.RootFS.Type == "" {
		img.RootFS = ocispecs.RootFS{
			Type: "layers",
		}
	}

	if img.Config.Labels == nil {
		img.Config.Labels = make(map[string]string)
	}
	for k, v := range annots {
		img.Config.Labels[k] = v
	}

	return img
}

func ApplyConfig(res *client.Result, annots map[string]string, image ocispecs.Image) error {
	ref, err := res.SingleRef()
	if err != nil {
		return fmt.Errorf("Failed to get reference build result: %v", err)
	}

	imageConfigJSON, err := json.Marshal(image)
	if err != nil {
		return fmt.Errorf("Failed to marshal image config: %v", err)
	}
	res.AddMeta(exptypes.ExporterImageConfigKey, imageConfigJSON)
	for annot, val := range annots {
		res.AddMeta(exptypes.AnnotationManifestKey(nil, annot), []byte(val))
	}
	res.SetRef(ref)

	return nil
}
