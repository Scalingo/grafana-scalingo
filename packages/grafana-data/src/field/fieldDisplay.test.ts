import merge from 'lodash/merge';
import { getFieldDisplayValues, GetFieldDisplayValuesOptions } from './fieldDisplay';
import { toDataFrame } from '../dataframe/processDataFrame';
import { ReducerID } from '../transformations/fieldReducer';
import { ThresholdsMode } from '../types/thresholds';
import { GrafanaTheme } from '../types/theme';
import { MappingType, FieldConfig } from '../types';
import { validateFieldConfig } from './fieldOverrides';
import { standardFieldConfigEditorRegistry } from './standardFieldConfigEditorRegistry';

describe('FieldDisplay', () => {
  beforeAll(() => {
    // Since FieldConfigEditors belong to grafana-ui we need to mock those here
    // as grafana-ui code cannot be imported in grafana-data.
    // TODO: figure out a way to share standard editors between data/ui tests
    const mappings = {
      id: 'mappings', // Match field properties
      process: (value: any) => value,
      shouldApply: () => true,
    } as any;

    console.log('Init tegistry');
    standardFieldConfigEditorRegistry.setInit(() => {
      return [mappings];
    });
  });

  it('show first numeric values', () => {
    const options = createDisplayOptions({
      fieldOptions: {
        calcs: [ReducerID.first],
        override: {},
        defaults: {
          title: '$__cell_0 * $__field_name * $__series_name',
        },
      },
    });
    const display = getFieldDisplayValues(options);
    expect(display.map(v => v.display.text)).toEqual(['1', '2']);
  });

  it('show last numeric values', () => {
    const options = createDisplayOptions({
      fieldOptions: {
        calcs: [ReducerID.last],
        override: {},
        defaults: {},
      },
    });
    const display = getFieldDisplayValues(options);
    expect(display.map(v => v.display.numeric)).toEqual([5, 6]);
  });

  it('show all numeric values', () => {
    const options = createDisplayOptions({
      fieldOptions: {
        values: true, //
        limit: 1000,
        calcs: [],
        override: {},
        defaults: {},
      },
    });
    const display = getFieldDisplayValues(options);
    expect(display.map(v => v.display.numeric)).toEqual([1, 3, 5, 2, 4, 6]);
  });

  it('show 2 numeric values (limit)', () => {
    const options = createDisplayOptions({
      fieldOptions: {
        values: true, //
        limit: 2,
        calcs: [],
        override: {},
        defaults: {},
      },
    });
    const display = getFieldDisplayValues(options);
    expect(display.map(v => v.display.numeric)).toEqual([1, 3]); // First 2 are from the first field
  });

  it('should restore -Infinity value for base threshold', () => {
    const config: FieldConfig = {
      thresholds: {
        mode: ThresholdsMode.Absolute,
        steps: [
          {
            color: '#73BF69',
            value: (null as any) as number, // -Infinity becomes null in JSON
          },
          {
            color: '#F2495C',
            value: 50,
          },
        ],
      },
    };
    validateFieldConfig(config);
    expect(config.thresholds!.steps.length).toEqual(2);
    expect(config.thresholds!.steps[0].value).toBe(-Infinity);
  });

  it('Should return field thresholds when there is no data', () => {
    const options = createEmptyDisplayOptions({
      fieldOptions: {
        defaults: {
          thresholds: { steps: [{ color: '#F2495C', value: 50 }] },
        },
      },
    });

    const display = getFieldDisplayValues(options);
    expect(display[0].field.thresholds!.steps!.length).toEqual(1);
    expect(display[0].display.numeric).toEqual(0);
  });

  it('Should return field with default text when no mapping or data available', () => {
    const options = createEmptyDisplayOptions();
    const display = getFieldDisplayValues(options);
    expect(display[0].display.text).toEqual('No data');
    expect(display[0].display.numeric).toEqual(0);
  });

  it('Should return field mapped value when there is no data', () => {
    const mapEmptyToText = '0';
    const options = createEmptyDisplayOptions({
      fieldOptions: {
        defaults: {
          mappings: [
            {
              id: 1,
              operator: '',
              text: mapEmptyToText,
              type: MappingType.ValueToText,
              value: 'null',
            },
          ],
        },
      },
    });

    const display = getFieldDisplayValues(options);
    expect(display[0].display.text).toEqual(mapEmptyToText);
    expect(display[0].display.numeric).toEqual(0);
  });

  it('Should always return display numeric 0 when there is no data', () => {
    const mapEmptyToText = '0';
    const options = createEmptyDisplayOptions({
      fieldOptions: {
        override: {
          mappings: [
            {
              id: 1,
              operator: '',
              text: mapEmptyToText,
              type: MappingType.ValueToText,
              value: 'null',
            },
          ],
        },
      },
    });

    const display = getFieldDisplayValues(options);
    expect(display[0].display.numeric).toEqual(0);
  });

  describe('Value mapping', () => {
    it('should apply value mapping', () => {
      const options = createDisplayOptions({
        fieldOptions: {
          calcs: [ReducerID.first],
          override: {},
          defaults: {
            mappings: [
              {
                id: 1,
                operator: '',
                text: 'Value mapped to text',
                type: MappingType.ValueToText,
                value: 1,
              },
            ],
          },
        },
      });

      const result = getFieldDisplayValues(options);
      expect(result[0].display.text).toEqual('Value mapped to text');
    });
    it('should apply range value mapping', () => {
      const mappedValue = 'Range mapped to text';
      const options = createDisplayOptions({
        fieldOptions: {
          values: true,
          override: {},
          defaults: {
            mappings: [
              {
                id: 1,
                operator: '',
                text: mappedValue,
                type: MappingType.RangeToText,
                value: 1,
                from: 1,
                to: 3,
              },
            ],
          },
        },
      });
      const result = getFieldDisplayValues(options);

      expect(result[0].display.text).toEqual(mappedValue);
      expect(result[2].display.text).toEqual('5');
      expect(result[3].display.text).toEqual(mappedValue);
    });
  });
});

function createEmptyDisplayOptions(extend = {}): GetFieldDisplayValuesOptions {
  const options = createDisplayOptions(extend);

  return Object.assign(options, {
    data: [
      {
        name: 'No data',
        fields: [],
        length: 0,
      },
    ],
  });
}

function createDisplayOptions(extend = {}): GetFieldDisplayValuesOptions {
  const options: GetFieldDisplayValuesOptions = {
    data: [
      toDataFrame({
        name: 'Series Name',
        fields: [
          { name: 'Field 1', values: ['a', 'b', 'c'] },
          { name: 'Field 2', values: [1, 3, 5] },
          { name: 'Field 3', values: [2, 4, 6] },
        ],
      }),
    ],
    replaceVariables: (value: string) => {
      return value;
    },
    fieldOptions: {
      calcs: [],
      defaults: {},
      overrides: [],
    },
    theme: {} as GrafanaTheme,
  };

  return merge<GetFieldDisplayValuesOptions, any>(options, extend);
}
