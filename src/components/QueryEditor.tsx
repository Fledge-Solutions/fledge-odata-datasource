import React, { PureComponent } from 'react';
import { Alert, Button, InlineFormLabel, LegacyForms, Input } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { ODataSource } from '../DataSource';
import { EntitySet, Metadata, ODataOptions, ODataQuery, Property, FilterOperators, NavigationProperty } from '../types';

const { Select } = LegacyForms;

type Props = QueryEditorProps<ODataSource, ODataQuery, ODataOptions>;

interface State {
  metadata: Metadata | undefined;
  entitySets: Array<SelectableValue<EntitySet>>;
  timeProperties: Array<SelectableValue<Property>>;
  allProperties: Array<SelectableValue<Property>>;
  filterOperators: Array<SelectableValue<string>>;
  metadataError: string | undefined;
}

enum PropertyKind {
  Time = 1,
  All = 2,
}

export class QueryEditor extends PureComponent<Props, State> {
  dataSource: ODataSource;
  private _isMounted = false;

  constructor(props: Props) {
    super(props);
    this.dataSource = this.props.datasource;
    this.state = {
      metadata: undefined,
      entitySets: [],
      timeProperties: [],
      allProperties: [],
      filterOperators: [],
      metadataError: undefined,
    };
  }

  componentDidMount() {
    this._isMounted = true;
    this.dataSource.getResource('metadata').then((metadata: Metadata) => {
      if (!this._isMounted) {
        return;
      }
      const entityType = this.props.query.entitySet?.entityType;
      this.setState({
        metadata: metadata,
        entitySets: metadata.entitySets.map((entitySet) => ({
          label: entitySet.name,
          value: entitySet,
        })),
        timeProperties: this.mapProperties(metadata, entityType, PropertyKind.Time),
        allProperties: this.mapProperties(metadata, entityType, PropertyKind.All),
      });
    }).catch((err: Error) => {
      if (!this._isMounted) {
        return;
      }
      this.setState({ metadataError: err?.message ?? 'Failed to load metadata' });
    });
    const filterOperators: Array<SelectableValue<string>> = FilterOperators.map((operator) => ({
      label: operator,
      value: operator,
    }));
    this.setState({
      filterOperators: filterOperators,
    });
  }

  componentWillUnmount() {
    this._isMounted = false;
  }

  mapProperties(metadata: Metadata | undefined, entityType: string | undefined, propertyKind: PropertyKind) {
    if (!metadata || !entityType || !metadata.entityTypes[entityType]) {
      return [];
    }
    return (propertyKind === PropertyKind.Time ? [{ label: '(None)', value: undefined as Property | undefined }] : [])
      .concat(metadata.entityTypes[entityType].properties
        .filter(property =>
          propertyKind === PropertyKind.All ||
          (propertyKind === PropertyKind.Time && (
            property.type === 'Edm.DateTimeOffset' ||
            property.type === 'Edm.DateTime' ||
            property.type === 'Edm.Date'
          ))
        )
        .map(property => ({
          label: property.name,
          value: property,
        }))
      );
  }

  update = (updatedQuery: ODataQuery) => {
    this.props.onChange(updatedQuery);
    this.props.onRunQuery();
  };

  onEntitySetChange = (option: SelectableValue<EntitySet>) => {
    if (this.props.query.entitySet?.name === option.value?.name) {
      return;
    }
    const { query } = this.props;
    const { metadata } = this.state;
    const entitySet = option.value;
    const updatedQuery: ODataQuery = { ...query, entitySet, timeProperty: null, properties: [], expands: [] };
    this.setState(
      {
        timeProperties: this.mapProperties(metadata, entitySet?.entityType, PropertyKind.Time),
        allProperties: this.mapProperties(metadata, entitySet?.entityType, PropertyKind.All),
      },
      () => this.update(updatedQuery)
    );
  };

  onTimePropertyChange = (option: SelectableValue<Property>) => {
    if (this.props.query.timeProperty === option.value) {
      return;
    }
    this.update({ ...this.props.query, timeProperty: option.value });
  };

  onPropertyChange = (option: SelectableValue<Property>, index: number) => {
    if (this.props.query.properties![index] === option.value) {
      return;
    }
    const properties = [...this.props.query.properties!];
    properties[index] = option.value ?? { name: '', type: '' };
    this.update({ ...this.props.query, properties });
  };

  addProperty = () => {
    const properties = [...(this.props.query.properties ?? []), { name: '', type: '' }];
    this.update({ ...this.props.query, properties });
  };

  removeProperty = (index: number) => {
    const properties = [...this.props.query.properties!];
    properties.splice(index, 1);
    this.update({ ...this.props.query, properties });
  };

  addExpand = () => {
    const expands = [...(this.props.query.expands ?? []), { expand: { name: '', type: '' }, properties: [] }];
    this.update({ ...this.props.query, expands });
  };

