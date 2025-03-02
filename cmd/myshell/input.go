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
	DELETE   = 127
	CR       = 13
	NL       = 10
	SIGINT   = 3
	TAB      = 9
	BELL     = 7
	LBRACKET = 91
	ESC      = 27
	A        = 65
	B        = 66
	C        = 67
	D        = 68
)
const MAX_INT = int((uint(1) << 63) - 1)

func ringBell() {
	_, _ = os.Stdout.Write([]byte{'\a'})
}

func handleRegularKeyPress(input []byte, key byte, prompt string) []byte {
	updatedInput := append(input, key)
	clearLine()
	drawPrompt(prompt)
	fmt.Printf("%s", updatedInput)
	return updatedInput
}

func handleDelete(input []byte) []byte {
	if len(input) > 0 {
		fmt.Fprint(os.Stdout, "\x1b[D \x1b[D")
		return input[:len(input)-1]
	}
	return input
}

func handleTab(input []byte, bellCnt int) ([]byte, int) {
	trimmedInput := stripLeft(input)
	updatedInput := []byte{}

	if len(trimmedInput) == 0 {
		// do nothing
		return input, bellCnt
	}

	matches := autoCmplBuiltin(input)
	// TODO: expand multiple options for builtins as well
	if len(matches) > 0 {
		updatedInput = []byte(matches[0])
		updatedInput = append(updatedInput, ' ')
		clearLine()
		drawPrompt(regularPrompt)
		fmt.Fprintf(os.Stdout, "%s", updatedInput)
		return updatedInput, bellCnt
	}

	matches = autoCmplBin(input)

	switch {
	case len(matches) == 0:
		ringBell()
		return input, bellCnt

	case len(matches) == 1:
		clearLine()
		drawPrompt(regularPrompt)
		updatedInput = cmplInput(input, matches[0])
		updatedInput := append(updatedInput, ' ')
		fmt.Fprintf(os.Stdout, "%s", updatedInput)
		return updatedInput, bellCnt

	case len(matches) > 1:
		commonPrefix := commonPrefix(matches)

		if len(commonPrefix) > len(trimmedInput) {
			updatedInput = cmplInput(input, commonPrefix)
			clearLine()
			drawPrompt(regularPrompt)
			fmt.Fprintf(os.Stdout, "%s", updatedInput)
			return updatedInput, bellCnt
		} else if commonPrefix == string(trimmedInput) {

			if bellCnt == 0 {
				ringBell()
				return input, bellCnt + 1
			}

			fmt.Fprint(os.Stdout, "\r\n")
			for _, match := range matches {
				fmt.Fprintf(os.Stdout, "%s  ", match)
			}

			fmt.Fprint(os.Stdout, "\r\n")
			drawPrompt(regularPrompt)
			fmt.Fprintf(os.Stdout, "%s", input)
			return input, bellCnt
		}
	}
	// unreachable
	return nil, -1
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

func autoCmplBuiltin(s []byte) []string {
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

func sharePrefix(lst []fs.DirEntry, prefix string) []string {
	var entries []string

	for _, e := range lst {
		if !strings.HasPrefix(e.Name(), string(prefix)) {
			continue
		}
		entries = append(entries, e.Name())
	}
	return entries
}

func commonPrefix(lst []string) string {
	maxPrefixLen := MAX_INT
	for _, e := range lst {
		if len(e) < maxPrefixLen {
			maxPrefixLen = len(e)
		}
	}

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

func autoCmplBin(s []byte) []string {
	clean := strings.TrimLeft(string(s), " \t")

	files, err := searchPathForBins(clean)
	if err != nil {
		return ([]string)(nil)
	}

	var results []string
	for _, file := range files {
		res := cmplInput(s, file)
		results = append(results, string(res))
	}

	return results
}

func stripLeft(s []byte) []byte {
	return []byte(strings.TrimLeft(string(s), "\t "))
}

func searchPathForBins(prefix string) ([]string, error) {
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

	var bins []string
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		entry, err := os.ReadDir(dir)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, err
			}
		}

		// TODO: use binary search?
		for _, e := range sharePrefix(entry, prefix) {
			execPath := filepath.Join(dir, e)
			err := isExec(execPath)

			if err != nil {
				return nil, err
			}

			if !slices.Contains(bins, e) {
				bins = append(bins, e)
			}
		}
	}
	return bins, nil
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
}

func drawPrompt(prompt string) {
	fmt.Fprint(os.Stdout, prompt)
}

func isMoveUpAnsiCode(buf []byte) bool {
	if len(buf) != 3 {
		return false
	}
	if buf[0] != ESC {
		return false
	}
	if buf[1] != LBRACKET {
		return false
	}
	if buf[2] != A {
		return false
	}
	return true
}

