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
	"testing"

	"github.com/stretchr/testify/require"
)

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
				require.NoError(t, err)
				require.NotNil(t, h)
			}
		})
	}
}

func TestParseContainerfileSyntax(t *testing.T) {
	tests := []ParseTestInfo{
		{
			name: "Valid all fields",
			input: []byte(`
FROM foo
COPY foo bar
LABEL foo=bar
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Valid no copy",
			input: []byte(`
FROM foo
LABEL foo=bar
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Valid no label",
			input: []byte(`
FROM foo
COPY foo bar
LABEL foo=bar
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Invalid Containerfile",
			input: []byte(`
version: 0.1
`),
			expectError: true,
			errorText:   "unknown instruction: version",
		},
		{
			name: "Invalid unsupported command",
			input: []byte(`
FROM foo
RUN bar
`),
			expectError: true,
			errorText:   "Unsupported command",
		},
		{
			name: "Invalid multi stage",
			input: []byte(`
FROM foo
FROM bar
`),
			expectError: true,
			errorText:   "Multi-stage builds are not supported",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i, err := ParseContainerfile(tc.input, "foo")
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

func TestParsefile(t *testing.T) {
	tests := []ParseTestInfo{
		{
			name: "Valid Containerfile",
			input: []byte(`#syntax=foo
FROM foo
COPY foo bar
LABEL foo=bar
`),
			expectError: false,
			errorText:   "",
		},
		{
			name: "Valid Containerfile empty first lines",
			input: []byte(`#syntax=foo




FROM foo
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
RUN bar
`),
			expectError: true,
			errorText:   "Unsupported command",
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i, err := ParseFile(tc.input, "foo")
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
