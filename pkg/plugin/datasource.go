package plugin

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/fledge-solutions/fledge-odata-datasource/pkg/plugin/odata"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

var (
	_ backend.QueryDataHandler    = (*ODataSource)(nil)
	_ backend.CheckHealthHandler  = (*ODataSource)(nil)
	_ backend.CallResourceHandler = (*ODataSource)(nil)
)

// maxPaginatedRows caps how many rows a single query will pull across
// @odata.nextLink pagination. Without a cap, a query with no effective
// server-side filter (e.g. every filterCondition resolved to the allValue
// sentinel) can silently walk an entire large entity set page by page -
// confirmed against SalesInvoiceLines, which has 56k+ rows across 600+
// pages. Past the cap we stop and return what's been fetched so far, with
// a warning notice telling the user to narrow their filter.
const maxPaginatedRows = 5000

type ODataSource struct {
	im instancemgmt.InstanceManager
}

type DatasourceSettings struct {
	URLSpaceEncoding string `json:"urlSpaceEncoding"`
	OAuth2Enabled    bool   `json:"oauth2Enabled"`
	OAuth2Url        string `json:"oauth2Url"`
	OauthPassThru    bool   `json:"oauthPassThru"`
}

func newDatasourceInstance(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	var dsSettings DatasourceSettings
	if len(settings.JSONData) > 1 {
		if err := json.Unmarshal(settings.JSONData, &dsSettings); err != nil {
			return nil, err
		}
	}

	clientOptions, err := settings.HTTPClientOptions(ctx)
	if err != nil {
		return nil, err
	}

	if dsSettings.OauthPassThru {
		clientOptions.ForwardHTTPHeaders = true
	}

	client, err := httpclient.New(clientOptions)
	if err != nil {
		return nil, err
	}

	// Get OAuth2 API key from secure settings
	oauth2ApiKey := settings.DecryptedSecureJSONData["oauth2ApiKey"]

	return &ODataSourceInstance{
		&ODataClientImpl{
			httpClient:       client,
			baseUrl:          settings.URL,
			urlSpaceEncoding: dsSettings.URLSpaceEncoding,
			oauth2Enabled:    dsSettings.OAuth2Enabled,
			oauth2Url:        dsSettings.OAuth2Url,
			oauth2ApiKey:     oauth2ApiKey,
		},
	}, nil
}

type ODataSourceInstance struct {
	client ODataClient
}

func NewODataSource(ctx context.Context, _ backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	im := datasource.NewInstanceManager(newDatasourceInstance)
	ds := &ODataSource{
		im: im,
	}
	return ds, nil
}

func (ds *ODataSource) getClientInstance(ctx context.Context, pluginContext backend.PluginContext) (ODataClient, error) {
	instance, err := ds.im.Get(ctx, pluginContext)
	if err != nil {
		return nil, err
	}
	clientInstance := instance.(*ODataSourceInstance).client
	return clientInstance, nil
}

func (ds *ODataSource) logTokenStatus(h http.Header) {
	if log.DefaultLogger.Level() <= log.Debug {
		auth := strings.ToLower(h.Get(backend.OAuthIdentityTokenHeaderName))
		bearerToken := strings.HasPrefix(auth, "bearer ") && strings.TrimSpace(auth[7:]) != ""
		log.DefaultLogger.Debug("bearer token", "present", bearerToken)
		idTokenPresent := h.Get(backend.OAuthIdentityIDTokenHeaderName) != ""
		log.DefaultLogger.Debug("id token", "present", idTokenPresent)
	}
}

func (ds *ODataSource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse,
	error) {
	ds.logTokenStatus(req.GetHTTPHeaders())
	clientInstance, err := ds.getClientInstance(ctx, req.PluginContext)
	if err != nil {
		return nil, err
	}
	response := backend.NewQueryDataResponse()
	for _, q := range req.Queries {
		res := ds.query(ctx, clientInstance, q)
		response.Responses[q.RefID] = res
	}
	return response, nil
}

