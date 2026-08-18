package game

import "fmt"

type Money struct {
	Zero        int `json:"0"`
	Ten         int `json:"10"`
	Fifty       int `json:"50"`
	Hundred     int `json:"100"`
	TwoHundred  int `json:"200"`
	FiveHundred int `json:"500"`
}

func (money Money) valid() bool {
	return money.Zero >= 0 && money.Ten >= 0 && money.Fifty >= 0 && money.Hundred >= 0 && money.TwoHundred >= 0 && money.FiveHundred >= 0
}

func (money Money) contains(other Money) bool {
	return money.Zero >= other.Zero && money.Ten >= other.Ten && money.Fifty >= other.Fifty && money.Hundred >= other.Hundred && money.TwoHundred >= other.TwoHundred && money.FiveHundred >= other.FiveHundred
}

func (money Money) add(other Money) Money {
	return Money{
		Zero:        money.Zero + other.Zero,
		Ten:         money.Ten + other.Ten,
		Fifty:       money.Fifty + other.Fifty,
		Hundred:     money.Hundred + other.Hundred,
		TwoHundred:  money.TwoHundred + other.TwoHundred,
		FiveHundred: money.FiveHundred + other.FiveHundred,
	}
}

func (money Money) subtract(other Money) (Money, error) {
	if !other.valid() || !money.contains(other) {
		return Money{}, fmt.Errorf("insufficient money")
	}
	return Money{
		Zero:        money.Zero - other.Zero,
		Ten:         money.Ten - other.Ten,
		Fifty:       money.Fifty - other.Fifty,
		Hundred:     money.Hundred - other.Hundred,
		TwoHundred:  money.TwoHundred - other.TwoHundred,
		FiveHundred: money.FiveHundred - other.FiveHundred,
	}, nil
}

func (money Money) total() int {
	return money.Ten*10 + money.Fifty*50 + money.Hundred*100 + money.TwoHundred*200 + money.FiveHundred*500
}

func (money Money) cardCount() int {
	return money.Zero + money.Ten + money.Fifty + money.Hundred + money.TwoHundred + money.FiveHundred
}

func (money Money) withoutZero() Money {
	money.Zero = 0
	return money
}