  removeExpand = (index: number) => {
    const expands = [...this.props.query.expands!];
    expands.splice(index, 1);
    this.update({ ...this.props.query, expands });
  };

  onExpandPathChange = (option: SelectableValue<NavigationProperty>, index: number) => {
    const expands = [...this.props.query.expands!];
    expands[index] = { expand: option.value ?? { name: '', type: '' }, properties: [] };
    this.update({ ...this.props.query, expands });
  };

  onExpandPropertiesChange = (options: Array<SelectableValue<Property>>, index: number) => {
    const expands = [...this.props.query.expands!];
    expands[index] = {
      ...expands[index],
      properties: options?.map((option) => option.value).filter((property): property is Property => Boolean(property)) ?? [],
    };
    this.update({ ...this.props.query, expands });
  };

  navigationPropertyOptions(entityType: string | undefined): Array<SelectableValue<NavigationProperty>> {
    const resolvedEntityType = this.resolveEntityType(entityType);
    if (!this.state.metadata) {
      return [];
    }
    const properties = resolvedEntityType ? (this.state.metadata.entityTypes[resolvedEntityType]?.navigationProperties ?? []) : [];
    const entitySetProperties = this.state.metadata.entitySets.map((entitySet) => ({
      name: entitySet.name,
      type: entitySet.entityType,
    }));
    const optionsByName = new Map<string, NavigationProperty>();
    for (const property of [...properties, ...entitySetProperties]) {
      optionsByName.set(property.name, property);
    }
    return Array.from(optionsByName.values()).map((property) => ({
      label: property.name,
      value: property,
    }));
  }

  propertyOptions(entityType: string | undefined): Array<SelectableValue<Property>> {
    const resolvedEntityType = this.resolveEntityType(entityType);
    if (!resolvedEntityType) {
      return [];
    }
    return (this.state.metadata?.entityTypes[resolvedEntityType]?.properties ?? []).map((property) => ({
      label: property.name,
      value: property,
    }));
  }

  expandTargetEntityType(expand: NavigationProperty | undefined) {
    return this.resolveEntityType(expand?.type ?? expand?.name);
  }

  resolveEntityType(entityTypeOrSetName: string | undefined) {
    const metadata = this.state.metadata;
    if (!metadata || !entityTypeOrSetName) {
      return undefined;
    }
    if (metadata.entityTypes[entityTypeOrSetName]) {
      return entityTypeOrSetName;
    }
    const entitySet = metadata.entitySets.find((item) => item.name === entityTypeOrSetName);
    if (entitySet?.entityType && metadata.entityTypes[entitySet.entityType]) {
      return entitySet.entityType;
    }
    const qualifiedName = Object.keys(metadata.entityTypes).find(
      (key) => metadata.entityTypes[key].name === entityTypeOrSetName || key.endsWith(`.${entityTypeOrSetName}`)
    );
    return qualifiedName;
  }

  addFilterCondition = () => {
    const filterConditions = [
      ...(this.props.query.filterConditions ?? []),
      { property: { name: '', type: '' }, operator: '', value: '' },
    ];
    this.update({ ...this.props.query, filterConditions });
  };

  removeFilterCondition = (index: number) => {
    const filterConditions = [...this.props.query.filterConditions!];
    filterConditions.splice(index, 1);
    this.update({ ...this.props.query, filterConditions });
  };

  onFilterPropertyChange = (option: SelectableValue<Property>, index: number) => {
    const filterCondition = this.props.query.filterConditions![index];
    if (option.value?.name === filterCondition.property.name) {
      return;
    }
    const property = option.value
      ? { name: option.value.name, type: this.state.allProperties.find((item) => item.value?.name === option.value?.name)?.value?.type ?? '' }
      : { name: '', type: '' };
    const filterConditions = [...this.props.query.filterConditions!];
    filterConditions[index] = { ...filterCondition, property };
    this.update({ ...this.props.query, filterConditions });
  };

  onFilterOperatorChange = (option: SelectableValue<string>, index: number) => {
    const filterCondition = this.props.query.filterConditions![index];
    const operator = option.value ?? '';
    if (operator === filterCondition.operator) {
      return;
    }
    const filterConditions = [...this.props.query.filterConditions!];
    filterConditions[index] = { ...filterCondition, operator };
    this.update({ ...this.props.query, filterConditions });
  };

  onFilterValueChange = (value: string, index: number) => {
    const filterCondition = this.props.query.filterConditions![index];
    if (value === filterCondition.value) {
      return;
    }
    const filterConditions = [...this.props.query.filterConditions!];
    filterConditions[index] = { ...filterCondition, value };
    this.props.onChange({ ...this.props.query, filterConditions });
  };