func (ds *ODataSource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult,
	error) {
	ds.logTokenStatus(req.GetHTTPHeaders())
	var status backend.HealthStatus
	var message string
	clientInstance, err := ds.getClientInstance(ctx, req.PluginContext)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: fmt.Sprintf("Health check failed: %s", err.Error()),
		}, nil
	}
	res, err := clientInstance.GetServiceRoot(ctx)
	if err != nil {
		status = backend.HealthStatusError
		message = fmt.Sprintf("Health check failed: %s", err.Error())
	} else {
		defer func() { _ = res.Body.Close() }()
		if res.StatusCode == 200 {
			status = backend.HealthStatusOk
			message = "Data Source is working as expected."
		} else {
			status = backend.HealthStatusError
			message = fmt.Sprintf("Health check failed, datasource exists but given path does not. "+
				"Statuscode: %d", res.StatusCode)
		}
	}
	return &backend.CheckHealthResult{
		Status:  status,
		Message: message,
	}, nil
}

func (ds *ODataSource) CallResource(ctx context.Context, req *backend.CallResourceRequest,
	sender backend.CallResourceResponseSender) error {
	ds.logTokenStatus(req.GetHTTPHeaders())
	switch req.Path {
	case "metadata":
		return ds.getMetadata(ctx, req, sender)
	case "rawquery":
		// TEMPORARY, test-only: authenticated passthrough for probing
		// $apply/lambda support on the live OData service directly.
		// Not wired into the production query model - see rawQuery's
		// doc comment. Must be removed before this ships.
		return ds.rawQuery(ctx, req, sender)
	default:
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusNotFound,
		})
	}
}

// rawQuery is a TEMPORARY diagnostic-only resource endpoint used to probe
// whether the live Fledge OData service supports $apply (Data Aggregation
// Extension) and lambda (any/all) operators - capabilities this plugin has
// no production support for today. It reuses the datasource's existing
// OAuth2 auth (via doRequest) so the probe hits the real, authenticated
// service instead of requiring separate throwaway credentials. It takes
// "entitySet" and "query" (a raw, already-encoded OData query string, e.g.
// "$apply=groupby((debtor_id),aggregate(price with sum as total))") from
// the resource request's query string and forwards a GET verbatim,
// returning the raw upstream status and body.
//
// This must be removed once the $apply/lambda probe is complete - it is
// not a sanctioned way to query the datasource and bypasses the plugin's
// normal query model entirely.
func (ds *ODataSource) rawQuery(ctx context.Context, req *backend.CallResourceRequest, sender backend.CallResourceResponseSender) error {
	clientInstance, err := ds.getClientInstance(ctx, req.PluginContext)
	if err != nil {
		return err
	}
	clientImpl, ok := clientInstance.(*ODataClientImpl)
	if !ok {
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusInternalServerError,
			Body:   []byte("client instance is not *ODataClientImpl"),
		})
	}

	parsedReqUrl, err := url.Parse(req.URL)
	if err != nil {
		return err
	}
	q := parsedReqUrl.Query()
	entitySet := q.Get("entitySet")
	rawQueryString := q.Get("query")

	target, err := url.Parse(clientImpl.baseUrl)
	if err != nil {
		return err
	}
	target.Path = path.Join(target.Path, entitySet)
	target.RawQuery = rawQueryString

	log.DefaultLogger.Info("TEMP rawquery probe", "url", target.String())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := clientImpl.doRequest(httpReq)
	if err != nil {
		return sender.Send(&backend.CallResourceResponse{
			Status: http.StatusBadGateway,
			Body:   []byte(err.Error()),
		})
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	log.DefaultLogger.Info("TEMP rawquery probe result", "url", target.String(), "status", resp.StatusCode, "body", string(body))

	return sender.Send(&backend.CallResourceResponse{
		Status: resp.StatusCode,
		Body:   body,
	})
}

// replaceSpacesForOData replaces + with %20 in URL-encoded strings for OData compatibility
func replaceSpacesForOData(encodedUrl string) string {
	return strings.ReplaceAll(encodedUrl, "+", "%20")
}

