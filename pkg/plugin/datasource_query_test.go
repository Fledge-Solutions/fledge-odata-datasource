package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/fledge-solutions/fledge-odata-datasource/pkg/plugin/odata"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Just test if multiple queries lead to multiple responses
func TestQueryData(t *testing.T) {
	tables := []struct {
		name     string
		query    backend.QueryDataRequest
		expected backend.QueryDataResponse
	}{
		{
			name:     "Zero queries",
			query:    aQueryDataRequest(),
			expected: aQueryDataResponse(),
		},
		{
			name:     "One query",
			query:    aQueryDataRequest(withDataQuery("one", withQueryModel())),
			expected: aQueryDataResponse(withDataResponse("one", withDefaultTestFrame())),
		},
		{
			name:  "Two queries",
			query: aQueryDataRequest(withDataQuery("one", withQueryModel()), withDataQuery("two", withQueryModel())),
			expected: aQueryDataResponse(
				withDataResponse("one", withDefaultTestFrame()),
				withDataResponse("two", withDefaultTestFrame())),
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			// Arrange
			im := managerMock{}
			ds := ODataSource{&im}

			body, _ := json.Marshal(odata.Response{})
			client := clientMock{body: body}
			is := ODataSourceInstance{&client}
			im.On("Get", context.TODO(), mock.Anything).Return(&is, nil)

			// Act
			result, err := ds.QueryData(context.TODO(), &table.query)

			// Assert
			assert.NoError(t, err)
			assert.Equal(t, len(table.expected.Responses), len(result.Responses))
		})
	}
}

