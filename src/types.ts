import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export interface ODataQuery extends DataQuery {
  entitySet?: EntitySet;
  timeProperty?: Property | null;
  properties?: Property[];
  expands?: ExpandProperty[];
  filterConditions?: FilterCondition[];
}

export const FilterOperators: string[] = ['eq', 'ne', 'gt', 'ge', 'lt', 'le', 'in'];

export interface ODataOptions extends DataSourceJsonData {
  urlSpaceEncoding: string;
  oauth2Enabled?: boolean;
  oauth2Url?: string;
}

export interface ODataSecureJsonData {
  oauth2ApiKey?: string;
}

export enum URLSpaceEncoding {
  Plus = '+',
  Percent = '%20',
}

export interface Metadata {
  entityTypes: { [name: string]: EntityType };
  entitySets: EntitySet[];
}

export interface EntityType {
  name: string;
  qualifiedName: string;
  properties: Property[];
  navigationProperties?: NavigationProperty[];
}

export interface EntitySet {
  name: string;
  entityType: string;
}

export interface Property {
  name: string;
  type: string;
}

export interface NavigationProperty {
  name: string;
  type: string;
}

export interface ExpandProperty {
  expand: NavigationProperty;
  properties?: Property[];
}

export interface FilterCondition {
  property: Property;
  operator: string;
  value: string;
}
