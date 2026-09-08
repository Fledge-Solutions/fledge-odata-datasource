import { DataQueryRequest, DataSourceInstanceSettings, Field, MetricFindValue, ScopedVars, TimeRange, getDefaultTimeRange } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';
import { lastValueFrom } from 'rxjs';
import { ExpandProperty, FilterCondition, ODataOptions, ODataQuery } from './types';

// Shape of the JSON that must be entered in the "Query" field of a template
// variable of type "Query" using this datasource, e.g.:
// {"entitySet":"SalesOrderHeaders","entityType":"nl.fledge.odata.SalesOrderHeaders","valueField":"sales_order_header_id","textField":"description"}
// filterConditions is optional and may reference other variables (e.g. "$sales_order_header_id")
// to build a variable whose options depend on another variable's current value.
// textField may reference a field pulled in via "expands" (e.g. "Administration.name") -
// in that case it must NOT also be listed as a flat property, since it isn't one.
interface ODataVariableQuery {
  entitySet: string;
  entityType: string;
  valueField: string;
  textField?: string;
  filterConditions?: FilterCondition[];
  expands?: ExpandProperty[];
}

export class ODataSource extends DataSourceWithBackend<ODataQuery, ODataOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<ODataOptions>) {
    super(instanceSettings);
  }

  // Custom "All" value for dashboard variables (sales_order_header_id, administration_id,
  // debtor_id). This MUST be a non-empty string: Grafana treats an empty-string allValue as
  // "not configured" and falls back to substituting a comma-joined list of every enumerated
  // option instead - for a variable with hundreds/thousands of rows that produces a request
  // line long enough to make the real OData server reject it with HTTP 400 "header too large".
  static readonly ALL_VALUE = '__all__';

  applyTemplateVariables(query: ODataQuery, scopedVars: ScopedVars) {
    const templateSrv = getTemplateSrv();

    // A filter condition whose value is a variable reference (e.g. "$sales_order_header_id")
    // that resolves to empty or to the ALL_VALUE sentinel means "All" was selected / nothing is
    // selected for that variable. Drop the condition entirely in that case so the query is
    // unfiltered on that property - e.g. selecting "All" for Sales Order Header shows an
    // aggregate overview across every project instead of matching zero rows. Conditions with a
    // literal empty value that was never a variable (e.g. `production_order_header_id ne ''`)
    // are left untouched.
    const filterConditions = query.filterConditions
      ?.map((filterCondition) => {
        const originalValue = filterCondition.value;
        const value = templateSrv.replace(originalValue, scopedVars);
        return { condition: { ...filterCondition, value }, wasVariable: originalValue !== '' };
      })
      .filter(({ condition, wasVariable }) => !(wasVariable && (condition.value === '' || condition.value === ODataSource.ALL_VALUE)))
      .map(({ condition }) => condition);

    return {
      ...query,
      filterConditions,
    };
  }

  async metricFindQuery(
    query: string,
    options?: { range?: TimeRange; scopedVars?: ScopedVars }
  ): Promise<MetricFindValue[]> {
    const interpolated = getTemplateSrv().replace(query, options?.scopedVars);
    if (!interpolated) {
      return [];
    }

    let variableQuery: ODataVariableQuery;
    try {
      variableQuery = JSON.parse(interpolated);
    } catch {
      return [];
    }

    if (!variableQuery.entitySet || !variableQuery.entityType || !variableQuery.valueField) {
      return [];
    }

    const properties = [{ name: variableQuery.valueField, type: 'Edm.String' }];
    const textFieldIsExpanded = variableQuery.textField?.includes('.') ?? false;
    if (variableQuery.textField && variableQuery.textField !== variableQuery.valueField && !textFieldIsExpanded) {
      properties.push({ name: variableQuery.textField, type: 'Edm.String' });
    }

    const target: ODataQuery = {
      refId: 'metricFindQuery',
      entitySet: { name: variableQuery.entitySet, entityType: variableQuery.entityType },
      properties,
      expands: variableQuery.expands ?? [],
      filterConditions: variableQuery.filterConditions ?? [],
    };

    const request: DataQueryRequest<ODataQuery> = {
      requestId: 'metricFindQuery',
      interval: '',
      intervalMs: 0,
      range: options?.range ?? getDefaultTimeRange(),
      scopedVars: options?.scopedVars ?? {},
      targets: [target],
      timezone: 'browser',
      app: 'dashboard',
      startTime: Date.now(),
    };

    const response = await lastValueFrom(this.query(request));
    const frame = response.data[0];
    if (!frame) {
      return [];
    }

    const valueField: Field | undefined = frame.fields.find(
      (field: Field) => field.name === variableQuery.valueField
    );
    if (!valueField) {
      return [];
    }
    const textField: Field | undefined = variableQuery.textField
      ? frame.fields.find((field: Field) => field.name === variableQuery.textField)
      : undefined;

    const seen = new Set<string>();
    const values: MetricFindValue[] = [];
    valueField.values.forEach((value: string, index: number) => {
      const key = String(value);
      if (seen.has(key)) {
        return;
      }
      seen.add(key);
      values.push({
        value: key,
        text: textField ? `${key} - ${textField.values[index]}` : key,
      });
    });

    values.sort((a, b) => String(a.value).localeCompare(String(b.value), undefined, { numeric: true }));

    return values;
  }
}
