package odata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapValueNumericTypes(t *testing.T) {
	tables := []struct {
		name         string
		value        interface{}
		propertyType string
		expected     interface{}
	}{
		{
			// OData v4 JSON requires Edm.Decimal to be serialized as a JSON
			// string to avoid precision loss - a spec-compliant server sends
			// this, not a plain JSON number.
			name:         "Decimal as JSON string (OData v4 spec-compliant server)",
			value:        "1234.56",
			propertyType: EdmDecimal,
			expected:     float64Ptr(1234.56),
		},
		{
			name:         "Decimal as plain float64 (lenient/mock server)",
			value:        1234.56,
			propertyType: EdmDecimal,
			expected:     float64Ptr(1234.56),
		},
		{
			name:         "Int64 as JSON string (OData v4 spec-compliant server)",
			value:        "123456789",
			propertyType: EdmInt64,
			expected:     int64Ptr(123456789),
		},
		{
			name:         "Int32 as plain float64",
			value:        float64(42),
			propertyType: EdmInt32,
			expected:     int32Ptr(42),
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			result := MapValue(table.value, table.propertyType)
			assert.Equal(t, table.expected, result)
		})
	}
}

func TestMapValueNumericGarbageDoesNotPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		result := MapValue("not-a-number", EdmDecimal)
		assert.Nil(t, result)
	})
}

func float64Ptr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64       { return &v }
func int32Ptr(v int32) *int32       { return &v }
