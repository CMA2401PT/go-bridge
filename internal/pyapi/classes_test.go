package pyapi

import (
	"strings"
	"testing"
)

func TestMemberNamespace(t *testing.T) {
	for _, source := range []string{
		`type S struct { Value int; Value_ int }`,
		`type S struct { URLValue int }; func (S) UrlValue() {}`,
		`type Text string; func(Text) Join() {}; func(Text) Join_() {}`,
		`type PtrInt int; func F(*int) {}`,
		`type S struct{}; func(S) F(self, self_ int) {}`,
	} {
		_, err := planSource(t, source)
		if err == nil || !strings.Contains(err.Error(), "collides") {
			t.Fatalf("missing member/type collision: %s: %v", source, err)
		}
	}
	p, err := planSource(t, `type Text string
func(Text) Join(self Text) Text {return self}
func(*Text) Clear() {}
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range p.Types {
		for _, m := range typ.Methods {
			if m.Go.Name == "Join" && (m.Name != "join_" || m.Params[0].Name != "self_") {
				t.Fatalf("wrong inherited builtin/receiver spelling: %+v", m)
			}
		}
	}
}
