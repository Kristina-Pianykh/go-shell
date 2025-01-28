# Shell written in Go

## Why Reinvent the Wheel?

This is a fun little project for me to get more familiar with Go, its Stdlib and idiomatic use. So why the heck not?

## Supported Features

- Shell builtins: `echo`, `type`, `pwd`, `cd`
- File System navigation
- File descriptor redirection with `[fd]>[|]` and `>>`
- SIGINT handling for cancelling a currently running process or not yet entered input on `Ctrl+C`
- Autocomplete with `Tab` for shell builtins and executables on `PATH`

## Next Up:

- Piping
- Flashy interface
