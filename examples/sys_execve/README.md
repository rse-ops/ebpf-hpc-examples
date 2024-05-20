# Dummy Test with Sysexecv

```bash
docker build --no-cache -t rse-ops/ebpf-hpc-examples:sys_execv .
docker build -f Dockerfile.bytecode --no-cache -t rse-ops/ebpf-hpc-examples:sys_execv-bytecode .
```

And:

```bash
docker push -t rse-ops/ebpf-hpc-examples:sys_execv
docker push -t rse-ops/ebpf-hpc-examples:sys_execv-bytecode
```

Or:

```bash
kind load docker-image rse-ops/ebpf-hpc-examples:sys_execv
kind load docker-image rse-ops/ebpf-hpc-examples:sys_execv-bytecode
```

And then for your kind cluster, install bpfman

```bash
export BPFMAN_REL=0.4.1
kubectl apply -f  https://github.com/bpfman/bpfman/releases/download/v${BPFMAN_REL}/bpfman-crds-install.yaml
```

Install the operator:

```bash
kubectl apply -f https://github.com/bpfman/bpfman/releases/download/v${BPFMAN_REL}/bpfman-operator-install.yaml
```

And install your program!

```bash
kubectl apply -f ebpf.yaml
```

Note that I am currently getting a message that the bytecode is invalid. I've tried varying the metadata / build strategy of the bytecode container quite a bit, but I suspect I'm missing a huge detail.

```console
$ kubectl get bpfprograms.bpfman.io 
NAME                                                          TYPE     STATUS                  AGE
go-kprobe-counter-example-kind-control-plane-try-to-wake-up   kprobe   bytecodeSelectorError   30s
```

And describe:

```console
$ kubectl describe bpfprograms.bpfman.io 
Name:         go-kprobe-counter-example-kind-control-plane-try-to-wake-up
Namespace:    
Labels:       bpfman.io/ownedByProgram=go-kprobe-counter-example
              kubernetes.io/hostname=kind-control-plane
Annotations:  bpfman.io.kprobeprogramcontroller/function: try_to_wake_up
API Version:  bpfman.io/v1alpha1
Kind:         BpfProgram
Metadata:
  Creation Timestamp:  2024-05-20T20:39:28Z
  Finalizers:
    bpfman.io.kprobeprogramcontroller/finalizer
  Generation:  1
  Owner References:
    API Version:           bpfman.io/v1alpha1
    Block Owner Deletion:  true
    Controller:            true
    Kind:                  KprobeProgram
    Name:                  go-kprobe-counter-example
    UID:                   e35f19bb-6c33-4684-b025-29cd4c52ed7f
  Resource Version:        1247
  UID:                     31ee91c9-3d81-42a9-840b-e1bb5161d43f
Spec:
  Type:  kprobe
Status:
  Conditions:
    Last Transition Time:  2024-05-20T20:39:28Z
    Message:               There was an error processing the provided bytecode selector
    Reason:                bytecodeSelectorError
    Status:                True
    Type:                  BytecodeSelectorError
Events:                    <none>
```

Daemon logs:

```console
$ kubectl logs -n bpfman bpfman-daemon-bj9gp 
Defaulted container "bpfman" out of: bpfman, bpfman-agent, node-driver-registrar, mount-bpffs (init)
[INFO  bpfman_rpc::serve] Using default Unix socket
[INFO  bpfman_rpc::serve] Using no inactivity timer
[INFO  bpfman_rpc::serve] Listening on /run/bpfman-sock/bpfman.sock
[INFO  bpfman_rpc::storage] CSI Plugin Listening on /run/bpfman/csi/csi.sock
[INFO  bpfman::utils] Has CAP_BPF: true
[INFO  bpfman::utils] Has CAP_SYS_ADMIN: true
[INFO  bpfman::utils] Has CAP_BPF: true
[INFO  bpfman::utils] Has CAP_SYS_ADMIN: true
[INFO  bpfman::utils] Has CAP_BPF: true
[INFO  bpfman::utils] Has CAP_SYS_ADMIN: true
[INFO  bpfman::utils] Has CAP_BPF: true
[INFO  bpfman::utils] Has CAP_SYS_ADMIN: true
[INFO  bpfman::utils] Has CAP_BPF: true
[INFO  bpfman::utils] Has CAP_SYS_ADMIN: true
```

Oh interesting - this might be it? Repository name must be canonical? What repository name?

```console
{"level":"error","ts":"2024-05-20T20:39:28Z","logger":"kprobe","msg":"Reconciling program failed","Program NameError":"json: unsupported type: func() string","ReconcileResult":"Updated","error":"failed to reconcile bpfman program: failed to process bytecode selector: repository name must be canonical","stacktrace":"github.com/bpfman/bpfman/bpfman-operator/controllers/bpfman-agent.(*ReconcilerCommon).reconcileCommon\n\t/usr/src/bpfman/bpfman-operator/controllers/bpfman-agent/common.go:148\ngithub.com/bpfman/bpfman/bpfman-operator/controllers/bpfman-agent.(*KprobeProgramReconciler).Reconcile\n\t/usr/src/bpfman/bpfman-operator/controllers/bpfman-agent/kprobe-program.go:163\nsigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).Reconcile\n\t/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.14.1/pkg/internal/controller/controller.go:122\nsigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).reconcileHandler\n\t/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.14.1/pkg/internal/controller/controller.go:323\nsigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).processNextWorkItem\n\t/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.14.1/pkg/internal/controller/controller.go:274\nsigs.k8s.io/controller-runtime/pkg/internal/controller.(*Controller).Start.func2.2\n\t/go/pkg/mod/sigs.k8s.io/controller-runtime@v0.14.1/pkg/internal/controller/controller.go:235"}
```