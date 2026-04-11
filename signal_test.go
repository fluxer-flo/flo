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

		repeatRemoval := func(h SignalListener[int]) {
			hCopy := h

			ok := h.Remove()
			if !ok {
				t.Errorf("expected first listener handle removal to return true")
			}

			ok = h.Remove()
			if ok {
				t.Errorf("expected second listener handle removal to return false")
			}

			ok = hCopy.Remove()
			if ok {
				t.Errorf("expected duplicated listener handle removal to return false")
			}
		}

		var outputs []int
		h := signal.OnSync(appender(&outputs))
		repeatRemoval(h)
		signal.OnSync(offset(appender(&outputs), 10))
		signal.OnSync(offset(appender(&outputs), 20))
		signal.OnSync(offset(appender(&outputs), 30))
		h2 := signal.OnSync(offset(appender(&outputs), 40))
		h3 := signal.OnSync(offset(appender(&outputs), 50))
		signal.OnSync(offset(appender(&outputs), 60))
		h4 := signal.OnSync(offset(appender(&outputs), 70))
		h5 := signal.OnSync(offset(appender(&outputs), 80))
		repeatRemoval(h2)
		repeatRemoval(h3)

		signal.emit(3)
		repeatRemoval(h4)
		signal.emit(2)
		repeatRemoval(h5)
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
