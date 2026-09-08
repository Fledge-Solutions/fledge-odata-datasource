package plugin

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fledge-solutions/fledge-odata-datasource/pkg/plugin/odata"
	"github.com/stretchr/testify/assert"
)

func TestMapFilter(t *testing.T) {
	tables := []struct {
		name             string
		filterConditions []filterCondition
		expected         string
	}{
		{
			name: "Time filter only",
			filterConditions: someFilterConditions(
				withFilterCondition(timeProp, "ge", aOneDayTimeRange().From.Format(time.RFC3339)),
				withFilterCondition(timeProp, "le", aOneDayTimeRange().To.Format(time.RFC3339)),
				int32Eq5),
			expected: "time ge '2022-04-21T12:30:50Z' and time le '2022-04-21T12:30:50Z' and int32 eq 5",
		},
		{
			name: "Time filter and int and string filter",
			filterConditions: someFilterConditions(
				withFilterCondition(timeProp, "ge", aOneDayTimeRange().From.Format(time.RFC3339)),
				withFilterCondition(timeProp, "le", aOneDayTimeRange().To.Format(time.RFC3339)),
				int32Eq5,
				withFilterCondition(stringProp, "eq", "Hello")),
			expected: "time ge '2022-04-21T12:30:50Z' and time le '2022-04-21T12:30:50Z' and int32 eq 5 and string eq 'Hello'",
		},
		{
			name: "Time filter and string filter",
			filterConditions: someFilterConditions(
				withFilterCondition(timeProp, "ge", aOneDayTimeRange().From.Format(time.RFC3339)),
				withFilterCondition(timeProp, "le", aOneDayTimeRange().To.Format(time.RFC3339)),
				withFilterCondition(stringProp, "eq", "")),
			expected: "time ge '2022-04-21T12:30:50Z' and time le '2022-04-21T12:30:50Z' and string eq ''",
		},
		{
			name:             "String filter only",
			filterConditions: someFilterConditions(withFilterCondition(stringProp, "eq", "")),
			expected:         "string eq ''",
		},
		{
			name: "DateTime filter - single quoted (e.g. SalesOrderLineStatuses.creation_date_time)",
			filterConditions: someFilterConditions(
				withFilterCondition(func(p *property) {
					p.Name = "creation_date_time"
					p.Type = odata.EdmDateTime
				}, "ge", "2024-01-01T00:00:00Z")),
			expected: "creation_date_time ge '2024-01-01T00:00:00Z'",
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			// Act
			var filterString = mapFilter(table.filterConditions)

			// Assert
			assert.Equal(t, table.expected, filterString)
		})
	}
}

func TestMapFilterAllValueSentinel(t *testing.T) {
	tables := []struct {
		name             string
		filterConditions []filterCondition
		expected         string
	}{
		{
			name: "allValue sentinel on eq is dropped entirely",
			filterConditions: someFilterConditions(
				withFilterCondition(int32Prop, "eq", allValueSentinel)),
			expected: "",
		},
		{
			name: "allValue sentinel combined with a real filter only keeps the real one",
			filterConditions: someFilterConditions(
				withFilterCondition(int32Prop, "eq", allValueSentinel),
				withFilterCondition(stringProp, "eq", "Hello")),
			expected: "string eq 'Hello'",
		},
		{
			name: "allValue sentinel on the in operator is dropped entirely",
			filterConditions: someFilterConditions(
				withFilterCondition(stringProp, "in", allValueSentinel)),
			expected: "",
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			assert.Equal(t, table.expected, mapFilter(table.filterConditions))
		})
	}
}

func TestMapFilterInOperator(t *testing.T) {
	// The "in" operator's value is a pre-formatted, already quoted-per-item
	// OData list literal built by the caller - mapFilter must use it
	// verbatim and must not wrap it in an extra pair of quotes (which would
	// double-quote it and break OData grammar), regardless of the
	// property's Edm type.
	filterConditions := someFilterConditions(
		withFilterCondition(stringProp, "in", "('ONBOARDING','MAATWERK')"))

	assert.Equal(t, "string in ('ONBOARDING','MAATWERK')", mapFilter(filterConditions))
}

