FROM golang:1.22 AS builder

COPY . /pun

WORKDIR /pun
RUN make

FROM scratch
COPY --from=builder /pun/dist/pun /bin/pun
ENTRYPOINT ["/bin/pun"]