func (ds *ODataSource) addFiltersToNextLink(nextLinkUrl string, filterConditions []filterCondition) string {
	parsedUrl, err := url.Parse(nextLinkUrl)
	if err != nil {
		log.DefaultLogger.Error("failed to parse nextLink URL", "url", nextLinkUrl, "error", err)
		return nextLinkUrl
	}

	query := parsedUrl.Query()
	existingFilter := query.Get("$filter")
	existingApply := query.Get(odata.Apply)

	if existingFilter != "" || existingApply != "" {
		// A $apply nextLink already has any filter(...) stage baked into
		// the apply expression itself - blindly appending a separate
		// $filter param alongside $apply would be redundant at best and
		// could conflict with server-side pagination state at worst, so
		// this path only fixes encoding, same as the existingFilter case.
		log.DefaultLogger.Info("nextLink already has filter/apply, fixing encoding", "existingFilter", existingFilter, "existingApply", existingApply)

		newQuery := url.Values{}
		for key, values := range query {
			for _, value := range values {
				newQuery.Add(key, value)
			}
		}

		encodedQuery := newQuery.Encode()
		encodedQuery = replaceSpacesForOData(encodedQuery)
		parsedUrl.RawQuery = encodedQuery
		fixedUrl := parsedUrl.String()

		log.DefaultLogger.Info("fixed nextLink encoding", "original", nextLinkUrl, "fixed", fixedUrl)
		return fixedUrl
	}

	if len(filterConditions) == 0 {
		return nextLinkUrl
	}

	filterParam := mapFilter(filterConditions)

	if filterParam != "" {
		encodedFilter := url.QueryEscape(filterParam)
		if parsedUrl.RawQuery != "" {
			parsedUrl.RawQuery += "&"
		}
		parsedUrl.RawQuery += "$filter=" + encodedFilter

		updatedUrl := parsedUrl.String()
		log.DefaultLogger.Info("added filter to nextLink",
			"original", nextLinkUrl,
			"updated", updatedUrl,
			"filterParam", filterParam)
		return updatedUrl
	}

	return nextLinkUrl
}

func (ds *ODataSource) fetchNextLink(ctx context.Context, clientInstance ODataClient, nextLinkUrl string) (*odata.Response, error) {
	clientImpl, ok := clientInstance.(*ODataClientImpl)
	if !ok {
		return nil, fmt.Errorf("client instance is not of type *ODataClientImpl")
	}

	log.DefaultLogger.Info("fetching nextLink", "url", nextLinkUrl)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, nextLinkUrl, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for nextLink: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := clientImpl.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute nextLink request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		responseBody := string(bodyBytes)

		var odataError struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(bodyBytes, &odataError) == nil && odataError.Error.Message != "" {
			return nil, fmt.Errorf("OData error on page fetch (status %d): %s", resp.StatusCode, odataError.Error.Message)
		}

		if responseBody != "" && responseBody != fmt.Sprintf("%d %s", resp.StatusCode, http.StatusText(resp.StatusCode)) {
			return nil, fmt.Errorf("nextLink request failed with status %d: %s", resp.StatusCode, responseBody)
		}
		return nil, fmt.Errorf("nextLink request failed with status %d (%s)", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read nextLink response body: %w", err)
	}

	var result odata.Response
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nextLink response: %w", err)
	}

	return &result, nil
}

