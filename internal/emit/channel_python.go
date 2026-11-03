package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/pyapi"
)

func stubChannel(b *strings.Builder, t *pyapi.Type) {
	if t.Underlying != nil {
		fmt.Fprintf(b, "    def __new__(_cls, value: %s) -> _typing.Self: ...\n    def __init__(_self, value: %s) -> None: ...\n", annotation(t.Underlying), annotation(t.Underlying))
	} else if t.CanSend && t.CanRecv {
		b.WriteString("    def __new__(_cls, capacity: _builtins.int = 0) -> _typing.Self: ...\n    def __init__(_self, capacity: _builtins.int = 0) -> None: ...\n")
	}
	if t.CanSend {
		fmt.Fprintf(b, "    def send(self, value: %s) -> None: ...\n    def close(self) -> None: ...\n", annotation(t.Elem))
	}
	if t.CanRecv {
		fmt.Fprintf(b, "    def recv(self) -> _builtins.tuple[%s, _builtins.bool]: ...\n", annotation(t.Elem))
	}
	if t.SendOnly != nil {
		fmt.Fprintf(b, "    def send_only(self) -> %s: ...\n    def recv_only(self) -> %s: ...\n", annotation(t.SendOnly), annotation(t.RecvOnly))
	}
}
