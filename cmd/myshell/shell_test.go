package main

import (
	"reflect"
	"testing"
)

func TestTokenization(t *testing.T) {
	tests := []struct {
		input          string
		expectedTokens []token
	}{
		{
			"\n", []token{},
		},
		{
			"echo Hello World!\n",
			[]token{newLiteralToken("echo"), newLiteralToken("Hello"), newLiteralToken("World!")},
		},
		{
			"echo    Hello     World!   \n",
			[]token{newLiteralToken("echo"), newLiteralToken("Hello"), newLiteralToken("World!")},
		},
		{
			"echo \"Hello World!\"\n",
			[]token{newLiteralToken("echo"), newLiteralToken("Hello World!")},
		},
		{
			"echo \\\"Hello World!\\\"\n",
			[]token{newLiteralToken("echo"), newLiteralToken("\"Hello"), newLiteralToken("World!\"")},
		},
		{
			"echo Hello > output.log\n",
			[]token{newLiteralToken("echo"), newLiteralToken("Hello"), newRedirectToken(">", 1), newLiteralToken("output.log")},
		},
		{
			"cat nonexistent 2> error.log\n",
			[]token{newLiteralToken("cat"), newLiteralToken("nonexistent"), newRedirectToken(">", 2), newLiteralToken("error.log")},
		},
		{
			"cat file | grep word \n",
			[]token{newLiteralToken("cat"), newLiteralToken("file"), newLiteralToken("|"), newLiteralToken("grep"), newLiteralToken("word")},
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

func testTokens(t *testing.T, testId int, tokens []token, expectedTs []token) {
	if len(tokens) != len(expectedTs) {
		t.Fatalf("%d: Expected %d tokens, got %d\n", testId, len(expectedTs), len(tokens))
	}
	for i := range tokens {
		tok := tokens[i]
		expT := expectedTs[i]

		if reflect.TypeOf(tok) != reflect.TypeOf(expT) {
			t.Fatalf("%d: expected token '%s' of type=%T, got '%s' of type %T\n",
				testId, expT.string(), expT, tok.string(), tok)
		}

		switch v := tok.(type) {
		case literalToken:
			expectedLiteralT := expT.(literalToken)

			if v.literal != expectedLiteralT.literal {
				t.Fatalf("%d: Expected tok.literal=%q, got %q\n", testId, v, expectedLiteralT)
			}

		case redirectToken:
			expRedirectT := expT.(redirectToken)

			if v.op != expRedirectT.op {
				t.Fatalf("%d: Expected tok.op=%q, got %q\n", testId, v.op, expRedirectT.op)
			}

			if v.fd != expRedirectT.fd {
				t.Fatalf("%d: Expected tok.fd=%d, got %d\n", testId, v.fd, expRedirectT.fd)
			}
		default:
			t.Fatalf("unexpected type: %q\n", v)
		}
	}
}

func TestSplitAtPipe(t *testing.T) {
	tests := []struct {
		input           string
		expectedTokSets [][]token
	}{
		{
			"echo hello | grep hello\n",
			[][]token{
				{newLiteralToken("echo"), newLiteralToken("hello")},
				{newLiteralToken("grep"), newLiteralToken("hello")},
			},
		},
		{
			"cat foo.bar | wc | grep 0\n",
			[][]token{
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
