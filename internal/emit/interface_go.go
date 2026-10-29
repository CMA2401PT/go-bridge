package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/abi"
)

func goInterfaceConversion(b *strings.Builder, c *abi.InterfaceConversion) {
	goExport(b, c.Symbol, "input C."+c.Source.C, c.Target.C)
	fmt.Fprintf(b, "var value %s = %s\n", c.Target.Go, goInput(c.Source, "input"))
	goOutput(b, c.Target, "out", "value")
	b.WriteString("return\n}\n")
}
