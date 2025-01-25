package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
)

func main() {
	// FIXME: sigterm as well?
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt)
	ctx, cancelCtx := context.WithCancel(context.Background())

	defer func() {
		signal.Stop(signalC)
		cancelCtx()
	}()

	for {
		err := cmdLifecycle(ctx, signalC)
		if errors.Is(err, ExitErr) {
			return
		}
	}
}

func cmdLifecycle(ctx context.Context, signalC chan os.Signal) error {
	var (
		cmd    *Cmd
		tokens []string
		ok     bool
	)
	tokenCh := make(chan []string)
	prompt := "$ "

	defer func() {
		if cmd != nil {
			cmd.closeFds()
		}
	}()

	fmt.Fprint(os.Stdout, prompt)
	os.Stdout.Sync()
	go parseInput(tokenCh)

	select {
	case tokens, ok = <-tokenCh:
		if !ok {
			fmt.Println()
			return SignalInterruptErr
		}
	}

	cmd, err := NewCmd(tokens)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		return errors.New(fmt.Sprintf("Error initializing *Cmd: %s\n", err.Error()))
	}

	if errors.As(err, &os.ErrNotExist) || errors.As(err, &os.ErrExist) || errors.As(err, &UnknownOperatorErr) {
		fmt.Fprintf(cmd.fds[STDERR], "%s\n", err.Error())
		return err
	}

	switch {
	case cmd.command == nil:
		fmt.Fprintf(cmd.fds[STDERR], notFound(cmd.argv[0]))
	case *cmd.command == EXIT:
		// TODO: call original binary instead of doing builtin
		// graceful shutdown with cancel context instead of killing with no defers run
		if err := cmd.exit(); err == nil {
			return ExitErr
		}
	case *cmd.command == ECHO:
		cmd.echo()
	case *cmd.command == TYPE:
		cmd.typeCommand()
	case *cmd.command == PWD:
		cmd.pwd()
	case *cmd.command == CD:
		cmd.cd()
	case cmd.command != nil && cmd.commandPath != nil:
		cmd.exec(ctx)
	}
	return nil
}

const del = 127
const cariageReturn = 13
const newLine = 10
const sigint = 3
const tab = 9

// TODO: don't allow cursor moves outside of input buffer boundary
func readInput(inputCh chan string) {
	var err error

	// logFile, err := os.OpenFile("keylog.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
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
			if cmpl, ok := autocomplete(input); !ok {
				fmt.Fprintf(os.Stdout, "%c", '\a')
			} else {
				clearPrompt()

				input = cmpl
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
			clearPrompt()
			fmt.Printf("%s", input)
		}
	}
}

func autocomplete(s []byte) ([]byte, bool) {
	clean := strings.TrimLeft(string(s), " \t")
	offset := len(s) - len(clean)
	new := []byte{}

	for _, builtin := range builtins {
		if len(clean) > 0 && strings.HasPrefix(builtin, clean) {

			for i := range offset {
				new = append(new, s[i])
			}
			new = append(new, builtin...)
			return new, true
		}
	}

	// fmt.Printf("checking PATH")
	if file, found := searchPath(clean); found {
		for i := range offset {
			new = append(new, s[i])
		}
		new = append(new, file...)
		return new, true
	}

	return s, false
}

func searchPath(prefix string) (string, bool) {
	// if file is a path
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
	for _, dir := range filepath.SplitList(path) {
		// fmt.Printf("dir: %s\r\n", dir)
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		entry, err := os.ReadDir(dir)
		if err != nil {
			panic(err)
		}
		for _, e := range entry {
			// TODO: use binary search
			// TODO: give options on multiple options?
			if strings.HasPrefix(e.Name(), prefix) {
				// fmt.Printf("entry: %s\r\n", e.Name())

				execPath := filepath.Join(dir, e.Name())
				if err := isExec(execPath); err == nil {
					return e.Name(), true
					// } else {
					// 	panic(err)
				}
			}
		}
		// path := filepath.Join(dir, file)
		// // fmt.Printf("path: %s\r\n", path)
		// if err := isExec(path); err == nil {
		// 	if !filepath.IsAbs(path) {
		// 		return "", false
		// 	}
		// 	return path, true
		// }
	}
	return "", false
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

func clearPrompt() {
	fmt.Print("\x1b[2K\r")
	// TODO set prompt as a global variable
	fmt.Printf("$ ")
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
		if err != nil && errors.As(err, &UnclosedQuoteErr) {
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
