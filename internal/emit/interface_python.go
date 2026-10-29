package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/pyapi"
)

func stubInterfaceConstructor(b *strings.Builder, t *pyapi.Type) {
	parts := make([]string, len(t.Implementors))
	for i, source := range t.Implementors {
		parts[i] = annotation(source)
	}
	input := strings.Join(parts, " | ")
	fmt.Fprintf(b, "    def __new__(cls, value: %s) -> _typing.Self: ...\n    def __init__(self, value: %s) -> None: ...\n", input, input)
}
