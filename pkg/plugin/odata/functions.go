package odata

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// ToArray maps OData property types to Grafana Field type
func ToArray(propertyType string) interface{} {
	switch propertyType {
	case EdmBoolean:
		return []*bool{}
	case EdmSingle:
		return []*float32{}
	case EdmDouble:
		return []*float64{}
	case EdmDecimal:
		return []*float64{}
	case EdmSByte:
		return []*int8{}
	case EdmByte:
		return []*uint8{}
	case EdmInt16:
		return []*int16{}
	case EdmInt32:
		return []*int32{}
	case EdmInt64:
		return []*int64{}
	case EdmDateTimeOffset, EdmDateTime:
		return []*time.Time{}
	case EdmDate:
		// EdmDate is date-only, but we still return time.Time for Grafana compatibility
		return []*time.Time{}
	default:
		return []*string{}
	}
}

// MapValue maps OData values to Grafana (Go) values
func MapValue(value interface{}, propertyType string) interface{} {
	if value == nil {
		return nil
	}
	switch propertyType {
	case EdmBoolean:
		boolValue := value.(bool)
		return &boolValue
	case EdmSingle, EdmDecimal, EdmDouble, EdmSByte, EdmByte, EdmInt16, EdmInt32, EdmInt64:
		numValue, err := toFloat64(value)
		if err != nil {
			log.DefaultLogger.Warn("failed to parse numeric value", "value", value, "propertyType", propertyType, "error", err)
			return nil
		}
		result, err := mapNumber(numValue, propertyType)
		if err != nil {
			return nil
		}
		return result
	case EdmDateTimeOffset, EdmDateTime, EdmDate:
		// All date/time types return time.Time, but EdmDate will be normalized to midnight UTC
		return parseDateTime(value, propertyType)
	default:
		x := fmt.Sprint(value)
		return &x
	}
}

// parseDateTime tries multiple date/time formats to parse OData date values
func parseDateTime(value interface{}, propertyType string) interface{} {
	if value == nil {
		return nil
	}

	dateStr := fmt.Sprint(value)
	if dateStr == "" {
		return nil
	}

	// Remove any quotes that might be present
	dateStr = strings.Trim(dateStr, "\"'")

	// List of date formats to try, in order of likelihood
	formats := []string{
		time.RFC3339Nano,           // 2006-01-02T15:04:05.999999999Z07:00
		time.RFC3339,               // 2006-01-02T15:04:05Z07:00
		"2006-01-02T15:04:05",      // Without timezone
		"2006-01-02T15:04:05.999",  // With milliseconds, no timezone
		"2006-01-02T15:04:05.999Z", // With milliseconds and Z
		"2006-01-02",               // Date only (for EdmDate)
		"2006-01-02 15:04:05",      // SQL-like format
		"2006-01-02 15:04:05.999",  // SQL-like with milliseconds
		"2006-01-02T15:04:05Z",     // ISO 8601 with Z
		"2006-01-02T15:04:05-07:00", // ISO 8601 with timezone
	}

	// Try each format
	for _, format := range formats {
		if timeValue, err := time.Parse(format, dateStr); err == nil {
			// For EdmDate (date-only fields), normalize to midnight UTC
			// This ensures consistent display in Grafana without timezone confusion
			if propertyType == EdmDate {
				timeValue = time.Date(timeValue.Year(), timeValue.Month(), timeValue.Day(), 0, 0, 0, 0, time.UTC)
				log.DefaultLogger.Debug("successfully parsed date (normalized to midnight UTC)",
					"value", dateStr,
					"format", format,
					"normalized", timeValue.Format(time.RFC3339),
					"propertyType", propertyType)
			} else {
				log.DefaultLogger.Debug("successfully parsed datetime",
					"value", dateStr,
					"format", format,
					"propertyType", propertyType)
			}
			return &timeValue
		}
	}

	// If all formats fail, log the error
	log.DefaultLogger.Warn("failed to parse date/time value",
		"value", dateStr,
		"propertyType", propertyType,
		"attemptedFormats", len(formats))

	return nil
}

// toFloat64 accepts both plain JSON numbers and the string-encoded numeric
// literals that OData v4 JSON requires for Edm.Decimal and Edm.Int64 (to
// avoid precision loss - see OData-JSON-Format section on IEEE754Compatible).
func toFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q as number: %w", v, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected numeric value type %T", value)
	}
}

func mapNumber(value float64, propertyType string) (interface{}, error) {
	switch propertyType {
	case EdmSingle:
		y := float32(value)
		return &y, nil
	case EdmDecimal, EdmDouble:
		return &value, nil
	case EdmSByte:
		y := int8(value)
		return &y, nil
	case EdmByte:
		y := uint8(value)
		return &y, nil
	case EdmInt16:
		y := int16(value)
		return &y, nil
	case EdmInt32:
		y := int32(value)
		return &y, nil
	case EdmInt64:
		y := int64(value)
		return &y, nil
	default:
		return nil, fmt.Errorf("unexpected property type: %s", propertyType)
	}
}
