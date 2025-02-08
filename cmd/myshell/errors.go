package main

import "fmt"

var (
	UnknownOperatorErr = NewUnknownOperatorError()
	UnclosedQuoteErr   = NewUnclosedQuoteError()
	// UnexpectedNewLineErr = NewUnexpectedTokenError()
	PipeHasNoTargetErr = NewPipeHasNoTargetError()
	ExitErr            = NewExitError()
	SignalInterruptErr = NewSignalInterruptError()
)

type SignalInterruptError struct{}

func (e *SignalInterruptError) Error() string {
	return "Signal interrupt"
}

func NewSignalInterruptError() error {
	return &SignalInterruptError{}
}

type ExitError struct{}

func (e *ExitError) Error() string {
	return "Exit error"
}

func NewExitError() error {
	return &ExitError{}
}

type UnknownOperatorError struct{}

func (e *UnknownOperatorError) Error() string {
	return "Unknown operator"
}

func NewUnknownOperatorError() error {
	return &UnknownOperatorError{}
}

type unclosedQuoteError struct{}

func (e *unclosedQuoteError) Error() string {
	return fmt.Sprintf("Unclosed quote")
}

func NewUnclosedQuoteError() error {
	return &unclosedQuoteError{}
}

type pipeHasNoTargetError struct{}

func (e *pipeHasNoTargetError) Error() string {
	return fmt.Sprintf("Pipe has no target")
}

func NewPipeHasNoTargetError() error {
	return &pipeHasNoTargetError{}
}

type unexpectedToken struct {
	token string
}

func (e *unexpectedToken) Error() string {
	return fmt.Sprintf("Unexpected token `%s`", e.token)
}

func NewUnexpectedTokenError(t string) error {
	return &unexpectedToken{t}
}
