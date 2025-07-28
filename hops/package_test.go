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
	"testing"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/stretchr/testify/require"
)

func TestPackHandleKernel(t *testing.T) {
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

func TestPackToPack(t *testing.T) {
	t.Run("Kernel local rootfs none", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "rumprun",
				Monitor:   "qemu",
			},
			Kernel: Kernel{
				From: "local",
				Path: "kernel",
			},
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel registry rootfs none", func(t *testing.T) {
		hops := &Hops{
			Platform: Platform{
				Framework: "unikraft",
				Monitor:   "firecracker",
			},
			Kernel: Kernel{
				From: "harbor.nbfc.io/foo",
				Path: "/kernel",
			},
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "foo")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel local rootfs local initrd type none implies initrd", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel local rootfs local type initrd and version", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Platform.Version, i.Annots["com.urunc.unikernel.unikernelVersion"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel local rootfs remote type initrd", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel local rootfs remote type none implies raw", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "true", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel local rootfs scratch type none implies initrd with include", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel local rootfs scratch type none implies raw with include", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "true", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel registry rootfs local initrd type none", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel remote rootfs remote type initrd", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel local rootfs remote type none implies raw ", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "true", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
	t.Run("Kernel registry rootfs scratch type none implies initrd with include", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.NoError(t, err)
		require.NotNil(t, i)
		require.Equal(t, "false", i.Annots["com.urunc.unikernel.mountRootfs"])
		require.Equal(t, hops.Platform.Framework, i.Annots["com.urunc.unikernel.unikernelType"])
		require.Equal(t, hops.Platform.Monitor, i.Annots["com.urunc.unikernel.hypervisor"])
		require.Equal(t, hops.Cmd, i.Annots["com.urunc.unikernel.cmdline"])
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
		_, arr = parseDef(t, def.Def)
		require.Equal(t, 2, len(arr))
		s := arr[0].Op.(*pb.Op_Source).Source
		require.Equal(t, "docker-image://harbor.nbfc.io/bar:latest", s.Identifier)
	})
	t.Run("Invalid rootfs type unsupported", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.ErrorContains(t, err, "Cannot build foo")
		require.Nil(t, i)
	})
	// TODO: Resume below test when a new framework that does not support
	// raw rootfs is introduced (e.g. Mewz, Rumprun)
	// t.Run("Invalid rootfs from registry implies unsupported raw rootfs type", func(t *testing.T) {
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
	//		Cmd: "cmd",
	//	}
	//	i, err := ToPack(hops, "context")
	//	require.ErrorContains(t, err, "unikraft does not support raw rootfs")
	//	require.Nil(t, i)
	// })
	t.Run("Invalid rootfs from scratch and wrong include format", func(t *testing.T) {
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
			Cmd: "cmd",
		}
		i, err := ToPack(hops, "context")
		require.ErrorContains(t, err, "Invalid format of the file")
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
	t.Run("Base scratch annots no copies", func(t *testing.T) {
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
	t.Run("Base invalid no annots no copies", func(t *testing.T) {
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
	t.Run("Base invalid no annots no copies", func(t *testing.T) {
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
