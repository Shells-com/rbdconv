package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Shells-com/rbdconv"
)

func main() {
	inFlag := flag.String("in", "", "specify input filename")
	outFlag := flag.String("out", "", "output filename")
	flag.Parse()

	if inFlag == nil || outFlag == nil || *inFlag == "" || *outFlag == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	in, err := os.Open(*inFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open %s for reading: %s", *inFlag, err)
		os.Exit(1)
	}
	defer in.Close()

	out, err := os.Create(*outFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open %s for writing: %s", *outFlag, err)
		os.Exit(1)
	}
	defer out.Close()

	err = rbdconv.RawToRbd(out, in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to convert: %s", err)
		os.Exit(1)
	}
}
