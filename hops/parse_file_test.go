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
	"strconv"
	"testing"

	"github.com/moby/buildkit/client/llb/sourceresolver"
	"github.com/moby/buildkit/frontend/gateway/client"
	digest "github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

// fakeConfigClient is a gateway client that only knows how to resolve image
// configs. Any other method panics through the nil embedded interface.
type fakeConfigClient struct {
	client.Client
	configs  map[string]ocispecs.Image
	resolved []string
}

func (f *fakeConfigClient) ResolveImageConfig(_ context.Context, ref string, _ sourceresolver.Opt) (string, digest.Digest, []byte, error) {
	f.resolved = append(f.resolved, ref)
	img, ok := f.configs[ref]
	if !ok {
		return "", "", nil, fmt.Errorf("unexpected image ref %s", ref)
	}
	b, err := json.Marshal(img)
	return ref, "", b, err
}

type ParseTestInfo struct {
	name string
	// The contents of the file
	input       []byte
	expectError bool
	errorText   string
}

func TestParseBunnyfile(t *testing.T) {
	tests := []ParseTestInfo{
		{
			name: "Valid all fields",
			input: []byte(`
version: 0.1
platforms:
  framework: foo
  monitor: bar
rootfs:
  from: local
  path: foo
kernel:
  from: local
  path: foo
cmdline: "foo bar"
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Valid without rootfs",
			input: []byte(`
version: 0.1
platforms:
  framework: foo
  monitor: bar
kernel:
  from: local
  path: foo
cmdline: "foo bar"
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Valid without cmdline",
			input: []byte(`
version: 0.1
platforms:
  framework: foo
  monitor: bar
kernel:
  from: local
  path: foo
rootfs:
  from: local
  path: foo
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Valid without cmdline and rootfs",
			input: []byte(`
version: 0.1
platforms:
  framework: foo
  monitor: bar
kernel:
  from: local
  path: foo
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Valid with workdir",
			input: []byte(`
version: 0.1
platforms:
  framework: foo
  monitor: bar
kernel:
  from: local
  path: foo
workdir: /app
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Invalid relative workdir",
			input: []byte(`
version: 0.1
platforms:
  framework: foo
  monitor: bar
kernel:
  from: local
  path: foo
workdir: app
`),
			expectError: true,
			errorText:   "The workdir field must be an absolute path",
		},
		{
			name:        "Invalid yaml",
			input:       []byte(`version: "0.1"::`),
			expectError: true,
			errorText:   "yaml: mapping values are not allowed in this context",
		},
		{
			name: "Invalid unsupported version",
			input: []byte(`
version: 999.99
platforms:
  framework: foo
  monitor: bar
rootfs:
  from: local
  path: foo
kernel:
  from: local
  path: foo
cmdline: "foo bar"
`),
			expectError: true,
			errorText:   "Unsupported version",
		},
		{
			name: "Invalid missing platform",
			input: []byte(`
version: 0.1
rootfs:
  from: local
  path: foo
kernel:
  from: local
  path: foo
cmdline: "foo bar"
`),
			expectError: true,
			errorText:   "The framework field of platforms is necessary",
		},
		{
			name: "Invalid missing kernel",
			input: []byte(`
version: 0.1
platforms:
  framework: foo
  monitor: bar
rootfs:
  from: local
  path: foo
cmdline: "foo bar"
`),
			expectError: true,
			errorText:   "The from field of kernel is necessary",
		},
		{
			name: "Invalid wrong rootfs",
			input: []byte(`
version: 0.1
platforms:
  framework: foo
  monitor: bar
kernel:
  from: local
  path: foo
rootfs:
  path: foo
cmdline: "foo bar"
`),
			expectError: true,
			errorText:   "The from field of rootfs can not be empty or scratch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, err := ParseBunnyfile(tc.input)
			if tc.expectError {
				require.Error(t, err, "Expected an error, got nil")
				require.Nil(t, h)
				require.Contains(t, err.Error(), tc.errorText)
			} else {
				// TODO: Update test to check rootfs and cmdline
				require.NoError(t, err)
				require.NotNil(t, h)
			}
		})
	}
}

func TestParseBunnyfileWorkdir(t *testing.T) {
	tests := []struct {
		workdir  string
		expected string
	}{
		{"", ""},
		{"/", "/"},
		{"/app", "/app"},
		{"/app/", "/app"},
		{"//app//data/.", "/app/data"},
		{"/missing/..", "/"},
		{"/app/../data", "/data"},
	}
	for _, tc := range tests {
		t.Run(strconv.Quote(tc.workdir), func(t *testing.T) {
			input := `
version: 0.1
platforms:
  framework: foo
  monitor: bar
kernel:
  from: local
  path: foo
`
			if tc.workdir != "" {
				input += "workdir: " + strconv.Quote(tc.workdir) + "\n"
			}
			h, err := ParseBunnyfile([]byte(input))
			require.NoError(t, err)
			require.Equal(t, tc.expected, h.Workdir)
		})
	}
}

func TestParsefile(t *testing.T) {
	tests := []ParseTestInfo{
		{
			name: "Valid Containerfile",
			input: []byte(`#syntax=foo
FROM scratch
COPY foo bar
LABEL foo=bar
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Valid Containerfile empty first lines",
			input: []byte(`#syntax=foo




FROM scratch
COPY foo bar
LABEL foo=bar
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Valid bunnyfile",
			input: []byte(`#syntax=foo
version: 0.1
platforms:
  framework: foo
  monitor: bar
rootfs:
  from: local
  path: foo
kernel:
  from: local
  path: foo
cmdline: "foo bar"
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Valid bunnyfile empty first line",
			input: []byte(`#syntax=foo




version: 0.1
platforms:
  framework: foo
  monitor: bar
rootfs:
  from: local
  path: foo
kernel:
  from: local
  path: foo
cmdline: "foo bar"
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Invalid no instructions",
			input: []byte(`#syntaxax=foo
`),
			expectError: true,
			errorText:   "The version field is necessary",
		},
		{
			name: "Invalid Containerfile unsupported command",
			input: []byte(`#syntax=foo
FROM foo
WHAT
`),
			expectError: true,
			errorText:   "while parsing as containerfile",
		},
		{
			name: "Invalid bunnyfile missing platform",
			input: []byte(`#syntax=foo
version: 0.1
cmdline: "foo bar"
`),
			expectError: true,
			errorText:   "The framework field of platforms is necessary",
		},
		{
			name:        "Invalid file single line",
			input:       []byte(`#syntax=foo`),
			expectError: true,
			errorText:   "The version field is necessary",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i, err := ParseFile(context.TODO(), tc.input, "foo", nil)
			if tc.expectError {
				require.Error(t, err, "Expected an error, got nil")
				require.Nil(t, i)
				require.Contains(t, err.Error(), tc.errorText)
			} else {
				require.NoError(t, err)
				require.NotNil(t, i)
			}
		})
	}
}

func TestParseHopsToPackWorkdir(t *testing.T) {
	const baseRef = "harbor.nbfc.io/foo/base:latest"
	baseImg := ocispecs.Image{
		Config: ocispecs.ImageConfig{
			WorkingDir: "/base",
			Env:        []string{"BASE=1"},
		},
	}
	// A raw rootfs from a registry becomes the image base, so its config
	// is fetched and merged with the bunnyfile values.
	withBase := `
version: v0.1
platforms:
  framework: rumprun
  monitor: qemu
rootfs:
  from: harbor.nbfc.io/foo/base
kernel:
  from: local
  path: kernel
`
	// Only a local kernel: there is no base image to inherit from.
	noBase := `
version: v0.1
platforms:
  framework: rumprun
  monitor: qemu
kernel:
  from: local
  path: kernel
`
	tests := []struct {
		name       string
		input      string
		configs    map[string]ocispecs.Image
		resolved   []string
		expectedWD string
	}{
		{
			name:       "Base workdir is inherited when workdir is unset",
			input:      withBase,
			configs:    map[string]ocispecs.Image{baseRef: baseImg},
			resolved:   []string{baseRef},
			expectedWD: "/base",
		},
		{
			name:       "Workdir overrides the base workdir",
			input:      withBase + "workdir: /app\n",
			configs:    map[string]ocispecs.Image{baseRef: baseImg},
			resolved:   []string{baseRef},
			expectedWD: "/app",
		},
		{
			name:       "Root workdir overrides the base workdir",
			input:      withBase + "workdir: /\n",
			configs:    map[string]ocispecs.Image{baseRef: baseImg},
			resolved:   []string{baseRef},
			expectedWD: "/",
		},
		{
			name:       "Non-canonical workdir is cleaned",
			input:      withBase + "workdir: /missing/..\n",
			configs:    map[string]ocispecs.Image{baseRef: baseImg},
			resolved:   []string{baseRef},
			expectedWD: "/",
		},
		{
			name:       "Workdir is set without a base image",
			input:      noBase + "workdir: /app\n",
			resolved:   nil,
			expectedWD: "/app",
		},
		{
			name:       "No workdir and no base image leaves it empty",
			input:      noBase,
			resolved:   nil,
			expectedWD: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &fakeConfigClient{configs: tc.configs}
			i, err := hopsToPack(context.TODO(), []byte(tc.input), "context", c)
			require.NoError(t, err)
			require.NotNil(t, i)
			require.Equal(t, tc.resolved, c.resolved)
			require.Equal(t, tc.expectedWD, i.Img.Config.WorkingDir)
			if tc.configs != nil {
				// The rest of the base config must survive the merge.
				require.Contains(t, i.Img.Config.Env, "BASE=1")
			}
		})
	}

	t.Run("Relative workdir is rejected", func(t *testing.T) {
		c := &fakeConfigClient{}
		i, err := hopsToPack(context.TODO(), []byte(noBase+"workdir: app\n"), "context", c)
		require.ErrorContains(t, err, "The workdir field must be an absolute path")
		require.Nil(t, i)
		require.Empty(t, c.resolved)
	})
}
