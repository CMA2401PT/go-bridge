package pyapi

import (
	"go/types"
	"testing"
)

func TestChannelDirectionsAndClosure(t *testing.T) {
	p, err := planSource(t, `
type Stream chan Stream
func (s Stream) Send() {}
func (s Stream) Recv() {}
func (s Stream) Close() {}
func (s Stream) SendOnly() {}
func (s Stream) RecvOnly() {}
`)
	if err != nil {
		t.Fatal(err)
	}
	var stream *Type
	for _, typ := range p.Types {
		if typ.Name == "Stream" {
			stream = typ
		}
	}
	if stream == nil || stream.Shape != ChannelObject || !stream.CanSend || !stream.CanRecv {
		t.Fatalf("missing named channel: %+v", stream)
	}
	if stream.SendOnly.Name != "SendChanStream" || stream.RecvOnly.Name != "RecvChanStream" {
		t.Fatal("direction dependencies not completed")
	}
	if stream.SendOnly.CanRecv || stream.RecvOnly.CanSend {
		t.Fatal("direction restrictions lost")
	}
	for _, method := range stream.Methods {
		if method.Name[len(method.Name)-1:] != "_" {
			t.Errorf("bridge member collision: %s", method.Name)
		}
	}
	size := len(p.Go.Types)
	if err := complete(p.Go); err != nil || len(p.Go.Types) != size {
		t.Fatal("recursive channel completion is not idempotent", err)
	}
	recv := p.Go.Find(types.NewChan(types.RecvOnly, stream.Go.Go))
	if recv == nil || recv.Elem != stream.Go {
		t.Fatal("channel element identity lost")
	}
}
