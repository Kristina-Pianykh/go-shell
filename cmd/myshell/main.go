package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var _ = fmt.Fprint

type Buffer []byte

type Cmd struct {
	buffer             *Buffer
	prompt             string
	validInput         bool
	needMatchingCh     bool
	recognizedCommands []string
}

const EXIT = "exit"
const ECHO = "echo"
const TYPE = "type"

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
		prompt:             "$ ",
		validInput:         true,
		needMatchingCh:     false,
		buffer:             &buffer,
		recognizedCommands: []string{EXIT, ECHO, TYPE},
	}

	// Wait for user input
	for {
		fmt.Fprint(os.Stdout, cmd.prompt)
		input, err := bufio.NewReader(os.Stdin).ReadString('\n')

		if err != nil {
			panic(err)
		}

		for i, ch := range input {
			if len(*cmd.buffer) == 0 && i == ' ' {
				continue
			}
			cmd.Push(byte(ch))
		}

		command, ok := cmd.isBuiltin(cmd.getBufAsString())

		if !ok {
			fmt.Fprintf(os.Stdout, undefined(cmd.getBufAsString()))
			cmd.Reset()
			continue
		}

		switch {
		case command == EXIT:
			argsv := strings.Split(trim(cmd.getBufAsString()), " ")
			if len(argsv) != 2 {
				fmt.Fprintf(os.Stdout, undefined(trim(cmd.getBufAsString())))
				break
			}
			v, err := strconv.Atoi(removeNewLineIfPresent(argsv[1]))
			if err != nil {
				fmt.Fprintf(os.Stdout, undefined(trim(cmd.getBufAsString())))
				cmd.Reset()
				continue
			}
			os.Exit(v)
		case command == ECHO:
			cmd.needMatchingCh = true

			if cmd.validInput {
				fmt.Fprintf(os.Stdout, fmt.Sprintf("%s\n", parseEcho(cmd.getBufAsString())))
			} else {
				continue
			}
			cmd.Reset()
		case command == TYPE:
			tokens := strings.Split(removeNewLineIfPresent(trim(cmd.getBufAsString())), " ")

			for _, tok := range tokens[1:] {
				commandArg := trim(tok)
				if _, ok := cmd.isBuiltin(commandArg); ok {
					fmt.Fprintf(os.Stdin, fmt.Sprintf("%s is a shell builtin\n", commandArg))
					continue // this is different from bash for shell builtins
				}
				path, exists := isInPath(commandArg)
				if exists {
					fmt.Fprintf(os.Stdin, fmt.Sprintf("%s is %s\n", commandArg, path))
				} else {
					fmt.Fprintf(os.Stdout, undefined(commandArg))
				}
			}

			cmd.Reset()
		default:
			break
		}
	}
}

func (cmd *Cmd) isBuiltin(str string) (string, bool) {
	for _, c := range cmd.recognizedCommands {
		if strings.HasPrefix(str, c) {
			return c, true
		}
	}
	return str, false
}

func isInPath(s string) (string, bool) {
	paths, exists := os.LookupEnv("PATH")
	if !exists {
		return "", false
	}
	for _, path := range strings.Split(paths, ":") {
		path = trim(path)
		absPath, err := filepath.Abs(path)
		info, err := os.Stat(absPath)
		if err != nil {
			// no such file or directory; ignore faulty paths
			continue
		}
		if !info.Mode().IsDir() {
			return "", false
		}
		dirEntries, err := os.ReadDir(absPath)
		if err != nil {
			panic(err)
		}
		for _, entry := range dirEntries {
			if entry.Name() == s {
				return filepath.Join(absPath, s), true
			}
		}
	}
	return "", false
}

func trim(v string) string {
	return strings.TrimSuffix(strings.TrimPrefix(v, " "), " ")
}

func undefined(input string) string {
	input = removeNewLineIfPresent(input)
	return fmt.Sprintf("%s: not found\n", input)
}

func parseEcho(buf string) string {
	var sb strings.Builder
	doubleQuotesOk := true
	singleQuotesOk := true

	str := strings.TrimPrefix(strings.Split(buf, "echo")[1], " ")
	for _, ch := range removeNewLineIfPresent(str) {
		if ch == '"' && singleQuotesOk {
			continue
		} else if ch == '\'' && doubleQuotesOk {
			continue
		}
		sb.WriteRune(ch)
	}
	return sb.String()
}

func removeNewLineIfPresent(s string) string {
	if strings.HasSuffix(s, "\n") {
		return string(s[:len(s)-1])
	}
	return s
}
