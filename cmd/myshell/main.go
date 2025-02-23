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
		tokens []token
		ok     bool
	)
	tokenCh := make(chan []token)
	fmt.Fprint(os.Stdout, regularPrompt)
	_ = os.Stdout.Sync()
	go parseInput(tokenCh)

	select {
	case tokens, ok = <-tokenCh:
		if !ok {
			fmt.Println()
			return SignalInterruptErr
		}
	}

	cmds := splitAtPipe(tokens)
	shell, err := NewShell(cmds, ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
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