func (ds *ODataSource) query(ctx context.Context, clientInstance ODataClient, query backend.DataQuery) backend.DataResponse {
	log.DefaultLogger.Debug("query", "query.JSON", string(query.JSON))
	response := backend.DataResponse{}
	var qm queryModel
	err := json.Unmarshal(query.JSON, &qm)
	if err != nil {
		response.Error = fmt.Errorf("error unmarshalling query json: %w", err)
		return response
	}

	// len(qm.Aggregates) > 0 signals the server-side $apply (groupby/
	// aggregate) path instead of the default raw-row path.
	if len(qm.Aggregates) > 0 {
		return ds.queryAggregate(ctx, clientInstance, query, qm)
	}

	// Prevent empty queries from being executed
	if qm.TimeProperty == nil && len(qm.Properties) == 0 {
		return response
	}

	frame := data.NewFrame("response")
	frame.Name = query.RefID
	if frame.Meta == nil {
		frame.Meta = &data.FrameMeta{}
	}
	frame.Meta.PreferredVisualization = data.VisTypeTable

	if qm.TimeProperty != nil {
		log.DefaultLogger.Debug("Time property configured", "name", qm.TimeProperty.Name)
		field := data.NewField(qm.TimeProperty.Name, nil, odata.ToArray(qm.TimeProperty.Type))
		frame.Fields = append(frame.Fields, field)
	}
	for _, prop := range qm.Properties {
		field := data.NewField(prop.Name, nil, odata.ToArray(prop.Type))
		frame.Fields = append(frame.Fields, field)
	}
	for _, expand := range qm.Expands {
		for _, prop := range expand.Properties {
			field := data.NewField(expandedFieldName(expand.Expand.Name, prop.Name), nil, odata.ToArray(prop.Type))
			frame.Fields = append(frame.Fields, field)
		}
	}

	props := qm.Properties
	if qm.TimeProperty != nil {
		props = append(props, *qm.TimeProperty)
	}

	// Build filter conditions including time range
	allFilterConditions := append(qm.FilterConditions, TimeRangeToFilter(query.TimeRange, qm.TimeProperty)...)

	resp, err := clientInstance.Get(ctx, qm.EntitySet.Name, props, qm.Expands, allFilterConditions)
	if err != nil {
		response.Error = err
		return response
	}

	allValues, truncated, err := ds.collectPaginatedValues(ctx, clientInstance, resp, allFilterConditions)
	if err != nil {
		response.Error = err
		return response
	}

	if truncated {
		log.DefaultLogger.Warn("query result truncated by row cap", "maxPaginatedRows", maxPaginatedRows, "entitySet", qm.EntitySet.Name)
		frame.Meta.Notices = append(frame.Meta.Notices, data.Notice{
			Severity: data.NoticeSeverityWarning,
			Text: fmt.Sprintf("Result truncated at %d rows - narrow your filter to see the full result set.",
				maxPaginatedRows),
		})
	}

	for _, entry := range allValues {
		var values []interface{}
		fieldCount := len(qm.Properties) + expandedPropertyCount(qm.Expands)

		if qm.TimeProperty != nil {
			values = make([]interface{}, fieldCount+1)
			values[0] = odata.MapValue(entry[qm.TimeProperty.Name], qm.TimeProperty.Type)
		} else {
			values = make([]interface{}, fieldCount)
		}

		for i, prop := range qm.Properties {
			index := i
			if qm.TimeProperty != nil {
				index++
			}

			if value, ok := entry[prop.Name]; ok {
				values[index] = odata.MapValue(value, prop.Type)
			} else {
				values[index] = nil
			}
		}
		expandedIndex := len(qm.Properties)
		if qm.TimeProperty != nil {
			expandedIndex++
		}
		for _, expand := range qm.Expands {
			for _, prop := range expand.Properties {
				if value, ok := expandedPropertyValue(entry, expand.Expand.Name, prop.Name); ok {
					values[expandedIndex] = odata.MapValue(value, prop.Type)
				} else {
					values[expandedIndex] = nil
				}
				expandedIndex++
			}
		}
		frame.AppendRow(values...)
	}
	response.Frames = append(response.Frames, frame)
	return response
}

// queryAggregate handles the server-side $apply (groupby/aggregate) path,
// signalled by a non-empty qm.Aggregates. Unlike the raw-row path in query,
// it synthesizes its output properties list from qm.GroupBy plus one
// property per aggregate alias (typed after the aggregated source field's
// Edm type - a sum of Edm.Decimal is still Edm.Decimal) instead of a
// $select-based Properties/Expands list, and skips TimeProperty-specific
// field handling (the time property, if wanted in the output, must already
// be present in GroupBy). The $apply response's flat rows
// ({groupByProp: ..., alias: ...}) fit the same entry[prop.Name] lookup the
// raw-row path uses, confirmed live - see the client_query_test.go /
// datasource_query_test.go aggregate cases and this task's report for the
// exact verified shape.
func (ds *ODataSource) queryAggregate(ctx context.Context, clientInstance ODataClient, query backend.DataQuery, qm queryModel) backend.DataResponse {
	response := backend.DataResponse{}

	frame := data.NewFrame("response")
	frame.Name = query.RefID
	if frame.Meta == nil {
		frame.Meta = &data.FrameMeta{}
	}
	frame.Meta.PreferredVisualization = data.VisTypeTable

	// Synthesized output properties: groupBy fields (real Edm types) first,
	// then one field per aggregate alias.
	props := make([]property, 0, len(qm.GroupBy)+len(qm.Aggregates))
	props = append(props, qm.GroupBy...)
	for _, agg := range qm.Aggregates {
		props = append(props, property{Name: agg.Alias, Type: agg.Property.Type})
	}

	for _, prop := range props {
		field := data.NewField(prop.Name, nil, odata.ToArray(prop.Type))
		frame.Fields = append(frame.Fields, field)
	}

	// Build filter conditions including time range - same TimeRangeToFilter
	// mechanism as the raw-row path, folded into the $apply filter(...)
	// stage instead of $filter=....
	allFilterConditions := append(qm.FilterConditions, TimeRangeToFilter(query.TimeRange, qm.TimeProperty)...)

	resp, err := clientInstance.GetAggregated(ctx, qm.EntitySet.Name, qm.GroupBy, qm.Aggregates, allFilterConditions)
	if err != nil {
		response.Error = err
		return response
	}

	allValues, truncated, err := ds.collectPaginatedValues(ctx, clientInstance, resp, allFilterConditions)
	if err != nil {
		response.Error = err
		return response
	}

	if truncated {
		log.DefaultLogger.Warn("aggregate query result truncated by row cap", "maxPaginatedRows", maxPaginatedRows, "entitySet", qm.EntitySet.Name)
		frame.Meta.Notices = append(frame.Meta.Notices, data.Notice{
			Severity: data.NoticeSeverityWarning,
			Text: fmt.Sprintf("Result truncated at %d rows - narrow your filter to see the full result set.",
				maxPaginatedRows),
		})
	}

	for _, entry := range allValues {
		values := make([]interface{}, len(props))
		for i, prop := range props {
			if value, ok := entry[prop.Name]; ok {
				values[i] = odata.MapValue(value, prop.Type)
			} else {
				values[i] = nil
			}
		}
		frame.AppendRow(values...)
	}

	response.Frames = append(response.Frames, frame)
	return response
}

