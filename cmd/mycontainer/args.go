package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"go.ajitem.com/gca-crs/internal/cgroup"
)

const usageText = `usage: mycontainer run [flags] <image> <command> [args...]

  -memory 64m   memory.max, with swap capped at nothing
  -cpus 0.5     cpu.max, as a fraction of one core
  -pids 20      pids.max`

type options struct {
	image  string
	argv   []string
	limits cgroup.Limits
}

// parseRun reads `run [flags] <image> <command> [args...]`. The image is a
// directory under rootfs/, pulled by img2rootfs: `make rootfs` puts
// ubuntu:24.04 in rootfs/ubuntu, so the image is "ubuntu". Flags stop at
// the image; anything after it belongs to the command.
func parseRun(args []string) (options, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	memory := fs.String("memory", "", "")
	cpus := fs.Float64("cpus", 0, "")
	pids := fs.Int("pids", 0, "")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}

	rest := fs.Args()
	if len(rest) < 2 {
		return options{}, errors.New(usageText)
	}
	o := options{image: rest[0], argv: rest[1:]}
	if o.image == "." || o.image == ".." || strings.ContainsRune(o.image, '/') {
		return options{}, fmt.Errorf("image %q: want a name under rootfs/, such as ubuntu", o.image)
	}

	if *memory != "" {
		m, err := cgroup.ParseMemory(*memory)
		if err != nil {
			return options{}, err
		}
		o.limits.Memory = m
	}
	if *cpus < 0 || *pids < 0 {
		return options{}, errors.New("limits must not be negative")
	}
	o.limits.CPUs, o.limits.Pids = *cpus, *pids
	return o, nil
}
