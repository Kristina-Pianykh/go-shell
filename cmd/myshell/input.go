package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"golang.org/x/term"
)

const (
	del           = 127
	cariageReturn = 13
	newLine       = 10
	sigint        = 3
	tab           = 9
	bell          = 7
)

func ringBell() {
	os.Stdout.Write([]byte{'\a'})
}

// TODO: don't allow cursor moves outside of input buffer boundary
func readInput(inputCh chan string) {
	var err error

	// logFile, err := os.OpenFile("keylog.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	// if err != nil {
	// 	panic(err)
	// }
	// defer logFile.Close()

	buf := make([]byte, 1)
	input := []byte{}

	defer func() {
		fmt.Fprint(os.Stdout, "\r\n")
		os.Stdout.Sync()
		input = append(input, '\n')
		inputCh <- string(input)
		close(inputCh)
	}()

	for {
		_, err = os.Stdin.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			} else {
				panic(err)
			}
		}
		b := buf[0]
		// fmt.Fprintf(logFile, "Received byte: %d (char: %q)\n", b, b)
		// _ = logFile.Sync()

		switch b {
		case sigint:
			fmt.Printf("^C")
			input = []byte{}
			return
		case cariageReturn, newLine:
			return
		case tab:
			if clean := stripLeft(input); len(clean) == 0 {
				input = append(input, tab)
				clearLine()
				drawPrompt()
				fmt.Printf("%s", input)
				break
			}
			matches := autocompleteBuiltin(input)
			// TODO: expand multiple options for builtins as well
			if len(matches) > 0 {
				clearLine()
				drawPrompt()
				input = []byte(matches[0])
				input = append(input, ' ')
				fmt.Printf("%s", input)
				break
			}
			matches = autocompleteBin(input)
			// fmt.Printf("%v\n", matches)

			if len(matches) == 0 {
				ringBell()
			} else if len(matches) > 1 {
				// fmt.Fprint(os.Stdout, "\r\n")
				// for _, match := range matches {
				// 	fmt.Fprintf(os.Stdout, "%s  ", match)
				// }
				// fmt.Fprint(os.Stdout, "\r\n")
				// drawPrompt()

				commonPrefix := commonPrefix(matches)
				input = cmplInput(input, commonPrefix)
				clearLine()
				drawPrompt()
				fmt.Fprintf(os.Stdout, "%s", input)
			} else {
				clearLine()
				drawPrompt()
				input = []byte(matches[0])
				input = append(input, ' ')

				for i := range input {
					fmt.Printf("%c", input[i])
				}
			}
		case del:
			if len(input) > 0 {
				fmt.Print("\x1b[D \x1b[D")
				input = input[:len(input)-1]
			}
			continue
		default:
			input = append(input, b)
			clearLine()
			drawPrompt()
			fmt.Printf("%s", input)
		}
	}
}

func cmplInput(input []byte, cmpl string) []byte {
	clean := strings.TrimLeft(string(input), " \t")
	offset := len(input) - len(clean)

	res := []byte{}
	for i := range offset {
		res = append(res, input[i])
	}
	if len(clean) > 0 {
		res = append(res, cmpl...)
	}
	return res
}

func autocompleteBuiltin(s []byte) []string {
	clean := strings.TrimLeft(string(s), " \t")
	results := []string{}

	for _, builtin := range builtins {
		if len(clean) > 0 && strings.HasPrefix(builtin, clean) {
			res := cmplInput(s, builtin)
			results = append(results, string(res))
		}
	}
	return results
}

const MAX_INT = int((uint(1) << 63) - 1)

func commonPrefix(lst []string) string {
	minPrefixLen := MAX_INT
	for _, e := range lst {
		if len(e) < minPrefixLen {
			minPrefixLen = len(e)
		}
	}

	for ln := minPrefixLen; ln > 0; ln-- {
		prefix := lst[0][:ln]
		for i, e := range lst {
			if !strings.HasPrefix(e, prefix) {
				break
			}
			if i == len(lst)-1 {
				return prefix
			}
		}
	}
	return ""
}

func autocompleteBin(s []byte) []string {
	clean := strings.TrimLeft(string(s), " \t")
	results := []string{}

	files := searchPath(clean)
	for _, file := range files {
		res := cmplInput(s, file)
		results = append(results, string(res))
	}

	return results
}

func stripLeft(s []byte) []byte {
	return []byte(strings.TrimLeft(string(s), "\t "))
}

func searchPath(prefix string) []string {
	// TODO: if file is a path
	// TODO: fix with the idea that `file` is incomplete

	// if strings.Contains(prefix, "/") {
	// 	err := isExec(prefix)
	// 	if err == nil {
	// 		return prefix, true
	// 	}
	// 	return "", false
	// }
	// if file is a binary name
	path := os.Getenv("PATH")
	bins := []string{}

	for _, dir := range filepath.SplitList(path) {
		// fmt.Printf("dir: %s\r\n", dir)
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		entry, err := os.ReadDir(dir)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				panic(err)
			}
		}
		for _, e := range entry {
			// TODO: use binary search
			if strings.HasPrefix(e.Name(), prefix) {

				execPath := filepath.Join(dir, e.Name())
				if err := isExec(execPath); err == nil {
					if !slices.Contains(bins, e.Name()) {
						bins = append(bins, e.Name())
					}
				}
			}
		}
	}
	return bins
}

func isExec(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	m := d.Mode()
	if m.IsDir() {
		return syscall.EISDIR
	}
	// at least one of the execute bits (owner, group, or others) is set
	if m&0111 != 0 {
		return nil
	}
	return fs.ErrPermission
}

func clearLine() {
	fmt.Print("\x1b[2K\r")
	// TODO set prompt as a global variable
}

func drawPrompt() {
	fmt.Fprint(os.Stdout, "$ ")
}

func parseInput(
	tokenCh chan []string,
) {
	parser := newParser()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}

	defer func() {
		close(tokenCh)
		err := term.Restore(int(os.Stdin.Fd()), oldState)
		if err != nil {
			panic(err)
		}
	}()

Loop:
	inputCh := make(chan string)
	go readInput(inputCh)
	select {
	case input, ok := <-inputCh:
		if !ok || len(input) == 0 {
			return
		}
		tokens, err := parser.parse(input)
		if err != nil && errors.Is(err, UnclosedQuoteErr) {
			fmt.Printf("$ ")
			goto Loop
		}

		if len(tokens) < 1 {
			return
		}

		tokenCh <- tokens
		return
	}
}
