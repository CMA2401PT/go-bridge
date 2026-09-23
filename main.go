package main

import (
	"go-bridge/gen"
	"os"
	"path/filepath"
)

func main() {
	packageIR, err := gen.ParseGoFile("target_go/01_vars/vars.go")
	if err != nil {
		panic(err)
	}
	os.MkdirAll("generated", 0755)

	os.WriteFile(
		filepath.Join("generated", "vars_export.go"),
		[]byte(gen.GenerateGoExport(packageIR)),
		0644,
	)

}
