package plugin

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
)

func TestTimeRangeToFilter(t *testing.T) {
	tables := []struct {
		name         string
		timeProperty *property
		timeRange    backend.TimeRange
		expected     []filterCondition
	}{
		{
			name: "Time property set",
			timeProperty: &property{
				Name: "time",
				Type: "Edm.DateTimeOffset",
			},
			timeRange: aOneDayTimeRange(),
			expected: someFilterConditions(
				withFilterCondition(timeProp, "ge", aOneDayTimeRange().From.Format(time.RFC3339)),
				withFilterCondition(timeProp, "le", aOneDayTimeRange().To.Format(time.RFC3339)),
			),
		},
		{
			name: "Date property set",
			timeProperty: &property{
				Name: "order_date",
				Type: "Edm.Date",
			},
			timeRange: aOneDayTimeRange(),
			expected: []filterCondition{
				{
					Property: property{
						Name: "order_date",
						Type: "Edm.Date",
					},
					Operator: "ge",
					Value:    aOneDayTimeRange().From.UTC().Format(time.DateOnly),
				},
				{
					Property: property{
						Name: "order_date",
						Type: "Edm.Date",
					},
					Operator: "le",
					Value:    aOneDayTimeRange().To.UTC().Format(time.DateOnly),
				},
			},
		},
		{
			name:         "No time property set",
			timeProperty: nil,
			timeRange:    aOneDayTimeRange(),
			expected:     []filterCondition{},
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			// Act
			result := TimeRangeToFilter(table.timeRange, table.timeProperty)

			// Assert
			assert.Equal(t, table.expected, result)
		})
	}
}
