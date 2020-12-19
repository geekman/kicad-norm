package main

import (
	"fmt"
	"io"
	"os"
)

func removeElem(list []*Node, i int) []*Node {
	copy(list[i:], list[i+1:])
	return list[:len(list)-1]
}

func findModule(src *Node, list []*Node) int {
	id := src.Id()

	for i, n := range list {
		if n.Id() == id {
			// exact match
			if n.Hash() == src.Hash() {
				return i
			}

			// match by path
			if n.FindChild("path").Id() == src.FindChild("path").Id() {
				return i
			}
		}
	}

	return -1
}

func copyModule(oldNode, newNode *Node,
	oldFile, newFile io.ReadSeeker, dstFile io.Writer) {

	// scan for whitespace first
	oldNode.ScanWhitespace()
	newNode.ScanWhitespace()

	dstFile.Write([]byte("("))

	// make a copy
	newNodes := newNode.Children
	for _, n := range oldNode.Children {
		//n.Dump(0)

		// look for the same node in newNodes
		var matchingNode *Node
		for i := 0; i < len(newNodes); i++ {
			if n.Hash() == newNodes[i].Hash() {
				matchingNode = newNodes[i]
				newNodes = removeElem(newNodes, i)

				//fmt.Printf("  %q [MATCHES]\n", n.Id())
			}
		}

		if matchingNode != nil {
			if n.StartPos-n.SpaceStart > 100 {
				fmt.Printf("ws anomaly %+v\n", n)
			}

			oldFile.Seek(int64(n.SpaceStart), 0)
			io.CopyN(dstFile, oldFile, int64(n.EndPos-n.SpaceStart+1))
		}

		// if we can't find the matching new node, skip it
	}

	// write out remaining newNodes
	for _, n := range newNodes {
		newFile.Seek(int64(n.SpaceStart), 0)
		io.CopyN(dstFile, newFile, int64(n.EndPos-n.SpaceStart+1))
	}

	dstFile.Write([]byte(")"))
}

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()

	f2, err := os.Open(os.Args[2])
	if err != nil {
		panic(err)
	}
	defer f2.Close()

	f3, err := os.Create(os.Args[2] + ".up")
	if err != nil {
		panic(err)
	}
	defer f3.Close()

	root, err := Parse(f)
	_ = err

	root2, err := Parse(f2)
	_ = err

	r1 := root.Children[0].Children
	r2 := root2.Children[0].Children

	// always points within f2
	copiedPos := 0

	for _, n2 := range r2 {
		//n2.Dump(0)
		//fmt.Println()

		changed := false
		i := -1

		// copy from last position to this Node
		if copiedPos < n2.StartPos {
			f2.Seek(int64(copiedPos), 0)
			_, err := io.CopyN(f3, f2, int64(n2.StartPos-copiedPos))
			if err != nil {
				panic(err)
			}
			copiedPos = n2.StartPos
		}

		var n1 *Node
		if n2.IdMatches("module") {
			i = findModule(n2, r1)
		}

		if i > -1 {
			n1 = r1[i]
			if n2.Hash() != n1.Hash() {
				changed = true
			}

			r1 = removeElem(r1, i)
		}

		if n1 != nil && !changed {
			// use unchanged Node
			f.Seek(int64(n1.StartPos), 0)
			_, err := io.CopyN(f3, f, int64(n1.EndPos-n1.StartPos+1))
			if err != nil {
				panic(err)
			}
		} else if n1 != nil {
			// found a matching module, which was changed
			// attempt to copy each child
			copyModule(n1, n2, f, f2, f3)
		} else {
			// copy this new Node
			f2.Seek(int64(n2.StartPos), 0)
			_, err := io.CopyN(f3, f2, int64(n2.EndPos-n2.StartPos+1))
			if err != nil {
				panic(err)
			}
		}

		// use end pos for f2 still
		copiedPos = n2.EndPos + 1

		changedStatus := ""
		if changed {
			changedStatus = " [CHANGED]"
		}
		fmt.Printf("%s%s\n", n2.Id(), changedStatus)
	}

	// copy remaining
	f2.Seek(int64(copiedPos), 0)
	_, err = io.Copy(f3, f2)
	if err != nil {
		panic(err)
	}
}
