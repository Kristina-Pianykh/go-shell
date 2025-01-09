package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var _ = fmt.Fprint

type Cmd Exit

type cmd interface {
	command() string
	args() []string
}

type Exit struct {
	code int
}

func (e Exit) command() string {
	return "exit"
}

type Undefined struct {
	input string
}

func main() {

	// Wait for user input
	for {
		fmt.Fprint(os.Stdout, "$ ")
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			panic(err)
		}
		input = strings.TrimPrefix(input, " ")
		input = strings.TrimSuffix(input, " ")
		input = strings.TrimSuffix(input, "\n")

		argsv := strings.Split(input, " ")
		switch {
		case argsv[0] == "exit":
			if len(argsv) != 2 {
				fmt.Fprintf(os.Stdout, undefined(input))
				continue
			}
			v, err := strconv.Atoi(argsv[1])
			if err != nil {
				panic(err)
			}
			os.Exit(v)
		default:
			fmt.Fprintf(os.Stdout, undefined(input))
		}
	}
}

func undefined(input string) string {
	return fmt.Sprintf("%s: command not found\n", input)
}
