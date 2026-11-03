package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/abi"
)

func goChannel(b *strings.Builder, c *abi.Channel) {
	t := c.Type
	if c.New != "" {
		goExport(b, c.New, "capacity C.intptr_t", t.C)
		fmt.Fprintf(b, "out = bridgePin(make(%s, int(capacity)))\nreturn\n}\n", t.Go)
	}
	if c.Send != "" {
		goExport(b, c.Send, "handle C.uintptr_t, value C."+c.Elem.C, "")
		fmt.Fprintf(b, "channel := %s\nchannel <- %s\n}\n", goInput(t, "handle"), goInput(c.Elem, "value"))
		goExport(b, c.Close, "handle C.uintptr_t", "")
		fmt.Fprintf(b, "close(%s)\n}\n", goInput(t, "handle"))
	}
	if c.Recv != "" {
		goExport(b, c.Recv, "handle C.uintptr_t", c.Result)
		fmt.Fprintf(b, "channel := %s\nvalue, ok := <-channel\nif ok { out.ok = 1 }\n", goInput(t, "handle"))
		goOutput(b, c.Elem, "out.value", "value")
		b.WriteString("return\n}\n")
	}
	for _, conversion := range []struct {
		symbol string
		target *abi.Type
	}{
		{c.SendOnly, c.SendType}, {c.RecvOnly, c.RecvType},
	} {
		if conversion.symbol == "" {
			continue
		}
		goExport(b, conversion.symbol, "handle C.uintptr_t", t.C)
		fmt.Fprintf(b, "value := (%s)(%s)\n", conversion.target.Go, goInput(t, "handle"))
		goOutput(b, conversion.target, "out", "value")
		b.WriteString("return\n}\n")
	}
}
