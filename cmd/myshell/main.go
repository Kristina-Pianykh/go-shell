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

	cmd := initCmd(signalC)
	parser := initParser()

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

	repl(ctx, cancel, cmd, parser)
}

func repl(ctx context.Context, cancel context.CancelFunc, cmd *Cmd, parser *Parser) {
	reader := bufio.NewReader(os.Stdin)

	go func() {
		for {
			select {
			case <-cmd.signalC:
				fmt.Fprintf(os.Stdout, "\n%s", cmd.prompt)
				cmd.promptPrinted = true
				parser.clear()
				cmd.Reset()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for user input
	for {
		if !cmd.promptPrinted {
			fmt.Fprint(os.Stdout, cmd.prompt)
			cmd.promptPrinted = true
		}

		input, err := reader.ReadString('\n')
		if err != nil {
			continue
		}

		tokens, err := parser.parse(input)
		if err != nil && errors.Is(err, unclosedQuoteErr) {
			continue
		}

		if len(*tokens) < 1 {
			goto reset
		}

		cmd.tokens = tokens
		cmd.setCommandAndPath(&(*tokens)[0])
		err = cmd.parse(tokens)

		if errors.As(err, &os.ErrNotExist) || errors.As(err, &os.ErrExist) || errors.As(err, &UnknownOperatorErr) {
			fmt.Fprintf(cmd.fds[2], "%s\n", err.Error())
			goto reset
		}

		cmd.argc = len(*cmd.argv)

		switch {
		case cmd.command == nil:
			fmt.Fprintf(os.Stdout, notFound((*cmd.tokens)[0]))
		case *cmd.command == EXIT:
			// TODO: call original binary instead of doing builtin
			// graceful shutdown with cancel context instead of killing with no defers run
			cmd.exit(cancel)
			return
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

	reset:
		parser.clear()
		cmd.Reset()
		cmd.promptPrinted = false
	}
}
