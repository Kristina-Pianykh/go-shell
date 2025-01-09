package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var _ = fmt.Fprint

type Buffer []byte

type Cmd struct {
	buffer         *Buffer
	prompt         string
	validInput     bool
	needMatchingCh bool
}

func (cmd *Cmd) getBufAsString() string {
	return string(*cmd.buffer)
}

func (cmd *Cmd) Push(v byte) {
	if v == '"' {
		cmd.validInput = !cmd.validInput
	}
	if v == '\'' {
		cmd.validInput = !cmd.validInput
	}
	// if !cmd.validInput {
	// 	cmd.prompt = "> "
	// } else {
	// 	cmd.prompt = "$ "
	// }
	*cmd.buffer = append(*cmd.buffer, v)
}

func (cmd *Cmd) Pop() byte {
	// FIXME: What do we do if the stack is empty, though?
	l := len(*cmd.buffer)
	v := (*cmd.buffer)[l-1]
	*cmd.buffer = (*cmd.buffer)[:l-1]
	return v
}

func (cmd *Cmd) Empty() {
	*cmd.buffer = (*cmd.buffer)[:0]
}

func (cmd *Cmd) Reset() {
	cmd.Empty()
	cmd.validInput = true
	cmd.needMatchingCh = false
	cmd.prompt = "$ "
}

func (cmd *Cmd) String() string {
	return fmt.Sprintf("Cmd{prompt: %s, validInput: %v, needMatchingCh: %v, buffer: %s}", cmd.prompt, cmd.validInput, cmd.needMatchingCh, string(*cmd.buffer))
}

func main() {

	var buffer Buffer = make([]byte, 0, 100)
	cmd := &Cmd{
		prompt:         "$ ",
		validInput:     true,
		needMatchingCh: false,
		buffer:         &buffer,
	}

	// Wait for user input
	for {
		fmt.Fprint(os.Stdout, cmd.prompt)
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')

		if err != nil {
			panic(err)
		}

		for _, ch := range input {
			cmd.Push(byte(ch))
		}

		switch {
		case strings.HasPrefix(trim(cmd.getBufAsString()), "exit"):
			argsv := strings.Split(trim(cmd.getBufAsString()), " ")
			if len(argsv) != 2 {
				fmt.Fprintf(os.Stdout, undefined(trim(cmd.getBufAsString())))
				break
			}
			v, err := strconv.Atoi(removeNewLineIfPresent(argsv[1]))
			if err != nil {
				panic(err)
			}
			os.Exit(v)
		case strings.HasPrefix(trim(cmd.getBufAsString()), "echo"):
			cmd.needMatchingCh = true
			// fmt.Printf("Cmd : %s\n", cmd.String())

			if cmd.validInput {
				fmt.Fprintf(os.Stdout, fmt.Sprintf("%s\n", parseEcho(cmd.getBufAsString())))
			} else {
				continue
			}
			cmd.Reset()
		default:
			fmt.Fprintf(os.Stdout, undefined(cmd.getBufAsString()))
			cmd.Reset()
		}
	}
}

func trim(v string) string {
	return strings.TrimSuffix(strings.TrimPrefix(v, " "), " ")
}

func undefined(input string) string {
	input = removeNewLineIfPresent(input)
	return fmt.Sprintf("%s: command not found\n", input)
}

func parseEcho(buf string) string {
	// fmt.Printf("before printing echo commang buf: %s\n", buf)
	var sb strings.Builder
	doubleQuotesOk := true
	singleQuotesOk := true

	str := strings.TrimPrefix(strings.Split(buf, "echo")[1], " ")
	for _, ch := range removeNewLineIfPresent(str) {
		if ch == '"' && singleQuotesOk {
			continue
			// } else if ch == '"' && !singleQuotesOk {
			// 	sb.WriteRune(ch)
		} else if ch == '\'' && doubleQuotesOk {
			continue
			// } else if ch == '\'' && !doubleQuotesOk {
			// 	sb.WriteRune(ch)
		}
		sb.WriteRune(ch)
	}

	// for i, p := range params {
	// 	c := strings.TrimSuffix(strings.TrimPrefix(p, " "), " ")
	// 	quoted := (strings.HasPrefix(c, "\"") && strings.HasSuffix(c, "\"")) ||
	// 		(strings.HasPrefix(c, "'") && strings.HasSuffix(c, "'"))
	// 	sb.WriteString(strings.Repeat(" ", i))
	//
	// 	if quoted {
	// 		sb.WriteString(string(c[1 : len(c)-1]))
	// 	} else {
	// 		sb.WriteString(c)
	// 	}
	//
	// }
	return sb.String()
}

func removeNewLineIfPresent(s string) string {
	if strings.HasSuffix(s, "\n") {
		return string(s[:len(s)-1])
	}
	return s
}
