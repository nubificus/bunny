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
	"runtime"
	"testing"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

func TestLLBFiles(t *testing.T) {
	t.Run("Single file", func(t *testing.T) {
		src := llb.Local("context")
		dst := llb.Scratch()
		files := []string{"foo1"}

		state, err := FilesLLB(files, src, dst)
		require.NoError(t, err)
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		m, arr := parseDef(t, def.Def)
		// We expect 3 steps
		require.Equal(t, 3, len(arr))
		// The last one should just have a single input
		last := arr[len(arr)-1]
		require.Equal(t, 1, len(last.Inputs))
		// which is the copy operation
		lastInputDgst := last.Inputs[0].Digest
		require.Equal(t, m[lastInputDgst], arr[1])
		c := arr[1]
		// the copy should have one input
		require.Equal(t, 1, len(c.Inputs))
		// whioch is the source we defined before
		sDgst := c.Inputs[0].Digest
		require.Equal(t, m[sDgst], arr[0])
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", s.Identifier)
		cf := c.Op.(*pb.Op_File).File
		require.Equal(t, 1, len(cf.Actions))
		cp := cf.Actions[0].Action.(*pb.FileAction_Copy).Copy
		require.Equal(t, "/foo1", cp.Src)
		require.Equal(t, "/foo1", cp.Dest)
	})
	t.Run("Multiple files", func(t *testing.T) {
		src := llb.Local("context")
		dst := llb.Image("foo")
		files := []string{"foo1:bar1", "foo2:bar2"}

		state, err := FilesLLB(files, src, dst)
		require.NoError(t, err)
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		m, arr := parseDef(t, def.Def)
		// We expect 5 steps
		require.Equal(t, 5, len(arr))
		// The last one should just have a single input
		last := arr[len(arr)-1]
		require.Equal(t, 1, len(last.Inputs))
		// which is the second copy
		lastInputDgst := last.Inputs[0].Digest
		require.Equal(t, m[lastInputDgst], arr[3])
		c2 := arr[3]
		// the second copy should have two inputs
		require.Equal(t, 2, len(c2.Inputs))
		// The first input should be the first copy
		c1Dgst := c2.Inputs[0].Digest
		require.Equal(t, m[c1Dgst], arr[2])
		// The second input should be the source state we defined before
		sDgst := c2.Inputs[1].Digest
		require.Equal(t, m[sDgst], arr[1])
		c1 := arr[2]
		// the first copy should have two inputs
		require.Equal(t, 2, len(c2.Inputs))
		// The first input should be the destination source we defined before
		dDgst := c1.Inputs[0].Digest
		require.Equal(t, m[dDgst], arr[0])
		// The second input should be the source state we defined before
		require.Equal(t, sDgst, c1.Inputs[1].Digest)
		s := arr[1].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", s.Identifier)
		d := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://docker.io/library/foo:latest", d.Identifier)
	})
	t.Run("Empty files list", func(t *testing.T) {
		src := llb.Local("context")
		dst := llb.Scratch()
		files := []string{}

		state, err := FilesLLB(files, src, dst)
		require.NoError(t, err)
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
	})
	t.Run("Invalid file list no dest", func(t *testing.T) {
		src := llb.Local("context")
		dst := llb.Scratch()
		files := []string{":foo"}

		state, err := FilesLLB(files, src, dst)
		require.EqualError(t, err, "Invalid format of the file list to copy")
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
	})
	t.Run("Invalid file list multiple sources", func(t *testing.T) {
		src := llb.Local("context")
		dst := llb.Scratch()
		files := []string{"foo:a:b"}

		state, err := FilesLLB(files, src, dst)
		require.EqualError(t, err, "Invalid format of the file list to copy")
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
	})
}

func TestLLBInitrd(t *testing.T) {
	content := llb.Image("foo")

	state := InitrdLLB(content)
	def, err := state.Marshal(context.TODO())

	require.NoError(t, err)
	m, arr := parseDef(t, def.Def)
	// We expect 6 steps
	require.Equal(t, 6, len(arr))
	// The last one should just have a single input
	last := arr[len(arr)-1]
	require.Equal(t, 1, len(last.Inputs))
	// which is the exec op
	lastInputDgst := last.Inputs[0].Digest
	require.Equal(t, m[lastInputDgst], arr[4])
	e := arr[4]
	// the exec should have three inputs
	require.Equal(t, 3, len(e.Inputs))
	// the last of which should be the state with the initrd content
	// that we passed as argument
	sDgst := e.Inputs[2].Digest
	require.Equal(t, m[sDgst], arr[3])
	s := arr[3]
	// the second of which should be the state that creates the directory
	// where the cpio file will get stored
	oDgst := e.Inputs[1].Digest
	require.Equal(t, m[oDgst], arr[2])
	o := arr[2]
	// the first of which should be the state that creates the tmp directory
	tDgst := e.Inputs[0].Digest
	require.Equal(t, m[tDgst], arr[1])
	tmp := arr[1]
	// The state that create the tmp directory has one input, which is the source of
	// the tools
	toolDgst := tmp.Inputs[0].Digest
	require.Equal(t, m[toolDgst], arr[0])
	tools := arr[0]
	t.Run("Exec command", func(t *testing.T) {
		exec := e.Op.(*pb.Op_Exec).Exec
		require.Equal(t, "/workdir", exec.Meta.Cwd)
		require.Equal(t, 3, len(exec.Meta.Args))
		require.Equal(t, "sh", exec.Meta.Args[0])
		require.Equal(t, "-c", exec.Meta.Args[1])
		expectedCmd := "find . -depth -print | tac | bsdcpio -o --format newc > " + DefaultRootfsPath
		require.Equal(t, expectedCmd, exec.Meta.Args[2])
		require.Equal(t, 3, len(exec.Mounts))
		require.Equal(t, "/", exec.Mounts[0].Dest)
		require.Equal(t, "/.boot", exec.Mounts[1].Dest)
		require.Equal(t, false, exec.Mounts[1].Readonly)
		require.Equal(t, "/workdir", exec.Mounts[2].Dest)
		require.Equal(t, true, exec.Mounts[2].Readonly)
	})
	t.Run("Input state", func(t *testing.T) {
		src := s.Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://docker.io/library/foo:latest", src.Identifier)
	})
	t.Run("Output state", func(t *testing.T) {
		of := o.Op.(*pb.Op_File).File
		require.Equal(t, 1, len(of.Actions))
		mkdir := of.Actions[0].Action.(*pb.FileAction_Mkdir).Mkdir
		require.Equal(t, "/.boot", mkdir.Path)
	})
	t.Run("Tmp directory", func(t *testing.T) {
		tf := tmp.Op.(*pb.Op_File).File
		require.Equal(t, 1, len(tf.Actions))
		mkdir := tf.Actions[0].Action.(*pb.FileAction_Mkdir).Mkdir
		require.Equal(t, "/tmp", mkdir.Path)
	})
	t.Run("Tool state", func(t *testing.T) {
		toolSrc := tools.Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://"+DefaultBsdcpioImage, toolSrc.Identifier)
	})
}

