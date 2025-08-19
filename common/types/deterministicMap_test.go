package types

import (
	"errors"
	"testing"
)

func TestNewDeterministicMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     map[string]string
		exLength int
	}{
		{
			name:     "EmptyMap",
			data:     map[string]string{},
			exLength: 0,
		},
		{
			name:     "NonEmptyMap",
			data:     map[string]string{"1": "a", "2": "b", "3": "c"},
			exLength: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := NewDeterministicMap(tt.data)
			if len(dm.data) != tt.exLength {
				t.Errorf("Expected length %d, got %d", tt.exLength, len(dm.data))
			}
		})
	}
}

func TestDeterministicMapEach(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     map[string]string
		exLength int
	}{
		{
			name:     "EmptyMap",
			data:     map[string]string{},
			exLength: 0,
		},
		{
			name:     "NonEmptyMap",
			data:     map[string]string{"1": "a", "2": "b", "3": "c"},
			exLength: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := NewDeterministicMap(tt.data)
			var count int
			err := dm.Each(func(key string, value string) error {
				count++
				return nil
			})
			if err != nil {
				t.Errorf("Expected error to be nil, got %v", err)
			}
			if count != tt.exLength {
				t.Errorf("Expected count %d, got %d", tt.exLength, count)
			}
		})
	}
}

func TestDeterministicMapEachError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		data     map[string]string
		exLength int
	}{
		{
			name:     "EmptyMap",
			data:     map[string]string{},
			exLength: 0,
		},
		{
			name:     "NonEmptyMap",
			data:     map[string]string{"1": "a", "2": "b", "3": "c"},
			exLength: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := NewDeterministicMap(tt.data)
			var count int
			var ErrTest = errors.New("ErrTest")
			err := dm.Each(func(key string, value string) error {
				count++
				return ErrTest
			})
			if tt.exLength == 0 {
				if err != nil {
					t.Errorf("Expected error to be nil, got %v", err)
				}
				if count != 0 {
					t.Errorf("Expected count 0, got %d", count)
				}
				return
			} else {
				if err != ErrTest {
					t.Errorf("Expected error to be ErrTest, got %v", err)
				}
				if count != 1 {
					t.Errorf("Expected count 1, got %d", count)
				}
			}
		})
	}
}

func TestDeterministicMapEach_Order(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		data         map[string]interface{}
		exKeyOrder   []string
		exValueOrder []interface{}
		exLength     int
		repeat       int
	}{
		{
			name:         "EmptyMap",
			data:         map[string]interface{}{},
			exKeyOrder:   make([]string, 0),
			exValueOrder: make([]interface{}, 0),
			exLength:     0,
			repeat:       1,
		},
		{
			name: "NonEmptyMap",
			data: map[string]interface{}{
				"key0": 0.6038107454499488,  // float64
				"key6": 0.3457760342363482,  // float64
				"key3": 0.6122646504299097,  // float64
				"key9": 0.3946332415496419,  // float64
				"key1": "value1",            // string
				"key4": "value4",            // string
				"key7": "value7",            // string
				"key2": 5577006791947779410, // int
				"key5": 8674665223082153551, // int
				"key8": 6129484611666145821, // int
			},
			exKeyOrder: []string{
				"key0", "key1", "key2", "key3", "key4", "key5", "key6", "key7", "key8", "key9",
			},
			exValueOrder: []interface{}{
				0.6038107454499488, "value1", 5577006791947779410, 0.6122646504299097, "value4",
				8674665223082153551, 0.3457760342363482, "value7", 6129484611666145821, 0.3946332415496419,
			},
			exLength: 10,
			// goland map is non deterministic, every time we run the test, the order of the map will be different
			// so we need to repeat the test to make sure the order is correct and not an accidental match
			repeat: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range make([]struct{}, tt.repeat) {
				dm := NewDeterministicMap(tt.data)
				var count int
				err := dm.Each(func(key string, value interface{}) error {
					if key != tt.exKeyOrder[count] {
						t.Errorf("Expected key %s, got %s", tt.exKeyOrder[count], key)
					}
					// ensures that the value is not lost or confused during execution. While keeping generic
					if value != tt.exValueOrder[count] {
						t.Errorf("Expected value %v, got %v", tt.exValueOrder[count], value)
					}

					count++
					return nil
				})
				if err != nil {
					t.Errorf("Expected error to be nil, got %v", err)
				}
				if count != tt.exLength {
					t.Errorf("Expected count %d, got %d", tt.exLength, count)
				}
			}
		})
	}
}

func TestDeterministicMapGetAt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		data      map[string]string
		index     int
		expectedV string
	}{
		{
			name:      "GetFirstElement",
			data:      map[string]string{"1": "a", "2": "b", "3": "c"},
			index:     0,
			expectedV: "a",
		},
		{
			name:      "GetMiddleElement",
			data:      map[string]string{"1": "a", "2": "b", "3": "c"},
			index:     1,
			expectedV: "b",
		},
		{
			name:      "GetLastElement",
			data:      map[string]string{"1": "a", "2": "b", "3": "c"},
			index:     2,
			expectedV: "c",
		},
		{
			name:      "GetLastElementNegativeIndex",
			data:      map[string]string{"1": "a", "2": "b", "3": "c"},
			index:     -1,
			expectedV: "c",
		},
		{
			name:      "GetElementOutOfBoundsPositive",
			data:      map[string]string{"1": "a", "2": "b", "3": "c"},
			index:     3,
			expectedV: "",
		},
		{
			name:      "GetElementOutOfBoundsNegative",
			data:      map[string]string{"1": "a", "2": "b", "3": "c"},
			index:     -4,
			expectedV: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := NewDeterministicMap(tt.data)
			value := dm.GetAt(tt.index)

			if value != tt.expectedV {
				t.Errorf("Expected value %s, got %s", tt.expectedV, value)
			}
		})
	}
}
