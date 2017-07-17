package main

import (
	"math/rand"
	"testing"
)

func Test_genSleepTime(t *testing.T) {
	rand.Seed(0)
	testVals := make([]float64, 5)
	testVals[0] = 20
	testVals[1] = 1.281275096938293
	testVals[2] = 20
	testVals[3] = 0.5108671561337503
	testVals[4] = 14.863133989807169

	for i := 0; i < 5; i++ {
		val := testVals[i]
		testRes := genSleepTime(8)
		if val != testRes {
			t.Errorf("Expected %s, got %s.", val, testRes)
		}
	}

}
