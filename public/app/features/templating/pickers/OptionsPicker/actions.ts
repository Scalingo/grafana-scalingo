import debounce from 'lodash/debounce';
import { StoreState, ThunkDispatch, ThunkResult } from 'app/types';
import {
  containsSearchFilter,
  QueryVariableModel,
  VariableOption,
  VariableRefresh,
  VariableTag,
  VariableWithMultiSupport,
  VariableWithOptions,
} from '../../variable';
import { variableAdapters } from '../../adapters';
import { getVariable } from '../../state/selectors';
import { NavigationKey } from '../types';
import {
  hideOptions,
  moveOptionsHighlight,
  OptionsPickerState,
  toggleOption,
  toggleTag,
  updateOptionsAndFilter,
  updateOptionsFromSearch,
  updateSearchQuery,
} from './reducer';
import { getDataSourceSrv } from '@grafana/runtime';
import { getTimeSrv } from 'app/features/dashboard/services/TimeSrv';
import { setCurrentVariableValue, changeVariableProp } from '../../state/sharedReducer';
import { toVariablePayload } from '../../state/types';

export const navigateOptions = (key: NavigationKey, clearOthers: boolean): ThunkResult<void> => {
  return async (dispatch, getState) => {
    if (key === NavigationKey.cancel) {
      return await dispatch(commitChangesToVariable());
    }

    if (key === NavigationKey.select) {
      return dispatch(toggleOptionByHighlight(clearOthers));
    }

    if (key === NavigationKey.selectAndClose) {
      dispatch(toggleOptionByHighlight(clearOthers));
      return await dispatch(commitChangesToVariable());
    }

    if (key === NavigationKey.moveDown) {
      return dispatch(moveOptionsHighlight(1));
    }

    if (key === NavigationKey.moveUp) {
      return dispatch(moveOptionsHighlight(-1));
    }

    return undefined;
  };
};

export const filterOrSearchOptions = (searchQuery: string): ThunkResult<void> => {
  return async (dispatch, getState) => {
    const { uuid } = getState().templating.optionsPicker;
    const { query, options } = getVariable<VariableWithOptions>(uuid!, getState());
    dispatch(updateSearchQuery(searchQuery));

    if (containsSearchFilter(query)) {
      return searchForOptionsWithDebounce(dispatch, getState, searchQuery);
    }
    return dispatch(updateOptionsAndFilter(options));
  };
};

export const commitChangesToVariable = (): ThunkResult<void> => {
  return async (dispatch, getState) => {
    const picker = getState().templating.optionsPicker;
    const existing = getVariable<VariableWithMultiSupport>(picker.uuid, getState());
    const currentPayload = { option: mapToCurrent(picker) };
    const searchQueryPayload = { propName: 'queryValue', propValue: picker.queryValue };

    dispatch(setCurrentVariableValue(toVariablePayload(existing, currentPayload)));
    dispatch(changeVariableProp(toVariablePayload(existing, searchQueryPayload)));
    const updated = getVariable<VariableWithMultiSupport>(picker.uuid, getState());

    if (existing.current.text === updated.current.text) {
      return dispatch(hideOptions());
    }

    const adapter = variableAdapters.get(updated.type);
    await adapter.setValue(updated, updated.current, true);
    return dispatch(hideOptions());
  };
};

export const toggleOptionByHighlight = (clearOthers: boolean): ThunkResult<void> => {
  return (dispatch, getState) => {
    const { uuid, highlightIndex } = getState().templating.optionsPicker;
    const variable = getVariable<VariableWithMultiSupport>(uuid, getState());
    const option = variable.options[highlightIndex];
    dispatch(toggleOption({ option, forceSelect: false, clearOthers }));
  };
};

export const toggleAndFetchTag = (tag: VariableTag): ThunkResult<void> => {
  return async (dispatch, getState) => {
    if (Array.isArray(tag.values)) {
      return dispatch(toggleTag(tag));
    }

    const values = await dispatch(fetchTagValues(tag.text.toString()));
    return dispatch(toggleTag({ ...tag, values }));
  };
};

const fetchTagValues = (tagText: string): ThunkResult<Promise<string[]>> => {
  return async (dispatch, getState) => {
    const picker = getState().templating.optionsPicker;
    const variable = getVariable<QueryVariableModel>(picker.uuid, getState());

    const datasource = await getDataSourceSrv().get(variable.datasource ?? '');
    const query = variable.tagValuesQuery.replace('$tag', tagText);
    const options = { range: getTimeRange(variable), variable };

    if (!datasource.metricFindQuery) {
      return [];
    }

    const results = await datasource.metricFindQuery(query, options);

    if (!Array.isArray(results)) {
      return [];
    }
    return results.map(value => value.text);
  };
};

const getTimeRange = (variable: QueryVariableModel) => {
  if (variable.refresh === VariableRefresh.onTimeRangeChanged) {
    return getTimeSrv().timeRange();
  }
  return undefined;
};

const searchForOptions = async (dispatch: ThunkDispatch, getState: () => StoreState, searchQuery: string) => {
  try {
    const { uuid } = getState().templating.optionsPicker;
    const existing = getVariable<VariableWithOptions>(uuid, getState());

    const adapter = variableAdapters.get(existing.type);
    await adapter.updateOptions(existing, searchQuery);

    const updated = getVariable<VariableWithOptions>(uuid, getState());
    dispatch(updateOptionsFromSearch(updated.options));
  } catch (error) {
    console.error(error);
  }
};

const searchForOptionsWithDebounce = debounce(searchForOptions, 500);

function mapToCurrent(picker: OptionsPickerState): VariableOption | undefined {
  const { options, queryValue: searchQuery, multi } = picker;

  if (options.length === 0 && searchQuery && searchQuery.length > 0) {
    return { text: searchQuery, value: searchQuery, selected: false };
  }

  if (!multi) {
    return options.find(o => o.selected);
  }

  const texts: string[] = [];
  const values: string[] = [];

  for (const option of options) {
    if (!option.selected) {
      continue;
    }

    texts.push(option.text.toString());
    values.push(option.value.toString());
  }

  return {
    value: values,
    text: texts.join(' + '),
    tags: picker.tags.filter(t => t.selected),
    selected: true,
  };
}
