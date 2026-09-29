// Package generate connects analysis, planning, emission and atomic publication.
package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"go-bridge/internal/abi"
	"go-bridge/internal/emit"
	"go-bridge/internal/gomodel"
	"go-bridge/internal/pyapi"
)

type Config struct {
	Dir    string
	Target string
	Name   string
	Output string
}

func Run(config Config) error {
	if config.Output == "" {
		return fmt.Errorf("an output directory is required")
	}
	out, err := filepath.Abs(config.Output)
	if err != nil {
		return err
	}
	if err := checkOutput(out); err != nil {
		return err
	}
	g, err := gomodel.Load(config.Dir, config.Target)
	if err != nil {
		return err
	}
	p, err := pyapi.Plan(g, config.Name)
	if err != nil {
		return err
	}
	a, err := abi.Plan(p)
	if err != nil {
		return err
	}
	files, err := emit.Files(a)
	if err != nil {
		return err
	}
	return publish(out, files)
}

func checkOutput(out string) error {
	info, err := os.Lstat(out)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("output %s is not a directory (symlinks are not followed)", out)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("output directory %s is not empty; choose a new or empty directory", out)
	}
	return nil
}

func publish(out string, files map[string][]byte) error {
	parent := filepath.Dir(out)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".go-bridge-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		destination := filepath.Join(stage, path)
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(destination, files[path], 0644); err != nil {
			return err
		}
	}
	// Recheck before publication. macOS refuses to rename over even an empty
	// directory, so remove that empty directory first. Remove fails if another
	// writer has added content; never use RemoveAll on the destination.
	if err := checkOutput(out); err != nil {
		return err
	}
	if err := os.Remove(out); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Chmod(stage, 0755); err != nil {
		return err
	}
	return os.Rename(stage, out)
}
