package main

import "fmt"

var (
	UnknownOperatorErr   = NewUnknownOperatorError()
	UnclosedQuoteErr     = NewUnclosedQuoteError()
	UnexpectedNewLineErr = NewUnexpectedNewLineError()
)

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

type unexpectedNewLine struct {
}

func (e *unexpectedNewLine) Error() string {
	return fmt.Sprintf("Unexpected 'newline'")
}

func NewUnexpectedNewLineError() error {
	return &unexpectedNewLine{}
}