func TestMapFilterNowSentinel(t *testing.T) {
	t.Run("Edm.Date resolves to a bare unquoted date literal", func(t *testing.T) {
		filterConditions := someFilterConditions(
			withFilterCondition(dateProp, "ge", nowValueSentinel))

		result := mapFilter(filterConditions)
		const prefix = "invoice_date ge "
		if !strings.HasPrefix(result, prefix) {
			t.Fatalf("expected result to start with %q, got %q", prefix, result)
		}
		dateLiteral := strings.TrimPrefix(result, prefix)
		assert.False(t, strings.Contains(dateLiteral, "'"), "Edm.Date literal must not be quoted, got %q", result)
		_, err := time.Parse("2006-01-02", dateLiteral)
		assert.NoError(t, err, "expected a bare YYYY-MM-DD literal, got %q", result)
	})

	t.Run("Edm.DateTimeOffset resolves to a quoted RFC3339 literal", func(t *testing.T) {
		filterConditions := someFilterConditions(
			withFilterCondition(timeProp, "ge", nowValueSentinel))

		result := mapFilter(filterConditions)
		matched := regexp.MustCompile(`^time ge '\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z'$`).MatchString(result)
		assert.True(t, matched, "expected a quoted RFC3339 literal, got %q", result)
	})
}

func TestBuildQueryUrl(t *testing.T) {
	tables := []struct {
		name             string
		baseUrl          string
		entitySet        string
		properties       []property
		expands          []expandProperty
		timeProperty     string
		timeRange        []filterCondition
		filterConditions []filterCondition
		expected         string
	}{
		{
			name:       "Success",
			baseUrl:    "http://localhost:5000",
			entitySet:  "Temperatures",
			properties: []property{aProperty(int32Prop), aProperty(timeProp)},
			filterConditions: someFilterConditions(
				withFilterCondition(timeProp, "ge", aOneDayTimeRange().From.Format(time.RFC3339)),
				withFilterCondition(timeProp, "le", aOneDayTimeRange().To.Format(time.RFC3339)),
				withFilterCondition(stringProp, "eq", "")),
			expected: "http://localhost:5000/Temperatures?%24filter=time+ge+%272022-04-21T12%3A30%3A50Z%27+and+time+le+%272022-04-21T12%3A30%3A50Z%27+and+string+eq+%27%27&%24select=int32%2Ctime",
		},
		{
			name:       "Nested expand with select properties",
			baseUrl:    "http://localhost:5000",
			entitySet:  "ServiceProductionLinks",
			properties: []property{{Name: "production_order_header_id", Type: odata.EdmString}},
			expands: []expandProperty{
				{
					Expand: navigationProperty{Name: "ProductionOrderHeader"},
					Properties: []property{
						{Name: "description_1", Type: odata.EdmString},
						{Name: "planning_date", Type: odata.EdmDate},
					},
				},
				{
					Expand: navigationProperty{Name: "ProductionOrderHeader/ProjectLeader"},
					Properties: []property{
						{Name: "first_name", Type: odata.EdmString},
						{Name: "prefix", Type: odata.EdmString},
						{Name: "name", Type: odata.EdmString},
					},
				},
			},
			filterConditions: []filterCondition{
				{Property: property{Name: "service_object_id", Type: odata.EdmString}, Operator: "eq", Value: "SO000002"},
			},
			expected: "http://localhost:5000/ServiceProductionLinks?%24expand=ProductionOrderHeader%28%24select%3Ddescription_1%2Cplanning_date%3B%24expand%3DProjectLeader%28%24select%3Dfirst_name%2Cprefix%2Cname%29%29&%24filter=service_object_id+eq+%27SO000002%27&%24select=production_order_header_id",
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			// Act
			var builtUrl, err = buildQueryUrl(table.baseUrl, table.entitySet, table.properties, table.expands, table.filterConditions, "+")

			// Assert
			assert.NoError(t, err)
			assert.Equal(t, table.expected, builtUrl.String())
		})
	}
}

