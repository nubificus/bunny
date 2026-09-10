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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFrameworkPreferredOS(t *testing.T) {
	tests := []struct {
		name      string
		framework string
		monitor   string
		expected  string
	}{
		{"unikraft qemu", "unikraft", "qemu", "qemu"},
		{"unikraft firecracker maps to fc", "unikraft", "firecracker", "fc"},
		{"freebsd ignores monitor", "freebsd", "qemu", "freebsd"},
		{"generic linux framework", "linux", "qemu", ""},
		{"unknown framework", "rumprun", "qemu", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFramework(Platform{Framework: tc.framework, Monitor: tc.monitor}, Rootfs{})
			require.Equal(t, tc.expected, f.PreferredOS())
		})
	}
}
