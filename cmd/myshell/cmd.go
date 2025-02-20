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
	"strconv"
	"strings"
)

type Shell struct {
	// argv        []string
	// argc        int
	// command     *string
	// commandPath *string
	cmds    []*exec.Cmd
	builtin []token
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

func NewShell(sets [][]token, ctx context.Context) (*Shell, error) {
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
	if len(sets) == 1 {
		set := sets[0]
		if token := *set[0].tok; isBuiltin(token) {
			shell.builtin = set
			return shell, nil
		}
	}

	// validate commands
	if err := shell.validateCmds(sets); err != nil {
		return nil, err
	}

	execCmds := []*exec.Cmd{}
	for _, tokenSet := range sets {
		execCmd, err := initCmd(ctx, tokenSet)
		if err != nil {
			return nil, err
		}
		execCmds = append(execCmds, execCmd)
	}

	shell.cmds = execCmds
	return shell, nil
}

func redirectFd(redirectToken redirectOp, filePath string) (*os.File, error) {
	// TODO: do we validate the fd value?
	switch redirectToken.op {
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
	token := (*shell).builtin[0]
	if token.tok == nil {
		return errors.New(fmt.Sprintf("expected builtin, got %s", token.string()))
	}

	switch *token.tok {
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

func (shell *Shell) validateCmds(cmds [][]token) error {
	// 1. check path if exists
	// 2. set redirections if applicable
	// redirectionOps := []string{">", ">|", ">>"}

	for _, cmd := range cmds {
		bin := cmd[0].tok
		if bin == nil {
			return errors.New(fmt.Sprintf("expected binary name, got %s", cmd[0].string()))
		}
		_, err := exec.LookPath(*bin)
		if err != nil {
			return err
		}
	}

	return nil
}

func stringify(lst []token) string {
	var sb strings.Builder
	for i, arg := range lst {

		switch {
		case arg.isSimpleTok():
			sb.WriteString(*arg.tok)
		case arg.isRedirectOp():
			sb.WriteString(arg.redirectOp.op)
		}

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
	var path string
	var err error
	openFile := os.Stdout

	argv := []string{}
	for i := 0; i < len(cmd); {
		token := cmd[i]

		switch {
		case token.isRedirectOp():
			path = *cmd[i+1].tok
			openFile, err = redirectFd(*token.redirectOp, path) // ???????
			if err != nil {
				return
			}
			i = i + 2
		case token.isSimpleTok():
			argv = append(argv, *token.tok)
			i++
		}
	}

	for i := 1; i < len(argv); i++ {
		// fmt.Printf("%s\n", cmd[i].string())
		arg := argv[i]
		sb.WriteString(arg)
		if i < len(argv)-1 {
			sb.WriteString(" ")
		} else {
			sb.WriteString("\n")
		}
	}
	fmt.Fprintf(openFile, sb.String())
}

func (shell *Shell) exit() error {
	cmd := shell.builtin

	if len(cmd) != 2 {
		fmt.Fprintf(shell.fds[STDERR], notFound(stringify(cmd)))
		return NewNotFoundError(stringify(cmd))
	}
	exitStatus := *cmd[1].tok
	v, err := strconv.Atoi(exitStatus)

	if err != nil || v != 0 {
		fmt.Fprintf(shell.fds[STDERR], notFound(stringify(cmd)))
		return NewNotFoundError(stringify(cmd))
	}

	return nil
}

func (shell *Shell) typeCommand() {
	cmd := shell.builtin

	for _, token := range cmd[1:] {
		arg := *token.tok

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
	path := *cmd[1].tok

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

	for i := 0; i < len(tokens); {
		token := tokens[i]

		switch {
		case token.isRedirectOp():
			path = *tokens[i+1].tok
			openFile, err = redirectFd(*token.redirectOp, path) // ???????
			fd = token.redirectOp.fd
			if err != nil {
				// fmt.Printf(err.Error())
				return nil, err
			}
			i = i + 2
		case token.isSimpleTok():
			argv = append(argv, *token.tok)
			i++
		}
	}

	// fmt.Printf("argv: %v\n", argv)
	// fmt.Printf("len(argv): %d\n", len(argv))

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	if openFile != nil {
		switch fd {
		case STDIN:
			cmd.Stdin = openFile
		case STDOUT:
			cmd.Stdout = openFile
		case STDERR:
			cmd.Stderr = openFile
		}
	}
	return cmd, nil
}

func executeCmd(
	pr io.Reader,
	pw *io.PipeWriter,
	cmd *exec.Cmd,
	prevCmd *exec.Cmd,
	lastCmd bool,
) (error, *io.PipeWriter, *io.PipeReader) {

	if cmd.Stdin == nil && pr != nil {
		cmd.Stdin = pr
	}

	var nextPw *io.PipeWriter = nil
	var nextPr *io.PipeReader = nil
	if !lastCmd {
		nextPr, nextPw = io.Pipe()
		if cmd.Stdout == nil {
			cmd.Stdout = nextPw
		}
	} else {
		if cmd.Stdout == nil {
			cmd.Stdout = os.Stdout
		}
	}

	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return err, nil, nil
	}

	if prevCmd != nil {
		go func() {
			// TODO: lift errors
			prevCmd.Wait()
			if pw != nil {
				pw.Close()
			}
		}()
	}

	defer func() {
		if cmd.Stdout != os.Stdout {
			if file, ok := cmd.Stdout.(*os.File); ok {
				file.Close()
			}
		}
		if cmd.Stderr != os.Stderr {
			if file, ok := cmd.Stderr.(*os.File); ok {
				file.Close()
			}
		}
	}()

	return nil, nextPw, nextPr
}

func (shell *Shell) executeCmds() error {
	var pw *io.PipeWriter
	var pr io.Reader = nil
	var prevCmd *exec.Cmd

	for i, cmd := range (*shell).cmds {
		lastCmd := i+1 == len((*shell).cmds)
		err, nextPw, nextPr := executeCmd(pr, pw, cmd, prevCmd, lastCmd)
		if err != nil {
			return err
		}
		pw = nextPw
		pr = nextPr
		prevCmd = cmd
	}

	if prevCmd != nil {
		if err := prevCmd.Wait(); err != nil {
			return err
		}
	}
	return nil
}
