package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

var useGit = flag.Bool("git", false, "retrieve reference version from Git")

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

		copyNew := false

		// try a fuzzy match by node id here
		shortId := n.ShortId()
		nodeId := n.Id()

		if matchingNode == nil && shortId != "fp_line" {
			for i := 0; i < len(newNodes); i++ {
				if nodeId == newNodes[i].Id() {
					matchingNode = newNodes[i]
					newNodes = removeElem(newNodes, i)
					copyNew = true
					break
				}
			}
		}

		if matchingNode == nil {
			// these short ids are unique
			uniqId := false
			switch shortId {
			case "layer", "tedit", "tstamp", "at", "descr", "tags", "path", "attr":
				uniqId = true
			}

			if uniqId {
				for i := 0; i < len(newNodes); i++ {
					if shortId == newNodes[i].ShortId() {
						matchingNode = newNodes[i]
						newNodes = removeElem(newNodes, i)
						copyNew = true
						break
					}
				}
			}
		}

		if matchingNode != nil {
			if copyNew {
				newFile.Seek(int64(matchingNode.SpaceStart), 0)
				io.CopyN(dstFile, newFile, int64(matchingNode.EndPos-matchingNode.SpaceStart+1))
			} else {
				oldFile.Seek(int64(n.SpaceStart), 0)
				io.CopyN(dstFile, oldFile, int64(n.EndPos-n.SpaceStart+1))
			}
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

func copyFile(src, dst string) error {
	srcf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcf.Close()

	st, err := srcf.Stat()
	if err != nil {
		return err
	}

	dstf, err := os.Create(dst)
	if err != nil {
		return err
	}

	_, err = io.Copy(dstf, srcf)
	if err != nil {
		dstf.Close()
		return err
	}

	dstf.Close()

	// keep the mtime of copy
	mtime := st.ModTime()
	os.Chtimes(dst, mtime, mtime)

	return nil
}

func openFile(fname string) (*os.File, *Node, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, nil, err
	}

	root, err := Parse(f)
	if err != nil {
		return nil, root, err
	}

	return f, root, nil
}

func checkGit() error {
	_, err := exec.Command("git", "status").Output()
	return err
}

func main() {
	flag.Parse()
	args := flag.Args()

	srcFname := ""

	if *useGit {
		err := checkGit()
		if err != nil {
			fmt.Println("git not found, or not in a git repository")
			return
		}

		targetFname := args[0]
		srcFile, err := ioutil.TempFile("", targetFname+"-git.*")
		if err != nil {
			panic(err)
		}
		srcFname = srcFile.Name()
		defer os.Remove(srcFname)

		cmd := exec.Command("git", "show", "HEAD:"+targetFname)
		cmd.Stdout = srcFile

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err = cmd.Run(); err != nil {
			fmt.Println("cannot get reference file from git")
			fmt.Println(stderr.String())
			return
		}

		srcFile.Close()
	} else {
		srcFname, args = args[0], args[1:]
	}

	f, root, err := openFile(srcFname)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	targetFname, args := args[0], args[1:]

	// we will rename the target to ".orig", and use that subsequently
	origFname := targetFname + ".orig"
	if _, err := os.Stat(origFname); os.IsNotExist(err) {
		err = copyFile(targetFname, origFname)
		if err != nil {
			panic(err)
		}
	}

	// open the ".orig" file as target
	f2, root2, err := openFile(origFname)
	if err != nil {
		panic(err)
	}
	defer f2.Close()

	// output will now be the file user specified
	f3, err := os.Create(targetFname)
	if err != nil {
		panic(err)
	}
	defer f3.Close()

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

	// parse output to ensure it matches
	f3.Seek(0, 0)
	r3, err := Parse(f3)
	if err != nil {
		panic(err)
	}

	expectedHash := root2.Hash()
	outputHash := r3.Hash()

	if outputHash != expectedHash {
		fmt.Printf("output file has incorrect hash %08x (expected %08x)\n",
			outputHash, expectedHash)
	} else {
		fmt.Printf("Done.\n")
	}
}
