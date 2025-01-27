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

	bellCnt := 0
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
			trimmedInput := stripLeft(input)
			if len(trimmedInput) == 0 {
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

				commonPrefix := commonPrefix(matches)
				if len(commonPrefix) > len(trimmedInput) {
					input = cmplInput(input, commonPrefix)
					clearLine()
					drawPrompt()
					fmt.Fprintf(os.Stdout, "%s", input)
				} else if commonPrefix == string(trimmedInput) {
					if bellCnt == 0 {
						ringBell()
						bellCnt++
						break
					}
					fmt.Fprint(os.Stdout, "\r\n")
					for _, match := range matches {
						fmt.Fprintf(os.Stdout, "%s  ", match)
					}
					fmt.Fprint(os.Stdout, "\r\n")
					drawPrompt()
					fmt.Fprintf(os.Stdout, "%s", input)
				}
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
			bellCnt = 0
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

func sharePrefix(lst []string, prefix []byte) []string {
	sharePrefix := []string{}

	for _, e := range lst {
		if !strings.HasPrefix(e, string(prefix)) {
			continue
		}
		sharePrefix = append(sharePrefix, e)
	}
	return sharePrefix
}

func commonPrefix(lst []string) string {
	maxPrefixLen := MAX_INT
	for _, e := range lst {
		if len(e) < maxPrefixLen {
			maxPrefixLen = len(e)
		}
	}
	// fmt.Printf("max prefix possible: %d\n", maxPrefixLen)

	for ln := maxPrefixLen; ln > 0; ln-- {
		prefix := lst[0][:ln]
		share := 0

		for _, e := range lst {
			if !strings.HasPrefix(e, prefix) {
				continue
			}
			share++
		}
		if share == len(lst) {
			return prefix
		}
	}
	return ""
}

// func commonPrefix(lst []string) (string, bool) {
// 	maxPrefixLen := MAX_INT
// 	for _, e := range lst {
// 		if len(e) < maxPrefixLen {
// 			maxPrefixLen = len(e)
// 		}
// 	}
// 	// fmt.Printf("max prefix possible: %d\n", maxPrefixLen)
// 	for _, i := range lst {
// 		if len(i) < maxPrefixLen {
// 			continue
// 		}
// 		for _, j := range lst {
// 			if !strings.HasPrefix(j, i) {
// 				continue
// 			}
// 			return i, true
// 		}
// 	}
//
// 	for ln := maxPrefixLen; ln > 0; ln-- {
// 		prefix := lst[0][:ln]
// 		for _, e := range lst {
// 			if !strings.HasPrefix(e, prefix) {
// 				continue
// 			}
// 		}
// 		return prefix, false
// 	}
// 	return "", false
// }

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
