package main

import (
	"flag"
	"io"
	"log"
	"os"
)

var (
	source      = flag.String("source", "", "(source mode) Input Go source file; enables source mode.")
	destination = flag.String("destination", "", "Output file; defaults to stdout.")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *source != "" {
	} else {
		if flag.NArg() != 2 {
			usage()
			log.Fatal("Expected exactly two arguments")
		}
	}
}

func usage() {
	io.WriteString(os.Stderr, usageText)
	flag.PrintDefaults()
}

const usageText = `athena-schema
Example:
	athena-schema -source=foo.go [other options]
`
