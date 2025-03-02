package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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

	history, err := loadHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
	}

	for {
		err := cmdLifecycle(ctx, history)
		if errors.Is(err, ExitErr) {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		}
	}
}

func loadHistory() (*os.File, error) {
	file := ".history"

	var err error
	var cwd string
	cwd, err = os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to load current working directory") // for debugging
	}

	filePath := filepath.Join(cwd, file)

	var f *os.File
	f, err = os.OpenFile(filePath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open history file %s\n", filePath)
	}
	return f, nil
}

func cmdLifecycle(ctx context.Context, history *os.File) error {
	var shell *Shell
	var tokens []Token

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
