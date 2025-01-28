package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
)

const prompt = "$ "

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
	}
}

func cmdLifecycle(ctx context.Context) error {
	var (
		cmd    *Cmd
		tokens []string
		ok     bool
	)
	tokenCh := make(chan []string)

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
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrExist) || errors.Is(err, UnknownOperatorErr) {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			return err
		}
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		return errors.New(fmt.Sprintf("Error initializing *Cmd: %s\n", err.Error()))
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
