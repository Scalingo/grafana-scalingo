import { action } from '@storybook/addon-actions';
import { useArgs } from '@storybook/client-api';
import { ComponentMeta, ComponentStory } from '@storybook/react';
import React from 'react';

import { DataSourceSettings } from '@grafana/data';

import { DataSourceHttpSettings } from './DataSourceHttpSettings';
import mdx from './DataSourceHttpSettings.mdx';

const settingsMock: DataSourceSettings<any, any> = {
  id: 4,
  orgId: 1,
  uid: 'x',
  name: 'gdev-influxdb',
  type: 'influxdb',
  typeName: 'Influxdb',
  typeLogoUrl: '',
  access: 'direct',
  url: 'http://localhost:8086',
  user: 'grafana',
  database: 'site',
  basicAuth: false,
  basicAuthUser: '',
  withCredentials: false,
  isDefault: false,
  jsonData: {
    timeInterval: '15s',
    httpMode: 'GET',
    keepCookies: ['cookie1', 'cookie2'],
    serverName: '',
  },
  secureJsonData: {
    password: true,
  },
  secureJsonFields: {},
  readOnly: true,
};

const meta: ComponentMeta<typeof DataSourceHttpSettings> = {
  title: 'Data Source/DataSourceHttpSettings',
  component: DataSourceHttpSettings,
  parameters: {
    controls: {
      exclude: ['onChange'],
    },
    docs: {
      page: mdx,
    },
  },
  args: {
    dataSourceConfig: settingsMock,
    defaultUrl: 'http://localhost:9999',
  },
};

export const Basic: ComponentStory<typeof DataSourceHttpSettings> = (args) => {
  const [, updateArgs] = useArgs();
  return (
    <DataSourceHttpSettings
      {...args}
      onChange={(change: typeof settingsMock) => {
        action('onChange')(change);
        updateArgs({ dataSourceConfig: change });
      }}
    />
  );
};

export default meta;
