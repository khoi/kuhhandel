package main

import (
	"fmt"

	"github.com/khoi/kuhhandel/internal/strategy"
)

func candidateConfigs(name string) ([]strategy.HeuristicConfig, error) {
	switch name {
	case "archetypes":
		return archetypeConfigs(), nil
	case "tuning":
		return tuningConfigs(), nil
	case "finalists":
		return finalistConfigs(), nil
	case "champions":
		return championConfigs(), nil
	case "probes":
		return probeConfigs(), nil
	default:
		return nil, fmt.Errorf("unknown policy suite %q", name)
	}
}

func selectConfigs(configs []strategy.HeuristicConfig, name string) ([]strategy.HeuristicConfig, error) {
	if name == "" {
		return configs, nil
	}
	for _, config := range configs {
		if config.Name == name {
			return []strategy.HeuristicConfig{config}, nil
		}
	}
	return nil, fmt.Errorf("unknown policy %q", name)
}

func archetypeConfigs() []strategy.HeuristicConfig {
	auctionFirst := baseConfig()
	auctionFirst.Name = "auction-first"
	return []strategy.HeuristicConfig{
		{
			Name: "cautious", AuctionFraction: 0.3, DenialFraction: 0.15, ReserveFraction: 0.65,
			FirstRefusalFraction: 0.4, TradeFraction: 0.45, TradeAtDeckRemaining: 0, BluffChance: 0.65,
		},
		{
			Name: "balanced", AuctionFraction: 0.5, DenialFraction: 0.35, ReserveFraction: 0.4,
			FirstRefusalFraction: 0.5, TradeFraction: 0.7, TradeAtDeckRemaining: 8, BluffChance: 0.4,
		},
		{
			Name: "collector", AuctionFraction: 0.65, DenialFraction: 0.1, ReserveFraction: 0.35,
			FirstRefusalFraction: 0.5, TradeFraction: 0.8, TradeAtDeckRemaining: 12, BluffChance: 0.35,
		},
		{
			Name: "denial", AuctionFraction: 0.5, DenialFraction: 0.8, ReserveFraction: 0.4,
			FirstRefusalFraction: 0.5, TradeFraction: 0.7, TradeAtDeckRemaining: 8, BluffChance: 0.4,
		},
		auctionFirst,
		{
			Name: "trader", AuctionFraction: 0.45, DenialFraction: 0.35, ReserveFraction: 0.5,
			FirstRefusalFraction: 0.5, TradeFraction: 0.9, TradeAtDeckRemaining: 40, BluffChance: 0.25,
		},
		{
			Name: "spender", AuctionFraction: 0.8, DenialFraction: 0.4, ReserveFraction: 0.15,
			FirstRefusalFraction: 0.6, TradeFraction: 1, TradeAtDeckRemaining: 40, BluffChance: 0.15,
		},
	}
}

func tuningConfigs() []strategy.HeuristicConfig {
	base := baseConfig()
	return []strategy.HeuristicConfig{
		base,
		variant(base, "auction-40", func(config *strategy.HeuristicConfig) { config.AuctionFraction = 0.4 }),
		variant(base, "auction-70", func(config *strategy.HeuristicConfig) { config.AuctionFraction = 0.7 }),
		variant(base, "denial-10", func(config *strategy.HeuristicConfig) { config.DenialFraction = 0.1 }),
		variant(base, "denial-65", func(config *strategy.HeuristicConfig) { config.DenialFraction = 0.65 }),
		variant(base, "reserve-20", func(config *strategy.HeuristicConfig) { config.ReserveFraction = 0.2 }),
		variant(base, "reserve-60", func(config *strategy.HeuristicConfig) { config.ReserveFraction = 0.6 }),
		variant(base, "refusal-35", func(config *strategy.HeuristicConfig) { config.FirstRefusalFraction = 0.35 }),
		variant(base, "refusal-70", func(config *strategy.HeuristicConfig) { config.FirstRefusalFraction = 0.7 }),
		variant(base, "trade-50", func(config *strategy.HeuristicConfig) { config.TradeFraction = 0.5 }),
		variant(base, "trade-100", func(config *strategy.HeuristicConfig) { config.TradeFraction = 1 }),
		variant(base, "trade-at-8", func(config *strategy.HeuristicConfig) { config.TradeAtDeckRemaining = 8 }),
		variant(base, "trade-at-40", func(config *strategy.HeuristicConfig) { config.TradeAtDeckRemaining = 40 }),
		variant(base, "bluff-10", func(config *strategy.HeuristicConfig) { config.BluffChance = 0.1 }),
		variant(base, "bluff-70", func(config *strategy.HeuristicConfig) { config.BluffChance = 0.7 }),
	}
}

