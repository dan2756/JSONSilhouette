// Command jsonshape prints the structure of JSON: types instead of values.
package main

import (
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

const usage = `Usage: jsonshape [flags] [file ...]

Print the structure of JSON: types instead of values. Reads the named
files, or stdin when there are none; "-" also means stdin.

Arrays merge the shapes of all their elements; keys missing from some
elements are marked optional with "?". Ints and floats seen in the same
place are reported as "number". Several files are merged the same way,
each one a sample of the same shape.

Flags:
      --depth N   show N levels of nesting; deeper ones collapse to {…} / […]
      --paths     print one path per line, e.g. ".[].owner.login  string"
      --counts    note how many samples had each key and each type
      --json-schema
                  print a JSON Schema (draft 2020-12) for the merged shape
      --lines     read NDJSON: every line is a JSON value, all merged
      --no-color  never color the output (also when NO_COLOR is set)
  -h, --help      show this help
      --version   print the version and exit

Types are colored when stdout is a terminal.

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
	noColor := fs.Bool("no-color", false, "")
	lines := fs.Bool("lines", false, "")
	depth := fs.Int("depth", 0, "")
	paths := fs.Bool("paths", false, "")
	counts := fs.Bool("counts", false, "")
	jsonSchema := fs.Bool("json-schema", false, "")
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
	if *depth < 0 {
		return usageError(stderr, fmt.Sprintf("invalid --depth %d: must be at least 1, or 0 for no limit", *depth))
	}
	if *jsonSchema && (*paths || *counts) {
		return usageError(stderr, "--json-schema cannot be combined with --paths or --counts")
	}
	if len(files) == 0 {
		files = []string{"-"}
	}

	root := shape.New()
	for _, path := range files {
		if err := readInput(root, path, stdin, *lines); err != nil {
			fmt.Fprintf(stderr, "jsonshape: %v\n", err)
			var se *syntaxError
			if errors.As(err, &se) {
				fmt.Fprint(stderr, se.context())
			}
			return 1
		}
	}
	if root.Count == 0 {
		fmt.Fprintln(stderr, "jsonshape: no JSON input")
		return 1
	}

	opts := shape.Options{
		Depth:  *depth,
		Counts: *counts,
		Color:  useColor(stdout, *noColor),
	}
	render := shape.Render
	switch {
	case *paths:
		render = shape.RenderPaths
	case *jsonSchema:
		render = shape.RenderSchema
	}
	if err := render(stdout, root, opts); err != nil {
		fmt.Fprintf(stderr, "jsonshape: %v\n", err)
		return 1
	}
	return 0
}

// readInput merges the JSON in the named file, or in stdin when path is
// "-", into root: one value, or with lines one value per line.
func readInput(root *shape.Shape, path string, stdin io.Reader, lines bool) error {
	name, r := "<stdin>", stdin
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		name, r = path, f
	}
	if lines {
		return decodeLines(root, name, r)
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

// useColor reports whether output to w should be colored: only when it is
// a terminal, and never with --no-color, NO_COLOR or TERM=dumb.
func useColor(w io.Writer, noColor bool) bool {
	if noColor || os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTerminal(w)
}

// isTerminal reports whether w is a terminal; tests replace it.
var isTerminal = func(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
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
