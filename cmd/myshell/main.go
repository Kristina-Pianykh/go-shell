package main

import (
	"bufio"
	"fmt"
	"os"
)

var _ = fmt.Fprint

func main() {
	// Uncomment this block to pass the first stage
	fmt.Fprint(os.Stdout, "$ ")

	// Wait for user input
	bufio.NewReader(os.Stdin).ReadString('\n')
}
