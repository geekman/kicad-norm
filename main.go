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
var keepOrig = flag.Bool("keep-orig", false, "keep original file after rewriting")
var outputFile = flag.String("output", "", "explicit output filename")

func removeElem(list []*Node, i int) (*Node, []*Node) {
	n := list[i]
	copy(list[i:], list[i+1:])
	return n, list[:len(list)-1]
}

func findModule(src *Node, list []*Node) int {
	id := src.Id()
	shortId := src.ShortId()

	pathId := ""
	if path := src.FindChild("path"); path != nil {
		pathId = path.Id()
	}

	for i, n := range list {
		if n.Id() == id {
			// exact match
			if n.Hash() == src.Hash() {
				return i
			}

			// match by path
			if pathId != "" && n.FindChild("path").Id() == pathId {
				return i
			}
		}
	}

	// if nothing was found on the first pass, try searching
	// using path value + short ID
	for i, n := range list {
		if n.ShortId() == shortId {
			if pathId != "" && n.FindChild("path").Id() == pathId {
				return i
			}
		}
	}

	return -1
}

// Copies a Node verbatim from srcFile into dstFile
func copyNode(n *Node, srcFile io.ReadSeeker, dstFile io.Writer) error {
	srcFile.Seek(int64(n.StartPos), 0)
	_, err := io.CopyN(dstFile, srcFile, int64(n.EndPos-n.StartPos+1))
	return err
}

func copyModule(oldNode, newNode *Node,
	oldFile, newFile io.ReadSeeker, dstFile io.Writer) {

	// scan for whitespace first
	oldNode.ScanWhitespace()
	newNode.ScanWhitespace()

	dstFile.Write([]byte("("))

	// make a copy
	newNodes := make([]*Node, len(newNode.Children))
	copy(newNodes[:], newNode.Children)

	for _, n := range oldNode.Children {
		//n.Dump(0)

		// look for the same node in newNodes
		var matchingNode *Node
		for i := 0; i < len(newNodes); i++ {
			if n.Hash() == newNodes[i].Hash() {
				matchingNode, newNodes = removeElem(newNodes, i)
				break

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
					matchingNode, newNodes = removeElem(newNodes, i)
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
						matchingNode, newNodes = removeElem(newNodes, i)
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

	// write out whitespace and close bracket
	oldFile.Seek(int64(oldNode.PreEndSpace), 0)
	io.CopyN(dstFile, oldFile, int64(oldNode.EndPos-oldNode.PreEndSpace+1))
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
		srcFile, err := ioutil.TempFile("", "kicad-norm-git.*")
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

	// determine the output filename
	if *outputFile == "" {
		*outputFile = targetFname
	}

	var f2 *os.File
	var root2 *Node

	// remove original file after rewriting
	removeOrigFname := ""

	if *outputFile == targetFname {
		// we will rename the target to ".orig", and use that subsequently
		origFname := targetFname + ".orig"
		if _, err := os.Stat(origFname); os.IsNotExist(err) {
			err = copyFile(targetFname, origFname)
			if err != nil {
				panic(err)
			}
		}

		// open the ".orig" file as target
		f2, root2, err = openFile(origFname)
		if err != nil {
			panic(err)
		}
		defer f2.Close()

		// before we overwrite target file, make sure that targetFile and ".orig"
		// file are actually identical. this operation is supposed to be idempotent.
		f2a, root2a, err := openFile(targetFname)
		if err != nil && !os.IsNotExist(err) {
			panic(err)
		} else {
			f2a.Close()

			if root2.Hash() != root2a.Hash() {
				fmt.Printf("%q contents differ from %q!\n"+
					"%[1]q could be from a previous run, and if so, please delete it first.",
					targetFname, origFname)
				return
			}
		}

		removeOrigFname = origFname
	} else {
		f2, root2, err = openFile(targetFname)
		if err != nil {
			panic(err)
		}
		defer f2.Close()
	}

	// output will now be the file user specified
	f3, err := os.Create(*outputFile)
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

			_, r1 = removeElem(r1, i)
		}

		if n1 != nil && !changed {
			// use unchanged Node
			if err = copyNode(n1, f, f3); err != nil {
				panic(err)
			}
		} else if n1 != nil {
			// found a matching module, which was changed
			// attempt to copy each child
			copyModule(n1, n2, f, f2, f3)
		} else {
			// copy this new Node
			if err = copyNode(n2, f2, f3); err != nil {
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
		// after an equivalent file has been emitted, we can remove the original
		if !*keepOrig && removeOrigFname != "" {
			// close the file now
			f2.Close()

			if err := os.Remove(removeOrigFname); err != nil {
				fmt.Printf("unable to remove orig file: %v\n", err)
			}
		}

		fmt.Printf("Done.\n")
	}
}
