package abi

// InterfaceConversion assigns a known Go value to a statically compatible
// interface. This is not a runtime assertion and preserves nil interfaces.
type InterfaceConversion struct {
	Target, Source *Type
	Symbol         string
}

const InterfaceBoxSymbol = "BrInterfaceAsAny"
