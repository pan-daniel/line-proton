package sequence_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/pan-daniel/line-proton/internal/shared/sequence"
)

func TestForEachVisitsEveryItemAndJoinsFailures(test *testing.T) {
	firstFailure, secondFailure := errors.New("first"), errors.New("second")
	failuresByItem := map[int]error{1: firstFailure, 3: secondFailure}
	err := sequence.ForEach(slices.Values([]int{1, 2, 3}), func(item int) error {
		return failuresByItem[item]
	})
	if !errors.Is(err, firstFailure) || !errors.Is(err, secondFailure) {
		test.Errorf("err = %v, want both failures joined", err)
	}
}

func TestForEachSucceedsWhenNothingFails(test *testing.T) {
	err := sequence.ForEach(slices.Values([]string{"item"}), func(string) error { return nil })
	if err != nil {
		test.Errorf("err = %v, want nil", err)
	}
}
