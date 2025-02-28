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

const (
	STDIN  = 0
	STDOUT = 1
	STDERR = 2
)

const (
	EXIT = "exit"
	ECHO = "echo"
	TYPE = "type"
	PWD  = "pwd"
	CD   = "cd"
)

var builtins [5]string = [5]string{EXIT, ECHO, TYPE, PWD, CD}

type Shell struct {
	cmds    []*exec.Cmd
	builtin []Token
}

func NewShell(sets [][]Token, ctx context.Context) (*Shell, error) {
	shell := &Shell{
		cmds:    nil,
		builtin: nil,
	}

	// TODO: refactor to be in the loop down below
	// check if builtin
	if len(sets) == 1 {
		set := sets[0]
		if token, ok := set[0].(*LiteralToken); ok {
			if isBuiltin(token.literal) {
				shell.builtin = set
				return shell, nil
			}
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

func redirectFd(redirectToken RedirectToken, filePath string) (*os.File, error) {
	// TODO: do we validate the fd value?
	switch redirectToken.op {
	case ">":
		if err := mkParentDirIfAbsent(filePath); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0644)
		if errors.Is(err, os.ErrExist) {
			return nil, err
		}
		if err != nil {
			panic(err) // FIXME: any other relevant errors?
		}

		return file, nil
	case ">|":
		if err := mkParentDirIfAbsent(filePath); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(filePath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			panic(err) // FIXME: any relevant errors?
		}

		return file, nil
	case ">>":
		file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(os.Stdout, "%s\n", err.Error())
				return nil, err
			} else {
				panic(err) // FIXME: any relevant errors?
			}
		}

		return file, nil
	default:
		return nil, UnknownOperatorErr
	}
}

func mkParentDirIfAbsent(path string) error {
	dir := filepath.Dir(path)
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(dir, 0750)
		if err != nil {
			return err
		}
	}
	return nil
}

func (shell *Shell) runBuiltin() error {
	token := (*shell).builtin[0]
	t, ok := token.(*LiteralToken)
	if !ok {
		return fmt.Errorf("expected builtin, got %s", token.String())
	}

	switch t.literal {
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
		if err := shell.cd(); err != nil {
			return err
		}
	}
	return nil
}

func (shell *Shell) validateCmds(cmds [][]Token) error {
	// 1. check path if exists
	// 2. set redirections if applicable
	// redirectionOps := []string{">", ">|", ">>"}

	for _, cmd := range cmds {
		tok, ok := cmd[0].(*LiteralToken)
		if !ok {
			return errors.New(fmt.Sprintf("expected binary name, got %s", stringify(cmd)))
		}
		bin := tok.literal
		_, err := exec.LookPath(bin)
		if err != nil {
			if errors.Is(err, exec.ErrDot) {
				return err
			} else {
				return NewNotFoundError(bin)
			}
		}
	}

	return nil
}

func stringify(lst []Token) string {
	var sb strings.Builder
	for i, arg := range lst {
		sb.WriteString(arg.String())

		if i < len(lst)-1 {
			sb.WriteString(" ")
		}
	}
	return sb.String()
}

func isBuiltin(str string) bool {
	for _, c := range builtins {
		if str == c {
			return true
		}
	}
	return false
}

func (shell *Shell) echo() {
	cmd := shell.builtin
	var sb strings.Builder
	var err error
	openFile := os.Stdout
	fd := STDOUT

	argv := []string{}
	for i := 0; i < len(cmd); {
		token := cmd[i]

		switch t := token.(type) {
		case *LiteralToken:
			argv = append(argv, t.literal)
			i++
		case *RedirectToken:
			pathTok, ok := cmd[i+1].(*LiteralToken)
			if !ok {
				fmt.Fprintf(os.Stderr, "Expected literalToken for path, got %s\n", cmd[i+1].String())
				return
			}
			openFile, err = redirectFd(*t, pathTok.literal)
			if err != nil {
				_ = openFile.Close()
				return
			}
			fd = t.fd
			i = i + 2
		}
	}

	for i := 1; i < len(argv); i++ {
		arg := argv[i]
		sb.WriteString(arg)
		if i < len(argv)-1 {
			sb.WriteString(" ")
		} else {
			sb.WriteString("\n")
		}
	}
	switch fd {
	case STDOUT:
		fmt.Fprintf(openFile, sb.String())
	case STDERR:
		fmt.Fprintf(openFile, "")
		fmt.Fprintf(os.Stdout, sb.String())
	default:
		fmt.Fprintf(openFile, sb.String())
	}
}

