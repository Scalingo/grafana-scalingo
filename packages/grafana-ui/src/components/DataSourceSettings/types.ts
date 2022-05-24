import React from 'react';

import { DataSourceSettings } from '@grafana/data';

export interface AzureAuthSettings {
  /** Set to true if Azure authentication supported by the datasource */
  readonly azureAuthSupported: boolean;

  /** Gets whether the Azure authentication currently enabled for the datasource */
  readonly getAzureAuthEnabled: (config: DataSourceSettings<any, any>) => boolean;

  /** Enables/disables the Azure authentication from the datasource */
  readonly setAzureAuthEnabled: (
    config: DataSourceSettings<any, any>,
    enabled: boolean
  ) => Partial<DataSourceSettings<any, any>>;

  /** Optional React component of additional Azure settings UI if authentication is enabled  */
  readonly azureSettingsUI?: React.ComponentType<HttpSettingsBaseProps>;
}

export interface HttpSettingsBaseProps<JSONData = any, SecureJSONData = any> {
  /** The configuration object of the data source */
  dataSourceConfig: DataSourceSettings<JSONData, SecureJSONData>;
  /** Callback for handling changes to the configuration object */
  onChange: (config: DataSourceSettings<JSONData, SecureJSONData>) => void;
  /** Show the Forward OAuth identity option */
  showForwardOAuthIdentityOption?: boolean;
}

export interface HttpSettingsProps extends HttpSettingsBaseProps {
  /** The default url for the data source */
  defaultUrl: string;
  /** Show the http access help box */
  showAccessOptions?: boolean;
  /** Show the SigV4 auth toggle option */
  sigV4AuthToggleEnabled?: boolean;
  /** Azure authentication settings **/
  azureAuthSettings?: AzureAuthSettings;
}
