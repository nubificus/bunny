# Bunny: Build and package unikernels like containers

Unikernels is a promising technology that can achieve extremely fast boot times,
a smaller memory footprint, increased performance, and more robust security than
containers. Therefore, Unikernels could be very useful in cloud deployments,
especially for microservices and function deployments. However, Unikernels are
notorious for being difficult to build and even execute them.

In order to provide a user-friendly build process for unikernels and let the
users build and use them as easy as containers we build `bunny`. The main goal
of `bunny` is to provide a unified and simplified process of building and
packaging unikernels. With `bunny`, a user can simply build an application as a
unikernel and get its binary or even package it as an OCI image with all
necessary metadata to run it with [urunc](https://github.com/nubificus/urunc).

`bunny` is based on [pun](https://github.com/nubificus/pun) a tool that packages
already built unikernels in OCI images for `urunc`. THis functionality is and
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
`bunny` directly without even building or fetching its binary.

### Using buildctl

In order to use `bunny` with buildctl, we have to build it locally and then feed
its output to buildctl.

#### How to build

We can build `bunny` by simply running make:

```
make
```

> **_NOTE:_**  `bunny` was created with Golang version 1.23.4

#### How to use

`bunny` makes use of Buildkit and LLB. As a result, the application itself does
not produce any artifacts, but just a LLB graph which we can later feed to
buildkit. Therefore, in order to use `bunny`, we need to firstly install buildkit.
For more information regarding building and installing buildkit, please refer
to buildkit
[instructions](https://github.com/moby/buildkit?tab=readme-ov-file#quick-start).

As long as buildkit is installed in our system, we can package any unikernel
with the following command:
```
./bunny --LLB -f Containerfile | sudo buildctl build ... --local context=<path_to_local_context> --output type=<type>,<type-specific-args>
```

`bunny` takes as an argument the `Containerfile` a
Dockerfile-syntax file with the packaging instructions.

Regarding the buildctl arguments:
- `--local context` specifies the directory where the user wants to set the
  local context. It is similar to the build context in the docker build command.
Therefore, if we specify any `COPY` instructions in the `Containerfile`, the
paths will be relative to this argument.
- `--output type=<type>` specifies the output format. Buildkit supports various
  [outputs](https://github.com/moby/buildkit/tree/master?tab=readme-ov-file#output).
  Just for convenience we mention the `docker` output, which produces an output
  that we can pass to `docker load` in order to place our image in the local
  docker registry. We can also specify the name of the image, using the
  `name=<name` in the ``<type-specific-option>`.

For instance:

```
./bunny --LLB -f Containerfile | sudo buildctl build ... --local context=/home/ubuntu/unikernels/ --output type=docker,name=harbor.nbfc.io/nubificus/urunc/bunny:latest | sudo docker load
```

#### The Containerfile format

`bunny` supports Dockerfile-style files as input. Therefore, any such file can be
given as input. However, it is important to note, that `bunny`'s OCI images are
built for `urunc`. Therefore, currently only the following
instructions are supported:
- `FROM`: Specifies the base image. It can be any image or just `scratch`
- `COPY`: Copies local files inside the image as a new layer.
- `LABEL`: Specifies annotations for the image.

All the other instructions will get ignored.

## Annotations

In order to execute unikernels as containers, we need `urunc` a container runtime that
can handle and manage the execution of unikernels. However, `urunc` needs to
differenciate typical containers from unikernels. One way of `urunc` to achieve
this is by using 
annotations in the container image. As such, `bunny` treats all Labels defined in the
Containerfile as annotations. In particular, the annotations will be stored in
the image manifest.

### Docker and annotations

In order to make use of this feature, `bunny` should be used from a tool that
can export the image in the OCI format. According to [docker's
documentation](https://docs.docker.com/build/exporters/oci-docker/), the
default docker driver does not support exports in the OCI format. Instead,
someone needs to use a [Docker container build
driver](https://docs.docker.com/build/builders/drivers/docker-container/).
Another way to use `bunny` and export the image in the OCI format is through
[buildctl](https://github.com/moby/buildkit?tab=readme-ov-file#output). In both
cases, the annotations will remain in the image setting the following options:
1. Choose image or OCI as an output type.
2. Use the `oci-mediatypes=true` option
3. make sure to immediately push the image in a registry.

Therofore, a docker buildx command could be:
```
docker buildx build --builder=<container-build-driver>  --output "type=image,oci-mediatypes=true" -f <path-to-Containerfile>r -t <image-name> --push=true <path-tobuild-context>
```

Similarly a buildctl command could be:
 ```
buildctl build --frontend gateway.v0 --opt source=harbor.nbfc.io/nubificus/urunc/bunny/llb:latest --output "type=image,name=<image-name>,oci-mediatypes=true,push=true" --local context=<path-to-build-context> --local dockerfile=<path-to-dir-containing-Containerfile> -opt filename=<name-of-Containerfile>
 ```

Furthermore, it is important to note that the Docker Engine does not support
annotations. For more information, take a look in [docker's
documentation](https://docs.docker.com/build/metadata/annotations/#add-annotations%CE%B5%CE%AF%CE%BD%CE%B1%CE%B9).
Subsequently, pulling any image built with `bunny` that has annotations in a local
docker Engine registry will result to losing all the annotations. For this
reason we need to push the output image immediately after build and not
store it locally.

## Examples

### Packaging a rumprun unikernel with `bunny` as buildkit's frontend

In case we have already built a rumprun unikernel, we can easily package it with
a normal docker command that will use `bunny`. To do that we need the following
`Containerfile`:
```
#syntax=harbor.nbfc.io/nubificus/bunny:latest
FROM scratch

COPY test-redis.hvt /unikernel/test-redis.hvt
COPY redis.conf /conf/redis.conf

LABEL com.urunc.unikernel.binary=/unikernel/test-redis.hvt
LABEL "com.urunc.unikernel.cmdline"='redis-server /data/conf/redis.conf'
LABEL "com.urunc.unikernel.unikernelType"="rumprun"
LABEL "com.urunc.unikernel.hypervisor"="hvt"
```

We can then build the image with the following command:
```
docker build -f Containerfile -t harbor.nbfc.io/nubificus/urunc/redis-rumprun-hvt:test .
```
The image will get loaded in the local docker registry. If we want to build with [annotations](#Docker-and-annotations)

```
docker buildx build --builder=<container-build-driver>  --output "type=image,oci-mediatypes=true" -f Containerfile -t harbor.nbfc.io/nubificus/urunc/redis-rumprun-hvt:test --push=true .
```

The image will get pushed in the registry.

### Packaging a unikraft unikernel with `bunny` as buildkit's frontend

In case we want to use an existing [unikraft](unikraft.org) unikernel image from
[unikraft's catalog](https://github.com/unikraft/catalog), we can transform it
to an image that `urunc` can execute with `bunny`. In that case the
`Containerfile` should look like:
```
#syntax=harbor.nbfc.io/nubificus/bunny:latest
FROM unikraft.org/nginx:1.15

LABEL com.urunc.unikernel.binary="/unikraft/bin/kernel"
LABEL "com.urunc.unikernel.cmdline"="nginx -c /nginx/conf/nginx.conf"
LABEL "com.urunc.unikernel.unikernelType"="unikraft"
LABEL "com.urunc.unikernel.hypervisor"="qemu"
```

We can then build the image with the following command:

```
docker build -f Containerfile -t harbor.nbfc.io/nubificus/urunc/nginx-unikraft-qemu:test .
```

The image will get loaded in the local docker registry. If we want to build
with [annotations](#Docker-and-annotations):

```
docker buildx build --builder=<container-build-driver>  --output "type=image,oci-mediatypes=true" -f Containerfile -t harbor.nbfc.io/nubificus/urunc/nginx-unikraft-qemu:test --push=true .
```

The image will get pushed in the registry.

### Packaging a rumprun unikernel with `bunny` and buildctl

In case we have already built a rumprun unikernel, we can easily package it with
`bunny` using the following `Containerfile`:
```
FROM scratch

COPY test-redis.hvt /unikernel/test-redis.hvt
COPY redis.conf /conf/redis.conf

LABEL com.urunc.unikernel.binary=/unikernel/test-redis.hvt
LABEL "com.urunc.unikernel.cmdline"='redis-server /data/conf/redis.conf'
LABEL "com.urunc.unikernel.unikernelType"="rumprun"
LABEL "com.urunc.unikernel.hypervisor"="hvt"
```

We can then build the image with the following command:
```
./bunny --LLB -f Containerfile | sudo buildctl build ... --local context=${PWD} --output type=docker,name=harbor.nbfc.io/nubificus/urunc/redis-rumprun-hvt:test | sudo docker load
--output "type=image,name=<image-name>,oci-mediatypes=true,push=true"
```
The image will get loaded in the local docker registry. If we want to build with [annotations](#Docker-and-annotations):
```
./bunny --LLB -f Containerfile | sudo buildctl build ... --local context=${PWD} --output "type=image,name=harbor.nbfc.io/nubificus/urunc/redis-rumprun-hvt:test,oci-mediatypes=true,push=true"
```

The image will get pushed in the registry.

### Packaging a unikraft unikernel with `bunny` with buildctl

In case we want to use an existing [unikraft](unikraft.org) unikernel image from
[unikraft's catalog](https://github.com/unikraft/catalog), we can transform it
to an image that `urunc` can execute with `bunny`. In that case the
`Containerfile` should look like:

```
FROM unikraft.org/nginx:1.15

LABEL com.urunc.unikernel.binary="/unikraft/bin/kernel"
LABEL "com.urunc.unikernel.cmdline"="nginx -c /nginx/conf/nginx.conf"
LABEL "com.urunc.unikernel.unikernelType"="unikraft"
LABEL "com.urunc.unikernel.hypervisor"="qemu"
```

We can then build the image with the following command:
```
./bunny -LLB -f Containerfile | sudo buildctl build ... --local context=${PWD} --output type=docker,name=harbor.nbfc.io/nubificus/urunc/nginx-unikraft-qemu:test | sudo docker load
```

The image will get loaded in the local docker registry. If we want to build
the image with [annotations](#Docker-and-annotations):

```
./bunny --LLB -f Containerfile | sudo buildctl build ... --local context=${PWD} --output "type=image,name=harbor.nbfc.io/nubificus/urunc/nginx-unikraft-qemu:test,oci-mediatypes=true,push=true"
```

The image will get pushed in the registry.
