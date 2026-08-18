package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/khoi/kuhhandel/internal/strategy"
)

type comparison struct {
	winShare       float64
	winShareSquare float64
	winShareSE     float64
	score          float64
	games          int
}

type ranking struct {
	config     strategy.HeuristicConfig
	winShare   float64
	winShareSE float64
	score      float64
}

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(output io.Writer, arguments []string) error {
	flags := flag.NewFlagSet("kuhhandel-strategy", flag.ContinueOnError)
	flags.SetOutput(output)
	games := flags.Int("games", 100, "games per seat and policy pairing")
	players := flags.Int("players", 3, "players per game")
	seed := flags.Uint64("seed", 1, "first shuffle seed")
	suite := flags.String("suite", "archetypes", "challenger suite: archetypes, tuning, finalists, champions, probes, or search")
	opponentSuite := flags.String("opponents", "", "opponent suite; defaults to the challenger suite")
	policy := flags.String("policy", "", "one challenger policy from the selected suite")
	opponentPolicy := flags.String("opponent-policy", "", "one opponent policy from the selected suite")
	samples := flags.Int("samples", 64, "policies in the search suite")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *games <= 0 {
		return fmt.Errorf("games must be positive")
	}
	if *players < 3 || *players > 5 {
		return fmt.Errorf("players must be between three and five")
	}
	if *samples <= 0 {
		return fmt.Errorf("samples must be positive")
	}
	configs, err := configsForSuite(*suite, *samples)
	if err != nil {
		return err
	}
	configs, err = selectConfigs(configs, *policy)
	if err != nil {
		return err
	}
	opponents := configs
	if *opponentSuite != "" {
		opponents, err = configsForSuite(*opponentSuite, *samples)
		if err != nil {
			return err
		}
	}
	opponents, err = selectConfigs(opponents, *opponentPolicy)
	if err != nil {
		return err
	}
	results := make([][]comparison, len(configs))
	for challenger := range configs {
		results[challenger] = make([]comparison, len(opponents))
		for opponent := range opponents {
			result, err := compare(configs[challenger], opponents[opponent], opponent, *players, *games, *seed)
			if err != nil {
				return err
			}
			results[challenger][opponent] = result
		}
	}
	rankings := printMatrix(output, configs, opponents, results, *players)
	if *suite == "search" {
		printParameters(output, rankings)
	}
	return nil
}

func compare(challenger, opponent strategy.HeuristicConfig, opponentIndex, players, games int, seed uint64) (comparison, error) {
	result := comparison{}
	for gameIndex := range games {
		deckSeed := seed + uint64(opponentIndex*games+gameIndex)
		seedWinShare := 0.0
		seedScore := 0.0
		for challengerSeat := range players {
			policies := make([]strategy.Policy, players)
			for seat := range players {
				config := opponent
				if seat == challengerSeat {
					config = challenger
				}
				policySeed := deckSeed*uint64(players+1) + uint64(seat+1)
				policies[seat] = strategy.NewHeuristic(config, policySeed)
			}
			played, err := strategy.Play(deckSeed, policies)
			if err != nil {
				return comparison{}, fmt.Errorf("%s against %s at seed %d seat %d: %w", challenger.Name, opponent.Name, deckSeed, challengerSeat, err)
			}
			playerID := played.Players[challengerSeat].ID
			winShare := 0.0
			for _, winnerID := range played.WinnerIDs {
				if winnerID == playerID {
					winShare = 1 / float64(len(played.WinnerIDs))
				}
			}
			seedWinShare += winShare / float64(players)
			seedScore += float64(played.Players[challengerSeat].Score) / float64(players)
		}
		result.winShare += seedWinShare
		result.winShareSquare += seedWinShare * seedWinShare
		result.score += seedScore
		result.games++
	}
	gamesPlayed := float64(result.games)
	result.winShare /= gamesPlayed
	if result.games > 1 {
		variance := (result.winShareSquare - gamesPlayed*result.winShare*result.winShare) / float64(result.games-1)
		result.winShareSE = math.Sqrt(math.Max(0, variance) / gamesPlayed)
	}
	result.score /= float64(result.games)
	return result, nil
}

func printMatrix(output io.Writer, configs, opponents []strategy.HeuristicConfig, results [][]comparison, players int) []ranking {
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintf(writer, "one row policy against %d-1 column policies\n\n", players)
	fmt.Fprint(writer, "policy\t")
	for _, config := range opponents {
		fmt.Fprintf(writer, "%s\t", config.Name)
	}
	fmt.Fprintln(writer, "mean")
	rankings := make([]ranking, len(configs))
	for row, config := range configs {
		fmt.Fprintf(writer, "%s\t", config.Name)
		for _, result := range results[row] {
			fmt.Fprintf(writer, "%.1f%%\t", result.winShare*100)
			rankings[row].winShare += result.winShare
			rankings[row].winShareSE += result.winShareSE * result.winShareSE
			rankings[row].score += result.score
		}
		rankings[row].config = config
		rankings[row].winShare /= float64(len(opponents))
		rankings[row].winShareSE = math.Sqrt(rankings[row].winShareSE) / float64(len(opponents))
		rankings[row].score /= float64(len(opponents))
		fmt.Fprintf(writer, "%.1f%%\n", rankings[row].winShare*100)
	}
	sort.Slice(rankings, func(first, second int) bool {
		return rankings[first].winShare > rankings[second].winShare
	})
	fmt.Fprintln(writer, "\nrank\tpolicy\twin share\tmean score")
	for index, result := range rankings {
		fmt.Fprintf(writer, "%d\t%s\t%.1f%% +/- %.1f%%\t%.0f\n", index+1, result.config.Name, result.winShare*100, result.winShareSE*100, result.score)
	}
	writer.Flush()
	return rankings
}

func printParameters(output io.Writer, rankings []ranking) {
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "\ntop search parameters")
	fmt.Fprintln(writer, "rank\tpolicy\tauction\tdenial\treserve\trefusal\ttrade\ttrade at\tbluff")
	limit := min(10, len(rankings))
	for index, result := range rankings[:limit] {
		config := result.config
		fmt.Fprintf(
			writer,
			"%d\t%s\t%.2f\t%.2f\t%.2f\t%.2f\t%.2f\t%d\t%.2f\n",
			index+1,
			config.Name,
			config.AuctionFraction,
			config.DenialFraction,
			config.ReserveFraction,
			config.FirstRefusalFraction,
			config.TradeFraction,
			config.TradeAtDeckRemaining,
			config.BluffChance,
		)
	}
	writer.Flush()
}
