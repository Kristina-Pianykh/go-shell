package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
)

const regularPrompt = "$ "
const awaitPrompt = "> "

func main() {
	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt)
	ctx, cancelCtx := context.WithCancel(context.Background())

	defer func() {
		signal.Stop(signalC)
		cancelCtx()
	}()

	for {
		err := cmdLifecycle(ctx)
		if errors.Is(err, ExitErr) {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
	}
}

func cmdLifecycle(ctx context.Context) error {
	var (
		shell  *Shell
		tokens []Token
	)
	tokenCh := make(chan []Token)
	errorCh := make(chan error, 1)
	fmt.Fprint(os.Stdout, regularPrompt)
	_ = os.Stdout.Sync()
	go parseInput(tokenCh, errorCh)

	var ok bool
	select {
	case err := <-errorCh:
		return err
	case tokens, ok = <-tokenCh:
		if !ok {
			return nil
		}
	}

	cmds := splitAtPipe(tokens)
	shell, err := NewShell(cmds, ctx)
	if err != nil {
		return err
	}

	switch {
	case shell.builtin != nil:
		return shell.runBuiltin()
	case shell.cmds != nil:
		return shell.executeCmds()
	}
	return nil
}
