import {
  DataSourcePluginOptionsEditorProps,
  SelectableValue
} from '@grafana/data';
import {DataSourceHttpSettings, FieldSet, InlineField, InlineFieldRow, Select, Input, SecretInput, Switch} from '@grafana/ui';
import React, {ComponentType, useCallback} from 'react';
import {ODataOptions, ODataSecureJsonData, URLSpaceEncoding} from '../types';

type Props = DataSourcePluginOptionsEditorProps<ODataOptions, ODataSecureJsonData>;

export const ConfigEditor: ComponentType<Props> = ({ options, onOptionsChange }) => {
  const onURLSpaceEncodingChange = useCallback((option: SelectableValue<URLSpaceEncoding>) => {
      const urlSpaceEncoding = option.value;
      onOptionsChange({
        ...options,
        jsonData: {
          ...options.jsonData,
          urlSpaceEncoding: urlSpaceEncoding || '+',
        },
      });
  }, [onOptionsChange, options]);

  const onOAuth2EnabledChange = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        oauth2Enabled: event.target.checked,
      },
    });
  }, [onOptionsChange, options]);

  const onOAuth2UrlChange = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...options.jsonData,
        oauth2Url: event.target.value,
      },
    });
  }, [onOptionsChange, options]);

  const onOAuth2ApiKeyChange = useCallback((event: React.ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        ...options.secureJsonData,
        oauth2ApiKey: event.target.value,
      },
    });
  }, [onOptionsChange, options]);

  const onResetOAuth2ApiKey = useCallback(() => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        oauth2ApiKey: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        oauth2ApiKey: '',
      },
    });
  }, [onOptionsChange, options]);

  const urlSpaceEncodings = Object.entries(URLSpaceEncoding)
    .map(([label, value]) => ({ label: `${label} (${value})`, value: value }));

  return (
    <>
      <DataSourceHttpSettings
        defaultUrl={'http://localhost:5000/odata'}
        dataSourceConfig={options}
        showAccessOptions={false}
        onChange={onOptionsChange}
      />
      <div className='gf-form-group'>
        <h3 className='page-heading'>Additional settings</h3>
        <FieldSet>
          <InlineFieldRow>
            <InlineField
              label='URL space encoding'
              labelWidth={26}
              tooltip={
                <p>
                  Select the standard for encoding spaces in URLs. <i>Percent</i> uses <code>%20</code> (see RFC 3986),
                  while <i>Plus</i> uses <code>+</code> (used in form data). E.g. <code>$filter=value%20EQ%201</code>
                  (Percent) and <code>`$filter=value+EQ+1`</code> (Plus).
                </p>
              }>
              <Select
                options={urlSpaceEncodings}
                value={options.jsonData.urlSpaceEncoding?.length > 0
                  ? urlSpaceEncodings.find((type) => type.value === options.jsonData.urlSpaceEncoding)
                  : URLSpaceEncoding.Plus}
                className='width-10'
                onChange={onURLSpaceEncodingChange}
              />
            </InlineField>
          </InlineFieldRow>
        </FieldSet>
      </div>
      <div className='gf-form-group'>
        <h3 className='page-heading'>OAuth2 Authentication</h3>
        <FieldSet>
          <InlineFieldRow>
            <InlineField label='Enable OAuth2' labelWidth={26}>
              <Switch
                value={options.jsonData.oauth2Enabled || false}
                onChange={onOAuth2EnabledChange}
              />
            </InlineField>
          </InlineFieldRow>
          {options.jsonData.oauth2Enabled && (
            <>
              <InlineFieldRow>
                <InlineField
                  label='OAuth2 Base URL'
                  labelWidth={26}
                  tooltip='The base URL of your OAuth2 server (e.g., https://oauth2.fledge.nl)'
                  grow
                >
                  <Input
                    value={options.jsonData.oauth2Url || ''}
                    onChange={onOAuth2UrlChange}
                    placeholder='https://oauth2.fledge.nl'
                  />
                </InlineField>
              </InlineFieldRow>
              <InlineFieldRow>
                <InlineField
                  label='API Key'
                  labelWidth={26}
                  tooltip='Your API key for OAuth2 authentication (stored securely)'
                  grow
                >
                  <SecretInput
                    isConfigured={Boolean(options.secureJsonFields?.oauth2ApiKey)}
                    value={options.secureJsonData?.oauth2ApiKey || ''}
                    onChange={onOAuth2ApiKeyChange}
                    onReset={onResetOAuth2ApiKey}
                    placeholder='your-api-key'
                  />
                </InlineField>
              </InlineFieldRow>
            </>
          )}
        </FieldSet>
      </div>
      </>
  );
};
