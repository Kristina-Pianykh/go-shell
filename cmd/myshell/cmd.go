package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Cmd struct {
	prompt      string
	tokens      *[]string
	argv        *[]string
	argc        int
	builtins    *[5]string
	command     *string
	commandPath *string
	fds         map[int]*os.File
}

const EXIT = "exit"
const ECHO = "echo"
const TYPE = "type"
const PWD = "pwd"
const CD = "cd"

var builtins [5]string = [5]string{EXIT, ECHO, TYPE, PWD, CD}
var fds map[int]*os.File = map[int]*os.File{0: os.Stdin, 1: os.Stdout, 2: os.Stderr}

func initCmd() *Cmd {
	tokens := []string{}
	argv := []string{}
	cmd := &Cmd{
		prompt:      "$ ",
		builtins:    &builtins,
		tokens:      &tokens,
		argv:        &argv,
		argc:        0,
		command:     nil,
		commandPath: nil,
		fds:         fds,
	}
	return cmd
}

func (cmd *Cmd) exec() {
	cmdC := exec.Command(*cmd.command, (*cmd.tokens)[1:]...)
	var out strings.Builder
	cmdC.Stdout = &out
	if err := cmdC.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	} else {
		// codecraters test expect truncated newline but it's not the right behavior
		fmt.Fprintf(cmd.fds[1], "%s\n", removeNewLineIfPresent(out.String()))
	}
}

// func (cmd *Cmd) redirectFd(fd int, op string) {
// 	var err error
// 	// redirectionOps := []string{">", ">|", ">>"}
// 	_, path := splitAtRedirectOp(cmd.tokens, op)
// 	*cmd.tokens[fd], err = os.OpenFile(pat)
// }

func (cmd *Cmd) parse(argv *[]string, op string) {
	var path string
	var err error
	redirectionOps := []string{">", ">|", ">>"}

	for _, op := range redirectionOps {
		if slices.Contains(*cmd.tokens, op) {

			for i := 0; i < len(*argv); {
				arg := (*argv)[i]

				if arg == op {
					path = (*argv)[i+1]
					i = i + 2
					continue
				}
				*cmd.argv = append(*cmd.argv, arg)
				i++
			}

      if op ==
			*cmd.fds[1], err = os.OpenFile(path)
		}
	}
}

func (cmd *Cmd) getArgv() string {
	var sb strings.Builder
	for _, arg := range *cmd.tokens {
		sb.WriteString(arg)
	}
	return sb.String()
}

func (cmd *Cmd) Reset() {
	*cmd.tokens = (*cmd.tokens)[:0]
	cmd.argc = 0
	cmd.command = nil
	cmd.commandPath = nil
	cmd.prompt = "$ "
	cmd.fds = fds
}

// func (cmd *Cmd) String() string {
// 	return fmt.Sprintf("Cmd{prompt: %s, validInput: %v, needMatchingCh: %v, buffer: %s}", cmd.prompt, cmd.validInput, cmd.needMatchingCh, cmd.getBufAsString())
// }

func (cmd *Cmd) isBuiltin(str string) bool {
	for _, c := range cmd.builtins {
		if str == c {
			return true
		}
	}
	return false
}

func (cmd *Cmd) setCommandAndPath(c *string) {
	if builtin := cmd.isBuiltin(*c); builtin {
		cmd.command = c
		return
	}
	if path, err := exec.LookPath(*c); err == nil {
		cmd.command = c
		cmd.commandPath = &path
	}
}

func (cmd *Cmd) echo() {
	// fmt.Printf("argv: %v\n", *cmd.argv)
	var sb strings.Builder
	for i := 1; i < cmd.argc; i++ {
		sb.WriteString((*cmd.tokens)[i])
		if i < cmd.argc-1 {
			sb.WriteString(" ")
		} else {
			sb.WriteString("\n")
		}
	}
	fmt.Fprintf(os.Stdout, sb.String())
}

func (cmd *Cmd) exit() {
	if len(*cmd.tokens) != 2 {
		fmt.Fprintf(os.Stdout, notFound(cmd.getArgv()))
		return
	}
	v, err := strconv.Atoi((*cmd.tokens)[1])
	if err != nil {
		fmt.Fprintf(os.Stdout, notFound(cmd.getArgv()))
		return
	}
	os.Exit(v)
}

func (cmd *Cmd) typeCommand() {
	for _, arg := range (*cmd.tokens)[1:] {
		if ok := cmd.isBuiltin(arg); ok {
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
	path := (*cmd.tokens)[1]

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
