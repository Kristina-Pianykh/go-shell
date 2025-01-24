package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

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

	// fmt.Printf("argv: %s\n", cmd.getArgv())
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
	var oldState *term.State
	success := false

	oldState, err = term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 1)
	input := []byte{}

	defer func() {
		input = append(input, '\n')
		if success {
			inputCh <- string(input)
		}
		fmt.Printf("\r\n")
		err := term.Restore(int(os.Stdin.Fd()), oldState)
		if err != nil {
			panic(err)
		}
		close(inputCh)
	}()

	for {
		_, err = os.Stdin.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				success = true
				return
			} else {
				panic(err)
			}
		}
		b := buf[0]

		switch b {
		case sigint:
			fmt.Printf("^C")
			return
		case cariageReturn, newLine:
			success = true
			return
		case tab:
			success = true
			if cmpl, ok := autocomplete(input); ok {
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
			fmt.Printf("%c", b)
			input = append(input, b)
		}
	}
}

func autocomplete(s []byte) ([]byte, bool) {
	clean := strings.TrimLeft(string(s), " \t")
	offset := len(s) - len(clean)

	for _, builtin := range builtins {
		if len(clean) > 0 && strings.HasPrefix(builtin, clean) {
			new := []byte{}

			for i := range offset {
				new = append(new, s[i])
			}
			new = append(new, builtin...)
			return new, true
		}
	}
	return s, false
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
	inputCh := make(chan string)

	defer close(tokenCh)

Loop:
	go readInput(inputCh)
	select {
	case input, ok := <-inputCh:
		if !ok {
			return
		}
		tokens, err := parser.parse(input)
		if err != nil && errors.As(err, &UnclosedQuoteErr) {
			goto Loop
		}

		if len(tokens) < 1 {
			return
		}

		tokenCh <- tokens
		return
	}
}
