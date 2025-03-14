# Annotations

In order to execute unikernels as containers, we need [urunc](https://github.com/nubificus/urunc) a container runtime that
can handle and manage the execution of unikernels. However, `urunc` needs to
differentiate typical containers from unikernels. One way of `urunc` to achieve
this is by using 
annotations in the container image. As such, `bunny` treats all Labels defined in the
Containerfile as annotations. In particular, the annotations will be stored in
the image manifest.

## Docker and annotations

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
