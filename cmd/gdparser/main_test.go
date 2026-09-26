package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunTreeFromStandardInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := run(nil, strings.NewReader("var answer: int = 42\n"), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "VariableDeclaration var answer") || !strings.Contains(stdout.String(), "Literal 42") {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := run([]string{"-format", "json"}, strings.NewReader("pass\n"), &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status %d: %s", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"kind": "KeywordStatement"`) {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunRejectsUnknownFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := run([]string{"-format", "yaml"}, strings.NewReader("pass\n"), &stdout, &stderr)
	if status != 2 || !strings.Contains(stderr.String(), "unknown format") {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
}
