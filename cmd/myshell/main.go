package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	// FIXME: sigterm as well?
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	defer func() {
		signal.Stop(signalC)
		cancel()
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			}
		}
	}()

	repl(ctx, cancel, signalC)
}

func repl(ctx context.Context, cancelReplCtx context.CancelFunc, signalC chan os.Signal) {
	var cmd *Cmd
	var tokens []string
	tokenCh := make(chan []string)
	inputCh := make(chan string)
	prompt := "$ "

	defer func() {
		if cmd != nil {
			cmd.closeFds()
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			}
		}
	}()

	go readInput(inputCh)

	for {
		// always close file descriptos on success or errors
		if cmd != nil {
			cmd.closeFds()
		}

		fmt.Fprint(os.Stdout, prompt)

		// parseCtx, cancelParseCtx := context.WithCancel(ctx)
		go parseInput(signalC, inputCh, tokenCh)

		select {
		case <-signalC:
			// cancelParseCtx()
			fmt.Println()
			continue
		case tokens = <-tokenCh:
		}

		cmd, err := NewCmd(tokens)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing *Cmd: %s\n", err.Error())
			continue
		}

		if errors.As(err, &os.ErrNotExist) || errors.As(err, &os.ErrExist) || errors.As(err, &UnknownOperatorErr) {
			fmt.Fprintf(cmd.fds[STDERR], "%s\n", err.Error())
			continue
		}

		switch {
		case cmd.command == nil:
			fmt.Fprintf(cmd.fds[STDERR], notFound(cmd.argv[0]))
		case *cmd.command == EXIT:
			// TODO: call original binary instead of doing builtin
			// graceful shutdown with cancel context instead of killing with no defers run
			if err := cmd.exit(cancelReplCtx); err == nil {
				return
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
	}
}

func readInput(inputCh chan string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		input, err := reader.ReadString('\n')
		if err != nil {
			panic(err)
		}
		inputCh <- input
	}
}

func parseInput(
	signalCh chan os.Signal,
	inputCh chan string,
	tokenCh chan []string,
) {
	parser := newParser()

Loop:
	select {
	case <-signalCh:
		return

	case input := <-inputCh:
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
