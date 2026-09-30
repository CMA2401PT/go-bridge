package abi

import "go-bridge/internal/pyapi"

// AnyConversion binds one planned conversion to its C symbols. Assert returns
// Result, a struct with an ok flag, shared by the strict and try forms.
type AnyConversion struct {
	Python              *pyapi.AnyConversion
	Type                *Type
	Box, Assert, Result string
}
