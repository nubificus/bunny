# Packaging a Rumprun unikernel with `bunny`

For this example, we assume we have already built a Redis
[Rumprun](https://github.com/cloudkernels/rumprun) unikernel from
[Rumprun-packages](https://github.com/cloudkernels/rumprun-packages) targeting
[Solo5-hvt](https://github.com/Solo5/solo5).
We can package it as an OCI image that [urunc](https://github.com/nubificus/urunc) can execute with `bunny`.

The respective `Containerfile` that we would use with
[pun](https://github.com/nubificus/pun) and
[bima](https://github.com/nubificus/bima) would be:

```
#syntax=harbor.nbfc.io/nubificus/pun:latest
FROM scratch

COPY redis.hvt /unikernel/redis.hvt
COPY redis.conf /conf/redis.conf

LABEL com.urunc.unikernel.binary=/unikernel/redis.hvt
LABEL "com.urunc.unikernel.cmdline"="redis-server /data/conf/redis.conf"
LABEL "com.urunc.unikernel.unikernelType"="rumprun"
LABEL "com.urunc.unikernel.hypervisor"="hvt"
```

In order to use `bunny`, instead, we need to specify the
following `bunnyfile` to package it as an OCI image for
[urunc](https://github.com/nubificus/urunc).

```
#syntax=harbor.nbfc.io/nubificus/bunny:latest
version: v0.1

platforms:
  framework: rumprun
  monitor: hvt
  architecture: x86

rootfs:
  from: scratch
  type: raw
  include:
  - redis.conf:/data/conf/redis.conf

kernel:
  from: local
  path: redis.hvt

cmdline: "redis-server /data/conf/redis.conf"
```

> **NOTE**: Since we use the raw type for rootfs, all the files in the include list
> will simply get copied inside the container's rootfs image. Therefore, to
> let the Rumprun unikernel access the files, we need to specify the `devmapper` as
> a snapshotter when we deploy the unikernel. In that way, the container's rootfs
> will be passed as a block device in the unikernel.

## Building the image with bunny as buildkit's frontend

In the case, we want to build the image directly through docker, using `bunny`
as a frontend for buildkit, we can simply run the following command:

```
docker build -f bunnyfile -t harbor.nbfc.io/nubificus/urunc/redis-rumprun-hvt:test .
```

The image will get loaded in the local docker registry. If we want to build with
[annotations](#https://github.com/nubificus/bunny/docs/annotations.md), then the
command should change to the following:

```
docker buildx build --builder=<container-build-driver>  --output "type=image,oci-mediatypes=true" -f Containerfile -t harbor.nbfc.io/nubificus/urunc/redis-rumprun-hvt:test --push=true .
```

The image will get directly pushed in the registry.

## Building the image with `bunny` and buildctl

In the case we want to use `bunny` and produce a LLB to pass it to buildctl,
We can build the image with the following command:

```
./bunny --LLB -f bunnyfile | sudo buildctl build ... --local context=${PWD} --output type=docker,name=harbor.nbfc.io/nubificus/urunc/redis-rumprun-hvt:test | docker load
--output "type=image,name=<image-name>,oci-mediatypes=true,push=true"
```

The image will get loaded in the local docker registry. If we want to build with
[annotations](#https://github.com/nubificus/bunny/docs/annotations.md), then the
command should change to the following:

```
./bunny --LLB -f Containerfile | sudo buildctl build ... --local context=${PWD} --output "type=image,name=harbor.nbfc.io/nubificus/urunc/redis-rumprun-hvt:test,oci-mediatypes=true,push=true"
```

The image will get pushed in the registry.

> **NOTE**: In the above commands by simply replacing bunnyfile with
> Containerfile and simply switch from one to the other file formats.
