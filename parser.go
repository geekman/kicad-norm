package main

import (
	"fmt"
	"hash/crc32"
	"io"
	"sort"
	"text/scanner"
)

type Node struct {
	Parent *Node

	// either Children is filled, or Content
	Children []*Node
	Content  string

	hash uint32

	SpaceStart       int // preceding whitespace
	StartPos, EndPos int
}

func (n *Node) AppendChild(child *Node) *Node {
	child.Parent = n
	n.Children = append(n.Children, child)
	return child
}

func (n *Node) IsLeaf() bool { return len(n.Children) == 0 }

// Gets the "id" of this node.
// Usually it's a string id like "gr_text" and it includes textual/string
// descriptions until it hits a sub-node (like "layer" or "at").
func (n *Node) Id() string {
	if len(n.Children) == 0 {
		return ""
	}

	return n.Children[0].Content
}

// Checks if this Node id matches the provided id string
func (n *Node) IdMatches(id string) bool {
	idstr := n.Id()
	if idstr == "" {
		return false
	}

	idlen := len(id)

	// match exactly id string, or id string and a space
	return len(idstr) >= idlen && idstr[:idlen] == id &&
		(len(idstr) == idlen || idstr[idlen] == ' ')
}

// Finds a child that matches the given id (prefix), like "layer" or "at".
func (n *Node) FindChild(id string) *Node {
	for _, c := range n.Children {
		if c.IdMatches(id) {
			return c
		}
	}

	return nil
}

func (n *Node) LastChild() *Node {
	l := len(n.Children)
	if l > 0 {
		return n.Children[l-1]
	}
	return nil
}

func uintStr(v uint32) string {
	s := []byte{}
	for i := 0; i < 32/4; i++ {
		s = append(s, "01234567890abcdef"[v&0xF])
		v >>= 4
	}
	return string(s)
}

func (n *Node) Hash() uint32 {
	if n.hash != 0 {
		return n.hash
	}

	contents := ""
	nodeHashes := []uint32{}

	for _, c := range n.Children {
		if c.IsLeaf() {
			contents += " " + c.Content
		} else {
			nodeHashes = append(nodeHashes, c.Hash())
		}
	}

	h := crc32.NewIEEE()
	h.Write([]byte(contents))

	sort.SliceStable(nodeHashes, func(a, b int) bool { return nodeHashes[a] < nodeHashes[b] })
	for _, v := range nodeHashes {
		h.Write([]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
	}

	n.hash = h.Sum32()
	return n.hash
}

// Merges a new token with the last child if possible, or adds a new one.
// A single space is used to separate the merged tokens.
func (n *Node) MergeToken(tok string) {
	last := n.LastChild()
	if last == nil || !last.IsLeaf() {
		last = &Node{}
		n.AppendChild(last)
	}
	if last.Content != "" {
		last.Content += " "
	}
	last.Content += tok
}

func (n *Node) Equals(other *Node) bool {
	// number of children must match
	if len(n.Children) != len(other.Children) {
		return false
	}

	// copy other children for comparison
	children2 := other.Children

	for _, c := range n.Children {
		chash := c.Hash()

		l2 := len(children2)
		for i := 0; i < l2; i++ {
			if children2[i].Hash() == chash {
				// if it matches, remove it
				children2[i] = children2[l2-1]
				children2 = children2[:l2-1]
				break
			}
		}
	}

	// if everything matched, there would be no leftovers
	return len(children2) == 0
}

func (n *Node) Dump(level int) {
	if n.IsLeaf() {
		fmt.Print(n.Content)
	} else {
		fmt.Print("\n")
		for i := 0; i < level; i++ {
			fmt.Print("  ")
		}

		fmt.Print("(")
		for _, nn := range n.Children {
			nn.Dump(level + 1)
		}
		fmt.Print(")")
	}
}

// Scans the node's children, computing whitespace between each of them.
func (n *Node) ScanWhitespace() {
	lastPos := n.StartPos + 1
	for _, c := range n.Children {
		if c.SpaceStart > 0 {
			continue
		}

		// if there's no whitespace, then c.SpaceStart == c.StartPos
		c.SpaceStart = lastPos
		lastPos = c.EndPos + 1
	}
}

// Updates the position of the last token
func (n *Node) updatePos(startpos, endpos int) {
	c := n.LastChild()
	if c.StartPos == 0 {
		c.StartPos = startpos
	}

	if endpos != 0 {
		c.EndPos = endpos
	}
}

func Parse(r io.Reader) (*Node, error) {
	s := scanner.Scanner{}
	s.Init(r)
	s.Mode = scanner.ScanIdents | scanner.ScanStrings | scanner.ScanRawStrings

	// root node
	root := &Node{}

	// parser state
	n := root
	lastTok := ""
	lastTokPos := 0

	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		//fmt.Printf("%s: %v, %s\n", s.Position, tok, s.TokenText())

		pos := s.Position.Offset

		termToken := tok == '(' || tok == ')' || tok == scanner.EOF

		// in case we don't catch everything as one token,
		// we will detect adjacent characters and treat them as one
		if !termToken {
			if lastTok == "" {
				lastTokPos = pos
			}

			if pos-len(lastTok) == lastTokPos { // same token still
				lastTok += s.TokenText()
			} else {
				n.MergeToken(lastTok)
				n.updatePos(lastTokPos, pos+len(s.TokenText()))

				lastTok = s.TokenText()
				lastTokPos = pos
			}
		}

		if termToken && lastTok != "" {
			n.MergeToken(lastTok)
			n.updatePos(lastTokPos, pos-1)

			lastTok = ""
			lastTokPos = pos
		}

		switch tok {
		case '(':
			n = n.AppendChild(&Node{StartPos: pos})

		case ')':
			n.EndPos = pos
			n = n.Parent
		}
	}

	return root, nil
}
