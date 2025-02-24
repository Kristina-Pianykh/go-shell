package main

import (
	"fmt"
	"strconv"
	"strings"
)

type Parser struct {
	tokens       *[]token
	singleQuoted bool
	doubleQuoted bool
	pipeComplete bool
}

type token interface {
	tokenType()
	string() string
}

type literalToken struct {
	literal string
}

func (t *literalToken) tokenType() {}

func (t *literalToken) string() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("literalToken{literal: %s}", t.literal))
	return sb.String()
}

type redirectToken struct {
	op string
	fd int
}

func (t *redirectToken) tokenType() {}

func (t *redirectToken) string() string {
	var sb strings.Builder
	s := fmt.Sprintf("redirectOp{op: %s, fd: %d}", t.op, t.fd)
	sb.WriteString(s)
	return sb.String()
}

func newLiteralToken(s string) token {
	var t token
	t = &literalToken{s}
	return t
}

func newRedirectOpWithFd(op string, fd int) token {
	var t token
	t = &redirectToken{op: op, fd: fd}
	return t
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
	tokens := []token{}

	if p.tokens != nil {
		tokens = *p.tokens
	}
	return fmt.Sprintf("tokens: %v, singleQuoted: %v, doubleQuoted: %v, pipeComplete: %v", tokens, p.singleQuoted, p.doubleQuoted, p.pipeComplete)
}

func (p *Parser) parse(input string) ([]token, error) {
	if p.tokens == nil {
		p.tokens = &[]token{}
	}
	arg := []byte{}

	i := 0
	for {
		if i == len(input) {
			break
		}
		ch := input[i]
		switch ch {
		case '|':

			if len(*p.tokens) == 0 && len(arg) == 0 {
				return nil, NewUnexpectedTokenError("|")
			}

			if len(arg) > 0 {
				token := newLiteralToken(string(arg))
				*p.tokens = append(*p.tokens, token)
				arg = arg[:0]
			}

			token := newLiteralToken(("|"))
			*p.tokens = append(*p.tokens, token)
			p.pipeComplete = false
			i++

		case '>':
			// 2454>
			// 2454>|
			// >|
			// >

			if !p.doubleQuoted && !p.singleQuoted {

				fd := STDOUT
				if len(arg) > 0 {
					if num, err := strconv.Atoi(truncateLeadingZeros(string(arg))); err == nil {
						fd = num
						arg = arg[:0]
					}
				}

				if i+1 < len(input) { // should always be the case cause inputs ends with '\n' but just to be sure

					// FIXME: handle invalid syntax errors like '>>>' or '>>!' (what to do in last case?)

					var j int // next char after the op '>[|]' or '>>'
					if input[i+1] == '|' {
						token := newRedirectOpWithFd(">|", fd)
						*p.tokens = append(*p.tokens, token)
						j = i + 2
						i = i + 2

						// FIXME: can pipe follow '>>': '>>[|]'?
					} else if input[i+1] == '>' {
						token := newRedirectOpWithFd(">>", fd)
						*p.tokens = append(*p.tokens, token)
						j = i + 2
						i = i + 2
					} else {
						token := newRedirectOpWithFd(">", fd)
						*p.tokens = append(*p.tokens, token)
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
					token := newLiteralToken(string(arg))
					*p.tokens = append(*p.tokens, token)
				}
				arg = arg[:0]
			}
			i++

		case '\n', '\r':

			if p.singleQuoted || p.doubleQuoted {
				// prompt user for more input: incomplete/invalid echo command!
				arg = append(arg, ch)
				token := newLiteralToken(string(arg))
				*p.tokens = append(*p.tokens, token)
				arg = arg[:0]
				return nil, UnclosedQuoteErr

			} else if !p.singleQuoted && !p.doubleQuoted {
				// we are done
				if len(arg) > 0 {
					token := newLiteralToken(string(arg))
					*p.tokens = append(*p.tokens, token)
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
	if endsWithRedirectOp(*p.tokens) {
		return nil, fmt.Errorf("Broken redirect")
	}

	return *p.tokens, nil
}

func endsWithRedirectOp(tokens []token) bool {
	n := len(tokens)
	if n == 0 {
		return false
	}
	if _, ok := tokens[n-1].(*redirectToken); ok {
		return true
	}
	return false
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

func notFound(input string) string {
	input = removeNewLinesIfPresent(input)
	return fmt.Sprintf("%s: not found", input)
}

func removeNewLinesIfPresent(s string) string {
	if strings.HasSuffix(s, "\n") {
		return strings.TrimRight(s, "\n")
	}
	return s
}
