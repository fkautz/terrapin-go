package main

import (
	"flag"
	"fmt"
	"os"

	terrapin "github.com/fkautz/terrapin-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "expected 'id', 'attest', 'validate', or 'cat' subcommands")
		os.Exit(1)
	}
	switch os.Args[1] {
	case "id":
		cmdID(os.Args[2:])
	case "attest":
		cmdAttest(os.Args[2:])
	case "validate":
		cmdValidate(os.Args[2:])
	case "cat":
		cmdCat(os.Args[2:])
	default:
		fmt.Fprintln(os.Stderr, "expected 'id', 'attest', 'validate', or 'cat' subcommands")
		os.Exit(1)
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}

func mustInput(fs *flag.FlagSet) string {
	if fs.NArg() < 1 {
		fail("an input file argument is required")
	}
	return fs.Arg(0)
}

// optRange converts -start/-end int64 flags (-1 = unset) to *uint64.
func optRange(start, end int64) (*uint64, *uint64) {
	var sp, ep *uint64
	if start >= 0 {
		u := uint64(start)
		sp = &u
	}
	if end >= 0 {
		u := uint64(end)
		ep = &u
	}
	return sp, ep
}

// id <file>
func cmdID(args []string) {
	fs := flag.NewFlagSet("id", flag.ExitOnError)
	fs.Parse(args)
	f, err := os.Open(mustInput(fs))
	if err != nil {
		fail("cannot open input: %v", err)
	}
	defer f.Close()
	id, err := terrapin.IdentifierFromReader(f)
	if err != nil {
		fail("hashing failed: %v", err)
	}
	fmt.Println(id)
}

// attest [-out base] <file>
func cmdAttest(args []string) {
	fs := flag.NewFlagSet("attest", flag.ExitOnError)
	out := fs.String("out", "", "output tree base name (default <input>.terra)")
	fs.Parse(args)
	input := mustInput(fs)
	f, err := os.Open(input)
	if err != nil {
		fail("cannot open input: %v", err)
	}
	defer f.Close()
	tree, err := terrapin.BuildFromReader(f)
	if err != nil {
		fail("hashing failed: %v", err)
	}
	base := *out
	if base == "" {
		base = input + ".terra"
	}
	if err := terrapin.WriteTree(base, tree); err != nil {
		fail("writing tree failed: %v", err)
	}
	fmt.Println(tree.Identifier())
}

// validate -tree base [-identifier id] [-start N] [-end M] <file>
func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	tree := fs.String("tree", "", "tree base name (<name>.head/.blocks)")
	identifier := fs.String("identifier", "", "trusted identifier the tree must match")
	start := fs.Int64("start", -1, "range start byte")
	end := fs.Int64("end", -1, "range end byte")
	fs.Parse(args)
	input := mustInput(fs)
	if *tree == "" {
		fail("-tree is required")
	}
	pt, err := terrapin.ReadTree(*tree)
	if err != nil {
		fail("Validation failed: %v", err)
	}
	if *identifier != "" {
		if err := pt.CheckAgainst(*identifier); err != nil {
			fail("Validation failed: %v", err)
		}
	}
	sp, ep := optRange(*start, *end)
	if err := pt.Validate(input, sp, ep, nil); err != nil {
		fail("Validation failed: %v", err)
	}
	fmt.Println("Validation successful: the data matches the tree.")
}

// cat -tree base [-start N] [-end M] <file>
func cmdCat(args []string) {
	fs := flag.NewFlagSet("cat", flag.ExitOnError)
	tree := fs.String("tree", "", "tree base name (<name>.head/.blocks)")
	start := fs.Int64("start", -1, "range start byte")
	end := fs.Int64("end", -1, "range end byte")
	fs.Parse(args)
	input := mustInput(fs)
	if *tree == "" {
		fail("-tree is required")
	}
	pt, err := terrapin.ReadTree(*tree)
	if err != nil {
		fail("Validation failed: %v", err)
	}
	sp, ep := optRange(*start, *end)
	if err := pt.Validate(input, sp, ep, os.Stdout); err != nil {
		fail("Validation failed: %v", err)
	}
}
