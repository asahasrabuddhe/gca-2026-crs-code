//go:build linux

// mycontainer: a container runtime, built one tag at a time.
//
//	mycontainer run <image> <command> [args...]
//
// One binary, two roles. "run" is the parent, on the host. "child" is the
// same binary started again inside the new namespaces; see package
// container for why there have to be two.
package main

import (
	"fmt"
	"os"

	"go.ajitem.com/gca-crs/internal/container"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	switch os.Args[1] {
	case "run":
		image, argv, err := parseRun(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		code, err := container.Run(image, argv)
		if err != nil {
			fail(err)
		}
		os.Exit(code)
	case container.ChildCommand:
		// Child only returns if it could not exec the command.
		fail(container.Child(os.Args[2:]))
	default:
		usage()
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, usageText)
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "mycontainer: %v\n", err)
	os.Exit(1)
}
