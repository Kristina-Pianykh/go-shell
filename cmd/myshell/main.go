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
		if cmd.needMatchingCh && !cmd.validInput {
			// fmt.Printf("incomplete input; continue\n")
			continue
		}

		switch {
		case cmd.command == nil:
			fmt.Fprintf(os.Stdout, notFound((*cmd.argv)[0]))
		case *cmd.command == EXIT:
			cmd.exit()
		case *cmd.command == ECHO:
			s, err := cmd.getEchoArgs()
			if err != nil {
				fmt.Fprintf(os.Stderr, err.Error())
				break
			}
			fmt.Fprintf(os.Stdout, s)
		case *cmd.command == TYPE:
			cmd.typeCommand()
		case *cmd.command == PWD:
			cmd.pwd()
		case *cmd.command == CD:
			cmd.cd()
		case cmd.command != nil && cmd.commandPath != nil:
			cmd.exec()
		}
		cmd.Reset()
	}
}
