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
	"encoding/base64"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/stretchr/testify/require"
)

func TestPackMakeCopy(t *testing.T) {
	e := PackEntry{
		SourceRef:   "local",
		SourceState: llb.Local("context"),
		FilePath:    "kernel",
	}

	pc := makeCopy(e, "path")
	require.Equal(t, e.FilePath, pc.SrcPath)
	require.Equal(t, "path", pc.DstPath)
	def, err := pc.SrcState.Marshal(context.TODO())
	require.NoError(t, err)
	_, arr := parseDef(t, def.Def)
	require.Equal(t, 2, len(arr))
	s := arr[0].Op.(*pb.Op_Source).Source
	require.Equal(t, "local://context", s.Identifier)
}

func TestPackHandleKernel(t *testing.T) {
	// nolint: dupl
	t.Run("Local", func(t *testing.T) {
		p := Platform{
			Framework: "rumprun",
			Monitor:   "qemu",
		}
		r := Rootfs{}
		k := Kernel{
			From: "local",
			Path: "kernel",
		}
		f := NewGeneric(p, r)

		e, err := handleKernel(f, "context", "mon", k)
		require.NoError(t, err)
		require.NotNil(t, e)
		require.Equal(t, k.From, e.SourceRef)
		require.Equal(t, k.Path, e.FilePath)
		def, err := e.SourceState.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", s.Identifier)
	})
	// nolint: dupl
	t.Run("Registry", func(t *testing.T) {
		p := Platform{
			Framework: "rumprun",
			Monitor:   "qemu",
		}
		r := Rootfs{}
		k := Kernel{
			From: "harbor.nbfc.io/foo",
			Path: "kernel",
		}
		f := NewGeneric(p, r)

		e, err := handleKernel(f, "context", "mon", k)
		require.NoError(t, err)
		require.NotNil(t, e)
		require.Equal(t, k.From, e.SourceRef)
		require.Equal(t, k.Path, e.FilePath)
		def, err := e.SourceState.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", s.Identifier)
	})
}

