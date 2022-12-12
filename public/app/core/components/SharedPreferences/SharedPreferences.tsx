import { css } from '@emotion/css';
import React, { PureComponent } from 'react';

import { FeatureState, SelectableValue } from '@grafana/data';
import { selectors } from '@grafana/e2e-selectors';
import { config } from '@grafana/runtime';
import {
  Button,
  Field,
  FieldSet,
  Form,
  Label,
  RadioButtonGroup,
  Select,
  stylesFactory,
  TimeZonePicker,
  WeekStartPicker,
  FeatureBadge,
} from '@grafana/ui';
import { DashboardPicker } from 'app/core/components/Select/DashboardPicker';
import { t, Trans } from 'app/core/internationalization';
import { LOCALES } from 'app/core/internationalization/constants';
import { PreferencesService } from 'app/core/services/PreferencesService';
import { UserPreferencesDTO } from 'app/types';

export interface Props {
  resourceUri: string;
  disabled?: boolean;
  onConfirm?: () => Promise<boolean>;
}

export type State = UserPreferencesDTO;

const themes: SelectableValue[] = [
  { value: '', label: t('shared-preferences.theme.default-label', 'Default') },
  { value: 'dark', label: t('shared-preferences.theme.dark-label', 'Dark') },
  { value: 'light', label: t('shared-preferences.theme.light-label', 'Light') },
];

function getLanguageOptions(): Array<SelectableValue<string>> {
  const languageOptions = LOCALES.map((v) => ({
    value: v.code,
    label: v.name,
  }));

  const options = [
    {
      value: '',
      label: t('common.locale.default', 'Default'),
    },
    ...languageOptions,
  ];

  return options;
}

const i18nFlag = Boolean(config.featureToggles.internationalization);

export class SharedPreferences extends PureComponent<Props, State> {
  service: PreferencesService;

  constructor(props: Props) {
    super(props);

    this.service = new PreferencesService(props.resourceUri);
    this.state = {
      theme: '',
      timezone: '',
      weekStart: '',
      locale: '',
      queryHistory: { homeTab: '' },
    };
  }

  async componentDidMount() {
    const prefs = await this.service.load();

    this.setState({
      homeDashboardUID: prefs.homeDashboardUID,
      theme: prefs.theme,
      timezone: prefs.timezone,
      weekStart: prefs.weekStart,
      locale: prefs.locale,
      queryHistory: prefs.queryHistory,
    });
  }

  onSubmitForm = async () => {
    const confirmationResult = this.props.onConfirm ? await this.props.onConfirm() : true;

    if (confirmationResult) {
      const { homeDashboardUID, theme, timezone, weekStart, locale, queryHistory } = this.state;
      await this.service.update({ homeDashboardUID, theme, timezone, weekStart, locale, queryHistory });
      window.location.reload();
    }
  };

  onThemeChanged = (value: string) => {
    this.setState({ theme: value });
  };

  onTimeZoneChanged = (timezone?: string) => {
    if (!timezone) {
      return;
    }
    this.setState({ timezone: timezone });
  };

  onWeekStartChanged = (weekStart: string) => {
    this.setState({ weekStart: weekStart });
  };

  onHomeDashboardChanged = (dashboardUID: string) => {
    this.setState({ homeDashboardUID: dashboardUID });
  };

  onLocaleChanged = (locale: string) => {
    this.setState({ locale });
  };

  render() {
    const { theme, timezone, weekStart, homeDashboardUID, locale } = this.state;
    const { disabled } = this.props;
    const styles = getStyles();
    const languages = getLanguageOptions();

    return (
      <Form onSubmit={this.onSubmitForm}>
        {() => {
          return (
            <FieldSet label={<Trans i18nKey="shared-preferences.title">Preferences</Trans>} disabled={disabled}>
              <Field label={t('shared-preferences.fields.theme-label', 'UI Theme')}>
                <RadioButtonGroup
                  options={themes}
                  value={themes.find((item) => item.value === theme)?.value}
                  onChange={this.onThemeChanged}
                />
              </Field>

              <Field
                label={
                  <Label htmlFor="home-dashboard-select">
                    <span className={styles.labelText}>
                      <Trans i18nKey="shared-preferences.fields.home-dashboard-label">Home Dashboard</Trans>
                    </span>
                  </Label>
                }
                data-testid="User preferences home dashboard drop down"
              >
                <DashboardPicker
                  value={homeDashboardUID}
                  onChange={(v) => this.onHomeDashboardChanged(v?.uid ?? '')}
                  defaultOptions={true}
                  isClearable={true}
                  placeholder={t('shared-preferences.fields.home-dashboard-placeholder', 'Default dashboard')}
                  inputId="home-dashboard-select"
                />
              </Field>

              <Field
                label={t('shared-dashboard.fields.timezone-label', 'Timezone')}
                data-testid={selectors.components.TimeZonePicker.containerV2}
              >
                <TimeZonePicker
                  includeInternal={true}
                  value={timezone}
                  onChange={this.onTimeZoneChanged}
                  inputId="shared-preferences-timezone-picker"
                />
              </Field>

              <Field
                label={t('shared-preferences.fields.week-start-label', 'Week start')}
                data-testid={selectors.components.WeekStartPicker.containerV2}
              >
                <WeekStartPicker
                  value={weekStart}
                  onChange={this.onWeekStartChanged}
                  inputId={'shared-preferences-week-start-picker'}
                />
              </Field>

              {i18nFlag ? (
                <Field
                  label={
                    <Label htmlFor="locale-select">
                      <span className={styles.labelText}>
                        <Trans i18nKey="shared-preferences.fields.locale-label">Language</Trans>
                      </span>
                      <FeatureBadge featureState={FeatureState.beta} />
                    </Label>
                  }
                  data-testid="User preferences language drop down"
                >
                  <Select
                    value={languages.find((lang) => lang.value === locale)}
                    onChange={(locale: SelectableValue<string>) => this.onLocaleChanged(locale.value ?? '')}
                    options={languages}
                    placeholder={t('shared-preferences.fields.locale-placeholder', 'Choose language')}
                    inputId="locale-select"
                  />
                </Field>
              ) : null}

              <div className="gf-form-button-row">
                <Button
                  type="submit"
                  variant="primary"
                  data-testid={selectors.components.UserProfile.preferencesSaveButton}
                >
                  <Trans i18nKey="common.save">Save</Trans>
                </Button>
              </div>
            </FieldSet>
          );
        }}
      </Form>
    );
  }
}

export default SharedPreferences;

const getStyles = stylesFactory(() => {
  return {
    labelText: css`
      margin-right: 6px;
    `,
  };
});
