package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/khoi/kuhhandel/internal/strategy"
)

type comparison struct {
	winShare float64
	score    float64
	games    int
}

type ranking struct {
	name     string
	winShare float64
	score    float64
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
	suite := flags.String("suite", "archetypes", "challenger suite: archetypes, tuning, finalists, champions, or probes")
	opponentSuite := flags.String("opponents", "", "opponent suite; defaults to the challenger suite")
	policy := flags.String("policy", "", "one challenger policy from the selected suite")
	opponentPolicy := flags.String("opponent-policy", "", "one opponent policy from the selected suite")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if *games <= 0 {
		return fmt.Errorf("games must be positive")
	}
	if *players < 3 || *players > 5 {
		return fmt.Errorf("players must be between three and five")
	}
	configs, err := candidateConfigs(*suite)
	if err != nil {
		return err
	}
	configs, err = selectConfigs(configs, *policy)
	if err != nil {
		return err
	}
	opponents := configs
	if *opponentSuite != "" {
		opponents, err = candidateConfigs(*opponentSuite)
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
			result, err := compare(configs[challenger], opponents[opponent], challenger, opponent, len(opponents), *players, *games, *seed)
			if err != nil {
				return err
			}
			results[challenger][opponent] = result
		}
	}
	printMatrix(output, configs, opponents, results, *players)
	return nil
}

func compare(challenger, opponent strategy.HeuristicConfig, challengerIndex, opponentIndex, candidateCount, players, games int, seed uint64) (comparison, error) {
	result := comparison{}
	for gameIndex := range games {
		deckSeed := seed + uint64((challengerIndex*candidateCount+opponentIndex)*games+gameIndex)
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
			for _, winnerID := range played.WinnerIDs {
				if winnerID == playerID {
					result.winShare += 1 / float64(len(played.WinnerIDs))
				}
			}
			result.score += float64(played.Players[challengerSeat].Score)
			result.games++
		}
	}
	result.winShare /= float64(result.games)
	result.score /= float64(result.games)
	return result, nil
}

func printMatrix(output io.Writer, configs, opponents []strategy.HeuristicConfig, results [][]comparison, players int) {
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
			rankings[row].score += result.score
		}
		rankings[row].name = config.Name
		rankings[row].winShare /= float64(len(opponents))
		rankings[row].score /= float64(len(opponents))
		fmt.Fprintf(writer, "%.1f%%\n", rankings[row].winShare*100)
	}
	sort.Slice(rankings, func(first, second int) bool {
		return rankings[first].winShare > rankings[second].winShare
	})
	fmt.Fprintln(writer, "\nrank\tpolicy\twin share\tmean score")
	for index, result := range rankings {
		fmt.Fprintf(writer, "%d\t%s\t%.1f%%\t%.0f\n", index+1, result.name, result.winShare*100, result.score)
	}
	writer.Flush()
}