func TestPackHandleRootfs(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		p := Platform{
			Framework: "rumprun",
			Monitor:   "qemu",
		}
		r := Rootfs{}
		f := NewGeneric(p, r)

		e, err := handleRootfs(f, "context", "mon", r)
		require.NoError(t, err)
		require.NotNil(t, e)
		require.Empty(t, e.SourceRef)
		require.Empty(t, e.FilePath)
		def, err := e.SourceState.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
	})
	t.Run("Local", func(t *testing.T) {
		p := Platform{
			Framework: "rumprun",
			Monitor:   "qemu",
		}
		r := Rootfs{
			From: "local",
			Path: "rootfs",
		}
		f := NewGeneric(p, r)

		e, err := handleRootfs(f, "context", "mon", r)
		require.NoError(t, err)
		require.NotNil(t, e)
		require.Equal(t, r.From, e.SourceRef)
		require.Equal(t, r.Path, e.FilePath)
		def, err := e.SourceState.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", s.Identifier)
	})
	t.Run("Local with includes", func(t *testing.T) {
		p := Platform{
			Framework: "rumprun",
			Monitor:   "qemu",
		}
		r := Rootfs{
			From:     "local",
			Path:     "rootfs",
			Includes: []string{"foo:bar"},
		}
		f := NewGeneric(p, r)

		e, err := handleRootfs(f, "context", "mon", r)
		require.NoError(t, err)
		require.NotNil(t, e)
		require.Equal(t, r.From, e.SourceRef)
		require.Equal(t, r.Path, e.FilePath)
		def, err := e.SourceState.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", s.Identifier)
	})
	t.Run("Scratch with includes and type initrd", func(t *testing.T) {
		p := Platform{
			Framework: "unikraft",
			Monitor:   "qemu",
		}
		r := Rootfs{
			From:     "scratch",
			Includes: []string{"foo:bar"},
		}
		f := NewUnikraft(p, r)

		e, err := handleRootfs(f, "context", "mon", r)
		require.NoError(t, err)
		require.NotNil(t, e)
		require.Equal(t, r.From, e.SourceRef)
		require.Equal(t, DefaultRootfsPath, e.FilePath)
		def, err := e.SourceState.Marshal(context.TODO())
		require.NoError(t, err)
		m, arr := parseDef(t, def.Def)
		// Same as TestUnikraftCreateRootfs
		require.Equal(t, 7, len(arr))
		last := arr[len(arr)-1]
		require.Equal(t, 1, len(last.Inputs))
		lastInputDgst := last.Inputs[0].Digest
		require.Equal(t, m[lastInputDgst], arr[5])
	})
	t.Run("Empty with includes and type raw", func(t *testing.T) {
		p := Platform{
			Framework: "linux",
			Monitor:   "qemu",
		}
		r := Rootfs{
			From:     "",
			Type:     "raw",
			Includes: []string{"foo:bar"},
		}
		f := NewGeneric(p, r)

		e, err := handleRootfs(f, "context", "mon", r)
		require.NoError(t, err)
		require.NotNil(t, e)
		require.Equal(t, "scratch", e.SourceRef)
		require.Empty(t, e.FilePath)
		def, err := e.SourceState.Marshal(context.TODO())
		require.NoError(t, err)
		m, arr := parseDef(t, def.Def)
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
	t.Run("Invalid unsupported type", func(t *testing.T) {
		p := Platform{
			Framework: "rumprun",
			Monitor:   "qemu",
		}
		r := Rootfs{
			From: "harbor.nbfc.io/foo",
			Path: "rootfs",
			Type: "foo",
		}
		f := NewGeneric(p, r)

		e, err := handleRootfs(f, "context", "mon", r)
		require.Nil(t, e)
		require.ErrorContains(t, err, "Cannot build foo")
	})
}

func TestPackSetAnnotations(t *testing.T) {
	type testInfo struct {
		name        string
		version     string
		cmd         []string
		kPath       string
		rPath       string
		rType       string
		expectError bool
		errorText   string
	}
	tests := []testInfo{
		{
			name:        "Valid without version and rootfs",
			cmd:         []string{"cli"},
			kPath:       "kernel",
			expectError: false,
		},
		{
			name:        "Valid with version but without rootfs",
			version:     "v0.1.1",
			cmd:         []string{"cli"},
			kPath:       "kernel",
			expectError: false,
		},
		{
			name:        "Valid with version and initrd rootfs",
			version:     "v0.1.1",
			cmd:         []string{"cli"},
			kPath:       "kernel",
			rPath:       "kernel",
			rType:       "initrd",
			expectError: false,
		},
		{
			name:        "Valid with version and raw rootfs",
			version:     "v0.1.1",
			cmd:         []string{"cli"},
			kPath:       "kernel",
			rType:       "raw",
			expectError: false,
		},
		{
			name:        "Invalid rootfs type",
			version:     "v0.1.1",
			cmd:         []string{"cli"},
			kPath:       "kernel",
			rType:       "foo",
			expectError: true,
			errorText:   "Unexpected RootfsType value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := Platform{
				Framework: "foo",
				Monitor:   "bar",
				Version:   tc.version,
			}
			annotations := map[string]string{}
			i := &PackInstructions{
				Annots: annotations,
			}
			err := i.SetAnnotations(p, tc.cmd, tc.kPath, tc.rPath, tc.rType)
			if tc.expectError {
				require.ErrorContains(t, err, tc.errorText)
			} else {
				require.NoError(t, err)
				require.Equal(t, p.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
				require.Equal(t, p.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
				require.Equal(t, p.Version, i.Annots["com.urunc.unikernel.unikernelVersion"])
				require.Equal(t, strings.Join(tc.cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
				require.Equal(t, tc.kPath, i.Annots["com.urunc.unikernel.binary"])
				if tc.rType == "raw" {
					require.Equal(t, "true", i.Annots["com.urunc.unikernel.mountRootfs"])
					require.Equal(t, tc.rPath, i.Annots["com.urunc.unikernel.initrd"])
				} else {
					require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
					require.Equal(t, tc.rPath, i.Annots["com.urunc.unikernel.initrd"])
				}
			}
		})
	}
}

func TestPackSetBaseAndGetPaths(t *testing.T) {
	t.Run("Kernel local Rootfs empty", func(t *testing.T) {
		k := &PackEntry{
			SourceRef:   "local",
			SourceState: llb.Local("context"),
			FilePath:    "kernel",
		}
		r := &PackEntry{}
		i := &PackInstructions{}

		kp, rp, err := i.SetBaseAndGetPaths(k, r)
		require.NoError(t, err)
		require.Equal(t, DefaultKernelPath, kp)
		require.Empty(t, rp)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
		require.Equal(t, 1, len(i.Copies))
		require.Equal(t, k.FilePath, i.Copies[0].SrcPath)
		require.Equal(t, DefaultKernelPath, i.Copies[0].DstPath)
		cDef, err := i.Copies[0].SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, cArr := parseDef(t, cDef.Def)
		require.Equal(t, 2, len(cArr))
		cs := cArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", cs.Identifier)
	})
	t.Run("Kernel registry Rootfs empty", func(t *testing.T) {
		k := &PackEntry{
			SourceRef:   "harbor.nbfc.io/foo",
			SourceState: llb.Image("harbor.nbfc.io/foo"),
			FilePath:    "kernel",
		}
		r := &PackEntry{}
		i := &PackInstructions{}

		kp, rp, err := i.SetBaseAndGetPaths(k, r)
		require.NoError(t, err)
		require.Equal(t, k.FilePath, kp)
		require.Empty(t, rp)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", s.Identifier)
		require.Equal(t, 0, len(i.Copies))
	})
	t.Run("Kernel local Rootfs local", func(t *testing.T) {
		k := &PackEntry{
			SourceRef:   "local",
			SourceState: llb.Local("context"),
			FilePath:    "kernel",
		}
		r := &PackEntry{
			SourceRef:   "local",
			SourceState: llb.Local("context"),
			FilePath:    "rootfs",
		}
		i := &PackInstructions{}

		kp, rp, err := i.SetBaseAndGetPaths(k, r)
		require.NoError(t, err)
		require.Equal(t, DefaultKernelPath, kp)
		require.Equal(t, DefaultRootfsPath, rp)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
		require.Equal(t, 2, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, k.FilePath, kc.SrcPath)
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", kcs.Identifier)
		rc := i.Copies[1]
		require.Equal(t, r.FilePath, rc.SrcPath)
		require.Equal(t, DefaultRootfsPath, rc.DstPath)
		rcDef, err := rc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, rcArr := parseDef(t, rcDef.Def)
		require.Equal(t, 2, len(rcArr))
		rcs := rcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", rcs.Identifier)
	})
	t.Run("Kernel local Rootfs scratch", func(t *testing.T) {
		k := &PackEntry{
			SourceRef:   "local",
			SourceState: llb.Local("context"),
			FilePath:    "kernel",
		}
		r := &PackEntry{
			SourceRef:   "scratch",
			SourceState: llb.Image("foo"),
			FilePath:    DefaultRootfsPath,
		}
		i := &PackInstructions{}

		kp, rp, err := i.SetBaseAndGetPaths(k, r)
		require.NoError(t, err)
		require.Equal(t, DefaultKernelPath, kp)
		require.Equal(t, DefaultRootfsPath, rp)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
		require.Equal(t, 2, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, k.FilePath, kc.SrcPath)
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", kcs.Identifier)
		rc := i.Copies[1]
		require.Equal(t, r.FilePath, rc.SrcPath)
		require.Equal(t, DefaultRootfsPath, rc.DstPath)
		rcDef, err := rc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, rcArr := parseDef(t, rcDef.Def)
		require.Equal(t, 2, len(rcArr))
		rcs := rcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://docker.io/library/foo:latest", rcs.Identifier)
	})
	// nolint: dupl
	t.Run("Kernel local Rootfs registry", func(t *testing.T) {
		k := &PackEntry{
			SourceRef:   "local",
			SourceState: llb.Local("context"),
			FilePath:    "kernel",
		}
		r := &PackEntry{
			SourceRef:   "harbor.nbfc.io/foo",
			SourceState: llb.Image("harbor.nbfc.io/foo"),
			FilePath:    "rootfs",
		}
		i := &PackInstructions{}

		kp, rp, err := i.SetBaseAndGetPaths(k, r)
		require.NoError(t, err)
		require.Equal(t, DefaultKernelPath, kp)
		require.Equal(t, r.FilePath, rp)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", s.Identifier)
		require.Equal(t, 1, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, k.FilePath, kc.SrcPath)
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", kcs.Identifier)
	})
	t.Run("Kernel registry Rootfs local", func(t *testing.T) {
		k := &PackEntry{
			SourceRef:   "harbor.nbfc.io/foo",
			SourceState: llb.Image("harbor.nbfc.io/foo"),
			FilePath:    "kernel",
		}
		r := &PackEntry{
			SourceRef:   "local",
			SourceState: llb.Local("context"),
			FilePath:    "rootfs",
		}
		i := &PackInstructions{}

		kp, rp, err := i.SetBaseAndGetPaths(k, r)
		require.NoError(t, err)
		require.Equal(t, k.FilePath, kp)
		require.Equal(t, DefaultRootfsPath, rp)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", s.Identifier)
		require.Equal(t, 1, len(i.Copies))
		rc := i.Copies[0]
		require.Equal(t, r.FilePath, rc.SrcPath)
		require.Equal(t, DefaultRootfsPath, rc.DstPath)
		rcDef, err := rc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, rcArr := parseDef(t, rcDef.Def)
		require.Equal(t, 2, len(rcArr))
		rcs := rcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", rcs.Identifier)
	})
	t.Run("Kernel registry Rootfs scratch", func(t *testing.T) {
		k := &PackEntry{
			SourceRef:   "harbor.nbfc.io/foo",
			SourceState: llb.Image("harbor.nbfc.io/foo"),
			FilePath:    "kernel",
		}
		r := &PackEntry{
			SourceRef:   "scratch",
			SourceState: llb.Image("foo"),
			FilePath:    DefaultRootfsPath,
		}
		i := &PackInstructions{}

		kp, rp, err := i.SetBaseAndGetPaths(k, r)
		require.NoError(t, err)
		require.Equal(t, k.FilePath, kp)
		require.Equal(t, DefaultRootfsPath, rp)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", s.Identifier)
		require.Equal(t, 1, len(i.Copies))
		rc := i.Copies[0]
		require.Equal(t, r.FilePath, rc.SrcPath)
		require.Equal(t, DefaultRootfsPath, rc.DstPath)
		rcDef, err := rc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, rcArr := parseDef(t, rcDef.Def)
		require.Equal(t, 2, len(rcArr))
		rcs := rcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://docker.io/library/foo:latest", rcs.Identifier)
	})
	// nolint: dupl
	t.Run("Kernel registry Rootfs registry", func(t *testing.T) {
		k := &PackEntry{
			SourceRef:   "harbor.nbfc.io/foo",
			SourceState: llb.Image("harbor.nbfc.io/foo"),
			FilePath:    "kernel",
		}
		r := &PackEntry{
			SourceRef:   "harbor.nbfc.io/bar",
			SourceState: llb.Image("harbor.nbfc.io/bar"),
			FilePath:    "rootfs",
		}
		i := &PackInstructions{}

		kp, rp, err := i.SetBaseAndGetPaths(k, r)
		require.NoError(t, err)
		require.Equal(t, DefaultKernelPath, kp)
		require.Equal(t, r.FilePath, rp)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/bar:latest", s.Identifier)
		require.Equal(t, 1, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, k.FilePath, kc.SrcPath)
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", kcs.Identifier)
	})
	t.Run("Invalid Kernel empty", func(t *testing.T) {
		k := &PackEntry{}
		r := &PackEntry{}
		i := &PackInstructions{}

		kp, rp, err := i.SetBaseAndGetPaths(k, r)
		require.ErrorContains(t, err, "Source of kernel State is empty")
		require.Empty(t, kp)
		require.Empty(t, rp)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
		require.Equal(t, 0, len(i.Copies))
	})
}

func TestPackToPack(t *testing.T) {
	t.Run("Kernel local Rootfs none", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "rumprun",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "local",
				Path: "kernel",
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, DefaultKernelPath, i.Annots["com.urunc.unikernel.binary"])
		require.Empty(t, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 1, len(i.Copies))
		c := i.Copies[0]
		require.Equal(t, DefaultKernelPath, c.DstPath)
		require.Equal(t, hops.Kernel.Path, c.SrcPath)
		cDef, err := c.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, cArr := parseDef(t, cDef.Def)
		require.Equal(t, 2, len(cArr))
		cs := cArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", cs.Identifier)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
	})
	t.Run("Kernel registry Rootfs none", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "unikraft",
				Monitor:   "firecracker",
			},
			Kernel: Kernel{
				From: "harbor.nbfc.io/foo",
				Path: "/kernel",
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "foo")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, hops.Kernel.Path, i.Annots["com.urunc.unikernel.binary"])
		require.Empty(t, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 0, len(i.Copies))
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", s.Identifier)
	})
	t.Run("Kernel local Rootfs local type none implies initrd", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "unikraft",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "local",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From: "local",
				Path: "rootfs",
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, DefaultKernelPath, i.Annots["com.urunc.unikernel.binary"])
		require.Equal(t, DefaultRootfsPath, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 2, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		require.Equal(t, hops.Kernel.Path, kc.SrcPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", kcs.Identifier)
		rc := i.Copies[1]
		require.Equal(t, DefaultRootfsPath, rc.DstPath)
		require.Equal(t, hops.Rootfs.Path, rc.SrcPath)
		rcDef, err := rc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, rcArr := parseDef(t, rcDef.Def)
		require.Equal(t, 2, len(rcArr))
		rcs := rcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", rcs.Identifier)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
	})
	t.Run("Kernel local Rootfs local type initrd and version", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "linux",
				Monitor:   "qemu",
				Version:   "v1.7.0",
			},
			Kernel: Kernel{
				From: "local",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From: "local",
				Path: "rootfs",
				Type: "initrd",
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Platform.Version, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, DefaultKernelPath, i.Annots["com.urunc.unikernel.binary"])
		require.Equal(t, DefaultRootfsPath, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 2, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		require.Equal(t, hops.Kernel.Path, kc.SrcPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", kcs.Identifier)
		rc := i.Copies[1]
		require.Equal(t, DefaultRootfsPath, rc.DstPath)
		require.Equal(t, hops.Rootfs.Path, rc.SrcPath)
		rcDef, err := rc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, rcArr := parseDef(t, rcDef.Def)
		require.Equal(t, 2, len(rcArr))
		rcs := rcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", rcs.Identifier)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
	})
	// nolint: dupl
	t.Run("Kernel local Rootfs remote type initrd", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "linux",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "local",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From: "harbor.nbfc.io/foo",
				Path: "rootfs",
				Type: "initrd",
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, DefaultKernelPath, i.Annots["com.urunc.unikernel.binary"])
		require.Equal(t, hops.Rootfs.Path, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 1, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		require.Equal(t, hops.Kernel.Path, kc.SrcPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", kcs.Identifier)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		sb := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", sb.Identifier)
	})
	// nolint: dupl
	t.Run("Kernel local Rootfs remote type none implies raw", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "linux",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "local",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From: "harbor.nbfc.io/foo",
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "true", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, DefaultKernelPath, i.Annots["com.urunc.unikernel.binary"])
		require.Empty(t, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 1, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		require.Equal(t, hops.Kernel.Path, kc.SrcPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", kcs.Identifier)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", s.Identifier)
	})
	t.Run("Kernel local Rootfs scratch type none implies initrd with includes", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "unikraft",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "local",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From:     "scratch",
				Includes: []string{"foo:bar"},
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, DefaultKernelPath, i.Annots["com.urunc.unikernel.binary"])
		require.Equal(t, DefaultRootfsPath, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 2, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		require.Equal(t, hops.Kernel.Path, kc.SrcPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", kcs.Identifier)
		rc := i.Copies[1]
		require.Equal(t, DefaultRootfsPath, rc.DstPath)
		require.Equal(t, DefaultRootfsPath, rc.SrcPath)
		rcDef, err := rc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		rm, rcArr := parseDef(t, rcDef.Def)
		// It should the same as TestUnikraftCreateRootfs
		require.Equal(t, 7, len(rcArr))
		last := rcArr[len(rcArr)-1]
		require.Equal(t, 1, len(last.Inputs))
		lastInputDgst := last.Inputs[0].Digest
		require.Equal(t, rm[lastInputDgst], rcArr[5])
		e := rcArr[5]
		require.Equal(t, 3, len(e.Inputs))
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 0, len(arr))
	})
	t.Run("Kernel local Rootfs scratch type none implies raw with includes", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "linux",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "local",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From:     "scratch",
				Includes: []string{"foo:bar"},
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "true", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, DefaultKernelPath, i.Annots["com.urunc.unikernel.binary"])
		require.Empty(t, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 1, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		require.Equal(t, hops.Kernel.Path, kc.SrcPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", kcs.Identifier)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		m, arr := parseDef(t, def.Def)
		// It should the same as TestUnikraftCreateRootfs
		require.Equal(t, 3, len(arr))
		last := arr[len(arr)-1]
		require.Equal(t, 1, len(last.Inputs))
		lastInputDgst := last.Inputs[0].Digest
		require.Equal(t, m[lastInputDgst], arr[1])
	})
	t.Run("Kernel registry Rootfs local type none implies initrd", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "unikraft",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "harbor.nbfc.io/foo",
				Path: "/kernel",
			},
			Rootfs: Rootfs{
				From: "local",
				Path: "rootfs",
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, hops.Kernel.Path, i.Annots["com.urunc.unikernel.binary"])
		require.Equal(t, DefaultRootfsPath, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 1, len(i.Copies))
		rc := i.Copies[0]
		require.Equal(t, DefaultRootfsPath, rc.DstPath)
		require.Equal(t, hops.Rootfs.Path, rc.SrcPath)
		rcDef, err := rc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, rcArr := parseDef(t, rcDef.Def)
		require.Equal(t, 2, len(rcArr))
		rcs := rcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", rcs.Identifier)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", s.Identifier)
	})
	// nolint: dupl
	t.Run("Kernel remote Rootfs remote type initrd", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "linux",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "harbor.nbfc.io/foo",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From: "harbor.nbfc.io/foo",
				Path: "rootfs",
				Type: "initrd",
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, DefaultKernelPath, i.Annots["com.urunc.unikernel.binary"])
		require.Equal(t, hops.Rootfs.Path, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 1, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		require.Equal(t, hops.Kernel.Path, kc.SrcPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", kcs.Identifier)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", s.Identifier)
	})
	// nolint: dupl
	t.Run("Kernel local Rootfs remote type none implies raw ", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "linux",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "harbor.nbfc.io/bar",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From: "harbor.nbfc.io/foo",
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "true", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, DefaultKernelPath, i.Annots["com.urunc.unikernel.binary"])
		require.Empty(t, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 1, len(i.Copies))
		kc := i.Copies[0]
		require.Equal(t, DefaultKernelPath, kc.DstPath)
		require.Equal(t, hops.Kernel.Path, kc.SrcPath)
		kcDef, err := kc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		_, kcArr := parseDef(t, kcDef.Def)
		require.Equal(t, 2, len(kcArr))
		kcs := kcArr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/bar:latest", kcs.Identifier)
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr := parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", s.Identifier)
	})
	t.Run("Kernel registry Rootfs scratch type none implies initrd with includes", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "unikraft",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "harbor.nbfc.io/bar",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From:     "scratch",
				Includes: []string{"foo:bar"},
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, strings.Join(hops.Cmd, " "), i.Annots["com.urunc.unikernel.cmdline"])
		require.Equal(t, hops.Kernel.Path, i.Annots["com.urunc.unikernel.binary"])
		require.Equal(t, DefaultRootfsPath, i.Annots["com.urunc.unikernel.initrd"])
		require.Empty(t, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Empty(t, i.Annots["com.urunc.unikernel.blkMntPoint"])
		require.Empty(t, i.Annots["com.urunc.unikernel.block"])
		require.Equal(t, 1, len(i.Copies))
		rc := i.Copies[0]
		require.Equal(t, DefaultRootfsPath, rc.DstPath)
		require.Equal(t, DefaultRootfsPath, rc.SrcPath)
		rcDef, err := rc.SrcState.Marshal(context.TODO())
		require.NoError(t, err)
		m, arr := parseDef(t, rcDef.Def)
		// It should the same as TestUnikraftCreateRootfs
		require.Equal(t, 7, len(arr))
		last := arr[len(arr)-1]
		require.Equal(t, 1, len(last.Inputs))
		lastInputDgst := last.Inputs[0].Digest
		require.Equal(t, m[lastInputDgst], arr[5])
		e := arr[5]
		require.Equal(t, 3, len(e.Inputs))
		def, err := i.Base.Marshal(context.TODO())
		require.NoError(t, err)
		_, arr = parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/bar:latest", s.Identifier)
	})
	t.Run("Invalid Rootfs type from local", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "rumprun",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "local",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From: "local",
				Path: "kernel",
				Type: "foo",
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.ErrorContains(t, err, "Error handling rootfs entry")
		require.Nil(t, i)
	})
	// TODO: Resume below test when a new framework that does not support
	// raw rootfs is introduced (e.g. Mewz, Rumprun)
	// t.Run("Invalid Rootfs from registry implies unsupported raw rootfs type", func(t *testing.T) {
	//	hops := &Hops{
	//		Platform: Platform{
	//			Framework: "unikraft",
	//			Monitor:   "qemu",
	//		},
	//		Kernel: Kernel{
	//			From: "local",
	//			Path: "kernel",
	//		},
	//		Rootfs: Rootfs{
	//			From: "harbor.nbfc.io/foo",
	//		},
	//		Cmd: []string{"cmd"},
	//	}
	//	i, err := ToPack(hops, "context")
	//	require.ErrorContains(t, err, "unikraft does not support raw rootfs")
	//	require.Nil(t, i)
	// })
	t.Run("Invalid Rootfs from scratch and wrong includes format", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "rumprun",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "local",
				Path: "kernel",
			},
			Rootfs: Rootfs{
				From:     "scratch",
				Includes: []string{":bar"},
			},
			Cmd: []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.ErrorContains(t, err, "Error handling rootfs entry")
		require.Nil(t, i)
	})
	t.Run("Invalid empty Kernel", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "rumprun",
				Monitor:   "qemu",
			},
			Kernel: Kernel{},
			Rootfs: Rootfs{},
			Cmd:    []string{"cmd"},
		}
		i, err := ToPack(hops, "context")
		require.ErrorContains(t, err, "Error choosing base state")
		require.Nil(t, i)
	})
}

