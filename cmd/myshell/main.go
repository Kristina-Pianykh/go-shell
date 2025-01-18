package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
)

func main() {
	cmd := initCmd()
	parser := initParser()

	// Wait for user input
	for {
		fmt.Fprint(os.Stdout, cmd.prompt)
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')

		if err != nil {
			panic(err)
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
			cmd.exit()
		case *cmd.command == ECHO:
			cmd.echo()
		case *cmd.command == TYPE:
			cmd.typeCommand()
		case *cmd.command == PWD:
			cmd.pwd()
		case *cmd.command == CD:
			cmd.cd()
		case cmd.command != nil && cmd.commandPath != nil:
			cmd.exec()
		}

	reset:
		parser.clear()
		cmd.Reset()
	}
}
