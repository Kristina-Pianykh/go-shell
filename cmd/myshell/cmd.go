package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Buffer []byte

type Cmd struct {
	buffer         *Buffer
	prompt         string
	validInput     bool
	needMatchingCh bool
	argv           *[]string
	builtins       [4]string
	builtin        bool
	command        *string
	commandPath    *string
}

const EXIT = "exit"
const ECHO = "echo"
const TYPE = "type"
const PWD = "pwd"

func cmdInit() *Cmd {
	var buffer Buffer = make([]byte, 0, 100)
	argv := []string{}
	cmd := &Cmd{
		prompt:         "$ ",
		validInput:     true,
		needMatchingCh: false,
		buffer:         &buffer,
		builtins:       [4]string{EXIT, ECHO, TYPE, PWD},
		argv:           &argv,
		command:        nil,
		commandPath:    nil,
	}
	return cmd
}

func (cmd *Cmd) parse(input string) {
	// echo parsing is a special case
	inputCleanedLeft := strings.TrimLeft(input, " \n\t")

	// FIXME: echo parsing on incomplete input
	if strings.HasPrefix(inputCleanedLeft, ECHO) || strings.HasPrefix(cmd.getBufAsString(), ECHO) {
		cmd.needMatchingCh = true

		if cmd.validInput {
			*cmd.buffer = append(*cmd.buffer, parseEcho(input)...)
		} else {
			return
		}
	}

	quoteMatched := true
	stack := []rune{}

	for _, ch := range input {
		// fmt.Printf("ch: %c\n", ch)
		// fmt.Printf("%v\n", *cmd.argv)

		if (ch == ' ' || ch == '\n') && quoteMatched {
			*cmd.argv = append(*cmd.argv, string(stack))
			// *cmd.buffer = append(*cmd.buffer, stack...)
			stack = stack[:0]
			continue
		}
		if ch == '\'' || ch == '"' {
			quoteMatched = !quoteMatched
			*cmd.argv = append(*cmd.argv, string(stack))
			stack = stack[:0]
			continue
		}
		stack = append(stack, ch)
		// fmt.Printf("stack: %s\n", string(stack))
	}

	// keep input in buffer
	for _, arg := range *cmd.argv {
		*cmd.buffer = append(*cmd.buffer, []byte(arg)...)
		*cmd.buffer = append(*cmd.buffer, ' ')
	}

	for i, arg := range *cmd.argv {
		(*cmd.argv)[i] = strings.TrimRight(strings.TrimLeft(arg, " "), " ")
		// fmt.Printf("arg %d: %s\n", i, (*cmd.argv)[i])
	}

	command, ok := cmd.isBuiltin((*cmd.argv)[0])

	if ok {
		cmd.builtin = true
		cmd.command = &command
		return
	}

	if path, err := exec.LookPath((*cmd.argv)[0]); err == nil {
		cmd.commandPath = &path
		cmd.command = &(*cmd.argv)[0]
	}

	return
}

func (cmd *Cmd) exec() {
	// codecraters test expect cmd.command instead of cmd.commandPath to pass
	cmdC := exec.Command(*cmd.command, (*cmd.argv)[1:]...)
	var out strings.Builder
	cmdC.Stdout = &out
	if err := cmdC.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	} else {
		// codecraters test expect truncated newline but it's not the right behavior
		fmt.Fprintf(os.Stdout, "%s\n", removeNewLineIfPresent(out.String()))
	}
}

func (cmd *Cmd) getBufAsString() string {
	return string(*cmd.buffer)
}

func (cmd *Cmd) inputAsString() string {
	var sb strings.Builder
	for _, arg := range *cmd.argv {
		sb.WriteString(arg)
	}
	return sb.String()
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
	*cmd.argv = (*cmd.argv)[:0]
	cmd.command = nil
	cmd.commandPath = nil
	cmd.validInput = true
	cmd.needMatchingCh = false
	cmd.prompt = "$ "
	cmd.builtin = false
}

func (cmd *Cmd) String() string {
	return fmt.Sprintf("Cmd{prompt: %s, validInput: %v, needMatchingCh: %v, buffer: %s}", cmd.prompt, cmd.validInput, cmd.needMatchingCh, string(*cmd.buffer))
}

func (cmd *Cmd) isBuiltin(str string) (string, bool) {
	for _, c := range cmd.builtins {
		if strings.HasPrefix(str, c) {
			return c, true
		}
	}
	return str, false
}

func trim(v string) string {
	return strings.TrimSuffix(strings.TrimPrefix(v, " "), " ")
}

func notFound(input string) string {
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
	if !strings.HasSuffix(sb.String(), "\n") {
		sb.WriteString("\n")
	}
	return sb.String()
}

func removeNewLineIfPresent(s string) string {
	if strings.HasSuffix(s, "\n") {
		return string(s[:len(s)-1])
	}
	return s
}

func (cmd *Cmd) exit() {
	if len(*cmd.argv) != 2 {
		fmt.Fprintf(os.Stdout, notFound(cmd.inputAsString()))
		return
	}
	v, err := strconv.Atoi((*cmd.argv)[1])
	if err != nil {
		fmt.Fprintf(os.Stdout, notFound(cmd.inputAsString()))
		return
	}
	os.Exit(v)
}

func (cmd *Cmd) typeCommand() {
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
}

func (cmd *Cmd) pwd() {
	if path, err := os.Getwd(); err == nil {
		fmt.Fprintf(os.Stdout, "%s\n", path)
	} else {
		fmt.Fprintln(os.Stderr, "Failed to get working directory")
	}
}