func isMoveDownAnsiCode(buf []byte) bool {
	if len(buf) != 3 {
		return false
	}
	if buf[0] != ESC {
		return false
	}
	if buf[1] != LBRACKET {
		return false
	}
	if buf[2] != B {
		return false
	}
	return true
}

func isMoveLeftAnsiCode(buf []byte) bool {
	if len(buf) != 3 {
		return false
	}
	if buf[0] != ESC {
		return false
	}
	if buf[1] != LBRACKET {
		return false
	}
	if buf[2] != D {
		return false
	}
	return true
}

func isMoveRightAnsiCode(buf []byte) bool {
	if len(buf) != 3 {
		return false
	}
	if buf[0] != ESC {
		return false
	}
	if buf[1] != LBRACKET {
		return false
	}
	if buf[2] != C {
		return false
	}
	return true
}

func readKeyStroke(logFile *os.File) ([]byte, error) {
	buf := make([]byte, 3)

	n, err := os.Stdin.Read(buf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, err
		} else {
			// FIXME: recover
			panic(err)
		}
	}
	if logFile != nil {
		fmt.Fprintf(logFile, "Received %x\n", buf)
		_ = logFile.Sync()
	}

	if n == 3 {
		switch {
		case isMoveUpAnsiCode(buf), isMoveDownAnsiCode(buf):
			// ignore vertical navigation
			return nil, fmt.Errorf("Out of line bounds")
		case isMoveRightAnsiCode(buf), isMoveLeftAnsiCode(buf):
			// TODO: handle input out of bounds issues
			// return buf, nil
			return nil, fmt.Errorf("Out of line bounds")
		}
	}

	return buf[:n], nil
}

func readInput(
	inputCh chan string,
	errorCh chan error,
	prompt string,
	historyF *os.File,
) {
	logFile, err := os.OpenFile("keylog.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	var input []byte

	defer func() {
		fmt.Fprint(os.Stdout, "\r\n")
		_ = os.Stdout.Sync()
		if input != nil {
			input = append(input, '\n')
			inputCh <- string(input)
		}
		close(inputCh)
	}()

	bellCnt := 0
	for {
		keystroke, err := readKeyStroke(logFile)
		// keystroke, err := readKeyStroke(nil)
		if err != nil {
			continue
		}

		if len(keystroke) == 3 {
			// ANSI CODE
			fmt.Fprint(os.Stdout, string(keystroke))

		} else if len(keystroke) == 1 {
			b := keystroke[0]
			switch b {
			case SIGINT:
				fmt.Printf("^C")
				errorCh <- SignalInterruptErr
				return
			case CR, NL:
				return
			case TAB:
				input, bellCnt = handleTab(input, bellCnt)
				if input == nil || bellCnt < 0 {
					panic("Reached unreachable state")
				}
			case DELETE:
				input = handleDelete(input)
				continue
			default:
				bellCnt = 0
				input = handleRegularKeyPress(input, b, prompt)
			}
		}
	}
}

func parseInput(
	tokenCh chan []Token,
	errorCh chan error,
	historyF *os.File,
) {
	parser := newParser()
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}

	prompt := regularPrompt

	defer func() {
		err := term.Restore(int(os.Stdin.Fd()), oldState)
		if err != nil {
			panic(err)
		}
		if len(parser.tokens) > 0 {
			tokenCh <- parser.tokens
		}
		// TODO: how to synchronize terminal mode
		// restoration with the main goroutine?
		close(tokenCh)
		close(errorCh)
	}()

Loop:
	inputCh := make(chan string)
	readInputErrorCh := make(chan error)
	go readInput(inputCh, readInputErrorCh, prompt, historyF)

	select {
	case err := <-readInputErrorCh:
		errorCh <- err
		return
	case input, ok := <-inputCh:
		if !ok || len(input) == 0 {
			return
		}

		err := parser.parse(input)
		if err != nil {
			if errors.Is(err, UnclosedQuoteErr) || errors.Is(err, PipeHasNoTargetErr) {
				prompt = awaitPrompt
				drawPrompt(awaitPrompt)
				goto Loop
			} else {
				errorCh <- err
				return
			}
		}

		return
	}
}

func splitAtPipe(tokens []Token) [][]Token {
	cmds := make([][]Token, 1)
	idx := 0

	for _, tok := range tokens {
		if t, ok := tok.(*LiteralToken); ok && t.literal == "|" {
			cmds = append(cmds, []Token{})
			idx++
			continue
		}
		cmds[idx] = append(cmds[idx], tok)
	}
	return cmds
}
