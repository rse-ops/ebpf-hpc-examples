FROM golang:1.21

# docker build -t rse-ops/ebpf-hpc-examples:sys_execv .

RUN apt-get update && apt-get install -y \
    clang \
    gcc-multilib \
    libbpf-dev

RUN go install github.com/cilium/ebpf/cmd/bpf2go@latest

WORKDIR /code
COPY . /code/

WORKDIR /code/
RUN rm bpf_* && go generate && go build
ENTRYPOINT ["/code/sys_execve", "--file", "/bpf_bpfel.o"]
