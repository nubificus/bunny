# Creating the initrd and packaging a Unikraft unikernel with `bunny`

For this example, we will use the [C HTTP Web
Server](https://github.com/unikraft/catalog/tree/main/examples/http-c) example
in Unikraft's catalog. To build the unikernel, we mainly need to:
1. Build the C HTTP server
2. Create the initrd for Unikraft
3. Use the latest base image of [Unikraft's catalog](https://github.com/unikraft/catalog) and package everything together.

We will perform the above steps with and without `bunnyfile` to showcase how we can
use `bunny`.

## Without `bunnyfile`

### Step 1: Building the C HTTP Server

We will build the C application as a static binary.

```
gcc -static-pie -o chttp http_server.c
```

### Step 2: Create the initrd file for Unikraft

We will create the initrd file for Unikraft, using the `bsdcpio` command:

```
mkdir rootfs
mv chttp rootfs/
cd rootfs
find -depth -print | tac | bsdcpio -o --format newc > ../rootfs.cpio
cd ../
```

Alternatively, we can use [kraftkit](https://github.com/unikraft/kraftkit) and
inside the `examples/http-c` directory of Unikraft's catalog run:

```
kraft build
```

Check the output of the command for the path of the generated cpio file. Usually
the path is `.unikraft/build/initramfs-x86_64.cpio`.

> **NOTE**: For simplicity we will use the rootfs.cpio as the filename of the cpio.

### Step 3: Package everything together.

To package everything together, we can use the following `Containerfile`:

```
#syntax=harbor.nbfc.io/nubificus/bunny:latest
FROM unikraft.org/base:latest

LABEL com.urunc.unikernel.binary="/unikraft/bin/kernel"
LABEL "com.urunc.unikernel.cmdline"="/chttp"
LABEL "com.urunc.unikernel.unikernelType"="unikraft"
LABEL "com.urunc.unikernel.hypervisor"="qemu"
```

## Using a `bunnyfile`

Due to the capabilities of `bunny`, if we choose to use a `bunnyfile`, the steps
2 and 3 can be performed in one run. Therefore, we build the C application in
the same way as in the [first step](#Step-1:-Building-the-C-HTTP-Server)
previously. Then, we can define the `bunnyfile` as:

```
#syntax=harbor.nbfc.io/nubificus/bunny:latest
version: v0.1

platforms:
  framework: unikraft
  version: 0.18.0
  monitor: qemu
  architecture: x86

rootfs:
  from: scratch
  include:
    - chttp:/chttp

kernel:
  from: unikraft.org/base:latest
  path: /unikraft/bin/kernel

cmdline: /chttp
```

With the above bunnyfile`, `bunny` will build the rootfs of Unikraft as a cpio
file with the contents we define in the `include` field of `rootfs`.

## Building the image with bunny as buildkit's frontend

In the case, we want to build the image directly through docker, using `bunny`
as a frontend for buildkit, we can simply run the following command:

```
docker build -f bunnyfile -t harbor.nbfc.io/nubificus/urunc/chttp-unikraft-qemu:test .
```

The image will get loaded in the local docker registry. If we want to build with
[annotations](#https://github.com/nubificus/bunny/docs/annotations.md), then the
command should change to the following:

```
docker buildx build --builder=<container-build-driver>  --output "type=image,oci-mediatypes=true" -f bunnyfile -t harbor.nbfc.io/nubificus/urunc/chttp-unikraft-qemu:test --push=true .
```
The image will get pushed in the registry.

## Building the image with `bunny` and buildctl

In the case we want to use `bunny` and produce a LLB to pass it to buildctl,
We can build the image with the following command:

```
./bunny -LLB -f bunnyfile | sudo buildctl build ... --local context=${PWD} --output type=docker,name=harbor.nbfc.io/nubificus/urunc/chttp-unikraft-qemu:test | sudo docker load
```

The image will get loaded in the local docker registry. If we want to build with
[annotations](#https://github.com/nubificus/bunny/docs/annotations.md), then the
command should change to the following:

```
./bunny --LLB -f bunnyfile | sudo buildctl build ... --local context=${PWD} --output "type=image,name=harbor.nbfc.io/nubificus/urunc/chttp-unikraft-qemu:test,oci-mediatypes=true,push=true"
```

The image will get pushed in the registry.

> **NOTE**: In the above commands by simply replacing bunnyfile with
> Containerfile and simply switch from one to the other file formats.
