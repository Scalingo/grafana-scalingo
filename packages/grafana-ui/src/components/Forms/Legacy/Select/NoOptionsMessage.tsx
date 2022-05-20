import React from 'react';
import { components, NoticeProps, GroupBase } from 'react-select';

import { SelectableValue } from '@grafana/data';

export type Props<T> = NoticeProps<SelectableValue<T>, boolean, GroupBase<SelectableValue<T>>>;

export const NoOptionsMessage = <T extends unknown>(props: Props<T>) => {
  const { children } = props;
  return (
    <components.NoOptionsMessage {...props}>
      <div className="gf-form-select-box__desc-option">
        <div className="gf-form-select-box__desc-option__body">{children}</div>
      </div>
    </components.NoOptionsMessage>
  );
};

export default NoOptionsMessage;
