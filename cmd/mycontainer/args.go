package main

import (
	"errors"
	"fmt"
	"strings"
)

const usageText = "usage: mycontainer run <image> <command> [args...]"

// parseRun reads `run <image> <command> [args...]`. The image is a
// directory under rootfs/, pulled by img2rootfs: `make rootfs` puts
// ubuntu:24.04 in rootfs/ubuntu, so the image is "ubuntu".
func parseRun(args []string) (image string, argv []string, err error) {
	if len(args) < 2 {
		return "", nil, errors.New(usageText)
	}
	image = args[0]
	if image == "." || image == ".." || strings.ContainsRune(image, '/') {
		return "", nil, fmt.Errorf("image %q: want a name under rootfs/, such as ubuntu", image)
	}
	return image, args[1:], nil
}
