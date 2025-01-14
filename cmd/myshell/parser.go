package main

import (
	"fmt"
	"regexp"
	"strings"
)

type Parser struct {
	currentInput *string
	buffer       *string
	// argc         int
	argv         *[]string
	singleQuoted bool
	doubleQuoted bool
}

func (p *Parser) clear() {
	p.currentInput = nil
	// p.argc = 0
	p.argv = nil
	p.singleQuoted = false
	p.doubleQuoted = false
}

func initParser() *Parser {
	return &Parser{
		currentInput: nil,
		// argc:         0,
		argv:         nil,
		singleQuoted: false,
		doubleQuoted: false,
	}
}

type unclosedQuoteError struct {
}

func (e *unclosedQuoteError) Error() string {
	return fmt.Sprintf("Unclosed quote")
}

func NewUnclosedQuoteError() error {
	return &unclosedQuoteError{}
}

var (
	unclosedQuoteErr = NewUnclosedQuoteError()
)

func (p *Parser) parse(input string) (*[]string, error) {
	if p.argv == nil {
		p.argv = &[]string{}
	}
	inputLeftTrimmed := strings.TrimLeft(input, " \n\t")

	arg := []byte{}

	i := 0
	for {
		if i == len(inputLeftTrimmed) {
			break
		}
		ch := inputLeftTrimmed[i]
		switch ch {
		case '\\':
			if !p.doubleQuoted && !p.singleQuoted {
				if i+1 < len(inputLeftTrimmed) {
					arg = append(arg, inputLeftTrimmed[i+1])
					i = i + 2
				}
			} else if p.doubleQuoted || p.singleQuoted {
				arg = append(arg, ch)
				i++
			}
			continue
		case '\'':

			if p.doubleQuoted && !p.singleQuoted {
				arg = append(arg, ch)
			} else if !p.doubleQuoted && p.singleQuoted {
				p.singleQuoted = false
			} else if !p.doubleQuoted && !p.singleQuoted {
				p.singleQuoted = true
			}
			i++
			continue

		case '"':

			if !p.doubleQuoted && p.singleQuoted {
				arg = append(arg, ch)
			} else if p.doubleQuoted && !p.singleQuoted {
				p.doubleQuoted = false
			} else if !p.doubleQuoted && !p.singleQuoted {
				p.doubleQuoted = true
			}
			i++
			continue

		case ' ', '\t':

			if p.singleQuoted || p.doubleQuoted {
				arg = append(arg, ch)
			} else if !p.singleQuoted && !p.doubleQuoted {
				if len(arg) > 0 {
					*p.argv = append(*p.argv, string(arg))
				}
				arg = arg[:0]
			}
			i++
			continue

		case '\n':

			if p.singleQuoted || p.doubleQuoted {
				// prompt user for more input: incomplete/invalid echo command!
				arg = append(arg, ch)
				*p.argv = append(*p.argv, string(arg))
				arg = arg[:0]
				tmp := strings.Join(*p.argv, " ")
				p.buffer = &tmp
				return nil, NewUnclosedQuoteError()

			} else if !p.singleQuoted && !p.doubleQuoted {
				// we are done
				// arg = append(arg, ch)
				if len(arg) > 0 {
					*p.argv = append(*p.argv, string(arg))
				}
				arg = arg[:0]
			}
			i++

		default:
			arg = append(arg, ch)
			i++
		}
	}

	return p.argv, nil
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

func removeMultipleWhitespaces(s []byte) []byte {
	multiSpace, err := regexp.Compile(" {2,}|\t+")
	if err != nil {
		panic(err)
	}
	return multiSpace.ReplaceAll(s, []byte{' '})
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

func trim(v string) string {
	return strings.TrimRight(strings.TrimLeft(v, " "), " ")
}
