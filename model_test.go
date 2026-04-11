package flo_test

import (
	"testing"
	"time"

	"github.com/fluxer-flo/flo"
)

func TestIDTimestamp(t *testing.T) {
	t.Run("correct ID timestamp is reported", func(t *testing.T) {
		id := flo.ID(1427764813854588940)
		expected := time.UnixMilli(1760476058210)

		if !id.CreatedAt().Equal(expected) {
			t.Errorf("for ID %d: expected timestamp %s but got %s", id, expected, id.CreatedAt())
		}
	})

	t.Run("generated ID has correct timestamp", func(t *testing.T) {
		timestamp := time.UnixMilli(time.Now().UnixMilli())
		id := flo.NewID(timestamp)

		if !id.CreatedAt().Equal(timestamp) {
			t.Errorf("for generated ID %d: expected timestamp %s but got %s", id, timestamp, id.CreatedAt())
		}
	})
}

func TestCollection(t *testing.T) {
	expect := func(t *testing.T, collection *flo.Collection[string], expected map[flo.ID]string) {
		if collection.Len() != len(expected) {
			t.Errorf("expected %d items but got %d items", len(expected), collection.Len())
		}

		for key, expectedValue := range expected {
			value, ok := collection.Get(key)
			if !ok {
				t.Errorf("expected collection to contain %d", key)
				continue
			}

			if value != expectedValue {
				t.Errorf("expected Get(%d) to be %s but got %s", key, expectedValue, value)
			}
		}
		
	}

	t.Run("limit 0", func(t *testing.T) {
		var collection flo.Collection[string]
		collection.Set(123, "")
		collection.Set(456, "")

		if collection.Len() != 0 {
			t.Error("collection has items after calling Set")
		}
	})

	t.Run("limit 1", func(t *testing.T) {
		collection := flo.NewCollection[string](1)
		collection.Set(123, "foo")
		collection.Set(456, "bar")
		
		expect(t, &collection, map[flo.ID]string{
			456: "bar",
		})
		
		collection.Delete(456)
		expect(t, &collection, map[flo.ID]string{})
	})

	t.Run("limit 2", func(t *testing.T) {
		collection := flo.NewCollection[string](2)
		collection.Set(123, "fee")
		collection.Set(456, "fi")

		collection.Get(123)

		collection.Set(789, "fo")

		expect(t, &collection, map[flo.ID]string{
			123: "fee",
			789: "fo",
		})

		collection.Delete(123)

		expect(t, &collection, map[flo.ID]string{
			789: "fo",
		})
	})
}
