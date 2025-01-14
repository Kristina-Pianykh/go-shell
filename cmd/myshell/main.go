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
		argv, err := parser.parse(input)
		if err != nil && errors.Is(err, unclosedQuoteErr) {
			continue
		}

		if len(*argv) < 1 {
			goto reset
		}

		cmd.argv = argv
		cmd.argc = len(*argv)
		cmd.setCommandAndPath(&(*argv)[0])

		switch {
		case cmd.command == nil:
			fmt.Fprintf(os.Stdout, notFound((*cmd.argv)[0]))
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