// collectPaginatedValues reads an initial OData response, follows
// @odata.nextLink pagination (via fetchNextLink/addFiltersToNextLink), and
// returns the combined row values along with whether the result was
// truncated at maxPaginatedRows. Shared by both the raw-row and $apply
// aggregate query paths - aggregated result sets are small (bucketed, not
// raw line items) so the truncation warning is far less likely to trigger
// there, but the same cap and pagination loop apply either way.
func (ds *ODataSource) collectPaginatedValues(ctx context.Context, clientInstance ODataClient, resp *http.Response, allFilterConditions []filterCondition) ([]map[string]interface{}, bool, error) {
	defer func() { _ = resp.Body.Close() }()

	log.DefaultLogger.Debug("request response status", "status", resp.Status)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		responseBody := string(bodyBytes)
		log.DefaultLogger.Error("OData request failed", "statusCode", resp.StatusCode, "responseBody", responseBody)

		var odataError struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(bodyBytes, &odataError) == nil && odataError.Error.Message != "" {
			return nil, false, fmt.Errorf("OData error (status %d): %s", resp.StatusCode, odataError.Error.Message)
		}

		return nil, false, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, responseBody)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	var result odata.Response
	err = json.Unmarshal(bodyBytes, &result)
	if err != nil {
		return nil, false, err
	}

	// Collect all values from initial response and follow pagination
	allValues := result.Value
	log.DefaultLogger.Debug("initial query response", "noOfEntities", len(allValues), "hasNextLink", result.NextLink != "")

	nextLink := result.NextLink
	pageCount := 1
	truncated := false
	if len(allValues) >= maxPaginatedRows {
		truncated = true
	}
	for nextLink != "" && !truncated {
		pageCount++
		log.DefaultLogger.Debug("fetching next page", "pageNumber", pageCount, "nextLink", nextLink)

		nextLinkWithFilters := ds.addFiltersToNextLink(nextLink, allFilterConditions)
		nextResp, err := ds.fetchNextLink(ctx, clientInstance, nextLinkWithFilters)
		if err != nil {
			log.DefaultLogger.Error("error fetching next link", "error", err, "pageNumber", pageCount)
			return nil, false, fmt.Errorf("error fetching page %d: %w", pageCount, err)
		}

		allValues = append(allValues, nextResp.Value...)
		nextLink = nextResp.NextLink
		log.DefaultLogger.Debug("fetched next page", "pageNumber", pageCount, "entitiesInPage", len(nextResp.Value), "totalEntities", len(allValues))

		if len(allValues) >= maxPaginatedRows {
			truncated = true
		}
	}

	if truncated && len(allValues) > maxPaginatedRows {
		allValues = allValues[:maxPaginatedRows]
	}

	log.DefaultLogger.Debug("query complete", "totalPages", pageCount, "totalEntities", len(allValues), "truncated", truncated)

	return allValues, truncated, nil
}

func expandedPropertyCount(expands []expandProperty) int {
	count := 0
	for _, expand := range expands {
		count += len(expand.Properties)
	}
	return count
}

func expandedFieldName(path string, propertyName string) string {
	trimmedPath := strings.Trim(path, "/")
	if trimmedPath == "" {
		return propertyName
	}
	return strings.ReplaceAll(trimmedPath, "/", ".") + "." + propertyName
}

