package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	cmd := cmdInit()

	// Wait for user input
	for {
		fmt.Fprint(os.Stdout, cmd.prompt)
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')

		if err != nil {
			panic(err)
		}
		cmd.parse(input)

		switch {
		case cmd.command == nil:
			fmt.Fprintf(os.Stdout, notFound((*cmd.argv)[0]))
		case *cmd.command == EXIT:
			cmd.exit()
		case *cmd.command == ECHO:
			fmt.Fprintf(os.Stdout, parseEcho(cmd.getBufAsString()))
		case *cmd.command == TYPE:
			cmd.typeCommand()
		case *cmd.command == PWD:
			cmd.pwd()
		case cmd.command != nil && cmd.commandPath != nil:
			cmd.exec()
		}
		cmd.Reset()
	}
}