func TestPackLLB(t *testing.T) {
	t.Run("Base scratch annots no copies", func(t *testing.T) {
		annotations := map[string]string{
			"foo":           "bar",
			"unikernelType": "unikraft",
			"cmdline":       "test-cmd",
			"hypervisor":    "qemu",
			"binary":        "/boot/kernel",
		}

		instr := PackInstructions{
			Base:   llb.Scratch(),
			Copies: []PackCopies{},
			Annots: annotations,
		}

		result, err := PackLLB(instr)
		require.NoError(t, err)
		require.NotNil(t, result)
		m, arr := parseDef(t, result.Def)
		require.Equal(t, 2, len(arr))
		ujs := arr[0].Op.(*pb.Op_File).File
		require.Equal(t, 1, len(arr[1].Inputs))
		require.Equal(t, m[arr[1].Inputs[0].Digest], arr[0])
		require.Equal(t, 0, int(arr[1].Inputs[0].Index))
		require.Equal(t, 1, len(ujs.Actions))

		action := ujs.Actions[0]
		require.Equal(t, -1, int(action.Input))
		require.Equal(t, -1, int(action.SecondaryInput))
		require.Equal(t, 0, int(action.Output))

		mkfile := action.Action.(*pb.FileAction_Mkfile).Mkfile

		require.Equal(t, "/urunc.json", mkfile.Path)
		require.Equal(t, 0644, int(mkfile.Mode))
		var annotJSON map[string]string
		err = json.Unmarshal(mkfile.Data, &annotJSON)
		require.NoError(t, err)
		for an, val := range instr.Annots {
			encoded := base64.StdEncoding.EncodeToString([]byte(val))
			require.Equal(t, string(encoded), annotJSON[an])
		}
	})
	t.Run("Base scratch no annots no copies", func(t *testing.T) {
		annotations := map[string]string{}

		instr := PackInstructions{
			Base:   llb.Scratch(),
			Copies: []PackCopies{},
			Annots: annotations,
		}

		result, err := PackLLB(instr)
		require.NoError(t, err)
		require.NotNil(t, result)
		m, arr := parseDef(t, result.Def)
		require.Equal(t, 2, len(arr))
		ujs := arr[0].Op.(*pb.Op_File).File
		require.Equal(t, 1, len(arr[1].Inputs))
		require.Equal(t, m[arr[1].Inputs[0].Digest], arr[0])
		require.Equal(t, 0, int(arr[1].Inputs[0].Index))
		require.Equal(t, 1, len(ujs.Actions))

		action := ujs.Actions[0]
		require.Equal(t, -1, int(action.Input))
		require.Equal(t, -1, int(action.SecondaryInput))
		require.Equal(t, 0, int(action.Output))

		mkfile := action.Action.(*pb.FileAction_Mkfile).Mkfile

		require.Equal(t, "/urunc.json", mkfile.Path)
		require.Equal(t, 0644, int(mkfile.Mode))
		var annotJSON map[string]string
		err = json.Unmarshal(mkfile.Data, &annotJSON)
		require.NoError(t, err)
		require.Equal(t, 0, len(annotJSON))
	})
	t.Run("Base scratch annots with copies", func(t *testing.T) {
		annotations := map[string]string{
			"foo":           "bar",
			"unikernelType": "unikraft",
			"cmdline":       "test-cmd",
			"hypervisor":    "qemu",
			"binary":        "/boot/kernel",
		}

		copies := []PackCopies{
			{
				SrcState: llb.Local("context"),
				SrcPath:  "foo",
				DstPath:  "bar",
			},
			{
				SrcState: llb.Image("harbor.nbfc.io/foo"),
				SrcPath:  "file1",
				DstPath:  "file2",
			},
		}
		instr := PackInstructions{
			Base:   llb.Scratch(),
			Copies: copies,
			Annots: annotations,
		}

		result, err := PackLLB(instr)
		require.NoError(t, err)
		require.NotNil(t, result)
		m, arr := parseDef(t, result.Def)
		require.Equal(t, 6, len(arr))
		last := arr[len(arr)-1]
		require.Equal(t, 1, len(last.Inputs))
		lastInputDgst := last.Inputs[0].Digest
		require.Equal(t, m[lastInputDgst], arr[4])

		ujs := arr[4].Op.(*pb.Op_File).File
		require.Equal(t, 1, len(ujs.Actions))
		action := ujs.Actions[0]
		require.Equal(t, 0, int(action.Input))
		require.Equal(t, -1, int(action.SecondaryInput))
		require.Equal(t, 0, int(action.Output))
		mkfile := action.Action.(*pb.FileAction_Mkfile).Mkfile
		require.Equal(t, "/urunc.json", mkfile.Path)
		require.Equal(t, 0644, int(mkfile.Mode))
		var annotJSON map[string]string
		err = json.Unmarshal(mkfile.Data, &annotJSON)
		require.NoError(t, err)
		for an, val := range instr.Annots {
			encoded := base64.StdEncoding.EncodeToString([]byte(val))
			require.Equal(t, string(encoded), annotJSON[an])
		}

		c1 := arr[3]
		require.Equal(t, 2, len(c1.Inputs))
		i1Dgst := c1.Inputs[1].Digest
		require.Equal(t, m[i1Dgst], arr[2])
		cf1 := c1.Op.(*pb.Op_File).File
		cp1 := cf1.Actions[0].Action.(*pb.FileAction_Copy).Copy
		require.Equal(t, "/file1", cp1.Src)
		require.Equal(t, "/file2", cp1.Dest)
		s1 := arr[2]
		require.Equal(t, 0, len(s1.Inputs))
		cs1 := s1.Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", cs1.Identifier)
		c2 := arr[1]
		i2Dgst := c1.Inputs[0].Digest
		require.Equal(t, m[i2Dgst], c2)
		require.Equal(t, 1, len(c2.Inputs))
		s2Dgst := c2.Inputs[0].Digest
		require.Equal(t, m[s2Dgst], arr[0])
		cf2 := c2.Op.(*pb.Op_File).File
		cp2 := cf2.Actions[0].Action.(*pb.FileAction_Copy).Copy
		require.Equal(t, "/foo", cp2.Src)
		require.Equal(t, "/bar", cp2.Dest)
		s2 := arr[0]
		require.Equal(t, 0, len(s2.Inputs))
		cs2 := s2.Op.(*pb.Op_Source).Source
		require.Equal(t, "local://context", cs2.Identifier)
	})
	t.Run("Base registry no annots no copies", func(t *testing.T) {
		annotations := map[string]string{}

		instr := PackInstructions{
			Base:   llb.Image("harbor.nbfc.io/foo"),
			Copies: []PackCopies{},
			Annots: annotations,
		}

		result, err := PackLLB(instr)
		require.NoError(t, err)
		require.NotNil(t, result)
		m, arr := parseDef(t, result.Def)
		require.Equal(t, 3, len(arr))
		ujs := arr[1].Op.(*pb.Op_File).File
		require.Equal(t, 1, len(arr[2].Inputs))
		require.Equal(t, m[arr[2].Inputs[0].Digest], arr[1])
		require.Equal(t, 0, int(arr[2].Inputs[0].Index))
		require.Equal(t, 1, len(ujs.Actions))

		action := ujs.Actions[0]
		require.Equal(t, 0, int(action.Input))
		require.Equal(t, -1, int(action.SecondaryInput))
		require.Equal(t, 0, int(action.Output))

		mkfile := action.Action.(*pb.FileAction_Mkfile).Mkfile

		require.Equal(t, "/urunc.json", mkfile.Path)
		require.Equal(t, 0644, int(mkfile.Mode))
		var annotJSON map[string]string
		err = json.Unmarshal(mkfile.Data, &annotJSON)
		require.NoError(t, err)
		require.Equal(t, 0, len(annotJSON))

		s := arr[0]
		require.Equal(t, 0, len(s.Inputs))
		src := s.Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/foo:latest", src.Identifier)
	})
	t.Run("Base scratch no annots no copies arch", func(t *testing.T) {
		annotations := map[string]string{}

		instr := PackInstructions{
			Base:   llb.Scratch(),
			Copies: []PackCopies{},
			Annots: annotations,
		}

		result, err := PackLLB(instr)
		switch runtime.GOARCH {
		case "amd64", "arm", "arm64":
			require.NoError(t, err)
			require.NotNil(t, result)
		default:
			require.Error(t, err)
			require.Nil(t, result)
			require.ErrorContains(t, err, "Unsupported architecture")
			require.ErrorContains(t, err, runtime.GOARCH)
		}
	})
	t.Run("Invalid Base no annots no copies", func(t *testing.T) {
		annotations := map[string]string{}

		instr := PackInstructions{
			Base:   llb.Image("/foo"),
			Copies: []PackCopies{},
			Annots: annotations,
		}

		result, err := PackLLB(instr)
		require.Error(t, err)
		require.Nil(t, result)
		require.ErrorContains(t, err, "Failed to marshal")
	})
	t.Run("Invalid Base no annots no copies", func(t *testing.T) {
		annotations := map[string]string{}

		copies := []PackCopies{
			{
				SrcState: llb.Local("context"),
				SrcPath:  "foo",
				DstPath:  "bar",
			},
			{
				SrcState: llb.Image("/foo"),
				SrcPath:  "file1",
				DstPath:  "file2",
			},
		}
		instr := PackInstructions{
			Base:   llb.Scratch(),
			Copies: copies,
			Annots: annotations,
		}

		result, err := PackLLB(instr)
		require.Error(t, err)
		require.Nil(t, result)
		require.ErrorContains(t, err, "Failed to marshal")
	})
}
