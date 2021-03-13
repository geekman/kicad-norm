package main

import (
	"strings"
	"testing"
)

func TestParser(t *testing.T) {
	a := strings.NewReader(`
	(fp_text reference U2 (at 1.9 -2.05) (layer F.SilkS)
		(effects (font (size 0.8 0.8) (thickness 0.12)))
	)
	`)

	n, err := Parse(a)
	if err != nil {
		panic(err)
	}
	n.Dump(0)
}

func TestParserStrings(t *testing.T) {
	a := strings.NewReader(`(abc "(5.1.5)" "b" raw?)`)

	n, err := Parse(a)
	if err != nil {
		panic(err)
	}

	if n.Children[0].Children[0].Content != `abc "(5.1.5)" "b" raw?` {
		for _, tok := range n.Children[0].Children {
			t.Logf("%+v\n", tok)
		}
		t.Fail()
	}
}

func TestEquals(t *testing.T) {
	tests := []string{
		`(fp_text r U2 (at 1.9 -2.05) (layer F.SilkS) (effects (font (size 0.8 0.8) (thickness 0.12))) )`,
		`(fp_text r U2 (layer F.SilkS) (at 1.9 -2.05) (effects (font (size 0.8 0.8) (thickness 0.12))) )`,
		`(fp_text r U2    (layer    F.SilkS)   (at 1.9 -2.05)  (effects (font (size 0.8 0.8) (thickness 0.12))) )`,
		`(fp_text (layer F.SilkS) r U2 (at 1.9 -2.05)  (effects (font (size 0.8 0.8) (thickness 0.12))) )`,
	}

	nodes := []*Node{}

	for _, s := range tests {
		r := strings.NewReader(s)
		n, err := Parse(r)
		if err != nil {
			panic(err)
		}

		nodes = append(nodes, n)
	}

	nn := nodes[0]
	for _, n := range nodes[1:] {
		t.Logf("%+v", n)

		if !nn.Equals(n) {
			t.Fail()
		}
	}
}

// Hash of a Node is consistent if contents of node are consistent, despite re-ordering
func TestHash(t *testing.T) {
	tests := []string{
		`(fp_text r U2 (at 1.9 -2.05) (layer F.SilkS) (effects (font (size 0.8 0.8) (thickness 0.12))) )`,
		`(fp_text r U2 (layer F.SilkS) (at 1.9 -2.05) (effects (font (size 0.8 0.8) (thickness 0.12))) )`,
		`(fp_text r U2    (layer    F.SilkS)   (at 1.9 -2.05)  (effects (font (size 0.8 0.8) (thickness 0.12))) )`,
		`(fp_text (layer F.SilkS) r U2 (at 1.9 -2.05)  (effects (font (size 0.8 0.8) (thickness 0.12))) )`,
	}

	v := uint32(0)

	for _, s := range tests {
		r := strings.NewReader(s)
		n, err := Parse(r)
		if err != nil {
			panic(err)
		}

		if v != 0 {
			if n.Hash() != v {
				t.Fail()
			}
		} else {
			v = n.Hash()
		}
		t.Logf("%x", n.Hash())
	}
}

// Test that leaf nodes (that just have text content) have hashes
func TestHashLeafValue(t *testing.T) {
	tests := []string{
		"(foo bar)",
		"(foo )",
	}

	for _, s := range tests {
		r := strings.NewReader(s)
		n, err := Parse(r)
		if err != nil {
			panic(err)
		}

		// get first node from root
		n = n.Children[0]

		// leaf nodes
		for _, c := range n.Children {
			t.Logf("%+v -> %x", c, c.Hash())
			if c.Hash() == 0 {
				t.Fatalf("node %+v has hash!", c)
			}
		}
	}
}
