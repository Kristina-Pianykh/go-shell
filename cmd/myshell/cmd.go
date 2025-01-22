package main

import (
	"context"
	"errors"
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
	argv        []string
	argc        int
	command     *string
	commandPath *string
	fds         map[int]*os.File
}

const STDIN = 0
const STDOUT = 1
const STDERR = 2

const EXIT = "exit"
const ECHO = "echo"
const TYPE = "type"
const PWD = "pwd"
const CD = "cd"

var (
	builtins [5]string       = [5]string{EXIT, ECHO, TYPE, PWD, CD}
	fds      map[int]os.File = map[int]os.File{STDIN: *os.Stdin, STDOUT: *os.Stdout, STDERR: *os.Stderr}
)

func copy(s []string) []string {
	newS := []string{}
	for i := range s {
		newS = append(newS, s[i])
	}
	return newS
}

func NewCmd(tokens []string) (*Cmd, error) {
	cmd := &Cmd{
		argv:        nil,
		argc:        0,
		command:     nil,
		commandPath: nil,
	}
	cmd.fds = map[int]*os.File{}
	for k := range fds {
		v := fds[k]
		cmd.fds[k] = &v
	}

	argv, err := cmd.parse(tokens)
	if err != nil {
		return nil, err
	}
	cmd.argv = argv
	cmd.argc = len(cmd.argv)
	cmd.setCommandAndPath()
	return cmd, nil
}

func (cmd *Cmd) exec(ctx context.Context) {
	cmdC := exec.CommandContext(ctx, *cmd.command, cmd.argv[1:]...)
	// fmt.Printf("exec(); argv: %v; len(argv): %d\n", *cmd.argv, len(*cmd.argv))
	var out strings.Builder
	var stdErr strings.Builder
	cmdC.Stdout = &out
	cmdC.Stderr = &stdErr

	err := cmdC.Run()

	if out.Len() > 0 {
		fmt.Fprintf(cmd.fds[STDOUT], "%s\n", removeNewLinesIfPresent(out.String()))
	}
	if err != nil {
		fmt.Fprintf(cmd.fds[STDERR], "%s\n", removeNewLinesIfPresent(stdErr.String()))
	}
}

func (cmd *Cmd) redirectFd(fd int, filePath, op string) error {
	// TODO: do we validate the fd value?
	switch op {
	case ">":
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0644)
		if errors.As(err, &os.ErrExist) {
			return err
		}
		if err != nil {
			panic(err) // FIXME: any other relevant errors?
		}

		cmd.fds[fd] = file
	case ">|":
		file, err := os.OpenFile(filePath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			panic(err) // FIXME: any relevant errors?
		}

		cmd.fds[fd] = file
	case ">>":
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			panic(err) // FIXME: any relevant errors?
		}

		cmd.fds[fd] = file
	default:
		return UnknownOperatorErr
	}
	return nil
}

func (cmd *Cmd) parse(tokens []string) ([]string, error) {
	argv := []string{}

	var path string
	var fd = STDOUT

	redirectionOps := []string{">", ">|", ">>"}

	for _, op := range redirectionOps {
		if slices.Contains(tokens, op) {

			for i := 0; i < len(tokens); {
				arg := tokens[i]

				if v, err := strconv.Atoi(tokens[i]); err == nil && tokens[i+1] == op {
					fd = v
					i++
					continue
				}

				if arg == op {
					path = tokens[i+1]
					i = i + 2
					continue
				}
				argv = append(argv, arg)
				i++
			}

			err := cmd.redirectFd(fd, path, op)
			if err != nil {
				return nil, err
			}
			break // technically should be able process recursive derivatives of all ops
		}
	}

	if len(argv) == 0 {
		argv = copy(tokens)
	}
	return argv, nil
}

func (cmd *Cmd) getArgv() string {
	var sb strings.Builder
	for i, arg := range cmd.argv {
		sb.WriteString(arg)
		if i < cmd.argc-1 {
			sb.WriteString(" ")
		}
	}
	return sb.String()
}

