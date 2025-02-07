# Bunny: Build and package unikernels like containers

Unikernels is a promising technology that can achieve extremely fast boot times,
a smaller memory footprint, increased performance, and more robust security than
containers. Therefore, Unikernels could be very useful in cloud deployments,
especially for microservices and function deployments. However, Unikernels are
notorious for being difficult to build and even deploy them.

In order to provide a user-friendly build process for unikernels and let the
users build and use them as easy as containers we build `bunny`. The main goal
of `bunny` is to provide a unified and simplified process of building and
packaging unikernels. With `bunny`, a user can simply build an application as a
unikernel and even package it as an OCI image with all
necessary metadata to run it with [urunc](https://github.com/nubificus/urunc).

`bunny` is based on [pun](https://github.com/nubificus/pun) a tool that packages
already built unikernels in OCI images for `urunc`. This functionality is and
will be supported from `bunny` as well. However, `bunny` is also able to build
unikernels from scratch.

## Execution modes

`bunny` supports two modes of execution: either a) local execution, printing a
LLB, or b) as a frontend for Docker's buildkit. The easiest way is to use it as
a frontend.

### As buildkit's frontend

In order to use `bunny` as Buildkit's frontend, we just need to add a new
line in the top of the Dockerfile. In particular, every file that we want to
use for 'bunny' needs to start with the following line.

```
#syntax=harbor.nbfc.io/nubificus/bunny:latest
```

Then, we can just execute docker build as usual. Buildkit will fetch an image
containing `bunny` and it will use it as a frontend. Therefore, anyone can use
`bunny` directly without even building or installing its binary.

### Using buildctl

In order to use `bunny` with buildctl, we have to build it locally, run it and then feed
its output to buildctl.

#### How to build

We can easily build `bunny` by simply running make:

```
make
```

> **_NOTE:_**  `bunny` was created with Golang version 1.23.4

#### How to use

