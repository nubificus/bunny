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
	"testing"

	"github.com/moby/buildkit/solver/pb"
	"github.com/stretchr/testify/require"
)

// nolint: dupl
func TestGenericNew(t *testing.T) {
	t.Run("Without rootfs type", func(t *testing.T) {
		plat := Platform{
			Version: "1.0",
			Monitor: "foo",
			Arch:    "bar",
		}
		rootfs := Rootfs{}

		generic := NewGeneric(plat, rootfs)
		require.Equal(t, plat.Version, generic.Version)
		require.Equal(t, plat.Monitor, generic.Monitor)
		require.Equal(t, plat.Arch, generic.Arch)
		require.Equal(t, "raw", generic.Rootfs.Type)
	})
	t.Run("With rootfs type", func(t *testing.T) {
		plat := Platform{
			Version: "1.0",
			Monitor: "foo",
			Arch:    "bar",
		}
		rootfs := Rootfs{
			From:     "foo",
			Path:     "bar",
			Type:     "initrd",
			Includes: []string{},
		}

		generic := NewGeneric(plat, rootfs)
		require.Equal(t, plat.Version, generic.Version)
		require.Equal(t, plat.Monitor, generic.Monitor)
		require.Equal(t, plat.Arch, generic.Arch)
		require.Equal(t, rootfs.From, generic.Rootfs.From)
		require.Equal(t, rootfs.Path, generic.Rootfs.Path)
		require.Equal(t, rootfs.Type, generic.Rootfs.Type)
	})
}

func TestGenericName(t *testing.T) {
	generic := &GenericInfo{}

	require.Equal(t, genericName, generic.Name())

}

func TestGenericGetRootfsType(t *testing.T) {
	t.Run("Without rootfs type", func(t *testing.T) {
		plat := Platform{
			Version: "1.0",
			Monitor: "foo",
			Arch:    "bar",
		}
		rootfs := Rootfs{}

		generic := NewGeneric(plat, rootfs)
		require.Equal(t, "raw", generic.GetRootfsType())

	})
	t.Run("With rootfs type", func(t *testing.T) {
		plat := Platform{
			Version: "1.0",
			Monitor: "foo",
			Arch:    "bar",
		}
		rootfs := Rootfs{
			From:     "foo",
			Path:     "bar",
			Type:     "initrd",
			Includes: []string{},
		}

		generic := NewGeneric(plat, rootfs)
		require.Equal(t, "initrd", generic.GetRootfsType())
	})
}

func TestGenericSupportsRootfsType(t *testing.T) {
	generic := &GenericInfo{}
	t.Run("Supported rootfs type", func(t *testing.T) {
		require.Equal(t, true, generic.SupportsRootfsType("initrd"))

	})
	t.Run("Unsupported rootfs type", func(t *testing.T) {
		require.Equal(t, true, generic.SupportsRootfsType("raw"))

	})
}

func TestGenericSupportsFsType(t *testing.T) {
	generic := &GenericInfo{}
	require.Equal(t, true, generic.SupportsFsType("foo"))

}

func TestGenericSupportsMonitor(t *testing.T) {
	generic := &GenericInfo{}
	require.Equal(t, true, generic.SupportsMonitor("foo"))
}

func TestGenericSupportsArch(t *testing.T) {
	generic := &GenericInfo{}
	require.Equal(t, true, generic.SupportsArch("foo"))
}

