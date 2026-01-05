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
	"testing"

	"github.com/moby/buildkit/solver/pb"
	"github.com/stretchr/testify/require"
)

// nolint: dupl
func TestUnikraftNew(t *testing.T) {
	t.Run("Without rootfs type", func(t *testing.T) {
		plat := Platform{
			Version: "1.0",
			Monitor: "foo",
			Arch:    "bar",
		}
		rootfs := Rootfs{}

		unikraft := NewUnikraft(plat, rootfs)
		require.Equal(t, plat.Version, unikraft.Version)
		require.Equal(t, plat.Monitor, unikraft.Monitor)
		require.Equal(t, plat.Arch, unikraft.Arch)
		require.Equal(t, "initrd", unikraft.Rootfs.Type)
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
			Type:     "raw",
			Includes: []string{},
		}

		unikraft := NewUnikraft(plat, rootfs)
		require.Equal(t, plat.Version, unikraft.Version)
		require.Equal(t, plat.Monitor, unikraft.Monitor)
		require.Equal(t, plat.Arch, unikraft.Arch)
		require.Equal(t, rootfs.From, unikraft.Rootfs.From)
		require.Equal(t, rootfs.Path, unikraft.Rootfs.Path)
		require.Equal(t, rootfs.Type, unikraft.Rootfs.Type)
	})
}

func TestUnikraftName(t *testing.T) {
	unikraft := &UnikraftInfo{}

	require.Equal(t, unikraftName, unikraft.Name())

}

func TestUnikraftGetRootfsType(t *testing.T) {
	t.Run("Without rootfs type", func(t *testing.T) {
		plat := Platform{
			Version: "1.0",
			Monitor: "foo",
			Arch:    "bar",
		}
		rootfs := Rootfs{}

		unikraft := NewUnikraft(plat, rootfs)
		require.Equal(t, "initrd", unikraft.GetRootfsType())

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
			Type:     "raw",
			Includes: []string{},
		}

		unikraft := NewUnikraft(plat, rootfs)
		require.Equal(t, "raw", unikraft.GetRootfsType())
	})
}

func TestUnikraftSupportsRootfsType(t *testing.T) {
	unikraft := &UnikraftInfo{}
	t.Run("Supported rootfs type initrd", func(t *testing.T) {
		require.Equal(t, true, unikraft.SupportsRootfsType("initrd"))

	})
	t.Run("Unsupported rootfs type raw", func(t *testing.T) {
		require.Equal(t, true, unikraft.SupportsRootfsType("raw"))

	})
	t.Run("Unsupported rootfs type block", func(t *testing.T) {
		require.Equal(t, false, unikraft.SupportsRootfsType("block"))

	})
}

func TestUnikraftSupportsFsType(t *testing.T) {
	unikraft := &UnikraftInfo{}

	require.Equal(t, false, unikraft.SupportsFsType("foo"))

}

func TestUnikraftSupportsMonitor(t *testing.T) {
	unikraft := &UnikraftInfo{}
	t.Run("Supported qemu", func(t *testing.T) {
		require.Equal(t, true, unikraft.SupportsMonitor("qemu"))

	})
	t.Run("Supported firecracker", func(t *testing.T) {
		require.Equal(t, true, unikraft.SupportsMonitor("firecracker"))

	})
	t.Run("Unsupported monitor", func(t *testing.T) {
		require.Equal(t, false, unikraft.SupportsMonitor("solo5-hvt"))

	})
}

func TestUnikraftSupportsArch(t *testing.T) {
	unikraft := &UnikraftInfo{}
	t.Run("Supported x86_64", func(t *testing.T) {
		require.Equal(t, true, unikraft.SupportsArch("x86_64"))

	})
	t.Run("Supported amd64", func(t *testing.T) {
		require.Equal(t, true, unikraft.SupportsArch("amd64"))

	})
	t.Run("Supported aarch64", func(t *testing.T) {
		require.Equal(t, true, unikraft.SupportsArch("aarch64"))

	})
	t.Run("Unsupported arch", func(t *testing.T) {
		require.Equal(t, false, unikraft.SupportsArch("riscv"))

	})
}

func TestUnikraftCreateRootfs(t *testing.T) {
	t.Run("Rootfs type initrd and single file", func(t *testing.T) {
		plat := Platform{
			Version: "1.0",
			Monitor: "foo",
			Arch:    "bar",
		}
		rootfs := Rootfs{
			From:     "scratch",
			Type:     "initrd",
			Includes: []string{"foo:bar"},
		}

		unikraft := NewUnikraft(plat, rootfs)
		state, err := unikraft.CreateRootfs("context")
		require.NoError(t, err)
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		m, arr := parseDef(t, def.Def)
		// We expect 7 output states: 2 for copy files and 4 for initrd and 1 for final
		// output
		require.Equal(t, 7, len(arr))
		// The last one (final output) should just have a single input
		last := arr[len(arr)-1]
		require.Equal(t, 1, len(last.Inputs))
		// which is the exec op of initrd
		lastInputDgst := last.Inputs[0].Digest
		require.Equal(t, m[lastInputDgst], arr[5])
		e := arr[5]
		exec := e.Op.(*pb.Op_Exec).Exec
		require.Equal(t, 3, len(exec.Meta.Args))
		// the exec should have three inputs
		require.Equal(t, 3, len(e.Inputs))
		// the last of which should be the state with the initrd content
		// that we passed as argument
		sDgst := e.Inputs[2].Digest
		require.Equal(t, m[sDgst], arr[4])
		c := arr[4]
		lDgst := c.Inputs[0].Digest
		require.Equal(t, m[lDgst], arr[3])
		l := arr[3].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", l.Identifier)
		cf := c.Op.(*pb.Op_File).File
		require.Equal(t, 1, len(cf.Actions))
		cp := cf.Actions[0].Action.(*pb.FileAction_Copy).Copy
		require.Equal(t, "/foo", cp.Src)
		require.Equal(t, "/bar", cp.Dest)
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
	t.Run("Invalid files structure", func(t *testing.T) {
		plat := Platform{
			Version: "1.0",
			Monitor: "foo",
			Arch:    "bar",
		}
		rootfs := Rootfs{
			From:     "foo",
			Path:     "bar",
			Type:     "initrd",
			Includes: []string{":bar", "ka"},
		}

		unikraft := NewUnikraft(plat, rootfs)
		_, err := unikraft.CreateRootfs("context")
		require.Error(t, err)
		require.ErrorContains(t, err, "Invalid format of the file")
	})
}

func TestUnikraftBuildKernel(t *testing.T) {
	unikraft := &UnikraftInfo{}
	state := unikraft.BuildKernel("ctx")
	def, err := state.Marshal(context.TODO())

	require.NoError(t, err)
	_, arr := parseDef(t, def.Def)
	require.Equal(t, 0, len(arr))
}
