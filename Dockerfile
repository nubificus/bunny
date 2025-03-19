FROM golang:1.24 AS builder

COPY . /bunny

WORKDIR /bunny
RUN make

FROM scratch
COPY --from=builder /bunny/dist/bunny /bin/bunny
ENTRYPOINT ["/bin/bunny"]
