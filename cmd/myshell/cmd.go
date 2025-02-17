package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Shell struct {
	// argv        []string
	// argc        int
	// command     *string
	// commandPath *string
	cmds    [][]token
	builtin []string
	fds     map[int]*os.File
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

func NewShell(cmds [][]string, ctx context.Context) (*Shell, error) {
	shell := &Shell{
		cmds:    nil,
		builtin: nil,
	}
	shell.fds = map[int]*os.File{}
	for k := range fds {
		v := fds[k]
		shell.fds[k] = &v
	}

	// check if builtin
	if len(cmds) == 1 {
		tokens := cmds[0]
		if isBuiltin(tokens[0]) {
			shell.builtin = tokens
			return shell, nil
		}
	}

	// validate commands
	validatedCmds, err := shell.parseCmds(cmds, ctx)
	if err != nil {
		return nil, err
	}
	shell.cmds = validatedCmds
	// shell.setCommandAndPath()
	return shell, nil
}

// func (shell *Shell) exec() {
// 	rec(shell.cmds, 0, nil, nil)
//
// 	cmdC := exec.CommandContext(ctx, *shell.command, shell.argv[1:]...)
// 	// fmt.Printf("exec(); argv: %v; len(argv): %d\n", *cmd.argv, len(*cmd.argv))
// 	var out strings.Builder
// 	var stdErr strings.Builder
// 	cmdC.Stdout = &out
// 	cmdC.Stderr = &stdErr
//
// 	err := cmdC.Run()
//
// 	if out.Len() > 0 {
// 		fmt.Fprintf(shell.fds[STDOUT], "%s\n", removeNewLinesIfPresent(out.String()))
// 	}
// 	if err != nil {
// 		fmt.Fprintf(shell.fds[STDERR], "%s\n", removeNewLinesIfPresent(stdErr.String()))
// 	}
// }

func redirectFd(fd int, filePath, op string) (*os.File, error) {
	// TODO: do we validate the fd value?
	switch op {
	case ">":
		mkParentDirIfAbsent(filePath)
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0644)
		if errors.Is(err, os.ErrExist) {
			// panic(err)
			return nil, err
		}
		if err != nil {
			panic(err) // FIXME: any other relevant errors?
		}

		return file, nil
		// cmd.fds[fd] = file
	case ">|":
		mkParentDirIfAbsent(filePath)
		file, err := os.OpenFile(filePath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			panic(err) // FIXME: any relevant errors?
		}

		// cmd.fds[fd] = file
		return file, nil
	case ">>":
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// fmt.Fprintf(cmd.fds[STDOUT], "%s\n", err.Error())
				fmt.Fprintf(os.Stdout, "%s\n", err.Error())
				return nil, err
			} else {
				panic(err) // FIXME: any relevant errors?
			}
		}

		// cmd.fds[fd] = file
		return file, nil
	default:
		return nil, UnknownOperatorErr
	}
}

func mkParentDirIfAbsent(path string) {
	dir := filepath.Dir(path)
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0750)
		if err != nil {
			panic(err)
		}
	}
}

func (shell *Shell) runBuiltin() error {
	switch (*shell).builtin[0] {
	case EXIT:
		// TODO: call original binary instead of doing builtin
		// graceful shutdown with cancel context instead of killing with no defers run
		if err := shell.exit(); err == nil {
			return ExitErr
		}
	case ECHO:
		shell.echo()
	case TYPE:
		shell.typeCommand()
	case PWD:
		shell.pwd()
	case CD:
		shell.cd()
	}
	return nil
}

