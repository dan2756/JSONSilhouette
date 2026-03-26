// Command jsonshape prints the structure of JSON read from stdin: types
// instead of values.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/dan2756/jsonshape/internal/shape"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = ""

const usage = `Usage: jsonshape [flags] < input.json

Print the structure of JSON read from stdin: types instead of values.
Arrays merge the shapes of all their elements; keys missing from some
elements are marked optional with "?". Ints and floats seen in the same
place are reported as "number".

Flags:
  -h, --help     show this help
      --version  print the version and exit
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("jsonshape", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	showVersion := fs.Bool("version", false, "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "jsonshape: %v\nRun 'jsonshape --help' for usage.\n", err)
		return 2
	}
	if *showVersion {
		fmt.Fprintf(stdout, "jsonshape %s\n", buildVersion())
		return 0
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "jsonshape: unexpected argument %q\nRun 'jsonshape --help' for usage.\n", fs.Arg(0))
		return 2
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "jsonshape: %v\n", err)
		return 1
	}
	root := shape.New()
	if err := decode(root, data); err != nil {
		fmt.Fprintf(stderr, "jsonshape: %v\n", err)
		return 1
	}

	w := bufio.NewWriter(stdout)
	if err := shape.Render(w, root); err != nil {
		fmt.Fprintf(stderr, "jsonshape: %v\n", err)
		return 1
	}
	if err := w.Flush(); err != nil {
		fmt.Fprintf(stderr, "jsonshape: %v\n", err)
		return 1
	}
	return 0
}

// decode merges the single JSON value held in data into root.
func decode(root *shape.Shape, data []byte) error {
	dec := shape.NewDecoder(bytes.NewReader(data))
	switch err := root.Add(dec); {
	case err == io.EOF:
		return errors.New("no JSON input")
	case err == io.ErrUnexpectedEOF:
		return errors.New("unexpected end of JSON input")
	case err != nil:
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("unexpected data after top-level value")
	}
	return nil
}

func buildVersion() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}