func finalistConfigs() []strategy.HeuristicConfig {
	base := baseConfig()
	common := strategy.HeuristicConfig{
		Name: "common", AuctionFraction: 0.7, DenialFraction: 0.65, ReserveFraction: 0.2,
		FirstRefusalFraction: 0.35, TradeFraction: 0.75, TradeAtDeckRemaining: 0, BluffChance: 0.1,
	}
	return []strategy.HeuristicConfig{
		base,
		common,
		variant(common, "three-player", func(config *strategy.HeuristicConfig) { config.DenialFraction = 0.1 }),
		variant(common, "four-player", func(config *strategy.HeuristicConfig) { config.TradeFraction = 0.5 }),
		variant(common, "five-player", func(config *strategy.HeuristicConfig) { config.TradeFraction = 1 }),
		variant(common, "auction-60", func(config *strategy.HeuristicConfig) { config.AuctionFraction = 0.6 }),
		variant(common, "denial-35", func(config *strategy.HeuristicConfig) { config.DenialFraction = 0.35 }),
		variant(common, "reserve-30", func(config *strategy.HeuristicConfig) { config.ReserveFraction = 0.3 }),
		variant(common, "refusal-25", func(config *strategy.HeuristicConfig) { config.FirstRefusalFraction = 0.25 }),
		variant(common, "refusal-45", func(config *strategy.HeuristicConfig) { config.FirstRefusalFraction = 0.45 }),
		variant(common, "bluff-0", func(config *strategy.HeuristicConfig) { config.BluffChance = 0 }),
		variant(common, "bluff-20", func(config *strategy.HeuristicConfig) { config.BluffChance = 0.2 }),
	}
}

func baseConfig() strategy.HeuristicConfig {
	return strategy.HeuristicConfig{
		Name: "base", AuctionFraction: 0.55, DenialFraction: 0.35, ReserveFraction: 0.4,
		FirstRefusalFraction: 0.5, TradeFraction: 0.75, TradeAtDeckRemaining: 0, BluffChance: 0.4,
	}
}

func championConfigs() []strategy.HeuristicConfig {
	threePlayer := strategy.HeuristicConfig{
		Name: "three-champion", AuctionFraction: 0.6, DenialFraction: 0.35, ReserveFraction: 0.3,
		FirstRefusalFraction: 0.25, TradeFraction: 0.75, TradeAtDeckRemaining: 0, BluffChance: 0,
	}
	largeGame := variant(threePlayer, "large-champion", func(config *strategy.HeuristicConfig) {
		config.DenialFraction = 0.65
		config.FirstRefusalFraction = 0.35
		config.TradeFraction = 1
	})
	return []strategy.HeuristicConfig{threePlayer, largeGame}
}

func probeConfigs() []strategy.HeuristicConfig {
	champions := championConfigs()
	result := probes("small", champions[0])
	return append(result, probes("large", champions[1])...)
}

func probes(prefix string, base strategy.HeuristicConfig) []strategy.HeuristicConfig {
	base.Name = prefix
	return []strategy.HeuristicConfig{
		base,
		variant(base, prefix+"-auction-45", func(config *strategy.HeuristicConfig) { config.AuctionFraction = 0.45 }),
		variant(base, prefix+"-auction-75", func(config *strategy.HeuristicConfig) { config.AuctionFraction = 0.75 }),
		variant(base, prefix+"-denial-10", func(config *strategy.HeuristicConfig) { config.DenialFraction = 0.1 }),
		variant(base, prefix+"-denial-65", func(config *strategy.HeuristicConfig) { config.DenialFraction = 0.65 }),
		variant(base, prefix+"-reserve-15", func(config *strategy.HeuristicConfig) { config.ReserveFraction = 0.15 }),
		variant(base, prefix+"-reserve-45", func(config *strategy.HeuristicConfig) { config.ReserveFraction = 0.45 }),
		variant(base, prefix+"-refusal-10", func(config *strategy.HeuristicConfig) { config.FirstRefusalFraction = 0.1 }),
		variant(base, prefix+"-refusal-45", func(config *strategy.HeuristicConfig) { config.FirstRefusalFraction = 0.45 }),
		variant(base, prefix+"-trade-50", func(config *strategy.HeuristicConfig) { config.TradeFraction = 0.5 }),
		variant(base, prefix+"-trade-100", func(config *strategy.HeuristicConfig) { config.TradeFraction = 1 }),
		variant(base, prefix+"-trade-at-8", func(config *strategy.HeuristicConfig) { config.TradeAtDeckRemaining = 8 }),
		variant(base, prefix+"-bluff-10", func(config *strategy.HeuristicConfig) { config.BluffChance = 0.1 }),
		variant(base, prefix+"-bluff-30", func(config *strategy.HeuristicConfig) { config.BluffChance = 0.3 }),
	}
}

func variant(base strategy.HeuristicConfig, name string, change func(*strategy.HeuristicConfig)) strategy.HeuristicConfig {
	base.Name = name
	change(&base)
	return base
}
