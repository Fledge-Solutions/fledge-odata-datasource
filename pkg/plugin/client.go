package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fledge-solutions/fledge-odata-datasource/pkg/plugin/odata"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

// allValueSentinel is the filter value a dashboard variable sends when its
// "All" option is selected (e.g. via a query-variable allValue of "__all__").
// It is not valid OData for a numeric property and the server rejects it
// outright, so any filterCondition whose value is exactly this sentinel is
// dropped entirely rather than sent to the server - "All" means "no filter
// on this property", not a literal value to compare against.
const allValueSentinel = "__all__"

// nowValueSentinel is a filter value a dashboard can use in place of a
// literal date/time so the comparison is resolved server-side at request
// time (e.g. "planning_date ge $NOW"), instead of baking a static literal
// into the dashboard at save time that goes stale.
const nowValueSentinel = "$NOW"

// resolveNowLiteral expands the $NOW sentinel to the current date/time,
// formatted per the target property's Edm type: OData v4 Edm.Date literals
// are bare "YYYY-MM-DD" (no quotes, no time component); everything else
// (Edm.DateTimeOffset, Edm.String, Edm.Time, ...) gets a full RFC3339
// timestamp, which is then quoted like any other quoted literal.
func resolveNowLiteral(edmType string) string {
	now := time.Now().UTC()
	if edmType == odata.EdmDate {
		return now.Format("2006-01-02")
	}
	return now.Format(time.RFC3339)
}

type ODataClient interface {
	GetServiceRoot(ctx context.Context) (*http.Response, error)
	GetMetadata(ctx context.Context) (*http.Response, error)
	Get(ctx context.Context, entitySet string, properties []property,
		expands []expandProperty, filterConditions []filterCondition) (*http.Response, error)
	GetAggregated(ctx context.Context, entitySet string, groupBy []property,
		aggregates []aggregateSpec, filterConditions []filterCondition) (*http.Response, error)
}

type ODataClientImpl struct {
	httpClient        *http.Client
	baseUrl           string
	urlSpaceEncoding  string
	oauth2Enabled     bool
	oauth2Url         string
	oauth2ApiKey      string
	oauth2Token       string
	oauth2TokenExpiry time.Time
	oauth2Mutex       sync.Mutex
}

type OAuth2TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

func (client *ODataClientImpl) getOAuth2Token() (string, error) {
	client.oauth2Mutex.Lock()
	defer client.oauth2Mutex.Unlock()

	// Check if token is still valid (with 30-second buffer)
	if client.oauth2Token != "" && time.Now().Before(client.oauth2TokenExpiry.Add(-30*time.Second)) {
		return client.oauth2Token, nil
	}

	// Request new token using API key grant type
	log.DefaultLogger.Debug("Requesting new OAuth2 token with API key grant")

	// Construct token URL
	tokenUrl := strings.TrimRight(client.oauth2Url, "/") + "/oauth2/token"

	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:api_key")
	data.Set("api_key", client.oauth2ApiKey)

	req, err := http.NewRequest("POST", tokenUrl, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create OAuth2 token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request OAuth2 token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OAuth2 token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp OAuth2TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode OAuth2 token response: %w", err)
	}

	client.oauth2Token = tokenResp.AccessToken
	client.oauth2TokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	log.DefaultLogger.Debug("OAuth2 token obtained successfully", "expires_in", tokenResp.ExpiresIn)
	return client.oauth2Token, nil
}