func (cmd *Cmd) closeFds() {
	for i := range []int{STDIN, STDOUT, STDERR} {
		// fmt.Printf("fd: %d\n", i)
		v := fds[i]
		stat1, err1 := (&v).Stat()
		if err1 != nil {
			panic(err1)
		}
		stat2, err2 := cmd.fds[i].Stat()
		if err2 != nil {
			panic(err2)
		}
		// fmt.Printf("stat1: %s\n", stat1.Name())
		// fmt.Printf("stat2: %s\n", stat1.Name())
		if !os.SameFile(stat1, stat2) {
			err := cmd.fds[i].Close()
			if err != nil {
				panic(err)
			}
		}
	}

	for k := range fds {
		v := fds[k]
		cmd.fds[k] = &v
	}

	for k := range cmd.fds {
		if k > 2 {
			err := cmd.fds[k].Close()
			if err != nil {
				panic(err)
			}
			delete(cmd.fds, k)
		}
	}

}

func keys(dict map[int]*os.File) []int {
	keys := []int{}
	for k := range dict {
		keys = append(keys, k)
	}
	return keys
}

func values(dict map[int]*os.File) []*os.File {
	values := []*os.File{}
	for k := range dict {
		values = append(values, dict[k])
	}
	return values
}

func (cmd *Cmd) isBuiltin(str string) bool {
	for _, c := range builtins {
		if str == c {
			return true
		}
	}
	return false
}

func (cmd *Cmd) setCommandAndPath() {
	if cmd.argv == nil || len(cmd.argv) == 0 {
		return
	}
	c := cmd.argv[0]
	if builtin := cmd.isBuiltin(cmd.argv[0]); builtin {
		cmd.command = &c
		return
	}
	if path, err := exec.LookPath(c); err == nil {
		cmd.command = &c
		cmd.commandPath = &path
	}
}

func (cmd *Cmd) echo() {
	var sb strings.Builder
	for i := 1; i < cmd.argc; i++ {
		sb.WriteString(cmd.argv[i])
		if i < cmd.argc-1 {
			sb.WriteString(" ")
		} else {
			sb.WriteString("\n")
		}
	}
	fmt.Fprintf(cmd.fds[STDOUT], sb.String())
}

func (cmd *Cmd) exit(cancelShellCtx context.CancelFunc) error {
	if len(cmd.argv) != 2 {
		fmt.Fprintf(cmd.fds[STDERR], notFound(cmd.getArgv()))
		return errors.New(notFound(cmd.getArgv()))
	}
	v, err := strconv.Atoi(cmd.argv[1])
	if err != nil || v != 0 {
		fmt.Fprintf(cmd.fds[STDERR], notFound(cmd.getArgv()))
		return errors.New(notFound(cmd.getArgv()))
	}
	cancelShellCtx()
	return nil
}

func (cmd *Cmd) typeCommand() {
	for _, arg := range cmd.argv[1:] {
		if ok := cmd.isBuiltin(arg); ok {
			fmt.Fprintf(cmd.fds[STDOUT], fmt.Sprintf("%s is a shell builtin\n", arg))
			continue // this is different from bash for shell builtins
		}

		if path, err := exec.LookPath(arg); err == nil {
			fmt.Fprintf(cmd.fds[STDOUT], "%s is %s\n", arg, path)
		} else {
			fmt.Fprintf(cmd.fds[STDOUT], notFound(arg))
		}
	}
}

func (cmd *Cmd) pwd() {
	if path, err := os.Getwd(); err == nil {
		fmt.Fprintf(cmd.fds[STDOUT], "%s\n", path)
	}
}

func (cmd *Cmd) cd() {
	var absPath string
	path := cmd.argv[1]

	if invalidPath, err := regexp.Match(".*[\\.]{3,}.*", []byte(path)); err == nil && invalidPath {
		fmt.Fprintf(cmd.fds[STDERR], "cd: %s: No such file or directory\n", absPath)
		return
	}

	if strings.HasPrefix(path, "~") {
		home := os.Getenv("HOME")
		if len(path) == 0 {
			fmt.Fprintf(cmd.fds[STDERR], "Failed to access HOME environment variable\n")
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
			fmt.Fprintln(cmd.fds[STDERR], "Failed to print current working directory")
		}
		absPath = filepath.Join(cwd, path)
	}

	if err := os.Chdir(absPath); err != nil {
		fmt.Fprintf(cmd.fds[STDERR], "cd: %s: No such file or directory\n", absPath)
		return
	}

	// not sure we need to do this
	err := os.Setenv("PWD", absPath)
	if err != nil {
		panic(err)
	}
}
