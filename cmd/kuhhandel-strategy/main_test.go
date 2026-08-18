package main

import (
	"bytes"
	"math"
	"reflect"
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
		{"-samples", "0"},
	} {
		if err := run(&bytes.Buffer{}, arguments); err == nil {
			t.Fatalf("run(%v) succeeded", arguments)
		}
	}
}

func TestSearchConfigsAreDeterministicAndBounded(t *testing.T) {
	first := searchConfigs(64)
	second := searchConfigs(64)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("search configs are not deterministic")
	}
	for _, config := range first {
		if config.AuctionFraction < 0.3 || config.AuctionFraction >= 0.9 ||
			config.DenialFraction < 0 || config.DenialFraction >= 1 ||
			config.ReserveFraction < 0.1 || config.ReserveFraction >= 0.6 ||
			config.FirstRefusalFraction < 0.05 || config.FirstRefusalFraction >= 0.6 ||
			config.TradeFraction < 0.3 || config.TradeFraction >= 1.3 ||
			config.BluffChance < 0 || config.BluffChance >= 0.4 {
			t.Fatalf("config is out of bounds: %+v", config)
		}
	}
}

func TestRunReportsTopSearchParameters(t *testing.T) {
	var output bytes.Buffer
	err := run(&output, []string{
		"-suite", "search",
		"-samples", "2",
		"-opponents", "champions",
		"-opponent-policy", "three-champion",
		"-players", "3",
		"-games", "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	report := output.String()
	for _, text := range []string{"search-001", "search-002", "top search parameters", "auction", "denial", "reserve"} {
		if !strings.Contains(report, text) {
			t.Fatalf("report omits %q", text)
		}
	}
}

func TestComparisonTreatsSeatRotationsAsOneSeed(t *testing.T) {
	config := baseConfig()
	result, err := compare(config, config, 0, 3, 10, 25)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(result.winShare-1.0/3.0) > 1e-12 {
		t.Fatalf("win share = %f, want one third", result.winShare)
	}
	if result.winShareSE > 1e-12 {
		t.Fatalf("standard error = %f, want zero", result.winShareSE)
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