func (client *ODataClientImpl) doRequest(req *http.Request) (*http.Response, error) {
	if client.oauth2Enabled {
		token, err := client.getOAuth2Token()
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return client.httpClient.Do(req)
}

func (client *ODataClientImpl) GetServiceRoot(ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return client.doRequest(req)
}

func (client *ODataClientImpl) GetMetadata(ctx context.Context) (*http.Response, error) {
	requestUrl, err := url.Parse(client.baseUrl)
	if err != nil {
		return nil, err
	}
	requestUrl.Path = path.Join(requestUrl.Path, odata.Metadata)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestUrl.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/xml")
	return client.doRequest(req)
}

func (client *ODataClientImpl) Get(ctx context.Context, entitySet string, properties []property, expands []expandProperty, filterConditions []filterCondition) (*http.Response, error) {
	requestUrl, err := buildQueryUrl(client.baseUrl, entitySet, properties,
		expands, filterConditions, client.urlSpaceEncoding)
	if err != nil {
		return nil, err
	}
	urlString := requestUrl.String()
	log.DefaultLogger.Info("Constructed request url", "url", urlString)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlString, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return client.doRequest(req)
}

// GetAggregated issues a server-side $apply (groupby/aggregate) query -
// exact sums/etc computed by the OData service itself, instead of pulling
// raw rows for client-side aggregation. See buildApplyQueryUrl for the
// generated query shape.
func (client *ODataClientImpl) GetAggregated(ctx context.Context, entitySet string, groupBy []property, aggregates []aggregateSpec, filterConditions []filterCondition) (*http.Response, error) {
	requestUrl, err := buildApplyQueryUrl(client.baseUrl, entitySet, groupBy,
		aggregates, filterConditions, client.urlSpaceEncoding)
	if err != nil {
		return nil, err
	}
	urlString := requestUrl.String()
	log.DefaultLogger.Info("Constructed aggregate request url", "url", urlString)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlString, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	return client.doRequest(req)
}

// buildApplyQueryUrl constructs a $apply=<expr> URL for a groupby/aggregate
// query, e.g.:
//
//	$apply=filter(order_date ge 2026-01-01)/groupby((order_date),aggregate(price_local_currency_net with sum as total))
//
// At least one aggregate is required - a groupby with no aggregate term is
// not a meaningful query in this plugin's model, so it's rejected outright
// rather than silently sent as a no-op. groupBy may be empty, which yields a
// bare aggregate(...) (a single grand-total row, no buckets). filterConditions
// reuses mapFilter verbatim - same quoting/sentinel handling as the
// $filter=... path.
func buildApplyQueryUrl(baseUrl string, entitySet string, groupBy []property, aggregates []aggregateSpec, filterConditions []filterCondition, urlSpaceEncoding string) (*url.URL, error) {
	if len(aggregates) == 0 {
		return nil, fmt.Errorf("at least one aggregate is required to build a $apply query")
	}

	requestUrl, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	requestUrl.Path = path.Join(requestUrl.Path, entitySet)

	params, err := url.ParseQuery(requestUrl.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("error parsing query: %w", err)
	}

	params.Add(odata.Apply, mapApply(groupBy, aggregates, filterConditions))

	encodedUrl := params.Encode()
	if urlSpaceEncoding == "%20" {
		encodedUrl = strings.ReplaceAll(encodedUrl, "+", "%20")
	}
	requestUrl.RawQuery = encodedUrl
	return requestUrl, nil
}

// mapApply builds the inner $apply expression: an optional filter(...)
// stage (reusing mapFilter verbatim) piped into groupby((...),aggregate(...))
// - or just aggregate(...) if groupBy is empty.
func mapApply(groupBy []property, aggregates []aggregateSpec, filterConditions []filterCondition) string {
	aggParts := make([]string, 0, len(aggregates))
	for _, agg := range aggregates {
		aggParts = append(aggParts, fmt.Sprintf("%s with %s as %s", agg.Property.Name, agg.Function, agg.Alias))
	}
	aggregateExpr := fmt.Sprintf("aggregate(%s)", strings.Join(aggParts, ","))

	innerExpr := aggregateExpr
	if len(groupBy) > 0 {
		groupByNames := make([]string, 0, len(groupBy))
		for _, prop := range groupBy {
			groupByNames = append(groupByNames, prop.Name)
		}
		innerExpr = fmt.Sprintf("groupby((%s),%s)", strings.Join(groupByNames, ","), aggregateExpr)
	}

	filterParam := mapFilter(filterConditions)
	if filterParam != "" {
		return fmt.Sprintf("filter(%s)/%s", filterParam, innerExpr)
	}
	return innerExpr
}

func buildQueryUrl(baseUrl string, entitySet string, properties []property, expands []expandProperty, filterConditions []filterCondition, urlSpaceEncoding string) (*url.URL, error) {
	requestUrl, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}

	requestUrl.Path = path.Join(requestUrl.Path, entitySet)

	params, err := url.ParseQuery(requestUrl.RawQuery)
	if err != nil {
		return nil, fmt.Errorf("error parsing query: %w", err)
	}
	filterParam := mapFilter(filterConditions)
	if len(filterParam) > 0 {
		params.Add(odata.Filter, filterParam)
	}
	selectParam := mapSelect(properties)
	if len(selectParam) > 0 {
		params.Add(odata.Select, selectParam)
	}
	expandParam := mapExpand(expands)
	if len(expandParam) > 0 {
		params.Add(odata.Expand, expandParam)
	}
	encodedUrl := params.Encode()
	if urlSpaceEncoding == "%20" {
		encodedUrl = strings.ReplaceAll(encodedUrl, "+", "%20")
	}
	requestUrl.RawQuery = encodedUrl
	return requestUrl, nil
}

