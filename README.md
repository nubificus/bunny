# Bunny: Build and package unikernels like containers

Welcome to `bunny`, a tool that promises to make the building of unikernels as
easy an building containers.

## Table of Contents

1. [Introduction](#introduction)
2. [The `bunnyfile`](#the-bunnyfile)
3. [Trying it out](#trying-it-out)
5. [Supported frameworks](#supported-frameworks)
7. [Execution modes](#execution-modes)
7. [Contributing](#contributing)
8. [License](#license)
9. [Contact](#contact)

## Introduction

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

## The `bunnyfile`

In order to instruct `bunny` how to build and package unikernels, we have
defined a yaml-based file, called `bunnyfile`. You can think of `bunnyfile` as
the equivalent of `Dockerfile` for containers. The currently supported fields of
the `bunnyfile` are the following:

```
#syntax=harbor.nbfc.io/nubificus/bunny:latest   # [1] Set bunnyfile syntax for automatic recognition from buildkit.
version: v0.1                                   # [2] Bunnyfile version.

platforms:                                      # [3] The target platform for building/packaging.
  framework: unikraft                           # [3a] The unikernel framework.
  version: v0.15.0                              # [3b] The version of the unikernel framework.
  monitor: qemu                                 # [3c] The hypervisor/VMM or any other kind of monitor.
  architecture: x86                             # [3d] The target architecture.

rootfs:                                         # [4] (Optional) Specifies the rootfs of the unikernel.
  from: local                                   # [4a] (Optional) The source or base of the rootfs.
  path: initrd                                  # [4b] (Required if from is not scratch) The path in the source, where the prebuilt rootfs file resides.
  type: initrd                                  # [4c] (optional) The type of rootfs (e.g. initrd, raw, block)
  include:                                      # [4d] (Optional) A list of local files to include in the rootfs
    - src:dst

kernel:                                         # [5] Specify a prebuilt kernel to use
  from: local                                   # [5a] Specify the source of a prebuilt kernel.
  path: local                                   # [5b] The path where the kernel image resides.

envs:                                           # [6] A list with all environment variables
  - HOME=/home/ubuntu

cmd: ["app"]                                    # [7] The command line arguments of the app

entrypoint: ["init"]                            # [8] The entrypoint of the container

```

The fields of `bunnyfile` in more details:

|    | Description | Required | Value type | Default value |
| -  | ----------- | -------- | ------------- | ------------ |
| 1  | instruct Buildkit to use `bunny` for parsing this file | yes | buildkit directive | - | 
| 2  | API version of `bunnyfile` format. Current version is v0.1 | yes | string in major version format (vX) | - |
| 3  | Information about targeting platform | yes | - | - |
| 3a | The unikernel/libOS to target | yes | string | - |
| 3b | The unikernel/libOS version | no | string | latest |
| 3c | The VMM or any kind of monitor, where the unikernel will run on top | no | string | framework-dependent |
| 3d | The target architecture| no | string | `bunny`'s host arch |
| 4  | Instructions about unikernel's rootfs | no | - | - |
| 4a | The base image or an image/location that contains a rootfs | no | "scratch", "local", "OCI image" | "scratch" |
| 4b | The path relative to the `from` field where a rootfs file resides | yes, if `from == "local"` | file path | - |
| 4c | The type of the rootfs | no | "raw", "initrd" | platform-dependent |
| 4d | Files from the build context to include in the rootfs | no | list of <build-context-path>:<rootfs-contextext-path> | - |
| 5  | Information about a prebuilt kernel | no | - | - |
| 5a | The location where the prebuilt kernel resides | no | "local", "OCI image" | - |
| 5b | The path relative to the `from` field where a kernel binary resides | yes, if `from` is set  | "local", "OCI image" | - |
| 6  | A list of environment variables| no | list of <ENVIRONMENT_VARIABLE>:<VALUE> | - |
| 7  | The command line of the application | no | []string | - |
| 8  | The entrypoint of the container | no | []string | - |

### The `rootfs` field

The unikernel and libOS landscape is very diverse and each framework/technology
comes with its own support for storage. The users can easily get lost on the
various storage technologies that each framework supports. For that purpose, `bunny`
aims to provide a common interface to let the users easily define the contents
of the unikernel's rootfs. Hence, let's take a closer look on this part of
`bunnyfile`.

#### The `from` field

In this field, we can instruct `bunny` to use an existing rootfs file or create
one. It is similar to the Dockerfile's `FROM` instruction, but in `bunny` it is
also used to define the location of an existing rootfs file.
The `from` field can have one of the following values:

- **scratch**: `bunny` will build the rootfs from scratch (starting with an empty
  directory).
- **local**": we can use this value to use an existing rootfs file that resides in
  the local build context of `bunny`.
- **OCI image**: an OCI image to use as a base for the rootfs, or an OCI image
  that contains an existing rootfs file.

#### The `path` field

This field makes sense only when we define "local" or "OCI image" in the `from`
field. In the `path` field users should define the path where an existing rootfs
file resides. Tsuch cases could be a rootfs file (e.g. initrd, virtio-block,
etc.) created locally or reusing one from another OCI image.

#### The `type` field

Some unikernel frameworks, or similar technologies, support a single type of
storage and hence this field is quite useless. If that is the case, `bunny` will
print the respective error message. On the other hand, some frameworks support
more than one storage types. For instance, Linux can use an initramfs, a
virtio-block or even shared-fs as the rootfs. These are the cases were this
field is useful. Users can choose (if they wish) a specific type of rootfs and
`bunny` will build it. Currently, `bunny` can create the following types:

- **initrd**: A typical cpio file that guests can use as an initial rootfs.
- **raw**: In this case `bunny` does not build any specific kind of file, but
  instead copies the files, that the user specifies, directly in the OCI image's
  rootfs. This type is useful when we want to pass the entire OCI image's rootfs
  to the guest, either through share-fs or devmapper.

#### The `include` field

In this field users can define the files from the local build context that they
want to include in the rootfs. It is equivalent to the `COPY` instruction in
Dockerfile. The field accepts a list of entries with the following format:

```
- <path_in the_local_build_context>:<path_inside_the_rootfs>
```

> **_NOTE:_**  Except of the `bunnyfile`, `bunny` supports the Dockerfile-like
> file that
> [pun](https://github.com/nubificus/pun?tab=readme-ov-file#the-containerfile-format)
> and
> [bima](https://github.com/nubificus/bima?tab=readme-ov-file#how-bima-works)
> takes as input. However, the Dockerfile-like syntex is limited only to
> packaging existing unikernels. With the `bunnyfile`, `bunny` is able to
> provide much more functionalities and features.

## Trying it out

The easiest and most effortless way to try out `bunny` would be using it as a
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

cmd: ["/server"]
```

We can then package everything with the following command:

```
docker build -f bunnyfile -t harbor.nbfc.io/nubificus/urunc/httpc-unikraft-qemu:test .
```

The above `bunnyfile` will perform the following steps:
1. Copy the file `initramfs-x86_64.cpio` from the local build context to an
   empty OCI image at `/boot/rootfs`.
2. Copy the file `kernel` from the local build context to the OCI image we used
   in the previous step at `/boot/kernel`.
3. Set up [`urunc`'s annotations](https://urunc.io/image-building/#annotations)
   using all the information in the file (e.g. framework, version, cmd,
   binary, initrd).
4. Produce the final OCI image.

For more examples, please take a look in the [examples
directory](https://github.com/nubificus/bunny/tree/main/examples/README.md)..

## Supported frameworks

At the moment, `bunny` is available on GNU/Linux for x86\_64 and arm64 architectures.
The main goals of `bunny` is to build and package unikernels as OCI images.
Packaging unikernels is agnostic of the frameworks and hence `bunny` can be used
for any unikernel framework and similar technology. However, for
building unikernels, `bunny` only provides experimental support which is not
merged in the main branch yet. We are working on it. However, `bunny` can still be
used to build the `rootfs` for some unikernels. The below table provides an
overview of the currently supported unikernels and frameworks:

| Unikernel  | Build    | Rootfs |
|----------- |--------- |------- |
| Rumprun    | :hammer: | raw    |
| Unikraft   | :hammer: | Initrd |

We plan to add support for more and more unikernel frameworks and similar technologies.
Feel free to [contact](#contact) us for a specific unikernel framework or similar
technologies that you would like to see in `bunny`.

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

## Contributing

We will be very happy to receive any feedback and any kind of contributions for
`bunny`. For more details please take a look in [`bunny`'s contributing
document](CONTRIBUTING.md).

## License

[Apache License 2.0](LICENSE)

## Contact

Feel free to contact any of the authors directly using their emails in the
commit messages.
