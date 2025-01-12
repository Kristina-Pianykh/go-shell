package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
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
			if len(*cmd.argv) != 2 {
				fmt.Fprintf(os.Stdout, notFound(cmd.inputAsString()))
				break
			}
			v, err := strconv.Atoi((*cmd.argv)[1])
			if err != nil {
				fmt.Fprintf(os.Stdout, notFound(cmd.inputAsString()))
				break
			}
			os.Exit(v)
		case *cmd.command == ECHO:
			fmt.Fprintf(os.Stdout, parseEcho(cmd.getBufAsString()))
		case *cmd.command == TYPE:
			for _, arg := range (*cmd.argv)[1:] {
				if arg, ok := cmd.isBuiltin(arg); ok {
					fmt.Fprintf(os.Stdin, fmt.Sprintf("%s is a shell builtin\n", arg))
					continue // this is different from bash for shell builtins
				}

				if path, err := exec.LookPath(arg); err == nil {
					fmt.Fprintf(os.Stdin, "%s is %s\n", arg, path)
				} else {
					fmt.Fprintf(os.Stdout, notFound(arg))
				}
			}
		case *cmd.command == PWD:
			cmd.pwd()
		case cmd.command != nil && cmd.commandPath != nil:
			cmd.exec()
		}
		cmd.Reset()
	}
}
