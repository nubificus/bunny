# Packaging a unikraft unikernel with `bunny`

For this example, we will use an existing [Unikraft](unikraft.org) Unikernel
image from [Unikraft's catalog](https://github.com/unikraft/catalog), we can
transform it to an image that [urunc](https://github.com/nubificus/urunc) can
execute with `bunny`. The respective `Containerfile` that we would use with
[pun](https://github.com/nubificus/pun) would be:

```
#syntax=harbor.nbfc.io/nubificus/bunny:latest FROM unikraft.org/nginx:1.15

LABEL com.urunc.unikernel.binary="/unikraft/bin/kernel"
LABEL "com.urunc.unikernel.cmdline"="nginx -c /nginx/conf/nginx.conf"
LABEL "com.urunc.unikernel.unikernelType"="unikraft"
LABEL "com.urunc.unikernel.hypervisor"="qemu"
```

In order to use `bunny`, instead, we need to specify the
following `bunnyfile` to package it as an OCI image for
[urunc](https://github.com/nubificus/urunc).

```
#syntax=harbor.nbfc.io/nubificus/bunny:latest
version: v0.1

platforms:
  framework: unikraft
  monitor: qemu
  architecture: x86

kernel:
  from: unikraft.org/nginx:1.15
  path: /unikraft/bin/kernel

cmdline: nginx -c /nginx/conf/nginx.conf
```

## Building the image with bunny as buildkit's frontend

In the case, we want to build the image directly through docker, using `bunny`
as a frontend for buildkit, we can simply run the following command:

```
docker build -f Containerfile -t harbor.nbfc.io/nubificus/urunc/nginx-unikraft-qemu:test .
```

The image will get loaded in the local docker registry. If we want to build with
[annotations](#https://github.com/nubificus/bunny/docs/annotations.md), then the
command should change to the following:

```
docker buildx build --builder=<container-build-driver>  --output "type=image,oci-mediatypes=true" -f Containerfile -t harbor.nbfc.io/nubificus/urunc/nginx-unikraft-qemu:test --push=true .
```

The image will get pushed in the registry.

## Building the image with `bunny` and buildctl

In the case we want to use `bunny` and produce a LLB to pass it to buildctl,
We can build the image with the following command:

```
./bunny -LLB -f Containerfile | sudo buildctl build ... --local context=${PWD} --output type=docker,name=harbor.nbfc.io/nubificus/urunc/nginx-unikraft-qemu:test | sudo docker load
```

The image will get loaded in the local docker registry. If we want to build with
[annotations](#https://github.com/nubificus/bunny/docs/annotations.md), then the
command should change to the following:

```
./bunny --LLB -f Containerfile | sudo buildctl build ... --local context=${PWD} --output "type=image,name=harbor.nbfc.io/nubificus/urunc/nginx-unikraft-qemu:test,oci-mediatypes=true,push=true"
```

The image will get pushed in the registry.