func mapSelect(properties []property) string {
	var result []string
	if len(properties) > 0 {
		for _, selectProp := range properties {
			result = append(result, selectProp.Name)
		}
	}
	return strings.Join(result[:], ",")
}

type expandNode struct {
	properties []property
	children   map[string]*expandNode
}

func mapExpand(expands []expandProperty) string {
	root := &expandNode{children: make(map[string]*expandNode)}
	var rootOrder []string

	for _, expand := range expands {
		pathParts := strings.Split(strings.Trim(expand.Expand.Name, "/"), "/")
		current := root
		for _, part := range pathParts {
			if part == "" {
				continue
			}
			if current.children == nil {
				current.children = make(map[string]*expandNode)
			}
			if _, ok := current.children[part]; !ok {
				current.children[part] = &expandNode{children: make(map[string]*expandNode)}
				if current == root {
					rootOrder = append(rootOrder, part)
				}
			}
			current = current.children[part]
		}
		if current != root {
			current.properties = expand.Properties
		}
	}

	return formatExpandChildren(root, rootOrder)
}

func formatExpandChildren(node *expandNode, preferredOrder []string) string {
	var names []string
	if len(preferredOrder) > 0 {
		names = append(names, preferredOrder...)
	} else {
		for name := range node.children {
			names = append(names, name)
		}
		sort.Strings(names)
	}

	var parts []string
	for _, name := range names {
		child := node.children[name]
		if child == nil {
			continue
		}
		var options []string
		if selectParam := mapSelect(child.properties); selectParam != "" {
			options = append(options, odata.Select+"="+selectParam)
		}
		if expandParam := formatExpandChildren(child, nil); expandParam != "" {
			options = append(options, odata.Expand+"="+expandParam)
		}
		if len(options) == 0 {
			parts = append(parts, name)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s(%s)", name, strings.Join(options, ";")))
	}

	return strings.Join(parts, ",")
}

func mapFilter(filterConditions []filterCondition) string {
	var parts []string
	for _, element := range filterConditions {
		log.DefaultLogger.Info("Processing filter condition", "propertyName", element.Property.Name, "propertyType", element.Property.Type, "operator", element.Operator, "value", element.Value)

		// Skip incomplete filter conditions (missing property or operator)
		if element.Property.Name == "" || element.Operator == "" {
			log.DefaultLogger.Warn("Skipping incomplete filter condition", "propertyName", element.Property.Name, "operator", element.Operator, "value", element.Value)
			continue
		}

		// A dashboard variable's "All" option resolves to this sentinel -
		// drop the condition entirely rather than filtering on it.
		if element.Value == allValueSentinel {
			log.DefaultLogger.Debug("Skipping filter condition with allValue sentinel", "propertyName", element.Property.Name)
			continue
		}

		value := element.Value
		if value == nowValueSentinel {
			value = resolveNowLiteral(element.Property.Type)
		}

		// The "in" operator's value is always a pre-formatted, already
		// quoted-per-item literal list (e.g. "('ONBOARDING','MAATWERK')"),
		// built by the caller. Use it verbatim - wrapping it in another pair
		// of quotes (as the type-based quoting below would for Edm.String)
		// would double-quote it and break OData grammar.
		if element.Operator == "in" {
			parts = append(parts, fmt.Sprintf("%s %s %s", element.Property.Name, element.Operator, value))
			continue
		}

		// String and time-like types require single-quoted literals.
		// Edm.Date uses an unquoted date literal in standard OData.
		needsQuotes := element.Property.Type == odata.EdmString ||
			element.Property.Type == odata.EdmDateTimeOffset ||
			element.Property.Type == odata.EdmDateTime ||
			element.Property.Type == odata.EdmTime
		if needsQuotes {
			parts = append(parts, fmt.Sprintf("%s %s '%s'", element.Property.Name, element.Operator, value))
		} else if value == "" {
			// Unquoted (e.g. numeric) types have no valid empty literal, so
			// "prop eq " would be malformed OData. This typically comes from
			// an unset Grafana template variable (e.g. no Administration
			// selected) - the intent is "nothing selected", so force a filter
			// that can never match any row rather than sending an invalid
			// request or silently returning unfiltered data.
			parts = append(parts, fmt.Sprintf("%s ne %s", element.Property.Name, element.Property.Name))
		} else {
			parts = append(parts, fmt.Sprintf("%s %s %s", element.Property.Name, element.Operator, value))
		}
	}

	return strings.Join(parts, " and ")
}