  render() {
    const { entitySets, timeProperties, allProperties, filterOperators, metadataError } = this.state;
    if (metadataError) {
      return <Alert title="Failed to load metadata" severity="error">{metadataError}</Alert>;
    }
    const listProperties = this.props.query.properties?.map((selectedProperty, index) => (
        <div key={index} className={'gf-form'}>
          <InlineFormLabel width={8} tooltip={'Add select'}>
            Select
          </InlineFormLabel>
          <Select
            value={allProperties.find((item) => item.value?.name === this.props.query.properties?.[index].name)}
            isClearable={true}
            placeholder="(Property)"
            onChange={(item) => this.onPropertyChange(item, index)}
            options={allProperties}
            isSearchable={false}
          />
          <Button variant={'secondary'} onClick={() => this.removeProperty(index)}>
            -
          </Button>
        </div>
    ));
    const listFilters = this.props.query.filterConditions?.map((filterCondition, index) => (
        <div key={index} className="gf-form-inline">
          <div className={'gf-form'}>
            <InlineFormLabel width={8} tooltip={'Add filter condition'}>
              {index === 0 ? 'Filter' : 'AND'}
            </InlineFormLabel>
            <Select
              value={allProperties.find(
                (item) => item.value?.name === this.props.query.filterConditions?.[index].property.name
              )}
              isClearable={true}
              placeholder="(Property)"
              onChange={(item) => this.onFilterPropertyChange(item, index)}
              options={allProperties}
              isSearchable={false}
            />
            <Select
              value={
                filterCondition.operator
                  ? { label: filterCondition.operator, value: filterCondition.operator }
                  : undefined
              }
              isClearable={true}
              placeholder="(Operator)"
              onChange={(item) => this.onFilterOperatorChange(item, index)}
              options={filterOperators}
              isSearchable={false}
            />
            <Input
              required={true}
              value={filterCondition.value}
              type="text"
              placeholder="(value)"
              onChange={(item) => this.onFilterValueChange(item.currentTarget.value, index)}
              onBlur={this.props.onRunQuery}
            />
            <Button variant={'secondary'} onClick={() => this.removeFilterCondition(index)}>
              -
            </Button>
          </div>
        </div>
    ));
    const listExpands = this.props.query.expands?.map((expand, index) => {
      const selectedPropertyNames = new Set((expand.properties ?? []).map((property) => property.name));
      const navigationOptions = this.navigationPropertyOptions(this.props.query.entitySet?.entityType);
      const selectedNavigation = navigationOptions.find((item) => item.value?.name === expand.expand?.name);
      const targetPropertyOptions = this.propertyOptions(this.expandTargetEntityType(expand.expand));
      return (
        <div key={index} className="gf-form-inline">
          <div className={'gf-form'}>
            <InlineFormLabel width={8} tooltip={'Expand a navigation property and optionally select fields from it'}>
              Expand
            </InlineFormLabel>
            <Select
              value={selectedNavigation}
              isClearable={true}
              placeholder="(Navigation)"
              onChange={(item) => this.onExpandPathChange(item, index)}
              options={navigationOptions}
              isSearchable={true}
              isDisabled={navigationOptions.length === 0}
            />
            <Select
              value={targetPropertyOptions.filter((item) => item.value && selectedPropertyNames.has(item.value.name))}
              isMulti={true}
              isClearable={true}
              placeholder="(Select fields)"
              onChange={(items) => this.onExpandPropertiesChange(items as Array<SelectableValue<Property>>, index)}
              options={targetPropertyOptions}
              isSearchable={false}
              isDisabled={!expand.expand?.name || targetPropertyOptions.length === 0}
            />
            <Button variant={'secondary'} onClick={() => this.removeExpand(index)}>
              -
            </Button>
          </div>
        </div>
      );
    });
    return (
      <div>
        <div className="gf-form-inline">
          <div className="gf-form">
            <InlineFormLabel width={8} tooltip="Select an entity set for a list of available metrics.">
              Entity set
            </InlineFormLabel>
            <Select
              value={entitySets.find((o) => o.value?.name === this.props.query.entitySet?.name)}
              isClearable={true}
              placeholder="(Entity set)"
              onChange={this.onEntitySetChange}
              options={entitySets}
              isSearchable={false}
            />
            <InlineFormLabel width={8} tooltip="Time property">
              Time property
            </InlineFormLabel>
            <Select
              value={timeProperties.find((o) => o.value?.name === this.props.query.timeProperty?.name)}
              isClearable={true}
              placeholder="(Property)"
              onChange={this.onTimePropertyChange}
              options={timeProperties}
              isSearchable={false}
            />
          </div>
        </div>
        {listProperties}
        <div className="gf-form-inline">
          <div className={'gf-form'}>
            <Button variant={'secondary'} onClick={this.addProperty}>
              + Select
            </Button>
          </div>
        </div>
        {listExpands}
        <div className="gf-form-inline">
          <div className={'gf-form'}>
            <Button variant={'secondary'} onClick={this.addExpand}>
              + Expand
            </Button>
          </div>
        </div>
        {listFilters}
        <div className={'gf-form'}>
          <Button variant={'secondary'} onClick={this.addFilterCondition}>
            + Filter condition
          </Button>
        </div>
      </div>
    );
  }
}
