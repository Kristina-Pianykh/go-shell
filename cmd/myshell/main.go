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
	}
}

func cmdLifecycle(ctx context.Context) error {
	var (
		shell  *Shell
		tokens []token
		ok     bool
	)
	tokenCh := make(chan []token)

	defer func() {
		if shell != nil {
			shell.closeFds()
		}
	}()

	fmt.Fprint(os.Stdout, regularPrompt)
	os.Stdout.Sync()
	go parseInput(tokenCh)

	select {
	case tokens, ok = <-tokenCh:
		if !ok {
			fmt.Println()
			return SignalInterruptErr
		}
	}

	cmds := splitAtPipe(tokens)
	// for _, cmd := range cmds {
	// 	for _, tok := range cmd {
	// 		fmt.Printf("tok: %s\n", tok.string())
	// 	}
	// }
	shell, err := NewShell(cmds, ctx)
	if err != nil {
		// if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrExist) || errors.Is(err, UnknownOperatorErr) || errors.Is(err, notFoundError) {
		// 	fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		// 	return err
		// }
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		return err
		// return errors.New(fmt.Sprintf("Error initializing *Shell: %s\n", err.Error()))
	}
	switch {
	case shell.builtin != nil:
		return shell.runBuiltin()
	case shell.cmds != nil:
		err := shell.execute(ctx, 0, nil, nil)
		// if err != nil {
		// 	fmt.Fprint(os.Stderr, err.Error())
		// }
		return err
	}

	// switch {
	// case shell.command == nil:
	// 	fmt.Fprintf(shell.fds[STDERR], notFound(shell.argv[0]))
	// case *shell.command == EXIT:
	// 	// TODO: call original binary instead of doing builtin
	// 	// graceful shutdown with cancel context instead of killing with no defers run
	// 	if err := shell.exit(); err == nil {
	// 		return ExitErr
	// 	}
	// case *shell.command == ECHO:
	// 	shell.echo()
	// case *shell.command == TYPE:
	// 	shell.typeCommand()
	// case *shell.command == PWD:
	// 	shell.pwd()
	// case *shell.command == CD:
	// 	shell.cd()
	// case shell.command != nil && shell.commandPath != nil:
	// 	shell.exec(ctx)
	// }
	return nil
}
