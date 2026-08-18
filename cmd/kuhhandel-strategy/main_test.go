package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRejectsInvalidArguments(t *testing.T) {
	for _, arguments := range [][]string{
		{"-games", "0"},
		{"-players", "2"},
		{"-players", "6"},
		{"-suite", "missing"},
		{"-opponents", "missing"},
		{"-policy", "missing"},
		{"-opponent-policy", "missing"},
	} {
		if err := run(&bytes.Buffer{}, arguments); err == nil {
			t.Fatalf("run(%v) succeeded", arguments)
		}
	}
}

func TestRunReportsEveryPolicy(t *testing.T) {
	var output bytes.Buffer
	if err := run(&output, []string{"-games", "1", "-players", "3", "-seed", "8"}); err != nil {
		t.Fatal(err)
	}
	report := output.String()
	configs, err := candidateConfigs("archetypes")
	if err != nil {
		t.Fatal(err)
	}
	for _, config := range configs {
		if !strings.Contains(report, config.Name) {
			t.Fatalf("report omits %s", config.Name)
		}
	}
}
