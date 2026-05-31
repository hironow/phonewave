package main

import "testing"

// white-box-reason: docgen normalization is an unexported implementation detail of the docgen command.
func TestNormalizeMarkdown_SeeAlsoSpacing(t *testing.T) {
	input := "* [phonewave](phonewave.md)\t - D-Mail courier daemon\n\n"

	got := normalizeMarkdown(input)

	want := "* [phonewave](phonewave.md)  - D-Mail courier daemon\n"
	if got != want {
		t.Fatalf("normalizeMarkdown() = %q, want %q", got, want)
	}
}
