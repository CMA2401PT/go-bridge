package abi

import (
	"fmt"

	"go-bridge/internal/pyapi"
)

// Channel uses owned handles for channel values and an owned element plus ok
// for receive. The callback stays in C++; it never crosses the C ABI.
type Channel struct {
	Type, Elem, SendType, RecvType *Type
	New, Send, Recv, Close, Result string
	SendOnly, RecvOnly             string
}

func planChannel(t *pyapi.Type, wire map[*pyapi.Type]*Type) *Channel {
	prefix := fmt.Sprintf("BrType%d", t.Go.ID)
	c := &Channel{Type: wire[t], Elem: wire[t.Elem], SendType: wire[t.SendOnly], RecvType: wire[t.RecvOnly]}
	if t.CanSend {
		c.Send, c.Close = prefix+"Send", prefix+"Close"
	}
	if t.CanRecv {
		c.Recv, c.Result = prefix+"Recv", prefix+"RecvResult"
	}
	if t.CanSend && t.CanRecv {
		c.SendOnly, c.RecvOnly = prefix+"SendOnly", prefix+"RecvOnly"
		if t.Underlying == nil {
			c.New = prefix + "Make"
		}
	}
	return c
}