func TestGenericCreateRootfs(t *testing.T) {
	t.Run("With raw rootfs type", func(t *testing.T) {
		plat := Platform{
			Version: "1.0",
			Monitor: "foo",
			Arch:    "bar",
		}
		rootfs := Rootfs{
			From:     "foo",
			Path:     "bar",
			Type:     "raw",
			Includes: []string{"foo:bar"},
		}

		generic := NewGeneric(plat, rootfs)
		state, err := generic.CreateRootfs("context")
		require.NoError(t, err)
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		m, arr := parseDef(t, def.Def)
		// We expect 3 output states: 2 for copy files and 1 for final
		// output
		require.Equal(t, 3, len(arr))
		// The last one (final output) should just have a single input
		last := arr[len(arr)-1]
		require.Equal(t, 1, len(last.Inputs))
		// which is the copy op
		lastInputDgst := last.Inputs[0].Digest
		require.Equal(t, m[lastInputDgst], arr[1])
		c := arr[1]
		lDgst := c.Inputs[0].Digest
		require.Equal(t, m[lDgst], arr[0])
		l := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", l.Identifier)
		cf := c.Op.(*pb.Op_File).File
		require.Equal(t, 1, len(cf.Actions))
		cp := cf.Actions[0].Action.(*pb.FileAction_Copy).Copy
		require.Equal(t, "/foo", cp.Src)
		require.Equal(t, "/bar", cp.Dest)
	})
	t.Run("With initrd rootfs type and multiple files", func(t *testing.T) {
		plat := Platform{
			Version: "1.0",
			Monitor: "foo",
			Arch:    "bar",
		}
		rootfs := Rootfs{
			From:     "foo",
			Path:     "bar",
			Type:     "initrd",
			Includes: []string{"foo:bar", "ka"},
		}

		generic := NewGeneric(plat, rootfs)
		state, err := generic.CreateRootfs("context")
		require.NoError(t, err)
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		m, arr := parseDef(t, def.Def)
		// We expect 8 output states: 3 for copy files, 4 for initrd and
		// 1 for final output
		require.Equal(t, 8, len(arr))
		// The last one (final output) should just have a single input
		last := arr[len(arr)-1]
		require.Equal(t, 1, len(last.Inputs))
		// which is the exec op of initrd
		lastInputDgst := last.Inputs[0].Digest
		require.Equal(t, m[lastInputDgst], arr[6])
		e := arr[6]
		exec := e.Op.(*pb.Op_Exec).Exec
		require.Equal(t, 3, len(exec.Meta.Args))
		// the exec should have three inputs
		require.Equal(t, 3, len(e.Inputs))
		// the last of which should be the state with the initrd content
		// that we passed as argument
		cDgst := e.Inputs[2].Digest
		require.Equal(t, m[cDgst], arr[5])
		c1 := arr[5]
		cf1 := c1.Op.(*pb.Op_File).File
		require.Equal(t, 1, len(cf1.Actions))
		cp1 := cf1.Actions[0].Action.(*pb.FileAction_Copy).Copy
		require.Equal(t, "/ka", cp1.Src)
		require.Equal(t, "/ka", cp1.Dest)
		require.Equal(t, 2, len(c1.Inputs))
		c2Dgst := c1.Inputs[0].Digest
		require.Equal(t, m[c2Dgst], arr[4])
		c2 := arr[4]
		cf2 := c2.Op.(*pb.Op_File).File
		cp2 := cf2.Actions[0].Action.(*pb.FileAction_Copy).Copy
		require.Equal(t, "/foo", cp2.Src)
		require.Equal(t, "/bar", cp2.Dest)
		require.Equal(t, 1, len(c2.Inputs))
		require.Equal(t, c2.Inputs[0], c1.Inputs[1])
		locDgst := c1.Inputs[1].Digest
		require.Equal(t, m[locDgst], arr[3])
		l := arr[3].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", l.Identifier)
		// the second of which should be the state that creates the directory
		// where the cpio file will get stored
		oDgst := e.Inputs[1].Digest
		require.Equal(t, m[oDgst], arr[2])
		// the first of which should be the state that creates the tmp directory
		tDgst := e.Inputs[0].Digest
		require.Equal(t, m[tDgst], arr[1])
		tmp := arr[1]
		// The state that create the tmp directory has one input, which is the source of
		// the tools
		toolDgst := tmp.Inputs[0].Digest
		require.Equal(t, m[toolDgst], arr[0])
	})
}

func TestGenericBuildKernel(t *testing.T) {
	generic := &GenericInfo{}
	state := generic.BuildKernel("ctx")
	def, err := state.Marshal(context.TODO())

	require.NoError(t, err)
	_, arr := parseDef(t, def.Def)
	require.Equal(t, 0, len(arr))
}
