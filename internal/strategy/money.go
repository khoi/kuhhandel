package strategy

import (
	"sort"

	"github.com/khoi/kuhhandel/internal/game"
)

type paymentOption struct {
	money game.Money
	total int
	cards int
}

func paymentAtLeast(available game.Money, minimum, maximum int) (game.Money, int, bool) {
	for _, option := range paymentOptions(available) {
		if option.total >= minimum && option.total <= maximum {
			return option.money, option.total, true
		}
	}
	return game.Money{}, 0, false
}

func paymentAtMost(available game.Money, maximum int) (game.Money, bool) {
	options := paymentOptions(available)
	for index := len(options) - 1; index >= 0; index-- {
		if options[index].total <= maximum {
			return options[index].money, true
		}
	}
	return game.Money{}, false
}

func smallestOffer(available game.Money) game.Money {
	switch {
	case available.Zero > 0:
		return game.Money{Zero: 1}
	case available.Ten > 0:
		return game.Money{Ten: 1}
	case available.Fifty > 0:
		return game.Money{Fifty: 1}
	case available.Hundred > 0:
		return game.Money{Hundred: 1}
	case available.TwoHundred > 0:
		return game.Money{TwoHundred: 1}
	default:
		return game.Money{FiveHundred: 1}
	}
}

func paymentOptions(available game.Money) []paymentOption {
	options := map[int]paymentOption{0: {}}
	denominations := []struct {
		value int
		count int
		add   func(game.Money, int) game.Money
	}{
		{10, available.Ten, func(money game.Money, count int) game.Money { money.Ten += count; return money }},
		{50, available.Fifty, func(money game.Money, count int) game.Money { money.Fifty += count; return money }},
		{100, available.Hundred, func(money game.Money, count int) game.Money { money.Hundred += count; return money }},
		{200, available.TwoHundred, func(money game.Money, count int) game.Money { money.TwoHundred += count; return money }},
		{500, available.FiveHundred, func(money game.Money, count int) game.Money { money.FiveHundred += count; return money }},
	}
	for _, denomination := range denominations {
		next := make(map[int]paymentOption, len(options)*(denomination.count+1))
		for total, option := range options {
			for count := 0; count <= denomination.count; count++ {
				candidate := paymentOption{
					money: denomination.add(option.money, count),
					total: total + denomination.value*count,
					cards: option.cards + count,
				}
				current, exists := next[candidate.total]
				if !exists || betterPayment(candidate, current) {
					next[candidate.total] = candidate
				}
			}
		}
		options = next
	}
	ordered := make([]paymentOption, 0, len(options)-1)
	for total, option := range options {
		if total > 0 {
			ordered = append(ordered, option)
		}
	}
	sort.Slice(ordered, func(first, second int) bool {
		return ordered[first].total < ordered[second].total
	})
	return ordered
}

func betterPayment(candidate, current paymentOption) bool {
	if candidate.cards != current.cards {
		return candidate.cards < current.cards
	}
	differences := [...]int{
		candidate.money.FiveHundred - current.money.FiveHundred,
		candidate.money.TwoHundred - current.money.TwoHundred,
		candidate.money.Hundred - current.money.Hundred,
		candidate.money.Fifty - current.money.Fifty,
		candidate.money.Ten - current.money.Ten,
	}
	for _, difference := range differences {
		if difference != 0 {
			return difference > 0
		}
	}
	return false
}

func moneyTotal(money game.Money) int {
	return money.Ten*10 + money.Fifty*50 + money.Hundred*100 + money.TwoHundred*200 + money.FiveHundred*500
}
