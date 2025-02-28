package main

import (
	"reflect"
	"testing"
)

func TestTokenization(t *testing.T) {
	tests := []struct {
		input          string
		expectedTokens []Token
	}{
		{
			"\n", []Token{},
		},
		{
			"echo Hello World!\n",
			[]Token{newLiteralToken("echo"), newLiteralToken("Hello"), newLiteralToken("World!")},
		},
		{
			"echo    Hello     World!   \n",
			[]Token{newLiteralToken("echo"), newLiteralToken("Hello"), newLiteralToken("World!")},
		},
		{
			"echo \"Hello World!\"\n",
			[]Token{newLiteralToken("echo"), newLiteralToken("Hello World!")},
		},
		{
			"echo \\\"Hello World!\\\"\n",
			[]Token{newLiteralToken("echo"), newLiteralToken("\"Hello"), newLiteralToken("World!\"")},
		},
		{
			"echo Hello > output.log\n",
			[]Token{newLiteralToken("echo"), newLiteralToken("Hello"), newRedirectToken(">", 1), newLiteralToken("output.log")},
		},
		{
			"cat nonexistent 2> error.log\n",
			[]Token{newLiteralToken("cat"), newLiteralToken("nonexistent"), newRedirectToken(">", 2), newLiteralToken("error.log")},
		},
		{
			"cat file | grep word \n",
			[]Token{newLiteralToken("cat"), newLiteralToken("file"), newLiteralToken("|"), newLiteralToken("grep"), newLiteralToken("word")},
		},
	}

	for i, tt := range tests {
		parser := newParser()
		err := parser.parse(tt.input)
		if err != nil {
			t.Fatal(err.Error())
		}
		testTokens(t, i, parser.tokens, tt.expectedTokens)
	}
}

func testTokens(t *testing.T, testId int, tokens []Token, expectedTs []Token) {
	if len(tokens) != len(expectedTs) {
		t.Fatalf("%d: Expected %d tokens, got %d\n", testId, len(expectedTs), len(tokens))
	}
	for i := range tokens {
		tok := tokens[i]
		expT := expectedTs[i]

		if reflect.TypeOf(tok) != reflect.TypeOf(expT) {
			t.Fatalf("%d: expected token '%s' of type=%T, got '%s' of type %T\n",
				testId, expT.String(), expT, tok.String(), tok)
		}

		switch expTok := expT.(type) {
		case *LiteralToken:
			literalTok, ok := tok.(*LiteralToken)
			if !ok {
				t.Fatalf("Expected *LiteralToken, got=%T\n", literalTok)
			}

			if literalTok.literal != expTok.literal {
				t.Fatalf("%d: Expected tok.literal=%q, got %q\n", testId, expTok, literalTok)
			}

		case *RedirectToken:
			redirectTok, ok := tok.(*RedirectToken)
			if !ok {
				t.Fatalf("Expected *RedirectToken, got=%T\n", redirectTok)
			}

			if expTok.op != redirectTok.op {
				t.Fatalf("%d: Expected tok.op=%q, got %q\n", testId, expTok.op, redirectTok.op)
			}

			if expTok.fd != redirectTok.fd {
				t.Fatalf("%d: Expected tok.fd=%d, got %d\n", testId, expTok.fd, redirectTok.fd)
			}
		default:
			t.Fatalf("unexpected type: %q\n", expTok)
		}
	}
}

func TestSplitAtPipe(t *testing.T) {
	tests := []struct {
		input           string
		expectedTokSets [][]Token
	}{
		{
			"echo hello | grep hello\n",
			[][]Token{
				{newLiteralToken("echo"), newLiteralToken("hello")},
				{newLiteralToken("grep"), newLiteralToken("hello")},
			},
		},
		{
			"cat foo.bar | wc | grep 0\n",
			[][]Token{
				{newLiteralToken("cat"), newLiteralToken("foo.bar")},
				{newLiteralToken("wc")},
				{newLiteralToken("grep"), newLiteralToken("0")},
			},
		},
	}

	for i, tt := range tests {
		parser := newParser()
		err := parser.parse(tt.input)
		if err != nil {
			t.Fatal(err.Error())
		}

		sets := splitAtPipe(parser.tokens)
		if len(sets) != len(tt.expectedTokSets) {
			t.Fatalf("%d: expected %d token sets in %s, got %d\n",
				i, len(tt.expectedTokSets), tt.input, len(sets))
		}

		for i := range sets {
			set := sets[i]
			expectedSet := tt.expectedTokSets[i]

			if len(set) != len(expectedSet) {
				t.Fatalf("%d: expected %d tokens in %s, got %d\n",
					i, len(tt.expectedTokSets), tt.input, len(sets))
			}

		}
	}
}
