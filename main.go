package main

import (
	"flag"
	"fmt"
	"os"

	"go-bridge/internal/generate"
)

func main() {
	var config generate.Config
	flag.StringVar(&config.Output, "out", "", "new or empty output directory (required)")
	flag.StringVar(&config.Name, "name", "", "Python package name (default: go_<Go package name>)")
	flag.StringVar(&config.Dir, "dir", "", "Go package loading directory (default: current directory)")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: go-bridge -out DIRECTORY [-name PYTHON_PACKAGE] [-dir GO_DIRECTORY] PACKAGE")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 || config.Output == "" {
		flag.Usage()
		os.Exit(2)
	}
	config.Target = flag.Arg(0)
	if err := generate.Run(config); err != nil {
		fmt.Fprintln(os.Stderr, "go-bridge:", err)
		os.Exit(1)
	}
	fmt.Println("Generated", config.Output)
}
