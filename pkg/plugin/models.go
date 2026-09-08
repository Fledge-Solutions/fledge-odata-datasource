package plugin

type queryModel struct {
	EntitySet        entitySet         `json:"entitySet"`
	TimeProperty     *property         `json:"timeProperty"`
	Properties       []property        `json:"properties"`
	Expands          []expandProperty  `json:"expands"`
	FilterConditions []filterCondition `json:"filterConditions"`
	// GroupBy and Aggregates opt a query into the server-side $apply
	// (groupby/aggregate) path instead of the default raw-row $filter/
	// $select path. len(Aggregates) > 0 is the signal that triggers it.
	// GroupBy must explicitly include the time property if time-bucketed
	// results are wanted - it is not auto-injected from TimeProperty.
	GroupBy    []property      `json:"groupBy,omitempty"`
	Aggregates []aggregateSpec `json:"aggregates,omitempty"`
}

// aggregateSpec describes one OData $apply aggregate term, e.g.
// "price_local_currency_net with sum as total".
type aggregateSpec struct {
	Property property `json:"property"`
	Function string   `json:"function"` // "sum" to start; avg/count/min/max are trivial to add later
	Alias    string   `json:"alias"`
}

type schema struct {
	EntityTypes map[string]entityType `json:"entityTypes"`
	EntitySets  []entitySet           `json:"entitySets"`
}

type entityType struct {
	Name                 string               `json:"name"`
	QualifiedName        string               `json:"qualifiedName"`
	Properties           []property           `json:"properties"`
	NavigationProperties []navigationProperty `json:"navigationProperties"`
}

type entitySet struct {
	Name       string `json:"name"`
	EntityType string `json:"entityType"`
}

type property struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type navigationProperty struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type expandProperty struct {
	Expand     navigationProperty `json:"expand"`
	Properties []property         `json:"properties"`
}

type filterCondition struct {
	Property property `json:"property"`
	Operator string   `json:"operator"`
	Value    string   `json:"value"`
}