func TestMapApply(t *testing.T) {
	tables := []struct {
		name             string
		groupBy          []property
		aggregates       []aggregateSpec
		filterConditions []filterCondition
		expected         string
	}{
		{
			name:       "single aggregate, no groupBy, no filter",
			groupBy:    nil,
			aggregates: []aggregateSpec{{Property: property{Name: "price"}, Function: "sum", Alias: "total"}},
			expected:   "aggregate(price with sum as total)",
		},
		{
			name:       "single aggregate with groupBy, no filter",
			groupBy:    []property{{Name: "order_date", Type: odata.EdmDate}},
			aggregates: []aggregateSpec{{Property: property{Name: "price_local_currency_net", Type: odata.EdmDecimal}, Function: "sum", Alias: "total"}},
			expected:   "groupby((order_date),aggregate(price_local_currency_net with sum as total))",
		},
		{
			name:    "multiple aggregates with multiple groupBy props, no filter",
			groupBy: []property{{Name: "order_date", Type: odata.EdmDate}, {Name: "debtor_id", Type: odata.EdmString}},
			aggregates: []aggregateSpec{
				{Property: property{Name: "price", Type: odata.EdmDecimal}, Function: "sum", Alias: "total"},
				{Property: property{Name: "quantity", Type: odata.EdmDecimal}, Function: "sum", Alias: "totalQuantity"},
			},
			expected: "groupby((order_date,debtor_id),aggregate(price with sum as total,quantity with sum as totalQuantity))",
		},
		{
			name:       "groupBy and aggregate with a filter condition",
			groupBy:    []property{{Name: "order_date", Type: odata.EdmDate}},
			aggregates: []aggregateSpec{{Property: property{Name: "price", Type: odata.EdmDecimal}, Function: "sum", Alias: "total"}},
			filterConditions: []filterCondition{
				{Property: property{Name: "order_date", Type: odata.EdmDate}, Operator: "ge", Value: "2026-01-01"},
			},
			expected: "filter(order_date ge 2026-01-01)/groupby((order_date),aggregate(price with sum as total))",
		},
		{
			name:             "empty filter conditions slice behaves the same as no filter",
			groupBy:          []property{{Name: "order_date", Type: odata.EdmDate}},
			aggregates:       []aggregateSpec{{Property: property{Name: "price", Type: odata.EdmDecimal}, Function: "sum", Alias: "total"}},
			filterConditions: []filterCondition{},
			expected:         "groupby((order_date),aggregate(price with sum as total))",
		},
		{
			name:       "no groupBy with a filter condition",
			groupBy:    nil,
			aggregates: []aggregateSpec{{Property: property{Name: "price"}, Function: "sum", Alias: "total"}},
			filterConditions: []filterCondition{
				{Property: property{Name: "debtor_id", Type: odata.EdmString}, Operator: "eq", Value: "12345"},
			},
			expected: "filter(debtor_id eq '12345')/aggregate(price with sum as total)",
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			assert.Equal(t, table.expected, mapApply(table.groupBy, table.aggregates, table.filterConditions))
		})
	}
}

