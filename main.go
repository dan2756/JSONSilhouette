// Command jsonshape prints the structure of JSON: types instead of values.
package main

import (
	"bufio"
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

const usage = `Usage: jsonshape [flags] [file]

Print the structure of JSON: types instead of values. Reads the named
file, or stdin when there is none or it is "-".

Arrays merge the shapes of all their elements; keys missing from some
elements are marked optional with "?". Ints and floats seen in the same
place are reported as "number".

Flags:
  -h, --help     show this help
      --version  print the version and exit

Exit status is 0 on success, 1 if the input cannot be read or is not
valid JSON, and 2 for usage errors.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("jsonshape", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	showVersion := fs.Bool("version", false, "")
	files, err := parseArgs(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		return usageError(stderr, err.Error())
	}
	if *showVersion {
		fmt.Fprintf(stdout, "jsonshape %s\n", buildVersion())
		return 0
	}
	if len(files) > 1 {
		return usageError(stderr, fmt.Sprintf("too many arguments: %q", files[1:]))
	}

	root := shape.New()
	path := "-"
	if len(files) == 1 {
		path = files[0]
	}
	if err := readInput(root, path, stdin); err != nil {
		fmt.Fprintf(stderr, "jsonshape: %v\n", err)
		var se *syntaxError
		if errors.As(err, &se) {
			fmt.Fprint(stderr, se.context())
		}
		return 1
	}

	w := bufio.NewWriter(stdout)
	err = shape.Render(w, root)
	if err == nil {
		err = w.Flush()
	}
	if err != nil {
		fmt.Fprintf(stderr, "jsonshape: %v\n", err)
		return 1
	}
	return 0
}

// readInput merges the JSON value in the named file, or in stdin when path
// is "-", into root.
func readInput(root *shape.Shape, path string, stdin io.Reader) error {
	name, r := "<stdin>", stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		name, r = path, f
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return decodeDocument(root, name, data)
}

// parseArgs parses flags and returns the remaining arguments. Unlike
// flag.FlagSet.Parse it accepts flags after the first file argument, so
// that "jsonshape data.json --version" works; "--" ends flag parsing.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var files []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return files, nil
		}
		if n := len(args) - len(rest); n > 0 && args[n-1] == "--" {
			return append(files, rest...), nil
		}
		files = append(files, rest[0])
		args = rest[1:]
	}
}

func usageError(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "jsonshape: %s\nRun 'jsonshape --help' for usage.\n", msg)
	return 2
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
