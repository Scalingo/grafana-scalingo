import React from 'react';
import { PanelPlugin } from '@grafana/data';
import { TagsInput } from '@grafana/ui';
import { AlertList } from './AlertList';
import { UnifiedAlertList } from './UnifiedAlertList';
import { AlertListOptions, ShowOption, SortOrder, UnifiedAlertListOptions } from './types';
import { alertListPanelMigrationHandler } from './AlertListMigrationHandler';
import { config, DataSourcePicker } from '@grafana/runtime';
import { RuleFolderPicker } from 'app/features/alerting/unified/components/rule-editor/RuleFolderPicker';
import {
  ALL_FOLDER,
  GENERAL_FOLDER,
  ReadonlyFolderPicker,
} from '../../../core/components/Select/ReadonlyFolderPicker/ReadonlyFolderPicker';
import { AlertListSuggestionsSupplier } from './suggestions';

function showIfCurrentState(options: AlertListOptions) {
  return options.showOptions === ShowOption.Current;
}

const alertList = new PanelPlugin<AlertListOptions>(AlertList)
  .setPanelOptions((builder) => {
    builder
      .addSelect({
        name: 'Show',
        path: 'showOptions',
        settings: {
          options: [
            { label: 'Current state', value: ShowOption.Current },
            { label: 'Recent state changes', value: ShowOption.RecentChanges },
          ],
        },
        defaultValue: ShowOption.Current,
        category: ['Options'],
      })
      .addNumberInput({
        name: 'Max items',
        path: 'maxItems',
        defaultValue: 10,
        category: ['Options'],
      })
      .addSelect({
        name: 'Sort order',
        path: 'sortOrder',
        settings: {
          options: [
            { label: 'Alphabetical (asc)', value: SortOrder.AlphaAsc },
            { label: 'Alphabetical (desc)', value: SortOrder.AlphaDesc },
            { label: 'Importance', value: SortOrder.Importance },
            { label: 'Time (asc)', value: SortOrder.TimeAsc },
            { label: 'Time (desc)', value: SortOrder.TimeDesc },
          ],
        },
        defaultValue: SortOrder.AlphaAsc,
        category: ['Options'],
      })
      .addBooleanSwitch({
        path: 'dashboardAlerts',
        name: 'Alerts from this dashboard',
        defaultValue: false,
        category: ['Options'],
      })
      .addTextInput({
        path: 'alertName',
        name: 'Alert name',
        defaultValue: '',
        category: ['Filter'],
        showIf: showIfCurrentState,
      })
      .addTextInput({
        path: 'dashboardTitle',
        name: 'Dashboard title',
        defaultValue: '',
        category: ['Filter'],
        showIf: showIfCurrentState,
      })
      .addCustomEditor({
        path: 'folderId',
        name: 'Folder',
        id: 'folderId',
        defaultValue: null,
        editor: function RenderFolderPicker({ value, onChange }) {
          return (
            <ReadonlyFolderPicker
              initialFolderId={value}
              onChange={(folder) => onChange(folder?.id)}
              extraFolders={[ALL_FOLDER, GENERAL_FOLDER]}
            />
          );
        },
        category: ['Filter'],
        showIf: showIfCurrentState,
      })
      .addCustomEditor({
        id: 'tags',
        path: 'tags',
        name: 'Tags',
        description: '',
        defaultValue: [],
        editor(props) {
          return <TagsInput tags={props.value} onChange={props.onChange} />;
        },
        category: ['Filter'],
        showIf: showIfCurrentState,
      })
      .addBooleanSwitch({
        path: 'stateFilter.ok',
        name: 'Ok',
        defaultValue: false,
        category: ['State filter'],
        showIf: showIfCurrentState,
      })
      .addBooleanSwitch({
        path: 'stateFilter.paused',
        name: 'Paused',
        defaultValue: false,
        category: ['State filter'],
        showIf: showIfCurrentState,
      })
      .addBooleanSwitch({
        path: 'stateFilter.no_data',
        name: 'No data',
        defaultValue: false,
        category: ['State filter'],
        showIf: showIfCurrentState,
      })
      .addBooleanSwitch({
        path: 'stateFilter.execution_error',
        name: 'Execution error',
        defaultValue: false,
        category: ['State filter'],
        showIf: showIfCurrentState,
      })
      .addBooleanSwitch({
        path: 'stateFilter.alerting',
        name: 'Alerting',
        defaultValue: false,
        category: ['State filter'],
        showIf: showIfCurrentState,
      })
      .addBooleanSwitch({
        path: 'stateFilter.pending',
        name: 'Pending',
        defaultValue: false,
        category: ['State filter'],
        showIf: showIfCurrentState,
      });
  })
  .setMigrationHandler(alertListPanelMigrationHandler)
  .setSuggestionsSupplier(new AlertListSuggestionsSupplier());

