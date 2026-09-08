package odata

import "encoding/xml"

const (
	EdmString         = "Edm.String"
	EdmBoolean        = "Edm.Boolean"
	EdmSingle         = "Edm.Single"
	EdmDouble         = "Edm.Double"
	EdmDecimal        = "Edm.Decimal"
	EdmSByte          = "Edm.SByte"
	EdmByte           = "Edm.Byte"
	EdmInt16          = "Edm.Int16"
	EdmInt32          = "Edm.Int32"
	EdmInt64          = "Edm.Int64"
	EdmDateTimeOffset = "Edm.DateTimeOffset"
	EdmDateTime       = "Edm.DateTime"
	EdmGuid           = "Edm.Guid"
	EdmTime           = "Edm.Time"
	EdmDate           = "Edm.Date"

	Metadata = "$metadata"
	Filter   = "$filter"
	Select   = "$select"
	Expand   = "$expand"
	Apply    = "$apply"
)

type Response struct {
	Value    []map[string]interface{} `json:"value"`
	NextLink string                   `json:"@odata.nextLink,omitempty"`
}

type Edmx struct {
	XMLName      xml.Name        `xml:"Edmx"`
	Version      string          `xml:"Version,attr"`
	XmlNs        string          `xml:"edmx,attr"`
	DataServices []*DataServices `xml:"DataServices"`
}

type DataServices struct {
	XMLName xml.Name  `xml:"DataServices"`
	Schemas []*Schema `xml:"Schema"`
}

type Schema struct {
	XMLName          xml.Name           `xml:"Schema"`
	Namespace        string             `xml:"Namespace,attr"`
	XmlNs            string             `xml:"xmlns,attr"`
	EntityTypes      []*EntityType      `xml:"EntityType"`
	EntityContainers []*EntityContainer `xml:"EntityContainer"`
}

type EntityType struct {
	XMLName              xml.Name              `xml:"EntityType"`
	Name                 string                `xml:"Name,attr"`
	Key                  []*Key                `xml:"Key"`
	Properties           []*Property           `xml:"Property"`
	NavigationProperties []*NavigationProperty `xml:"NavigationProperty"`
}

type Key struct {
	XMLName     xml.Name       `xml:"Key"`
	PropertyRef []*PropertyRef `xml:"PropertyRef"`
}

type PropertyRef struct {
	XMLName xml.Name `xml:"PropertyRef"`
	Name    string   `xml:"Name,attr"`
}

type Property struct {
	XMLName  xml.Name `xml:"Property"`
	Name     string   `xml:"Name,attr"`
	Type     string   `xml:"Type,attr"`
	Nullable string   `xml:"Nullable,attr"`
}

type NavigationProperty struct {
	XMLName xml.Name `xml:"NavigationProperty"`
	Name    string   `xml:"Name,attr"`
	Type    string   `xml:"Type,attr"`
}

type EntityContainer struct {
	XMLName   xml.Name     `xml:"EntityContainer"`
	Name      string       `xml:"Name,attr"`
	EntitySet []*EntitySet `xml:"EntitySet"`
}

type EntitySet struct {
	XMLName    xml.Name `xml:"EntitySet"`
	Name       string   `xml:"Name,attr"`
	EntityType string   `xml:"EntityType,attr"`
}
