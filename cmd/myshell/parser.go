package main

import (
	"fmt"
	"regexp"
	"strings"
)

type Parser struct {
	buffer       *string
	tokens       *[]string
	singleQuoted bool
	doubleQuoted bool
	pipeComplete bool
}

func newParser() *Parser {
	return &Parser{
		tokens:       nil,
		singleQuoted: false,
		doubleQuoted: false,
		pipeComplete: true,
	}
}

func (p *Parser) state() string {
	tokens := []string{}

	if p.tokens != nil {
		tokens = *p.tokens
	}
	return fmt.Sprintf("tokens: %v, singleQuoted: %v, doubleQuoted: %v, pipeComplete: %v", tokens, p.singleQuoted, p.doubleQuoted, p.pipeComplete)
}

func (p *Parser) parse(input string) ([]string, error) {
	if p.tokens == nil {
		p.tokens = &[]string{}
	}
	arg := []byte{}

	i := 0
	for {
		if i == len(input) {
			break
		}
		ch := input[i]
		// fmt.Printf("ch: %d\n", ch)
		switch ch {
		case '|':

			if len(*p.tokens) == 0 && len(arg) == 0 {
				return nil, NewUnexpectedTokenError("|")
			}

			if len(arg) > 0 {
				*p.tokens = append(*p.tokens, string(arg))
				arg = arg[:0]
			}

			*p.tokens = append(*p.tokens, "|")
			p.pipeComplete = false
			i++
			// fmt.Printf("arg: %s\n", arg)
			// fmt.Printf("tokens: %v\n", *p.tokens)

		case '>':
			// 2454>
			// 2454>|
			// >|
			// >

			if !p.doubleQuoted && !p.singleQuoted {
				if len(arg) > 0 && isNumber(string(arg)) {
					*p.tokens = append(*p.tokens, truncateLeadingZeros(string(arg)))
					arg = arg[:0]
				}

				if i+1 < len(input) { // should always be the case cause inputs ends with '\n' but just to be sure

					// FIXME: handle invalid syntax errors like '>>>' or '>>!' (what to do in last case?)

					var j int // next char after the op '>[|]' or '>>'
					if input[i+1] == '|' {
						*p.tokens = append(*p.tokens, ">|")
						j = i + 2
						i = i + 2

						// FIXME: can pipe follow '>>': '>>[|]'?
					} else if input[i+1] == '>' {
						*p.tokens = append(*p.tokens, ">>")
						j = i + 2
						i = i + 2
					} else {
						*p.tokens = append(*p.tokens, ">")
						j = i + 1
						i++
					}

					// check that '>[|]' is followed by something and doesn't end with '\n'
					foundNonWhiteSpaceCh := false
					for ; j < len(input); j++ {
						if j != ' ' && j != '\t' && j != '\n' {
							foundNonWhiteSpaceCh = true
						}
					}
					if !foundNonWhiteSpaceCh {
						return nil, NewUnexpectedTokenError("newline")
					}
				}

			} else if p.doubleQuoted || p.singleQuoted {
				arg = append(arg, ch)
				i++
			}

		case '\\':

			if !p.doubleQuoted && !p.singleQuoted && i+1 < len(input) {
				arg = append(arg, input[i+1])
				i = i + 2
			} else if p.doubleQuoted && !p.singleQuoted {
				if i+1 < len(input) && input[i+1] == '$' || input[i+1] == '`' || input[i+1] == '"' || input[i+1] == '\\' {
					arg = append(arg, input[i+1])
					i = i + 2
				} else {
					arg = append(arg, ch)
					i++
				}
			} else if !p.doubleQuoted && p.singleQuoted {
				arg = append(arg, ch)
				i++
			}

		case '\'':

			if p.doubleQuoted && !p.singleQuoted {
				arg = append(arg, ch)
			} else if !p.doubleQuoted && p.singleQuoted {
				p.singleQuoted = false
			} else if !p.doubleQuoted && !p.singleQuoted {
				p.singleQuoted = true
			}
			i++

		case '"':

			if !p.doubleQuoted && p.singleQuoted {
				arg = append(arg, ch)
			} else if p.doubleQuoted && !p.singleQuoted {
				p.doubleQuoted = false
			} else if !p.doubleQuoted && !p.singleQuoted {
				p.doubleQuoted = true
			}
			i++

		case ' ', '\t':

			if p.singleQuoted || p.doubleQuoted {
				arg = append(arg, ch)
			} else if !p.singleQuoted && !p.doubleQuoted {
				if len(arg) > 0 {
					*p.tokens = append(*p.tokens, string(arg))
				}
				arg = arg[:0]
			}
			i++

		case '\n', '\r':

			if p.singleQuoted || p.doubleQuoted {
				// prompt user for more input: incomplete/invalid echo command!
				arg = append(arg, ch)
				*p.tokens = append(*p.tokens, string(arg))
				arg = arg[:0]
				tmp := strings.Join(*p.tokens, " ")
				p.buffer = &tmp
				return nil, UnclosedQuoteErr

			} else if !p.singleQuoted && !p.doubleQuoted {
				// we are done
				// arg = append(arg, ch)
				if len(arg) > 0 {
					*p.tokens = append(*p.tokens, string(arg))
				}
				arg = arg[:0]
			}

			if !p.pipeComplete {
				return nil, PipeHasNoTargetErr
			}

			i++

		default:
			if !p.pipeComplete {
				p.pipeComplete = true
			}
			arg = append(arg, ch)
			i++
		}
	}

	// for i, arg := range *p.tokens {
	// 	fmt.Printf("arg #%d: %s\n", i, arg)
	// }
	return *p.tokens, nil
}

func truncateLeadingZeros(s string) string {
	if len(s) == 0 {
		return s
	}

	var sb strings.Builder
	leadingZeros := true

	for _, ch := range s {
		if ch == '0' && leadingZeros {
			continue
		}
		sb.WriteRune(ch)
		leadingZeros = false
	}
	return sb.String()
}

func isNumber(s string) bool {
	s = trim(s)
	leadingZero := true

	if len(s) == 0 {
		return false
	}
	for _, ch := range s {
		// leading zeros are truncated
		if len(s) > 1 && ch == '0' && leadingZero {
			continue
		}

		if !('0' <= ch && ch <= '9') {
			return false
		}
		leadingZero = false
	}
	return true
}

func notFound(input string) string {
	input = removeNewLinesIfPresent(input)
	return fmt.Sprintf("%s: not found\n", input)
}

func removeNewLinesIfPresent(s string) string {
	if strings.HasSuffix(s, "\n") {
		return strings.TrimRight(s, "\n")
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
