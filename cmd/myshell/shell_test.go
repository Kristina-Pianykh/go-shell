package main

import (
	"fmt"
	"testing"
)

func TestIsNumber(t *testing.T) {
	nums := []string{
		"1234",
		"0",
		"034",
		"0000034",
		"",
		"324o34",
	}
	for _, n := range nums {
		fmt.Printf("isNumber(%s): %v\n", n, isNumber(n))
	}
}

func TestSplitAt(t *testing.T) {
	argv, path := splitAtRedirectOp(&[]string{"echo", "hello", ">>", "test.txt"}, ">>")
	fmt.Printf("%v, %v\n", argv, path)
}
