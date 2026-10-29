package emit

import (
	"fmt"
	"go-bridge/internal/pyapi"
	"strings"
)

func stubArray(b *strings.Builder, t *pyapi.Type) {
	input := "values: _typing.Iterable[" + annotation(t.Elem) + "] = ..."
	if t.Underlying != nil {
		input = "value: " + annotation(t.Underlying)
	}
	fmt.Fprintf(b, `    def __new__(cls, %s) -> _typing.Self: ...
    def __init__(self, %s) -> None: ...
    __iter__: _typing.ClassVar[None]
    def __len__(self) -> _builtins.int: ...
    def __getitem__(self, index: _typing.SupportsIndex) -> %s: ...
    def __setitem__(self, index: _typing.SupportsIndex, value: %s) -> None: ...
    def copy_to_py_list(self) -> _builtins.list[%s]: ...
`, input, input, annotation(t.Elem), annotation(t.Elem), annotation(t.Elem))
}