In the case of buildctl, `bunny` does not produce any artifact by itself.
Instead, it outputs a LLB that we can then feed in buildctl. 
Therefore, in order to use `bunny`, we need to firstly install buildkit.
For more information regarding building and installing buildkit, please refer
to buildkit
[instructions](https://github.com/moby/buildkit?tab=readme-ov-file#quick-start).

As long as buildkit is running in our system, we can use `bunny`
with the following command:
```
./bunny --LLB -f bunnyfile | sudo buildctl build ... --local context=<path_to_local_context> --output type=<type>,<type-specific-args>
```

`bunny` takes as an argument the `bunnyfile` a
yaml file with instructions to build and package unikernels. For more
information regarding bunnyfile please check the [respective
section](#Bunnyfile)

Regarding the buildctl arguments:
- `--local context` specifies the directory of the local context. It is similar
  to the build context in the docker build command.  Therefore, if we specify
  want to copy any file from the host, the path should be relative to the
  directory of this argument.
- `--output type=<type>` specifies the output format. Buildkit supports various
  [outputs](https://github.com/moby/buildkit/tree/master?tab=readme-ov-file#output).
  Just for convenience we mention the `docker` output, which produces an output
  that we can pass to `docker load` in order to place our image in the local
  docker registry. We can also specify the name of the image, using the
  `name=<name` in the `<type-specific-option>`.

For instance:

```
./bunny --LLB -f bunnyfile | sudo buildctl build ... --local context=/home/ubuntu/unikernels/ --output type=docker,name=harbor.nbfc.io/nubificus/urunc/built-by-bunny:latest | sudo docker load
```

## Bunnyfile

Similarly to `docker`, `bunny` takes as an in put a file following the
`bunnyfile` format. It is based on yaml and it includes all the information for
building an application as a Unikernel, along with instructions on how to
package the Unikernel, along with other files that the application requires.

```
#syntax=harbor.nbfc.io/nubificus/bunny:latest   # [1] Set bunnyfile syntax for automatic recognition from docker.
version: v0.1                                   # [2] Bunnyfile version.

platforms:                                      # [3] The target unikernel platform for building/packaging.
  framework: unikraft                           # [3a] The unikernel framework.
  version: v0.15.0                              # [3b] The version of the unikernel framework.
  monitor: qemu                                 # [3c] The hypervisor/VMM or any other kind of monitor, where the unikernel will run  on top.
  architecture: x86                             # [3d] The host architecture where the unikernel will run.

rootfs:                                         # [4] (Optional) Specifies the rootfs of the unikernel.
  from: local                                   # [4a] (Optional) The source of the initrd.
  path: initrd                                  # [4b] (Required if from is not scratch) The path in the source, where a prebuilt rootfs file resides.
  type: initrd                                  # [4c] The type of rootfs, in case the unikernel framework supports more than one (e.g. initrd, raw, block)
  include:                                      # [4d] (Optional) A list of files to include in the rootfs
    - src:dst

kernel:                                         # [5] Specify a prebuilt kernel to use
  from: local                                   # [5a] Specify the source of an existing prebuilt kernel.
  path: local                                   # [5b] Specify the path to the kernel image inside the KernelSource.

cmdline: hello                                  # [6] The cmdline of the app

```

The fields of `bunnyfile` in more details:
1. A docker syntax directive. It is required to let the buildkit choose `bunny`
   as a frontend, instead of the default dockerfile frontend.
2. The version of the `bunnyfile` format. It is required to prevent
   incompatibilities with different versions.
3. The platform to target for building/packaging the unikernel.
    3a. The unikernel framework to target.
    3b. The version of unikernel framework to target.
    3c. The hypervisor/VMM or any other runtime/monitor where the unikernel will
        run on top. Current supported values are:
        [qemu](https://www.qemu.org/),
        [firecracker](https://github.com/firecracker-microvm/firecracker),
        [hvt](https://github.com/Solo5/solo5),
        [Spt](https://github.com/Solo5/solo5).
    3d. The architecture of the host where the unikernel will run.
4. (Optional) The rootfs of the unikernel. Currently, `bunny` is able to either
   package existing rootfs files, or to build the rootfs in the form of an
   initrd.
    4a. The source where an existing rootfs file is stored. Current supported
        values are: i) `scratch`, meaning that the rootfs should get built from
        scratch, and ii) `local`, meaning that an existing rootfs file resides
        somewhere locally. The default value is `scratch`.
    4b. The path for the file in the aformentioned source. This field is
        required, if `from` has a value other than `scratch`..In the
        case where the `from` field has the value `local`, then the `path` should
        be relative to the build context.
    4c. The type of rootfs. In most cases the unikernel framework supports only
        one type of a rootfs (e.g. initramfs). However, there are frameworks where
        the rootfs can have various forms, such as initrd, block device and
        shared-fs. For the time being, bunny can construct only two types of
        rootfs; initrd and raw. In the case of raw all the files are simply copied
        inside the container rootfs and we can use shared-fs or transform the
        container rootfs to a block (this can easily happen using devmapper as a
        snapshotter). This field is optional and `bunny` will create the default type
        of rootfs for the respective unikernel framework.
    4d. A list of files to include in the rootfs. This field takes effect only
        when the `from` field has the value `scratch`. The files can be defined
        in the following format: `- <path_in_the_build_context>:<path_inside_rootfs>`. The `<path_inside_rootfs>` can be omitted and then the same path as the one in `<path_in_the_build_context>` will be used.
5. Information regarding an existing prebuilt kernel to use. For the time
   being, `bunny` supports only prebuilt unikernels and `bunny` will package
   everything as an OCI image with the respective annotations for
   [urunc](https://github.com/Solo5/solo5).
    5a. The source where an existing kernel is stored. Currently, only the
        `local` value is supported, meaning that the file resides in somewhere locally.
    5b. The path in the source where an existing kernel resides. In the case
        where the `kernelSource` field has the value `local`, then the
        `kernelPath` should be relative to the build context.

> **_NOTE:_**  Except of the `bunnyfile`, `bunny` supports the Dockerfile-like
> file that [pun](https://github.com/nubificus/pun?tab=readme-ov-file#the-containerfile-format) and
> [bima](https://github.com/nubificus/bima?tab=readme-ov-file#how-bima-works) takes as input.

## Trying it out

The easiest and more effortless way to try out `bunny` would be using it as a
buildkit's frontend. Therefore, assuming that we have an existing Unikraft
unikernel with its initrd already prebuilt, we can package everything as an OCI
image for [urunc](https://github.com/nubificus/urunc) with the following
`bunnyfile`:

```
#syntax=harbor.nbfc.io/nubificus/bunny:latest
version: v0.1

platforms:
  framework: unikraft
  version: 0.17.0
  monitor: qemu
  architecture: x86

rootfs:
  from: local
  path: initramfs-x86_64.cpio

kernel:
  from: local
  path: kernel

cmdline: /server
```

We can then package everything with the following command:

```
docker build -f bunnyfile -t harbor.nbfc.io/nubificus/urunc/httpc-unikraft-qemu:test .
```

The above `bunnyfile` will package an existing Unikraft unikernel that we
obtained from [http-c example in Unikraft's
catalog](https://github.com/unikraft/catalog/tree/main/examples/http-c). 

For more examples, please take a look in the [examples
directory](https://github.com/nubificus/bunny/tree/main/examples/README.md)..
