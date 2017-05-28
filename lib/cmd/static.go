package cmd

import (
	"math/rand"
	"sync"
	"time"
)

var (
	random  *rand.Rand
	getRand sync.Once

	getLocation sync.Once

	now    time.Time
	getNow sync.Once
)

func GetRand() *rand.Rand {
	getRand.Do(func() {
		random = rand.New(rand.NewSource(time.Now().UnixNano()))
	})
	return random
}

func GetLocation() *time.Location {
	getLocation.Do(func() {
		loc, _ := time.LoadLocation(GetFlags().Location)
		time.Local = loc
	})
	return time.Local
}

func Now() time.Time {
	getNow.Do(func() {
		timeString := GetFlags().Now
		if len(timeString) < 1 {
			now = time.Now()
		} else {
			t, _ := time.ParseInLocation("2006-01-02 15:04:05.999999999", timeString, GetLocation())
			now = t
		}
	})
	return now
}