func TestQuery(t *testing.T) {
	tables := []struct {
		name              string
		mockODataResponse odata.Response
		query             backend.DataQuery
		expected          backend.DataResponse
	}{
		{
			name: "success simple",
			query: aDataQuery("defaultTestFrame", withQueryModel(withTimeProperty("time"),
				withFilterConditions(int32Eq5, withFilterCondition(stringProp, "eq", "Hello")),
				withProperties(int32Prop, booleanProp, stringProp))),
			mockODataResponse: anOdataResponse(withDefaultEntity()),
			expected:          aDataResponse(withDefaultTestFrame()),
		},
		{
			name: "success ordered",
			query: aDataQuery("defaultTestFrame", withQueryModel(withTimeProperty("time"),
				withProperties(int32Prop, booleanProp, stringProp))),
			mockODataResponse: anOdataResponse(
				withEntity(
					withProp("string", "Hello"),
					withProp("int32", 10.0),
					withProp("boolean", false),
					withProp("time", "2022-01-02T00:00:00Z")),
				withEntity(
					withProp("time", "2000-01-02T00:00:00Z"),
				),
				withEntity(
					withProp("time", "2010-01-02T00:00:00Z"),
					withProp("string", "World"),
				),
			),
			expected: aDataResponse(withBaseFrame("defaultTestFrame",
				withTimeField("time"),
				withField("int32", []*int32{}),
				withField("boolean", []*bool{}),
				withField("string", []*string{}),
				withRow(
					withRowValue(time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)),
					withRowValue(int32(10)),
					withRowValue(false),
					withRowValue("Hello"),
				),
				withRow(
					withRowValue(time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)),
					nil, nil, nil,
				),
				withRow(
					withRowValue(time.Date(2010, 1, 2, 0, 0, 0, 0, time.UTC)),
					nil, nil,
					withRowValue("World"),
				),
			)),
		},
		{
			name:  "success select time without time property",
			query: aDataQuery("defaultTestFrame", withQueryModel(withProperties(timeProp, int32Prop, booleanProp, stringProp))),
			mockODataResponse: anOdataResponse(
				withEntity(
					withProp("string", "Hello"),
					withProp("int32", 10.0),
					withProp("boolean", false),
					withProp("time", "2022-01-02T00:00:00Z")),
			),
			expected: aDataResponse(withBaseFrame("defaultTestFrame",
				withTimeField("time"),
				withField("int32", []*int32{}),
				withField("boolean", []*bool{}),
				withField("string", []*string{}),
				withRow(
					withRowValue(time.Date(2022, 1, 2, 0, 0, 0, 0, time.UTC)),
					withRowValue(int32(10)),
					withRowValue(false),
					withRowValue("Hello"),
				),
			)),
		},
		{
			name:  "success select no time property",
			query: aDataQuery("defaultTestFrame", withQueryModel(withProperties(int32Prop, booleanProp, stringProp))),
			mockODataResponse: anOdataResponse(
				withEntity(
					withProp("string", "Hello"),
					withProp("int32", 10.0),
					withProp("boolean", false)),
			),
			expected: aDataResponse(withBaseFrame("defaultTestFrame",
				withField("int32", []*int32{}),
				withField("boolean", []*bool{}),
				withField("string", []*string{}),
				withRow(
					withRowValue(int32(10)),
					withRowValue(false),
					withRowValue("Hello"),
				),
			)),
		},
		{
			name: "success expanded property",
			query: aDataQuery("defaultTestFrame", withQueryModel(
				withProperties(func(p *property) {
					p.Name = "production_order_header_id"
					p.Type = odata.EdmString
				}),
				func(qm *queryModel) {
					qm.Expands = []expandProperty{
						{
							Expand: navigationProperty{Name: "ProductionOrderHeader"},
							Properties: []property{
								{Name: "sales_order_header_id", Type: odata.EdmString},
							},
						},
					}
				},
			)),
			mockODataResponse: anOdataResponse(
				withEntity(
					withProp("production_order_header_id", "PO190101"),
					withProp("ProductionOrderHeader", []interface{}{map[string]interface{}{
						"sales_order_header_id": "VK180183",
					}})),
			),
			expected: aDataResponse(withBaseFrame("defaultTestFrame",
				withField("production_order_header_id", []*string{}),
				withField("ProductionOrderHeader.sales_order_header_id", []*string{}),
				withRow(
					withRowValue("PO190101"),
					withRowValue("VK180183"),
				),
			)),
		},
		{
			name:              "success minimal",
			query:             aDataQuery("baseFrame", withQueryModel()),
			mockODataResponse: anOdataResponse(),
			expected:          aDataResponse(),
		},
		{
			name:  "success select time property that does not exist",
			query: aDataQuery("defaultTestFrame", withQueryModel(withProperties(timeProp, int32Prop, booleanProp, stringProp))),
			mockODataResponse: anOdataResponse(
				withEntity(
					withProp("string", "Hello"),
					withProp("int32", 10.0),
					withProp("boolean", false),
					withProp("otherTimePropName", "2022-01-02T00:00:00Z")),
			),
			expected: aDataResponse(withBaseFrame("defaultTestFrame",
				withTimeField("time"),
				withField("int32", []*int32{}),
				withField("boolean", []*bool{}),
				withField("string", []*string{}),
				withRow(
					nil,
					withRowValue(int32(10)),
					withRowValue(false),
					withRowValue("Hello"),
				),
			)),
		},
		{
			name: "failure",
			query: aDataQuery("one", withQueryModel(withTimeProperty("time"),
				withFilterConditions(int32Eq5, withFilterCondition(stringProp, "eq", "Hello")),
				withProperties(int32Prop, booleanProp, stringProp))),
			mockODataResponse: anOdataResponse(),
			expected:          aDataResponse(withErrorResponse(errors.New("something went wrong"))),
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			// Arrange
			im := managerMock{}
			ds := ODataSource{&im}

			body, _ := json.Marshal(table.mockODataResponse)
			client := clientMock{
				body:       body,
				err:        table.expected.Error,
				statusCode: 200,
			}
			is := ODataSourceInstance{&client}
			im.On("Get", context.TODO(), mock.Anything).Return(&is, nil)

			// Act
			resp := ds.query(context.TODO(), &client, table.query)

			// Assert
			assert.Equal(t, table.expected, resp)
		})
	}
}

