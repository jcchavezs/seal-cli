package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestConfirm_Yes verifies the set of inputs we consider affirmative.
// We deliberately accept "y", "yes", any case, and surrounding whitespace -
// these are what real users type when answering quickly.
// Anything else (including empty input) must NOT count as yes; that invariant
// is the whole point of the "[y/N]" capitalisation convention.
func TestConfirm_Yes(t *testing.T) {
	cases := []string{
		"y\n",   // bare lowercase
		"Y\n",   // bare uppercase
		"yes\n", // word, lowercase
		"YES\n", // word, uppercase
		" y \n", // padded whitespace - common when the user is fast
	}
	for _, in := range cases {
		// stderr is captured but unused here; the affirmative-cases test only cares
		// about the return value.
		var stderr bytes.Buffer
		got := Confirm(strings.NewReader(in), &stderr, "Apply?")
		if !got {
			t.Errorf("input %q: expected true, got false", in)
		}
	}
}

// TestConfirm_No verifies the negative path, including the critical "bare
// Enter is no" rule. That rule exists because a user who hits
// Enter without thinking should NEVER trigger a destructive action;
// Garbage input ("wat") is also treated as no - fail-safe.
func TestConfirm_No(t *testing.T) {
	cases := []string{
		"n\n",   // explicit no, lowercase
		"N\n",   // explicit no, uppercase
		"no\n",  // word form
		"\n",    // bare Enter - the load-bearing case
		"wat\n", // anything unrecognised
	}
	for _, in := range cases {
		var stderr bytes.Buffer
		got := Confirm(strings.NewReader(in), &stderr, "Apply?")
		if got {
			t.Errorf("input %q: expected false, got true", in)
		}
	}
}

// TestConfirm_PromptText pins the exact string we write to stderr.
// It matters because the same wording appears in user-facing docs and CI
// logs; changing it silently would be a regression for any downstream tool
// that greps it. Format is "<prompt> [y/N] ".
func TestConfirm_PromptText(t *testing.T) {
	var stderr bytes.Buffer
	// Pass "n\n" so the function returns quickly; we only care about what gets
	// written to stderr.
	Confirm(strings.NewReader("n\n"), &stderr, "Apply?")
	want := "Apply? [y/N] "
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("missing prompt %q in stderr output: %q", want, stderr.String())
	}
}

// TestConfirm_EOFIsNo verifies an immediate EOF (e.g. a closed pipe,
// /dev/null on stdin) returns false. This is the contract the CLI relies on
// when stdin is redirected: no input ⇒ refuse the prompt, never silently
// proceed. Without this, `seal pin < /dev/null` would be a destructive
// footgun.
func TestConfirm_EOFIsNo(t *testing.T) {
	var stderr bytes.Buffer
	// An empty reader returns EOF on the first Scan() call.
	got := Confirm(strings.NewReader(""), &stderr, "Apply?")
	if got {
		t.Fatal("EOF input should be treated as no, got true")
	}
}