const unifiedAlertList = new PanelPlugin<UnifiedAlertListOptions>(UnifiedAlertList).setPanelOptions((builder) => {
  builder
    .addNumberInput({
      name: 'Max items',
      path: 'maxItems',
      description: 'Maximum alerts to display',
      defaultValue: 20,
      category: ['Options'],
    })
    .addSelect({
      name: 'Sort order',
      path: 'sortOrder',
      description: 'Sort order of alerts and alert instances',
      settings: {
        options: [
          { label: 'Alphabetical (asc)', value: SortOrder.AlphaAsc },
          { label: 'Alphabetical (desc)', value: SortOrder.AlphaDesc },
          { label: 'Importance', value: SortOrder.Importance },
          { label: 'Time (asc)', value: SortOrder.TimeAsc },
          { label: 'Time (desc)', value: SortOrder.TimeDesc },
        ],
      },
      defaultValue: SortOrder.AlphaAsc,
      category: ['Options'],
    })
    .addBooleanSwitch({
      path: 'dashboardAlerts',
      name: 'Alerts from this dashboard',
      description: 'Show alerts from this dashboard',
      defaultValue: false,
      category: ['Options'],
    })
    .addBooleanSwitch({
      path: 'showInstances',
      name: 'Show alert instances',
      description: 'Show individual alert instances for multi-dimensional rules',
      defaultValue: false,
      category: ['Options'],
    })
    .addTextInput({
      path: 'alertName',
      name: 'Alert name',
      description: 'Filter for alerts containing this text',
      defaultValue: '',
      category: ['Filter'],
    })
    .addTextInput({
      path: 'alertInstanceLabelFilter',
      name: 'Alert instance label',
      description: 'Filter alert instances using label querying, ex: {severity="critical", instance=~"cluster-us-.+"}',
      defaultValue: '',
      category: ['Filter'],
    })
    .addCustomEditor({
      path: 'folder',
      name: 'Folder',
      description: 'Filter for alerts in the selected folder',
      id: 'folder',
      defaultValue: null,
      editor: function RenderFolderPicker(props) {
        return (
          <RuleFolderPicker
            {...props}
            enableReset={true}
            onChange={({ title, id }) => {
              return props.onChange({ title, id });
            }}
          />
        );
      },
      category: ['Filter'],
    })
    .addCustomEditor({
      path: 'datasource',
      name: 'Datasource',
      description: 'Filter alerts from selected datasource',
      id: 'datasource',
      defaultValue: null,
      editor: function RenderDatasourcePicker(props) {
        return (
          <DataSourcePicker
            {...props}
            type={['prometheus', 'loki', 'grafana']}
            noDefault
            current={props.value}
            onChange={(ds) => props.onChange(ds.name)}
            onClear={() => props.onChange('')}
          />
        );
      },
      category: ['Filter'],
    })
    .addBooleanSwitch({
      path: 'stateFilter.firing',
      name: 'Alerting / Firing',
      defaultValue: true,
      category: ['Alert state filter'],
    })
    .addBooleanSwitch({
      path: 'stateFilter.pending',
      name: 'Pending',
      defaultValue: true,
      category: ['Alert state filter'],
    })
    .addBooleanSwitch({
      path: 'stateFilter.inactive',
      name: 'Inactive',
      defaultValue: false,
      category: ['Alert state filter'],
    })
    .addBooleanSwitch({
      path: 'stateFilter.noData',
      name: 'No Data',
      defaultValue: false,
      category: ['Alert state filter'],
    })
    .addBooleanSwitch({
      path: 'stateFilter.normal',
      name: 'Normal',
      defaultValue: false,
      category: ['Alert state filter'],
    })
    .addBooleanSwitch({
      path: 'stateFilter.error',
      name: 'Error',
      defaultValue: true,
      category: ['Alert state filter'],
    });
});

export const plugin = config.unifiedAlertingEnabled ? unifiedAlertList : alertList;