func (shell *Shell) parseCmds(cmds [][]string, ctx context.Context) ([][]token, error) {
	// 1. check path if exists
	// 2. set redirections if applicable
	// redirectionOps := []string{">", ">|", ">>"}

	for _, tokens := range cmds {
		_, err := exec.LookPath(tokens[0])
		if err != nil {
			return nil, err
		}
	}

	// argv := []string{}
	// var fd = STDOUT
	// var openFile *os.File
	// var path string

	// for _, op := range redirectionOps {
	// 	if slices.Contains(tokens, op) {
	//
	// 		for i := 0; i < len(tokens); {
	// 			arg := tokens[i]
	//
	// 			if v, err := strconv.Atoi(tokens[i]); err == nil && tokens[i+1] == op {
	// 				fd = v
	// 				i++
	// 				continue
	// 			}
	//
	// 			if arg == op {
	// 				path = tokens[i+1]
	// 				i = i + 2
	// 				continue
	// 			}
	// 			argv = append(argv, arg)
	// 			i++
	// 		}
	//
	// 		openFile, err = redirectFd(fd, path, op) // ???????
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		break // technically should be able process recursive derivatives of all ops
	// 	}
	// }

	// 	var command *exec.Cmd
	// 	if openFile == nil {
	// 		argv = copy(tokens)
	// 	}
	// 	command = exec.CommandContext(ctx, argv[0], argv[1:]...)
	//
	// 	if openFile != nil { // no redirection
	// 		switch fd {
	// 		case STDIN:
	// 			command.Stdin = openFile
	// 		case STDOUT:
	// 			command.Stdout = openFile
	// 		case STDERR:
	// 			command.Stderr = openFile
	// 		default:
	// 			command.ExtraFiles = append(command.ExtraFiles, openFile)
	// 		}
	// 	}
	//
	// 	commands = append(commands, command)
	// }
	// fmt.Printf("commands: %v\n", commands)
	// return commands, nil
	return cmds, nil
}

// func (shell *Shell) parse(cmds [][]string) ([]string, error) {
// 	argv := []string{}
//
// 	var path string
// 	var fd = STDOUT
//
// 	redirectionOps := []string{">", ">|", ">>"}
//
// 	for _, op := range redirectionOps {
// 		if slices.Contains(tokens, op) {
//
// 			for i := 0; i < len(tokens); {
// 				arg := tokens[i]
//
// 				if v, err := strconv.Atoi(tokens[i]); err == nil && tokens[i+1] == op {
// 					fd = v
// 					i++
// 					continue
// 				}
//
// 				if arg == op {
// 					path = tokens[i+1]
// 					i = i + 2
// 					continue
// 				}
// 				argv = append(argv, arg)
// 				i++
// 			}
//
// 			err := shell.redirectFd(fd, path, op)
// 			if err != nil {
// 				return nil, err
// 			}
// 			break // technically should be able process recursive derivatives of all ops
// 		}
// 	}
//
// 	if len(argv) == 0 {
// 		argv = copy(tokens)
// 	}
// 	return argv, nil
// }

func stringify(lst []string) string {
	var sb strings.Builder
	for i, arg := range lst {
		sb.WriteString(arg)
		if i < len(lst)-1 {
			sb.WriteString(" ")
		}
	}
	return sb.String()
}

