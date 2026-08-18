package game

type Animal string

const (
	AnimalRooster Animal = "rooster"
	AnimalGoose   Animal = "goose"
	AnimalCat     Animal = "cat"
	AnimalDog     Animal = "dog"
	AnimalSheep   Animal = "sheep"
	AnimalGoat    Animal = "goat"
	AnimalDonkey  Animal = "donkey"
	AnimalPig     Animal = "pig"
	AnimalCow     Animal = "cow"
	AnimalHorse   Animal = "horse"
)

var animals = []Animal{
	AnimalRooster,
	AnimalGoose,
	AnimalCat,
	AnimalDog,
	AnimalSheep,
	AnimalGoat,
	AnimalDonkey,
	AnimalPig,
	AnimalCow,
	AnimalHorse,
}

var animalValues = map[Animal]int{
	AnimalRooster: 10,
	AnimalGoose:   40,
	AnimalCat:     90,
	AnimalDog:     160,
	AnimalSheep:   250,
	AnimalGoat:    350,
	AnimalDonkey:  500,
	AnimalPig:     650,
	AnimalCow:     800,
	AnimalHorse:   1000,
}

func shuffledDeck(seed uint64) []Animal {
	deck := make([]Animal, 0, 40)
	for _, animal := range animals {
		for range 4 {
			deck = append(deck, animal)
		}
	}
	random := splitMix64{state: seed}
	for index := len(deck) - 1; index > 0; index-- {
		other := int(random.next() % uint64(index+1))
		deck[index], deck[other] = deck[other], deck[index]
	}
	return deck
}

func Score(owned map[Animal]int) int {
	total := 0
	completed := 0
	for animal, count := range owned {
		if count == 4 {
			total += animalValues[animal]
			completed++
		}
	}
	return total * completed
}

type splitMix64 struct {
	state uint64
}

func (random *splitMix64) next() uint64 {
	random.state += 0x9e3779b97f4a7c15
	value := random.state
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}
