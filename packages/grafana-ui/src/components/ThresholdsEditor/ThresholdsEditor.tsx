import React, { PureComponent, ChangeEvent } from 'react';
import { Threshold, sortThresholds, ThresholdsConfig, ThresholdsMode, SelectableValue } from '@grafana/data';
import { colors } from '../../utils';
import { getColorFromHexRgbOrName } from '@grafana/data';
import { Input } from '../Input/Input';
import { ColorPicker } from '../ColorPicker/ColorPicker';
import { Themeable } from '../../types';
import { css } from 'emotion';
import Select from '../Select/Select';
import { PanelOptionsGroup } from '../PanelOptionsGroup/PanelOptionsGroup';

const modes: Array<SelectableValue<ThresholdsMode>> = [
  { value: ThresholdsMode.Absolute, label: 'Absolute', description: 'Pick thresholds based on the absolute values' },
  {
    value: ThresholdsMode.Percentage,
    label: 'Percentage',
    description: 'Pick threshold based on the percent between min/max',
  },
];

export interface Props extends Themeable {
  showAlphaUI?: boolean;
  thresholds: ThresholdsConfig;
  onChange: (thresholds: ThresholdsConfig) => void;
}

interface State {
  steps: ThresholdWithKey[];
}

interface ThresholdWithKey extends Threshold {
  key: number;
}

let counter = 100;

function toThresholdsWithKey(steps?: Threshold[]): ThresholdWithKey[] {
  if (!steps || steps.length === 0) {
    steps = [{ value: -Infinity, color: 'green' }];
  }

  return steps.map(t => {
    return {
      color: t.color,
      value: t.value === null ? -Infinity : t.value,
      key: counter++,
    };
  });
}

export class ThresholdsEditor extends PureComponent<Props, State> {
  constructor(props: Props) {
    super(props);

    const steps = toThresholdsWithKey(props.thresholds!.steps);
    steps[0].value = -Infinity;

    this.state = { steps };
  }

  onAddThresholdAfter = (threshold: ThresholdWithKey) => {
    const { steps } = this.state;

    const maxValue = 100;
    const minValue = 0;

    let prev: ThresholdWithKey | undefined = undefined;
    let next: ThresholdWithKey | undefined = undefined;
    for (const t of steps) {
      if (prev && prev.key === threshold.key) {
        next = t;
        break;
      }
      prev = t;
    }

    const prevValue = prev && isFinite(prev.value) ? prev.value : minValue;
    const nextValue = next && isFinite(next.value) ? next.value : maxValue;

    const color = colors.filter(c => !steps.some(t => t.color === c))[1];
    const add = {
      value: prevValue + (nextValue - prevValue) / 2.0,
      color: color,
      key: counter++,
    };
    const newThresholds = [...steps, add];
    sortThresholds(newThresholds);

    this.setState(
      {
        steps: newThresholds,
      },
      () => this.onChange()
    );
  };

  onRemoveThreshold = (threshold: ThresholdWithKey) => {
    const { steps } = this.state;
    if (!steps.length) {
      return;
    }
    // Don't remove index 0
    if (threshold.key === steps[0].key) {
      return;
    }
    this.setState(
      {
        steps: steps.filter(t => t.key !== threshold.key),
      },
      () => this.onChange()
    );
  };

  onChangeThresholdValue = (event: ChangeEvent<HTMLInputElement>, threshold: ThresholdWithKey) => {
    const cleanValue = event.target.value.replace(/,/g, '.');
    const parsedValue = parseFloat(cleanValue);
    const value = isNaN(parsedValue) ? '' : parsedValue;

    const steps = this.state.steps.map(t => {
      if (t.key === threshold.key) {
        t = { ...t, value: value as number };
      }
      return t;
    });
    if (steps.length) {
      steps[0].value = -Infinity;
    }
    this.setState({ steps });
  };

  onChangeThresholdColor = (threshold: ThresholdWithKey, color: string) => {
    const { steps } = this.state;

    const newThresholds = steps.map(t => {
      if (t.key === threshold.key) {
        t = { ...t, color: color };
      }

      return t;
    });

    this.setState(
      {
        steps: newThresholds,
      },
      () => this.onChange()
    );
  };

  onBlur = () => {
    const steps = [...this.state.steps];
    sortThresholds(steps);
    this.setState(
      {
        steps,
      },
      () => this.onChange()
    );
  };

  onChange = () => {
    this.props.onChange(thresholdsWithoutKey(this.props.thresholds, this.state.steps));
  };

  onModeChanged = (item: SelectableValue<ThresholdsMode>) => {
    if (item.value) {
      this.props.onChange({
        ...this.props.thresholds,
        mode: item.value,
      });
    }
  };

  renderInput = (threshold: ThresholdWithKey) => {
    const isPercent = this.props.thresholds.mode === ThresholdsMode.Percentage;
    return (
      <div className="thresholds-row-input-inner">
        <span className="thresholds-row-input-inner-arrow" />
        <div className="thresholds-row-input-inner-color">
          {threshold.color && (
            <div className="thresholds-row-input-inner-color-colorpicker">
              <ColorPicker
                color={threshold.color}
                onChange={color => this.onChangeThresholdColor(threshold, color)}
                enableNamedColors={true}
              />
            </div>
          )}
        </div>
        {!isFinite(threshold.value) ? (
          <div className="thresholds-row-input-inner-value">
            <Input type="text" value="Base" readOnly />
          </div>
        ) : (
          <>
            <div className="thresholds-row-input-inner-value">
              <Input
                type="number"
                step="0.0001"
                onChange={(event: ChangeEvent<HTMLInputElement>) => this.onChangeThresholdValue(event, threshold)}
                value={threshold.value}
                onBlur={this.onBlur}
              />
            </div>
            {isPercent && (
              <div className={css(`margin-left:-20px; margin-top:5px;`)}>
                <i className="fa fa-percent" />
              </div>
            )}
            <div className="thresholds-row-input-inner-remove" onClick={() => this.onRemoveThreshold(threshold)}>
              <i className="fa fa-times" />
            </div>
          </>
        )}
      </div>
    );
  };

  render() {
    const { steps } = this.state;
    const { theme } = this.props;
    const t = this.props.thresholds;
    return (
      <PanelOptionsGroup title="Thresholds">
        <>
          <div className="thresholds">
            {steps
              .slice(0)
              .reverse()
              .map(threshold => {
                return (
                  <div className="thresholds-row" key={`${threshold.key}`}>
                    <div className="thresholds-row-add-button" onClick={() => this.onAddThresholdAfter(threshold)}>
                      <i className="fa fa-plus" />
                    </div>
                    <div
                      className="thresholds-row-color-indicator"
                      style={{ backgroundColor: getColorFromHexRgbOrName(threshold.color, theme.type) }}
                    />
                    <div className="thresholds-row-input">{this.renderInput(threshold)}</div>
                  </div>
                );
              })}
          </div>

          {this.props.showAlphaUI && (
            <div>
              <Select options={modes} value={modes.filter(m => m.value === t.mode)} onChange={this.onModeChanged} />
            </div>
          )}
        </>
      </PanelOptionsGroup>
    );
  }
}

export function thresholdsWithoutKey(thresholds: ThresholdsConfig, steps: ThresholdWithKey[]): ThresholdsConfig {
  const mode = thresholds.mode ?? ThresholdsMode.Absolute;
  return {
    mode,
    steps: steps.map(t => {
      const { key, ...rest } = t;
      return rest; // everything except key
    }),
  };
}