func TestBuildApplyQueryUrl(t *testing.T) {
	t.Run("rejects empty aggregates", func(t *testing.T) {
		_, err := buildApplyQueryUrl("http://localhost:5000", "SalesOrderLines",
			[]property{{Name: "order_date", Type: odata.EdmDate}}, nil, nil, "+")
		assert.Error(t, err)
	})

	tables := []struct {
		name             string
		baseUrl          string
		entitySet        string
		groupBy          []property
		aggregates       []aggregateSpec
		filterConditions []filterCondition
		expected         string
	}{
		{
			name:      "groupby and aggregate, no filter",
			baseUrl:   "http://localhost:5000",
			entitySet: "SalesOrderLines",
			groupBy:   []property{{Name: "order_date", Type: odata.EdmDate}},
			aggregates: []aggregateSpec{
				{Property: property{Name: "price_local_currency_net", Type: odata.EdmDecimal}, Function: "sum", Alias: "total"},
			},
			expected: "http://localhost:5000/SalesOrderLines?%24apply=groupby%28%28order_date%29%2Caggregate%28price_local_currency_net+with+sum+as+total%29%29",
		},
		{
			name:      "groupby and aggregate with a filter condition",
			baseUrl:   "http://localhost:5000",
			entitySet: "SalesOrderLines",
			groupBy:   []property{{Name: "order_date", Type: odata.EdmDate}},
			aggregates: []aggregateSpec{
				{Property: property{Name: "price_local_currency_net", Type: odata.EdmDecimal}, Function: "sum", Alias: "total"},
			},
			filterConditions: []filterCondition{
				{Property: property{Name: "order_date", Type: odata.EdmDate}, Operator: "ge", Value: "2026-01-01"},
			},
			expected: "http://localhost:5000/SalesOrderLines?%24apply=filter%28order_date+ge+2026-01-01%29%2Fgroupby%28%28order_date%29%2Caggregate%28price_local_currency_net+with+sum+as+total%29%29",
		},
		{
			name:      "multiple aggregates, no groupBy, no filter",
			baseUrl:   "http://localhost:5000",
			entitySet: "SalesOrderLines",
			aggregates: []aggregateSpec{
				{Property: property{Name: "price", Type: odata.EdmDecimal}, Function: "sum", Alias: "total"},
				{Property: property{Name: "quantity", Type: odata.EdmDecimal}, Function: "sum", Alias: "totalQuantity"},
			},
			expected: "http://localhost:5000/SalesOrderLines?%24apply=aggregate%28price+with+sum+as+total%2Cquantity+with+sum+as+totalQuantity%29",
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			builtUrl, err := buildApplyQueryUrl(table.baseUrl, table.entitySet, table.groupBy, table.aggregates, table.filterConditions, "+")

			assert.NoError(t, err)
			assert.Equal(t, table.expected, builtUrl.String())
		})
	}
}

func TestGetEntities(t *testing.T) {
	tables := []struct {
		name             string
		expectedError    error
		expectedRespCode int
		handlerCallback  func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name:             "Success",
			expectedRespCode: 200,
			handlerCallback: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("{\"value\":[{\"hello\":\"world\"}]}"))
			},
		},
		{
			name:          "Server Timeout",
			expectedError: &url.Error{},
			handlerCallback: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(10 * time.Second)
			},
		},
		{
			name:             "Server 500 error",
			expectedRespCode: 500,
			handlerCallback: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			// Arrange
			client := GetOC("*", table.handlerCallback)

			// Act
			resp, err := client.Get(context.TODO(), "Temperatures", []property{aProperty(int32Prop)}, []expandProperty{}, someFilterConditions(int32Eq5))

			// Assert
			if table.expectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, table.expectedRespCode, resp.StatusCode)
			} else {
				assert.Error(t, err)
				assert.IsType(t, table.expectedError, err)
			}
		})
	}
}

func TestGetMetadata(t *testing.T) {
	tables := []struct {
		name             string
		expectedResult   odata.Edmx
		expectedRespCode int
		expectedError    error
		handlerCallback  func(w http.ResponseWriter, r *http.Request)
	}{
		{
			name:             "Success",
			expectedResult:   anOdataEdmx("4.0"),
			expectedError:    nil,
			expectedRespCode: 200,
			handlerCallback: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("<?xml version=\"1.0\" encoding=\"utf-8\"?><edmx:Edmx Version=\"4.0\" xmlns:edmx=\"https://docs.oasis-open.org/odata/ns/edmx\"></edmx:Edmx>"))
			},
		},
		{
			name:          "Server Timeout",
			expectedError: &url.Error{},
			handlerCallback: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(10 * time.Second)
			},
		},
		{
			name:             "Server 500 error",
			expectedRespCode: 500,
			handlerCallback: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			// Arrange
			client := GetOC("*", table.handlerCallback)

			// Act
			resp, err := client.GetMetadata(context.TODO())

			// Assert
			if table.expectedError == nil {
				assert.NoError(t, err)
				assert.Equal(t, table.expectedRespCode, resp.StatusCode)
			} else {
				assert.IsType(t, table.expectedError, err)
			}
		})
	}
}