func (shell *Shell) exit() error {
	cmd := shell.builtin

	if len(cmd) != 2 {
		fmt.Fprintf(os.Stderr, "%s\n", notFound(stringify(cmd)))
		return NewNotFoundError(stringify(cmd))
	}
	t, ok := cmd[1].(*LiteralToken)
	if !ok {
		return fmt.Errorf("Expected int, got %s", cmd[1].String())
	}
	exitStatus, err := strconv.Atoi(t.literal)

	if err != nil || exitStatus != 0 {
		fmt.Fprintf(os.Stderr, "%s\n", notFound(stringify(cmd)))
		return NewNotFoundError(stringify(cmd))
	}

	return nil
}

func (shell *Shell) typeCommand() {
	cmd := shell.builtin

	for _, token := range cmd[1:] {
		t, ok := token.(*LiteralToken)
		if !ok {
			fmt.Fprintf(os.Stderr, "Expected literalToken as arg, got %s\n", token.String())
		}

		arg := t.literal
		if ok := isBuiltin(arg); ok {
			fmt.Fprintf(os.Stderr, fmt.Sprintf("%s is a shell builtin\n", arg))
			continue // this is different from bash for shell builtins
		}

		if path, err := exec.LookPath(arg); err == nil {
			fmt.Fprintf(os.Stderr, "%s is %s\n", arg, path)
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", notFound(arg))
		}
	}
}

func (cmd *Shell) pwd() {
	if path, err := os.Getwd(); err == nil {
		fmt.Fprintf(os.Stderr, "%s\n", path)
	}
}

func (shell *Shell) cd() error {
	var absPath string

	cmd := shell.builtin
	t, ok := cmd[1].(*LiteralToken)
	if !ok {
		fmt.Fprintf(os.Stderr, "Expected literalToken as arg, got %s\n", cmd[1].String())
	}

	path := t.literal
	if invalidPath, err := regexp.Match(".*[\\.]{3,}.*", []byte(path)); err == nil && invalidPath {
		return fmt.Errorf("cd: %s: No such file or directory", absPath)
	}

	if strings.HasPrefix(path, "~") {
		home := os.Getenv("HOME")
		if len(path) == 0 {
			return errors.New("Failed to access HOME environment variable")
		}
		path = filepath.Join(home, path[1:])
	}

	if filepath.IsAbs(path) {
		absPath = path
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			return errors.New("Failed to fetch current working directory")
		}
		absPath = filepath.Join(cwd, path)
	}

	if err := os.Chdir(absPath); err != nil {
		return fmt.Errorf("cd: %s: No such file or directory", absPath)
	}

	if err := os.Setenv("PWD", absPath); err != nil {
		return err
	}
	return nil
}

func initCmd(ctx context.Context, tokens []Token) (*exec.Cmd, error) {
	argv := []string{}
	var fd = STDOUT
	var err error
	var openFile *os.File

	for i := 0; i < len(tokens); {
		token := tokens[i]

		switch t := token.(type) {
		case *LiteralToken:
			argv = append(argv, t.literal)
			i++
		case *RedirectToken:
			pathTok, ok := tokens[i+1].(*LiteralToken)
			if !ok {
				return nil, fmt.Errorf("Expected literalToken for path, got %s\n", tokens[i+1].String())
			}

			openFile, err = redirectFd(*t, pathTok.literal)
			if err != nil {
				return nil, err
			}
			fd = t.fd
			i = i + 2
		}
	}

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)

	if openFile != nil {
		switch fd {
		case STDIN:
			cmd.Stdin = openFile
		case STDOUT:
			cmd.Stdout = openFile
		case STDERR:
			cmd.Stderr = openFile
		default:
			// TODO: handle redirection for non-std fds (also close after use!)
			cmd.ExtraFiles = append(cmd.ExtraFiles, openFile)
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
			_ = prevCmd.Wait()
			if pw != nil {
				_ = pw.Close()
			}
		}()
	}

	defer func() {
		if cmd.Stdout != os.Stdout {
			if file, ok := cmd.Stdout.(*os.File); ok {
				_ = file.Close()
			}
		}
		if cmd.Stderr != os.Stderr {
			if file, ok := cmd.Stderr.(*os.File); ok {
				_ = file.Close()
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
			var exitError *exec.ExitError
			if errors.Is(err, exitError) {
				return err
			}
		}
	}
	return nil
}
