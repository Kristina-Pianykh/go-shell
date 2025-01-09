package main

import (
	"bufio"
	"fmt"
	"os"
)

var _ = fmt.Fprint

func main() {

	// Wait for user input
	for {
		fmt.Fprint(os.Stdout, "$ ")
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			panic(err)
		}
		cmd := input[:len(input)-1]
		fmt.Fprintf(os.Stdout, fmt.Sprintf("%s: command not found\n", cmd))
	}
}