func (cmd *Shell) closeFds() {
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

func isBuiltin(str string) bool {
	for _, c := range builtins {
		if str == c {
			return true
		}
	}
	return false
}

// func (cmd *Shell) setCommandAndPath() {
// 	if cmd.argv == nil || len(cmd.argv) == 0 {
// 		return
// 	}
// 	c := cmd.argv[0]
// 	if builtin := cmd.isBuiltin(cmd.argv[0]); builtin {
// 		cmd.command = &c
// 		return
// 	}
// 	if path, err := exec.LookPath(c); err == nil {
// 		cmd.command = &c
// 		cmd.commandPath = &path
// 	}
// }

func (shell *Shell) echo() {
	cmd := shell.builtin
	var sb strings.Builder

	for i := 1; i < len(cmd); i++ {
		sb.WriteString(cmd[i])
		if i < len(cmd)-1 {
			sb.WriteString(" ")
		} else {
			sb.WriteString("\n")
		}
	}
	// fmt.Fprintf(shell.fds[STDOUT], sb.String())
	fmt.Fprintf(shell.fds[STDOUT], sb.String()) // where to????
}

func (shell *Shell) exit() error {
	cmd := shell.builtin
	if len(cmd) != 2 {
		fmt.Fprintf(shell.fds[STDERR], notFound(stringify(cmd)))
		return NewNotFoundError(stringify(cmd))
	}
	v, err := strconv.Atoi(cmd[1])
	if err != nil || v != 0 {
		fmt.Fprintf(shell.fds[STDERR], notFound(stringify(cmd)))
		return NewNotFoundError(stringify(cmd))
		// return errors.New(notFound(cmd.getArgv()))
	}
	return nil
}

func (shell *Shell) typeCommand() {
	cmd := shell.builtin
	for _, arg := range cmd[1:] {
		if ok := isBuiltin(arg); ok {
			fmt.Fprintf(shell.fds[STDOUT], fmt.Sprintf("%s is a shell builtin\n", arg))
			continue // this is different from bash for shell builtins
		}

		if path, err := exec.LookPath(arg); err == nil {
			fmt.Fprintf(shell.fds[STDOUT], "%s is %s\n", arg, path)
		} else {
			fmt.Fprintf(shell.fds[STDOUT], notFound(arg))
		}
	}
}

func (cmd *Shell) pwd() {
	if path, err := os.Getwd(); err == nil {
		fmt.Fprintf(cmd.fds[STDOUT], "%s\n", path)
	}
}

func (shell *Shell) cd() {
	cmd := shell.builtin
	var absPath string
	path := cmd[1]

	if invalidPath, err := regexp.Match(".*[\\.]{3,}.*", []byte(path)); err == nil && invalidPath {
		fmt.Fprintf(shell.fds[STDERR], "cd: %s: No such file or directory\n", absPath)
		return
	}

	if strings.HasPrefix(path, "~") {
		home := os.Getenv("HOME")
		if len(path) == 0 {
			fmt.Fprintf(shell.fds[STDERR], "Failed to access HOME environment variable\n")
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
			fmt.Fprintln(shell.fds[STDERR], "Failed to print current working directory")
		}
		absPath = filepath.Join(cwd, path)
	}

	if err := os.Chdir(absPath); err != nil {
		fmt.Fprintf(shell.fds[STDERR], "cd: %s: No such file or directory\n", absPath)
		return
	}

	// not sure we need to do this
	err := os.Setenv("PWD", absPath)
	if err != nil {
		panic(err)
	}
}

func initCmd(ctx context.Context, tokens []token) (*exec.Cmd, error) {
	argv := []string{}
	var fd = STDOUT
	var err error
	var openFile *os.File
	var path string

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

			openFile, err = redirectFd(fd, path, op) // ???????
			if err != nil {
				return nil, err
			}
			break // technically should be able process recursive derivatives of all ops
		}
	}
	fmt.Printf("argv: %v\n", argv)

	if openFile == nil {
		argv = copy(tokens)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	if openFile != nil { // no redirection
		switch fd {
		case STDIN:
			cmd.Stdin = openFile
		case STDOUT:
			cmd.Stdout = openFile
		case STDERR:
			cmd.Stderr = openFile
		default:
			cmd.ExtraFiles = append(cmd.ExtraFiles, openFile)
		}
	}
	return cmd, nil
}

func (shell *Shell) execute(ctx context.Context, idx int, out *io.ReadCloser, prevCmd *exec.Cmd) error {
	var stdout io.ReadCloser
	var err error

	if idx == len(shell.cmds) {
		if prevCmd != nil {
			if err := prevCmd.Wait(); err != nil {
				return err
			}
		}
		return nil
	}

	cmd, err := initCmd(ctx, (*shell).cmds[idx])
	if err != nil {
		return err
	}

	// fmt.Printf("command: %v\n", cmd)

	if cmd.Stdin != nil {
		// TODO: implement stdin redirection with `<`
	} else if cmd.Stdin == nil && out != nil {
		cmd.Stdin = *out
	}

	writers := []io.Writer{}
	if cmd.Stdout != nil {
		writers = append(writers, cmd.Stdout)
		// write to file
	}

	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}

	for _, file := range cmd.ExtraFiles {
		writers = append(writers, file)
	}

	if idx == len((*shell).cmds)-1 {
		if cmd.Stdout == nil {
			cmd.Stdout = os.Stdout
		}
	} else {
		stdout, err = cmd.StdoutPipe()
		if err != nil {
			panic(err)
		}
		if len(writers) > 0 { // remove if condition?
			cmd.Stdout = io.MultiWriter(writers...)
		}
	}

	if err := cmd.Start(); err != nil {
		panic(err)
	}

	// if prevCmd != nil {
	// 	prevCmd.Wait()
	// }

	return shell.execute(ctx, idx+1, &stdout, cmd)
}
