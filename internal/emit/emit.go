// Package emit renders validated Python and ABI plans into source files. It
// makes no mapping decisions: every name and wire type comes from the plans.
package emit

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"go/format"
	"path/filepath"
	"strings"

	"go-bridge/internal/abi"
)

// cppRuntime holds the C++ conversion helpers shared by every binding.
//
//go:embed templates/runtime.cpp.tmpl
var cppRuntime string

//go:embed templates/objects.cpp.tmpl
var cppObjects string

// buildScript is copied verbatim; it reads its parameters from bridge.json.
//
//go:embed templates/build.py
var buildScript string

const readme = `# %s

Generated from ` + "`%s`" + `. Requires the original Go source tree.

Build with ` + "`python build.py`" + ` (pybind11 and setuptools required).
Import from ` + "`.build/python`" + ` using that same interpreter.
The adapter and extension are a matched pair; regenerate and rebuild together.
Import waits for the target Go package to finish initialization.
Go panics are not recovered by the bridge. An unhandled panic terminates the
entire Python process and cannot be caught with Python try/except.
`

// buildConfig is written to bridge.json and read by build.py.
type buildConfig struct {
	SourceDir  string `json:"source_dir"`
	Package    string `json:"package"`
	ImportPath string `json:"import_path"`
}

// Files returns the generated project, keyed by slash-separated relative path.
func Files(p *abi.Package) (map[string][]byte, error) {
	adapter, err := format.Source([]byte(goSource(p)))
	if err != nil {
		return nil, fmt.Errorf("format generated Go: %w", err)
	}
	py, g := p.Python, p.Python.Go
	config, err := json.MarshalIndent(buildConfig{SourceDir: g.Dir, Package: py.Name, ImportPath: g.Path}, "", "  ")
	if err != nil {
		return nil, err
	}
	result := map[string][]byte{
		"go/main.go":                           adapter,
		"bindings.cpp":                         []byte(cppSource(p)),
		filepath.Join(py.Name, "__init__.pyi"): []byte(stubSource(py)),
		filepath.Join(py.Name, "__init__.py"):  []byte(pythonSource(py)),
		filepath.Join(py.Name, "py.typed"):     {},
		"build.py":                             []byte(buildScript),
		"bridge.json":                          append(config, '\n'),
		"README.md":                            []byte(fmt.Sprintf(readme, py.Name, g.Path)),
	}
	if len(p.Slices) > 0 {
		holder, err := format.Source([]byte(holderSource))
		if err != nil {
			return nil, fmt.Errorf("format holder: %w", err)
		}
		result["go/holder.go"] = holder
	}
	return result, nil
}

// join renders n items separated by ", ".
func join(n int, item func(i int) string) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = item(i)
	}
	return strings.Join(parts, ", ")
}
