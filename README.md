kicad-norm
===========

**kicad-norm** tries to normalize KiCad PCB files for more meaningful version control.
While the files emitted by KiCad _are_ text files, KiCad seems to like
shuffling around lines within components, generating unnecessarily large diffs.
This utility tries to "normalize" the files by preserving the previous file
version as much as possible, making sure the diff is as compact and possible
for humans to read.

Installation
=============

    $ go get -v github.com/geekman/kicad-norm

Usage
======

After editing and saving with KiCad, run `kicad-norm` to normalize the changes.
Assuming your project is kept under source control with Git:

    kicad-norm -git my-proj.kicad_pcb

This will copy `my-proj.kicad_pcb` to `my-proj.kicad_pcb.orig` first, 
then extract the previous version of the file from `HEAD` and use that as a
reference to normalize changes into `my-proj.kicad_pcb`, thereby overwriting
the destination file.

If you are not using Git, or want to do it manaully:

```sh
# get the previous file version
git show HEAD:my-proj.kicad_pcb > my-proj.kicad_pcb-old

# normalize it
kicad-norm my-proj.kicad_pcb-old my-proj.kicad_pcb
```

License
========

**kicad-norm is licensed under the 3-clause ("modified") BSD License.**

Copyright (C) 2020 Darell Tan

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions
are met:

1. Redistributions of source code must retain the above copyright
   notice, this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright
   notice, this list of conditions and the following disclaimer in the
   documentation and/or other materials provided with the distribution.
3. The name of the author may not be used to endorse or promote products
   derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE AUTHOR "AS IS" AND ANY EXPRESS OR
IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES
OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY DIRECT, INDIRECT,
INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT
NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF
THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

