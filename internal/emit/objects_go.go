package emit

import (
	"fmt"
	"strings"

	"go-bridge/internal/abi"
	"go-bridge/internal/pyapi"
)

func goExport(b *strings.Builder, symbol, params, result string) {
	fmt.Fprintf(b, "\n//export %s\nfunc %s(%s)", symbol, symbol, params)
	if result != "" {
		fmt.Fprintf(b, " (out C.%s)", result)
	}
	b.WriteString(" {\n")
}

// goClass renders independently planned constructor, conversion and pointer
// operations. Field views hold Go pointers; ordinary struct results copy.
func goClass(b *strings.Builder, c *abi.Class) {
	t := c.Type
	if c.New != "" {
		goClassNew(b, c)
	}
	if c.Unname != "" {
		goExport(b, c.Unname, "handle C.uintptr_t", c.Underlying.C)
		fmt.Fprintf(b, "value := (%s)(%s)\n", c.Underlying.Go, goInput(t, "handle"))
		goOutput(b, c.Underlying, "out", "value")
		b.WriteString("return\n}\n")
	}
	if c.Copy != "" {
		goExport(b, c.Copy, "handle C.uintptr_t", t.C)
		fmt.Fprintf(b, "value := %s\n", goInput(t, "handle"))
		goOutput(b, t, "out", "value")
		b.WriteString("return\n}\n")
	}
	if c.Address != "" {
		goExport(b, c.Address, "handle C.uintptr_t", c.Pointer.C)
		fmt.Fprintf(b, "value := bridgeGet[%s](handle)\n", t.Cell)
		goOutput(b, c.Pointer, "out", "value")
		b.WriteString("return\n}\n")
	}
	if c.ToPointer != "" {
		goExport(b, c.ToPointer, "value C."+t.C, c.Pointer.C)
		fmt.Fprintf(b, "copied := %s\n", goInput(t, "value"))
		goOutput(b, c.Pointer, "out", "&copied")
		b.WriteString("return\n}\n")
	}
	if c.Load != "" {
		goPointerAccess(b, c)
	}
	for _, f := range c.Fields {
		goField(b, c, f)
	}
}

func goClassNew(b *strings.Builder, c *abi.Class) {
	t := c.Type
	if t.Python.Shape == pyapi.StructObject {
		params := join(len(c.Constructor), func(i int) string { return fmt.Sprintf("a%d C.%s, p%d C.uint8_t", i, c.Constructor[i].Type.C, i) })
		goExport(b, c.New, params, t.C)
		fmt.Fprintf(b, "value := new(%s)\n", t.Go)
		for i, f := range c.Constructor {
			fmt.Fprintf(b, "if p%d != 0 {value.%s = %s}\n", i, f.Python.Go.Name, goInput(f.Type, fmt.Sprintf("a%d", i)))
		}
		b.WriteString("out = bridgePin(value)\nreturn\n}\n")
		return
	}
	input := c.Elem
	if c.Underlying != nil {
		input = c.Underlying
	}
	goExport(b, c.New, "value C."+input.C, t.C)
	fmt.Fprintf(b, "copied := %s\n", goInput(input, "value"))
	expression := "&copied"
	if c.Underlying != nil {
		expression = t.Go + "(copied)"
	}
	goOutput(b, t, "out", expression)
	b.WriteString("return\n}\n")
}

func goPointerAccess(b *strings.Builder, c *abi.Class) {
	t, e := c.Type, c.Elem
	goExport(b, c.Load, "handle C.uintptr_t", e.C)
	fmt.Fprintf(b, "pointer := %s\n", goInput(t, "handle"))
	if e.Python.Shape == pyapi.StructObject {
		// Loading *p must panic on nil even though a view only stores p.
		fmt.Fprintf(b, "_ = *pointer\nout = bridgePin((%s)(pointer))\n", e.Cell)
	} else {
		b.WriteString("value := *pointer\n")
		goOutput(b, e, "out", "value")
	}
	b.WriteString("return\n}\n")
	goExport(b, c.Store, "handle C.uintptr_t, value C."+e.C, "")
	fmt.Fprintf(b, "*(%s) = %s\n}\n", goInput(t, "handle"), goInput(e, "value"))
}

func goField(b *strings.Builder, c *abi.Class, f *abi.Field) {
	receiver := "bridgeGet[" + c.Type.Cell + "](handle)"
	if c.Type.Python.Shape == pyapi.PointerObject {
		receiver = goInput(c.Type, "handle")
	}
	goExport(b, f.Get, "handle C.uintptr_t", f.Type.C)
	fmt.Fprintf(b, "owner := %s\n", receiver)
	if f.Type.Python.Shape == pyapi.StructObject {
		fmt.Fprintf(b, "out = bridgePin(&owner.%s)\n", f.Python.Go.Name)
	} else {
		fmt.Fprintf(b, "value := owner.%s\n", f.Python.Go.Name)
		goOutput(b, f.Type, "out", "value")
	}
	b.WriteString("return\n}\n")
	goExport(b, f.Set, "handle C.uintptr_t, value C."+f.Type.C, "")
	fmt.Fprintf(b, "owner := %s\nowner.%s = %s\n}\n", receiver, f.Python.Go.Name, goInput(f.Type, "value"))
}
