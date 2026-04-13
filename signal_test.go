package flo

import (
	"slices"
	"testing"
)

func TestSignal(t *testing.T) {
	appender := func(out *[]int) func(int) {
		return func(i int) {
			*out = append(*out, i)
		}
	}
	offset := func(f func(i int), offset int) func(i int) {
		return func(i int) {
			f(i + offset)
		}
	}

	t.Run("basic", func(t *testing.T) {
		var signal Signal[int]

		var outputs []int
		signal.OnSync(appender(&outputs))
		signal.OnSync(appender(&outputs))

		signal.emit(3)
		signal.emit(2)
		signal.emit(1)

		expectedOutputs := []int{3, 3, 2, 2, 1, 1}
		if !slices.Equal(outputs, expectedOutputs) {
			t.Errorf("expected outputs to be %v but got %v", expectedOutputs, outputs)
		}
	})

	t.Run("listener removal", func(t *testing.T) {
		var signal Signal[int]

		var outputs []int
		rm := signal.OnSync(appender(&outputs))
		rm()
		rm()
		signal.OnSync(offset(appender(&outputs), 10))
		signal.OnSync(offset(appender(&outputs), 20))
		signal.OnSync(offset(appender(&outputs), 30))
		rm2 := signal.OnSync(offset(appender(&outputs), 40))
		rm3 := signal.OnSync(offset(appender(&outputs), 50))
		signal.OnSync(offset(appender(&outputs), 60))
		rm4 := signal.OnSync(offset(appender(&outputs), 70))
		rm5 := signal.OnSync(offset(appender(&outputs), 80))
		rm2()
		rm2()
		rm3()
		rm3()

		signal.emit(3)
		rm4()
		rm4()
		signal.emit(2)
		rm5()
		rm5()
		signal.emit(1)

		expectedOutputs := []int{
			13, 23, 33, 63, 73, 83,
			12, 22, 32, 62, 82,
			11, 21, 31, 61,
		}
		if !slices.Equal(outputs, expectedOutputs) {
			t.Errorf("expected outputs to be %v but got %v", expectedOutputs, outputs)
		}

	})

	t.Run("one time listeners", func(t *testing.T) {
		var signal Signal[int]

		var outputs []int
		signal.OnceSync(appender(&outputs))
		signal.OnSync(offset(appender(&outputs), 10))
		signal.OnceSync(offset(appender(&outputs), 20))
		signal.OnceSync(offset(appender(&outputs), 30))
		signal.OnSync(offset(appender(&outputs), 40))
		signal.OnceSync(offset(appender(&outputs), 50))

		signal.emit(6)
		signal.emit(7)
		signal.emit(8)

		expectedOutputs := []int{
			6, 16, 26, 36, 46, 56,
			17, 47,
			18, 48,
		}
		if !slices.Equal(outputs, expectedOutputs) {
			t.Errorf("expected outputs to be %v but got %v", expectedOutputs, outputs)
		}
	})
}