// A query with no effective server-side filter (e.g. every filterCondition
// resolved to the allValue sentinel) must not silently paginate through an
// entire large entity set - it should stop at maxPaginatedRows and surface
// a warning notice. This exercises the real *ODataClientImpl pagination
// path (via GetOC's httptest server), not clientMock, since
// addFiltersToNextLink/fetchNextLink require a concrete *ODataClientImpl.
func TestQueryPaginationCap(t *testing.T) {
	const pageSize = 2000
	callCount := 0

	handlerFn := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		values := make([]map[string]interface{}, pageSize)
		for i := range values {
			values[i] = map[string]interface{}{"int32": float64(callCount*pageSize + i)}
		}
		resp := odata.Response{
			Value:    values,
			NextLink: oc.baseUrl + "/Temperatures?$skiptoken=page" + strconv.Itoa(callCount),
		}
		body, _ := json.Marshal(resp)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}
	clientInstance := GetOC("*", handlerFn)

	ds := &ODataSource{}
	query := aDataQuery("capped", withQueryModel(withProperties(int32Prop)))

	// Act
	response := ds.query(context.TODO(), clientInstance, query)

	// Assert
	assert.NoError(t, response.Error)
	if assert.Len(t, response.Frames, 1) {
		frame := response.Frames[0]
		assert.Equal(t, maxPaginatedRows, frame.Fields[0].Len(), "result should be trimmed to exactly the cap")

		if assert.NotNil(t, frame.Meta) && assert.Len(t, frame.Meta.Notices, 1) {
			assert.Equal(t, data.NoticeSeverityWarning, frame.Meta.Notices[0].Severity)
			assert.Contains(t, frame.Meta.Notices[0].Text, "5000")
		}
	}
	// 1 initial request + 2 nextLink follows = 3 pages of 2000 rows each
	// (6000 total before the cap trims back down to 5000); a 4th page must
	// never be requested once the cap is hit mid-loop.
	assert.Equal(t, 3, callCount)
}

// len(qm.Aggregates) > 0 takes the $apply (groupby/aggregate) path instead
// of the raw-row path: no $select-based Properties/Expands, output
// properties synthesized from GroupBy + one field per aggregate alias, and
// the $apply response's flat rows are consumed with the same entry[prop.Name]
// lookup the raw-row path uses.
func TestQueryAggregate(t *testing.T) {
	query := aDataQuery("aggregateFrame", withQueryModel(
		withGroupBy(func(p *property) {
			p.Name = "order_date"
			p.Type = odata.EdmDate
		}),
		withAggregates(anAggregate(decimalProp, "sum", "total")),
	))

	mockResponse := anOdataResponse(
		withEntity(
			withProp("order_date", "2026-01-02"),
			withProp("total", 123.45),
		),
		withEntity(
			withProp("order_date", "2026-01-09"),
			withProp("total", 67.5),
		),
	)
	body, _ := json.Marshal(mockResponse)
	client := clientMock{body: body, statusCode: 200}

	ds := &ODataSource{}

	// Act
	resp := ds.query(context.TODO(), &client, query)

	// Assert
	expected := aDataResponse(withBaseFrame("aggregateFrame",
		withTimeField("order_date"),
		withField("total", []*float64{}),
		withRow(
			withRowValue(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
			withRowValue(123.45),
		),
		withRow(
			withRowValue(time.Date(2026, 1, 9, 0, 0, 0, 0, time.UTC)),
			withRowValue(67.5),
		),
	))
	assert.Equal(t, expected, resp)
}

// A query with no aggregates behaves exactly as before - GroupBy/Aggregates
// being absent (nil) is the default zero value, so an existing dashboard's
// query JSON (which never sets these fields) must take the raw-row path
// unchanged. This is a regression guard for backward compatibility.
func TestQueryAggregateBackwardCompatibility(t *testing.T) {
	query := aDataQuery("defaultTestFrame", withQueryModel(withTimeProperty("time"),
		withProperties(int32Prop, booleanProp, stringProp)))

	mockResponse := anOdataResponse(withDefaultEntity())
	body, _ := json.Marshal(mockResponse)
	client := clientMock{body: body, statusCode: 200}

	ds := &ODataSource{}

	resp := ds.query(context.TODO(), &client, query)

	assert.Equal(t, aDataResponse(withDefaultTestFrame()), resp)
}

func TestInvalidQueryModels(t *testing.T) {
	tables := []struct {
		name             string
		query            backend.DataQuery
		expectedErrorMsg string
	}{
		{
			name: "Invalid json",
			query: backend.DataQuery{
				JSON: []byte(`{`),
			},
			expectedErrorMsg: "error unmarshalling query json",
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			// Arrange
			im := managerMock{}
			ds := ODataSource{&im}

			client := clientMock{}
			is := ODataSourceInstance{&client}
			im.On("Get", context.TODO(), mock.Anything).Return(&is, nil)

			// Act
			resp := ds.query(context.TODO(), &client, table.query)

			// Assert
			assert.NotNil(t, resp.Error)
			assert.Contains(t, resp.Error.Error(), table.expectedErrorMsg)
		})
	}
}
