package plugin

import (
	"time"

	"github.com/fledge-solutions/fledge-odata-datasource/pkg/plugin/odata"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TimeRangeToFilter(timeRange backend.TimeRange, timeProperty *property) []filterCondition {
	if timeProperty == nil {
		return []filterCondition{}
	}

	fromValue := timeRange.From.UTC().Format(time.RFC3339)
	toValue := timeRange.To.UTC().Format(time.RFC3339)
	if timeProperty.Type == odata.EdmDate {
		fromValue = timeRange.From.UTC().Format(time.DateOnly)
		toValue = timeRange.To.UTC().Format(time.DateOnly)
	}

	return []filterCondition{
		{
			Property: *timeProperty,
			Operator: "ge",
			Value:    fromValue,
		},
		{
			Property: *timeProperty,
			Operator: "le",
			Value:    toValue,
		},
	}
}