func TestLLBCopy(t *testing.T) {
	dest := llb.Image("foo")
	from := PackCopies{
		SrcState: llb.Local("context"),
		SrcPath:  "src",
		DstPath:  "dst",
	}
	state := CopyLLB(dest, from)
	def, err := state.Marshal(context.TODO())

	require.NoError(t, err)
	m, arr := parseDef(t, def.Def)
	require.Equal(t, 4, len(arr))
	// The last one should just have a single input
	last := arr[len(arr)-1]
	require.Equal(t, 1, len(last.Inputs))
	// which is the copy operation
	lastInputDgst := last.Inputs[0].Digest
	require.Equal(t, m[lastInputDgst], arr[2])
	c := arr[2]
	cf := c.Op.(*pb.Op_File).File
	require.Equal(t, 1, len(cf.Actions))
	cp := cf.Actions[0].Action.(*pb.FileAction_Copy).Copy
	require.Equal(t, "/src", cp.Src)
	require.Equal(t, "/dst", cp.Dest)
	require.Equal(t, true, cp.CreateDestPath)
	// the copy should have two inputs
	require.Equal(t, 2, len(c.Inputs))
	// the first is the source we defined before
	sDgst := c.Inputs[1].Digest
	require.Equal(t, m[sDgst], arr[1])
	s := arr[1].Op.(*pb.Op_Source).Source
	require.Equal(t, "local://context", s.Identifier)
	// the second is the destination state
	dDgst := c.Inputs[0].Digest
	require.Equal(t, m[dDgst], arr[0])
	d := arr[0].Op.(*pb.Op_Source).Source
	require.Equal(t, "docker-image://docker.io/library/foo:latest", d.Identifier)
}

func TestLLBBase(t *testing.T) {
	t.Run("From scratch", func(t *testing.T) {
		state := BaseLLB("scratch", "")
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
	})
	t.Run("From scratch and monitor", func(t *testing.T) {
		state := BaseLLB("scratch", "foo")
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
	})
	t.Run("From unikraft and qemu", func(t *testing.T) {
		state := BaseLLB("unikraft.org/foo", "qemu")
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://unikraft.org/foo:latest", s.Identifier)
		p := arr[0].Platform
		require.NotNil(t, p)
		require.Equal(t, runtime.GOARCH, p.Architecture)
		require.Equal(t, "qemu", p.OS)
	})
	t.Run("From unikraft and firecracker", func(t *testing.T) {
		state := BaseLLB("unikraft.org/foo", "firecracker")
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://unikraft.org/foo:latest", s.Identifier)
		p := arr[0].Platform
		require.NotNil(t, p)
		require.Equal(t, runtime.GOARCH, p.Architecture)
		require.Equal(t, "fc", p.OS)
	})
	t.Run("From foo", func(t *testing.T) {
		state := BaseLLB("foo", "")
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://docker.io/library/foo:latest", s.Identifier)
		p := arr[0].Platform
		require.NotNil(t, p)
		require.Equal(t, runtime.GOARCH, p.Architecture)
		require.Equal(t, "linux", p.OS)
	})
	t.Run("From foo and monitor", func(t *testing.T) {
		state := BaseLLB("foo", "bar")
		def, err := state.Marshal(context.TODO())

		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://docker.io/library/foo:latest", s.Identifier)
		p := arr[0].Platform
		require.NotNil(t, p)
		require.Equal(t, runtime.GOARCH, p.Architecture)
		require.Equal(t, "linux", p.OS)
	})
}

func parseDef(t *testing.T, def [][]byte) (map[string]*pb.Op, []*pb.Op) {
	m := map[string]*pb.Op{}
	arr := make([]*pb.Op, 0, len(def))

	for _, dt := range def {
		var op pb.Op
		err := op.Unmarshal(dt)
		require.NoError(t, err)
		dgst := digest.FromBytes(dt)
		m[string(dgst)] = &op
		arr = append(arr, &op)
		// fmt.Printf(":: %T %+v\n", op.Op, op)
	}

	return m, arr
}
