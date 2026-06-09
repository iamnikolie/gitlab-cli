package cmd

import (
	"fmt"
	"io"
	"os"
)

// readFileArg reads text from a file path, or from stdin when the path is "-".
func readFileArg(file string) (string, error) {
	if file == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(b), nil
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", file, err)
	}
	return string(b), nil
}

// descArg resolves a description from an inline value or a file (path, or "-"
// for stdin). The two are mutually exclusive. provided reports whether a
// description was supplied at all (so update can decide to include it).
func descArg(inline, file string) (text string, provided bool, err error) {
	switch {
	case inline != "" && file != "":
		return "", false, fmt.Errorf("use either --description or --description-file, not both")
	case file != "":
		t, err := readFileArg(file)
		if err != nil {
			return "", false, err
		}
		return t, true, nil
	case inline != "":
		return inline, true, nil
	default:
		return "", false, nil
	}
}

// textFromArgOrFile resolves body text from an optional positional argument or
// a file (path, or "-" for stdin). Exactly one source must be provided.
func textFromArgOrFile(arg string, argGiven bool, file string) (string, error) {
	switch {
	case argGiven && file != "":
		return "", fmt.Errorf("provide the text inline or via --body-file, not both")
	case file != "":
		return readFileArg(file)
	case argGiven:
		return arg, nil
	default:
		return "", fmt.Errorf("provide text inline or via --body-file")
	}
}
