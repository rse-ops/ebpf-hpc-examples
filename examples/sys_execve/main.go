//go:build linux
// +build linux

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	bpfmanHelpers "github.com/bpfman/bpfman/bpfman-operator/pkg/helpers"
	gobpfman "github.com/bpfman/bpfman/clients/gobpfman/v1"
	configMgmt "github.com/bpfman/bpfman/examples/pkg/config-mgmt"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
)

const (
	KprobeProgramName   = "go-kprobe-counter-example"
	BpfProgramMapIndex  = "kprobe_stats_map"
	DefaultByteCodeFile = "bpf_bpfel.o"

	mapKey uint32 = 0

	// MapsMountPoint is the "go-kprobe-counter-maps" volumeMount "mountPath" from "deployment.yaml"
	MapsMountPoint = "/run/kprobe/maps"
)

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -no-strip -cflags "-O2 -g -Wall" bpf ./bpf/sys_execve.c -- -I.:/usr/include/bpf:/usr/include/linux -I./headers
func main() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	// pull the BPFMAN config management data to determine if we're running on a
	// system with BPFMAN available.
	paramData, err := configMgmt.ParseParamData(configMgmt.ProgTypeKprobe, DefaultByteCodeFile)
	if err != nil {
		log.Printf("error processing parameters: %v\n", err)
		return
	}

	// determine the path to the kprobe_stats_map, whether provided via CRD
	// or BPFMAN or otherwise.
	var mapPath string
	// If running in a Kubernetes deployment, the eBPF program is already loaded.
	// Only need the map path, which is at a known location in the pod using VolumeMounts
	// and the CSI Driver.
	if paramData.CrdFlag {
		// 3. Get access to our map
		mapPath = fmt.Sprintf("%s/%s", MapsMountPoint, BpfProgramMapIndex)
	} else { // if not on k8s, find the map path from the system
		ctx := context.Background()

		// connect to the BPFMAN server
		conn, err := configMgmt.CreateConnection(ctx)
		if err != nil {
			log.Printf("failed to create client connection: %v", err)
			return
		}

		c := gobpfman.NewBpfmanClient(conn)

		// If the bytecode src is a Program ID, skip the loading and unloading of the bytecode.
		if paramData.BytecodeSrc != configMgmt.SrcProgId {
			var loadRequest *gobpfman.LoadRequest
			if paramData.MapOwnerId != 0 {
				mapOwnerId := uint32(paramData.MapOwnerId)
				loadRequest = &gobpfman.LoadRequest{
					Bytecode:    paramData.BytecodeSource,
					Name:        "sys_execve",
					ProgramType: *bpfmanHelpers.Kprobe.Uint32(),
					Attach: &gobpfman.AttachInfo{
						Info: &gobpfman.AttachInfo_KprobeAttachInfo{
							KprobeAttachInfo: &gobpfman.KprobeAttachInfo{
								FnName: "try_to_wake_up",
							},
						},
					},
					MapOwnerId: &mapOwnerId,
				}
			} else {
				loadRequest = &gobpfman.LoadRequest{
					Bytecode:    paramData.BytecodeSource,
					Name:        "sys_execve",
					ProgramType: *bpfmanHelpers.Kprobe.Uint32(),
					Attach: &gobpfman.AttachInfo{
						Info: &gobpfman.AttachInfo_KprobeAttachInfo{
							KprobeAttachInfo: &gobpfman.KprobeAttachInfo{
								FnName: "try_to_wake_up",
							},
						},
					},
				}
			}

			// 1. Load Program using bpfman
			var res *gobpfman.LoadResponse
			res, err = c.Load(ctx, loadRequest)
			if err != nil {
				conn.Close()
				log.Print(err)
				return
			}

			kernelInfo := res.GetKernelInfo()
			if kernelInfo != nil {
				paramData.ProgId = uint(kernelInfo.GetId())
			} else {
				conn.Close()
				log.Printf("kernelInfo not returned in LoadResponse")
				return
			}
			log.Printf("Program registered with id %d\n", paramData.ProgId)

			// 2. Set up defer to unload program when this is closed
			defer func(id uint) {
				log.Printf("unloading program: %d\n", id)
				_, err = c.Unload(ctx, &gobpfman.UnloadRequest{Id: uint32(id)})
				if err != nil {
					conn.Close()
					log.Print(err)
					return
				}
				conn.Close()
			}(paramData.ProgId)

			// 3. Get access to our map
			mapPath, err = configMgmt.CalcMapPinPath(res.GetInfo(), "kprobe_stats_map")
			if err != nil {
				log.Print(err)
				return
			}
		} else {
			// 2. Set up defer to close connection
			defer func(id uint) {
				log.Printf("Closing Connection for Program: %d\n", id)
				conn.Close()
			}(paramData.ProgId)

			// 3. Get access to our map
			mapPath, err = configMgmt.RetrieveMapPinPath(ctx, c, paramData.ProgId, "kprobe_stats_map")
			if err != nil {
				log.Print(err)
				return
			}
		}
	}

	// Name of the kernel function to trace.
	fn := "sys_execve"

	var objs bpfObjects
	if err := loadBpfObjects(&objs, &ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			// Pin the map to the BPF filesystem and configure the
			// library to automatically re-write it in the BPF
			// program so it can be re-used if it already exists or
			// create it if not
			PinPath: mapPath,
		},
	}); err != nil {
		log.Fatalf("loading objects: %v", err)
	}
	defer objs.Close()

	// Open a Kprobe at the entry point of the kernel function and attach the
	// pre-compiled program. Each time the kernel function enters, the program
	// will increment the execution counter by 1. The read loop below polls this
	// map value once per second.
	kp, err := link.Kprobe(fn, objs.SysExecve, nil)
	if err != nil {
		log.Fatalf("opening kprobe: %s", err)
	}
	defer kp.Close()

	// Read loop reporting the total amount of times the kernel
	// function was entered, once per second.
	ticker := time.NewTicker(1 * time.Second)
	log.Println("Waiting for events..")

	go func() {
		for range ticker.C {

			var value uint64
			if err := objs.KprobeMap.Lookup(mapKey, &value); err != nil {
				log.Fatalf("reading map: %v", err)
			}
			log.Printf("%s called %d times\n", fn, value)
		}
	}()

	<-stop
	log.Printf("Exiting...\n")
}
