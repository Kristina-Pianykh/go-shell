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

// func TestCommonPrefix(t *testing.T) {
// 	lst := []string{
// 		"callgraph", "cargo", "cargo-clippy", "cargo-fmt", "cargo-miri", "catman", "caca-config", "cacaclock", "cacademo", "cacafire", "cacaplay", "cacaserver", "cacaview", "cairo-trace", "cal", "calfjackhost", "canberra-boot", "canberra-gtk-play", "capsh", "captest", "captoinfo",
// 	}
// 	fmt.Printf("has prefix ca: %v\n", sharePrefix(lst, []byte("ca")))
// 	fmt.Printf("has prefix ca: %v\n", sharePrefix(lst, []byte("c")))
// 	fmt.Printf("has prefix ca: %v\n", sharePrefix(lst, []byte("cal")))
// 	prefix := commonPrefix(lst)
// 	fmt.Printf("common prefix: %s\n", prefix)
// 	lst = []string{
// 		"xyz_bar", "xyz_baz", "xyz_quz",
// 	}
// 	fmt.Printf("has prefix ca: %v\n", sharePrefix(lst, []byte("x")))
// 	fmt.Printf("has prefix ca: %v\n", sharePrefix(lst, []byte("xyz_")))
// 	fmt.Printf("has prefix ca: %v\n", sharePrefix(lst, []byte("xyz_ba")))
// 	prefix = commonPrefix(lst)
// 	fmt.Printf("common prefix: %s\n", prefix)
//
// 	lst = []string{
// 		"xyz_bar", "xyz_bar_baz", "xyz_bar_baz_qux",
// 	}
// 	prefix = commonPrefix(lst)
// 	fmt.Printf("common prefix: %s\n", prefix)
// }
