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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type testInfo struct {
	name string
	// In case the input is a struct, we can define the values of the struct
	// with a string in the following form: <value1>/<value2>/<value3>
	// each substring between "/" defines the value of the respective
	// field in the struct
	input       string
	expectError bool
	errorText   string
}

func TestValidateBunnyfileVersion(t *testing.T) {
	tests := []testInfo{
		{
			name:        "Invalid empty version",
			input:       "",
			expectError: true,
			errorText:   "The version field is necessary",
		},
		{
			name:        "Invalid malformed version",
			input:       "not.a.version",
			expectError: true,
			errorText:   "Could not parse version",
		},
		{
			name:        "Valid supported version",
			input:       "0.1",
			expectError: false,
		},
		{
			name:        "Valid older version",
			input:       "0.0.9",
			expectError: false,
		},
		{
			name:        "Invalid newer version",
			input:       "0.2.0",
			expectError: true,
			errorText:   "Unsupported version",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckBunnyfileVersion(tc.input)
			if tc.expectError {
				require.Error(t, err, "Expected an error, got nil")
				require.Contains(t, err.Error(), tc.errorText)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// nolint: dupl
func TestValidateBunnyfilePlatform(t *testing.T) {
	tests := []testInfo{
		{
			name:        "Valid framework and monitor",
			input:       "foo/bar",
			expectError: false,
			errorText:   "",
		},
		{
			name:        "Invalid missing framework",
			input:       "/bar",
			expectError: true,
			errorText:   "framework",
		},
		{
			name:        "Invalid missing monitor",
			input:       "foo/",
			expectError: true,
			errorText:   "monitor",
		},
		{
			name:        "Invalid missing framework and monitor",
			input:       "/",
			expectError: true,
			errorText:   "framework",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fields := strings.Split(tc.input, "/")
			plat := Platform{Framework: fields[0], Monitor: fields[1]}
			err := ValidatePlatform(plat)
			if tc.expectError {
				require.Error(t, err, "Expected an error, got nil")
				require.Contains(t, err.Error(), tc.errorText)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// nolint: dupl
func TestValidateBunnyfileRootfs(t *testing.T) {
	tests := []testInfo{
		{
			name:        "Valid from and path",
			input:       "foo/bar//",
			expectError: false,
			errorText:   "",
		},
		{
			name:        "Valid scratch and include",
			input:       "scratch///srcs:dst",
			expectError: false,
			errorText:   "",
		},
		{
			name:        "Valid empty from, non-empty include",
			input:       "///srcs:dst",
			expectError: false,
			errorText:   "",
		},
		{
			name:        "Valid scratch, with type and include",
			input:       "scratch//type/srcs:dst",
			expectError: false,
			errorText:   "",
		},
		{
			name:        "Valid empty from, non-empty type,includes",
			input:       "//type/src",
			expectError: false,
			errorText:   "",
		},
		{
			name:        "Invalid empty from, non-empty path",
			input:       "/path//",
			expectError: true,
			errorText:   "The from field of rootfs can not be empty",
		},
		{
			name:        "Invalid scratch and path",
			input:       "scratch/path//",
			expectError: true,
			errorText:   "The from field of rootfs can not be empty",
		},
		{
			name:        "Invalid non-empty path, type raw",
			input:       "local/path/raw/",
			expectError: true,
			errorText:   "The path field in rootfs can not be combined",
		},
		{
			name:        "Invalid from local with type raw",
			input:       "local//raw/",
			expectError: true,
			errorText:   "If type of rootfs is raw, then from can not",
		},
		{
			name:        "Invalid from local with includes",
			input:       "local/path//foo:bar",
			expectError: true,
			errorText:   "Adding files to an existing rootfs is not yet",
		},
		{
			name:        "Invalid include with no source",
			input:       "///:bar",
			expectError: true,
			errorText:   "Invalid syntax in rootf's include",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fields := strings.Split(tc.input, "/")
			var rfs Rootfs
			rfs.From = fields[0]
			rfs.Path = fields[1]
			rfs.Type = fields[2]
			if fields[3] != "" {
				rfs.Includes = []string{fields[3]}
			}
			err := ValidateRootfs(rfs)
			if tc.expectError {
				require.Error(t, err, "Expected an error, got nil")
				require.Contains(t, err.Error(), tc.errorText)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// nolint: dupl
func TestValidateBunnyfileKernel(t *testing.T) {
	tests := []testInfo{
		{
			name:        "Valid from and path",
			input:       "foo/bar",
			expectError: false,
			errorText:   "",
		},
		{
			name:        "Invalid empty from, non-empty path",
			input:       "/path",
			expectError: true,
			errorText:   "The from field of kernel is necessary",
		},
		{
			name:        "Invalid empty path, non-empty from",
			input:       "from/",
			expectError: true,
			errorText:   "The path field of kernel is necessary",
		},
		{
			name:        "Invalid empty from and path",
			input:       "/",
			expectError: true,
			errorText:   "The from field of kernel is necessary",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fields := strings.Split(tc.input, "/")
			k := Kernel{From: fields[0], Path: fields[1]}
			err := ValidateKernel(k)
			if tc.expectError {
				require.Error(t, err, "Expected an error, got nil")
				require.Contains(t, err.Error(), tc.errorText)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
