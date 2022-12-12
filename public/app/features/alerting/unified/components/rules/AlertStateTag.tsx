import React, { FC } from 'react';

import { AlertState } from '@grafana/data';
import { GrafanaAlertState, GrafanaAlertStateWithReason, PromAlertingRuleState } from 'app/types/unified-alerting-dto';

import { alertStateToReadable, alertStateToState } from '../../utils/rules';
import { StateTag } from '../StateTag';
interface Props {
  state: PromAlertingRuleState | GrafanaAlertState | GrafanaAlertStateWithReason | AlertState;
  size?: 'md' | 'sm';
}

export const AlertStateTag: FC<Props> = ({ state, size = 'md' }) => (
  <StateTag state={alertStateToState(state)} size={size}>
    {alertStateToReadable(state)}
  </StateTag>
);
