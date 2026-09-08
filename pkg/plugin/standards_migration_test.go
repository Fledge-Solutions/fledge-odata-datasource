package plugin

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"testing"
	"time"

	"github.com/fledge-solutions/fledge-odata-datasource/pkg/plugin/odata"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBuildQueryUrlUsesEntitySetPathForAdministrationFilter(t *testing.T) {
	builtURL, err := buildQueryUrl(
		"http://localhost:5000/odata",
		"SalesOrders",
		[]property{aProperty(int32Prop)},
		[]expandProperty{},
		someFilterConditions(
			withFilterCondition(func(p *property) {
				p.Name = "administration_id"
				p.Type = odata.EdmInt32
			}, "eq", "1"),
		),
		"+",
	)

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:5000/odata/SalesOrders?%24filter=administration_id+eq+1&%24select=int32", builtURL.String())
}

func TestGetMetadataDoesNotCreateVirtualEntitySets(t *testing.T) {
	respXML := anOdataEdmx("4.0",
		withDataService(
			withSchema("Fledge.Models",
				withEntityType("SalesOrder",
					withKey("administration_id"),
					withKey("id"),
					withProperty("administration_id", odata.EdmInt32),
					withProperty("id", odata.EdmInt32),
					withProperty("amount", odata.EdmDecimal)),
				withEntityContainer("Default"))),
	)

	body, _ := xml.Marshal(respXML)
	client := &clientMock{body: body, statusCode: 200}
	im := managerMock{}
	ds := ODataSource{&im}
	is := ODataSourceInstance{client}
	im.On("Get", context.TODO(), mock.Anything).Return(&is, nil)
	crs := callResourceResponseSenderMock{}

	err := ds.getMetadata(context.TODO(), &backend.CallResourceRequest{Path: "metadata"}, &crs)
	require.NoError(t, err)
	require.Equal(t, 200, crs.csr.Status)

	var resp schema
	require.NoError(t, json.Unmarshal(crs.csr.Body, &resp))
	assert.Empty(t, resp.EntitySets)
}

func TestCallResourceAdministrationsIsNotFound(t *testing.T) {
	ds := ODataSource{}
	crs := callResourceResponseSenderMock{}

	err := ds.CallResource(context.TODO(), &backend.CallResourceRequest{Path: "administrations"}, &crs)
	require.NoError(t, err)
	require.Equal(t, 404, crs.csr.Status)
}

func TestMapFilterStandardScenarios(t *testing.T) {
	tables := []struct {
		name             string
		filterConditions []filterCondition
		expected         string
	}{
		{
			name: "Administration ID filter stays a normal numeric filter",
			filterConditions: someFilterConditions(
				withFilterCondition(func(p *property) {
					p.Name = "administration_id"
					p.Type = odata.EdmInt32
				}, "eq", "1")),
			expected: "administration_id eq 1",
		},
		{
			name: "DateTimeOffset filter remains single quoted",
			filterConditions: someFilterConditions(
				withFilterCondition(timeProp, "ge", "2024-01-01T00:00:00Z")),
			expected: "time ge '2024-01-01T00:00:00Z'",
		},
		{
			name: "Date filter uses unquoted date literal",
			filterConditions: someFilterConditions(
				withFilterCondition(func(p *property) {
					p.Name = "order_date"
					p.Type = odata.EdmDate
				}, "ge", "2024-01-01")),
			expected: "order_date ge 2024-01-01",
		},
		{
			name: "Combined administration and time range filter",
			filterConditions: someFilterConditions(
				withFilterCondition(func(p *property) {
					p.Name = "administration_id"
					p.Type = odata.EdmInt32
				}, "eq", "5"),
				withFilterCondition(timeProp, "ge", aOneDayTimeRange().From.Format(time.RFC3339)),
				withFilterCondition(timeProp, "le", aOneDayTimeRange().To.Format(time.RFC3339))),
			expected: "administration_id eq 5 and time ge '2022-04-21T12:30:50Z' and time le '2022-04-21T12:30:50Z'",
		},
	}

	for _, table := range tables {
		t.Run(table.name, func(t *testing.T) {
			assert.Equal(t, table.expected, mapFilter(table.filterConditions))
		})
	}
}