func expandedPropertyValue(entry map[string]interface{}, expandPath string, propertyName string) (interface{}, bool) {
	var current interface{} = entry
	for _, part := range strings.Split(strings.Trim(expandPath, "/"), "/") {
		if part == "" {
			continue
		}
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = currentMap[part]
		if !ok || current == nil {
			return nil, false
		}
		if currentSlice, ok := current.([]interface{}); ok {
			if len(currentSlice) == 0 {
				return nil, false
			}
			current = currentSlice[0]
		}
	}

	currentMap, ok := current.(map[string]interface{})
	if !ok {
		return nil, false
	}
	value, ok := currentMap[propertyName]
	return value, ok
}

func (ds *ODataSource) getMetadata(ctx context.Context, req *backend.CallResourceRequest,
	sender backend.CallResourceResponseSender) error {
	clientInstance, err := ds.getClientInstance(ctx, req.PluginContext)
	if err != nil {
		return err
	}
	resp, err := clientInstance.GetMetadata(ctx)

	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("get metadata failed with status code %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.DefaultLogger.Error("error reading response body")
		return err
	}
	var edmx odata.Edmx
	err = xml.Unmarshal(bodyBytes, &edmx)
	if err != nil {
		log.DefaultLogger.Error("error unmarshalling response body")
		return err
	}

	metadata := schema{
		EntityTypes: make(map[string]entityType),
		EntitySets:  []entitySet{},
	}

	entitySetsMap := make(map[string]entitySet)

	log.DefaultLogger.Info("Parsing metadata", "dataServicesCount", len(edmx.DataServices))

	for _, ds := range edmx.DataServices {
		log.DefaultLogger.Info("Processing DataService", "schemasCount", len(ds.Schemas))
		for _, s := range ds.Schemas {
			log.DefaultLogger.Info("Processing Schema", "namespace", s.Namespace, "entityTypesCount", len(s.EntityTypes), "entityContainersCount", len(s.EntityContainers))
			for _, et := range s.EntityTypes {
				qualifiedName := s.Namespace + "." + et.Name
				properties := []property{}
				navigationProperties := []navigationProperty{}
				for _, p := range et.Properties {
					prop := property{
						Name: p.Name,
						Type: p.Type,
					}
					properties = append(properties, prop)
				}
				for _, p := range et.NavigationProperties {
					prop := navigationProperty{
						Name: p.Name,
						Type: normalizeNavigationPropertyType(p.Type),
					}
					navigationProperties = append(navigationProperties, prop)
				}

				// Sort properties alphabetically by name
				sort.Slice(properties, func(i, j int) bool {
					return properties[i].Name < properties[j].Name
				})
				sort.Slice(navigationProperties, func(i, j int) bool {
					return navigationProperties[i].Name < navigationProperties[j].Name
				})

				metadata.EntityTypes[qualifiedName] = entityType{
					Name:                 et.Name,
					QualifiedName:        qualifiedName,
					Properties:           properties,
					NavigationProperties: navigationProperties,
				}
			}
			for _, ec := range s.EntityContainers {
				log.DefaultLogger.Info("Processing EntityContainer", "name", ec.Name, "entitySetCount", len(ec.EntitySet))
				for _, es := range ec.EntitySet {
					log.DefaultLogger.Info("Adding EntitySet", "name", es.Name, "entityType", es.EntityType)
					entitySetsMap[es.Name] = entitySet{
						Name:       es.Name,
						EntityType: es.EntityType,
					}
				}
			}
		}
	}

	log.DefaultLogger.Info("Metadata parsed", "totalEntityTypes", len(metadata.EntityTypes), "totalEntitySets", len(entitySetsMap))

	// Convert map to sorted slice
	for _, es := range entitySetsMap {
		metadata.EntitySets = append(metadata.EntitySets, es)
	}
	sort.Slice(metadata.EntitySets, func(i, j int) bool {
		return metadata.EntitySets[i].Name < metadata.EntitySets[j].Name
	})

	responseBody, err := json.Marshal(metadata)
	if err != nil {
		log.DefaultLogger.Error("error marshalling response body")
		return err
	}
	return sender.Send(&backend.CallResourceResponse{
		Status: http.StatusOK,
		Body:   responseBody,
	})
}

func normalizeNavigationPropertyType(propertyType string) string {
	const collectionPrefix = "Collection("
	if strings.HasPrefix(propertyType, collectionPrefix) && strings.HasSuffix(propertyType, ")") {
		return strings.TrimSuffix(strings.TrimPrefix(propertyType, collectionPrefix), ")")
	}
	return propertyType
}
