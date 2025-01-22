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
	inputCh := make(chan string)

	defer func() {
		signal.Stop(signalC)
		cancel()
		close(inputCh)
	}()

	go readInput(inputCh)

	for {
		err := repl(ctx, cancel, signalC, inputCh)
		if errors.Is(err, ExitErr) {
			return
		}
	}
}

func repl(ctx context.Context, cancelReplCtx context.CancelFunc, signalC chan os.Signal, inputCh chan string) error {
	var cmd *Cmd
	var tokens []string
	tokenCh := make(chan []string)
	prompt := "$ "

	defer func() {
		if cmd != nil {
			cmd.closeFds()
		}
	}()

	fmt.Fprint(os.Stdout, prompt)
	inputCtx, inputCtxCancel := context.WithCancel(ctx)
	go parseInput(inputCtx, inputCh, tokenCh)

	select {
	case <-signalC:
		inputCtxCancel()
		fmt.Println()
		return SignalInterruptErr
	case tokens = <-tokenCh:
	}

	cmd, err := NewCmd(tokens)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing *Cmd: %s\n", err.Error())
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
	ctx context.Context,
	inputCh chan string,
	tokenCh chan []string,
) {
	parser := newParser()

Loop:
	select {
	case <-ctx.Done():
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
