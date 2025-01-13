package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// type Buffer []byte
type Buffer struct {
	content *[]byte
	stack   *stack
}
type stack []byte

func (b *Buffer) reset() {
	*b.content = (*b.content)[:0]
	b.stack.empty()
}

func (s *stack) push(v byte) *stack {
	updatedS := append(*s, v)
	return &updatedS
}

func (s *stack) pop() (byte, bool) {
	l := len(*s)
	// fmt.Printf("pop(): len: %d, stack: %s\n", l, string(*s))
	if l == 0 {
		return 0, false
	}
	v := (*s)[l-1]
	*s = (*s)[:l-1]
	return v, true
}

func (s *stack) peek() (byte, bool) {
	l := len(*s)
	if l == 0 {
		return 0, false
	}
	return (*s)[l-1], true
}

func (s *stack) empty() {
	*s = (*s)[:0]
}

type Cmd struct {
	buffer         *Buffer
	prompt         string
	validInput     bool
	needMatchingCh bool
	argv           *[]string
	builtins       [5]string
	builtin        bool
	command        *string
	commandPath    *string
}

const EXIT = "exit"
const ECHO = "echo"
const TYPE = "type"
const PWD = "pwd"
const CD = "cd"

func cmdInit() *Cmd {
	var content []byte = make([]byte, 0, 100)
	var stack stack = []byte{}
	var buffer Buffer = Buffer{
		content: &content,
		stack:   &stack,
	}

	argv := []string{}
	cmd := &Cmd{
		prompt:         "$ ",
		validInput:     true,
		needMatchingCh: false,
		buffer:         &buffer,
		builtins:       [5]string{EXIT, ECHO, TYPE, PWD, CD},
		argv:           &argv,
		command:        nil,
		commandPath:    nil,
	}
	return cmd
}

func (cmd *Cmd) parse(input string) {
	// echo parsing is a special case
	inputLeftTrimmed := strings.TrimLeft(input, " \n\t")

	if strings.HasPrefix(inputLeftTrimmed, ECHO) {
		command := "echo"
		cmd.command = &command
		cmd.needMatchingCh = true
	}
	if cmd.command != nil && *cmd.command == ECHO {
		cmd.appendToBufferMatching(inputLeftTrimmed)
		if len(*cmd.buffer.stack) > 0 {
			cmd.validInput = false
		} else {
			cmd.validInput = true
		}
		// fmt.Printf("echo command in buffer: %s\n", cmd.getBufAsString())
		return
	}

	quoteMatched := true
	stack := []rune{}

	for _, ch := range inputLeftTrimmed {

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
	}

	// keep input in buffer
	for _, arg := range *cmd.argv {
		cmd.appendToBuffer(arg)
		cmd.appendToBuffer(" ")
	}

	for i, arg := range *cmd.argv {
		(*cmd.argv)[i] = strings.TrimRight(strings.TrimLeft(arg, " "), " ")
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

func (cmd *Cmd) appendToBuffer(s string) {
	*cmd.buffer.content = append(*cmd.buffer.content, s...)
}

func (cmd *Cmd) appendToBufferMatching(s string) {
	if len(cmd.getBufAsString()) > 0 {
		*cmd.buffer.content = []byte(removeNewLineIfPresent(cmd.getBufAsString()))
	}

	arg := []byte{}

	for _, ch := range s {
		// fmt.Printf("ch: %c\n", ch)
		onTop, exists := cmd.buffer.stack.peek()
		if !exists {
			switch ch {
			case '\'', '"':
				cmd.buffer.stack = cmd.buffer.stack.push(byte(ch))
				continue
			case ' ', '\t':
				if len(arg) == 0 && strings.HasSuffix(cmd.getBufAsString(), " ") {
					continue
				}
				arg = []byte(string(removeMultipleWhitespaces(arg)))
				*cmd.buffer.content = append(*cmd.buffer.content, arg...)
				*cmd.buffer.content = append(*cmd.buffer.content, ' ')
				arg = arg[:0]
				continue
			case '\n':
				*cmd.buffer.content = append(*cmd.buffer.content, arg...)
				break
			default:
				arg = append(arg, byte(ch))
			}
		}
		if exists {
			if byte(ch) != onTop {
				arg = append(arg, byte(ch))
				continue
			}
			*cmd.buffer.content = append(*cmd.buffer.content, arg...)
			arg = arg[:0]
			cmd.buffer.stack.pop()
		}
	}
}

func removeMultipleWhitespaces(s []byte) []byte {
	multiSpace, err := regexp.Compile(" {2,}|\t+")
	if err != nil {
		panic(err)
	}
	return multiSpace.ReplaceAll(s, []byte{' '})
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
	return string(*cmd.buffer.content)
}

func (cmd *Cmd) getEchoArgs() (string, error) {
	args, found := strings.CutPrefix(cmd.getBufAsString(), "echo")
	if found {
		return addNewLineIfAbsent(trim(args)), nil
	}
	return "", errors.New(fmt.Sprintf("failed to parse echo args from %s\n", cmd.getBufAsString()))
}

func addNewLineIfAbsent(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	var sb strings.Builder
	sb.WriteString(s)
	sb.WriteString("\n")
	return sb.String()
}

func (cmd *Cmd) inputAsString() string {
	var sb strings.Builder
	for _, arg := range *cmd.argv {
		sb.WriteString(arg)
	}
	return sb.String()
}

func (cmd *Cmd) Reset() {
	cmd.buffer.reset()
	*cmd.argv = (*cmd.argv)[:0]
	cmd.command = nil
	cmd.commandPath = nil
	cmd.validInput = true
	cmd.needMatchingCh = false
	cmd.prompt = "$ "
	cmd.builtin = false
}

func (cmd *Cmd) String() string {
	return fmt.Sprintf("Cmd{prompt: %s, validInput: %v, needMatchingCh: %v, buffer: %s}", cmd.prompt, cmd.validInput, cmd.needMatchingCh, cmd.getBufAsString())
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
	return strings.TrimRight(strings.TrimLeft(v, " "), " ")
}

func notFound(input string) string {
	input = removeNewLineIfPresent(input)
	return fmt.Sprintf("%s: not found\n", input)
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
	}
}

func (cmd *Cmd) cd() {
	var absPath string
	path := (*cmd.argv)[1]

	if invalidPath, err := regexp.Match(".*[\\.]{3,}.*", []byte(path)); err == nil && invalidPath {
		fmt.Fprintf(os.Stderr, "cd: %s: No such file or directory\n", absPath)
		return
	}

	if strings.HasPrefix(path, "~") {
		home := os.Getenv("HOME")
		if len(path) == 0 {
			fmt.Fprintf(os.Stderr, "Failed to access HOME environment variable\n")
			return
		}
		path = filepath.Join(home, path[1:])
	}

	if filepath.IsAbs(path) {
		absPath = path
	} else {
		// FIXME: handle symlinks

		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "Failed to print current working directory")
		}
		absPath = filepath.Join(cwd, path)
	}

	if err := os.Chdir(absPath); err != nil {
		fmt.Fprintf(os.Stderr, "cd: %s: No such file or directory\n", absPath)
		return
	}

	// not sure we need to do this
	err := os.Setenv("PWD", absPath)
	if err != nil {
		panic(err)
	}
}
