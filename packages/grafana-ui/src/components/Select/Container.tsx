import { css, cx } from '@emotion/css';
import React from 'react';
import { components, ContainerProps, GroupBase } from 'react-select';

import { GrafanaTheme2 } from '@grafana/data';

import { stylesFactory } from '../../themes';
import { useTheme2 } from '../../themes/ThemeContext';
import { focusCss } from '../../themes/mixins';
import { sharedInputStyle } from '../Forms/commonStyles';
import { getInputStyles } from '../Input/Input';

export const SelectContainer = <Option, isMulti extends boolean, Group extends GroupBase<Option>>(
  props: ContainerProps<Option, isMulti, Group> & { isFocused: boolean }
) => {
  const { isDisabled, isFocused, children } = props;

  const theme = useTheme2();
  const styles = getSelectContainerStyles(theme, isFocused, isDisabled);

  return (
    <components.SelectContainer {...props} className={cx(styles.wrapper, props.className)}>
      {children}
    </components.SelectContainer>
  );
};

const getSelectContainerStyles = stylesFactory((theme: GrafanaTheme2, focused: boolean, disabled: boolean) => {
  const styles = getInputStyles({ theme, invalid: false });

  return {
    wrapper: cx(
      styles.wrapper,
      sharedInputStyle(theme, false),
      focused &&
        css`
          ${focusCss(theme.v1)}
        `,
      disabled && styles.inputDisabled,
      css`
        position: relative;
        box-sizing: border-box;
        display: flex;
        flex-direction: row;
        flex-wrap: wrap;
        align-items: center;
        justify-content: space-between;

        min-height: 32px;
        height: auto;
        max-width: 100%;

        /* Input padding is applied to the InputControl so the menu is aligned correctly */
        padding: 0;
        cursor: ${disabled ? 'not-allowed' : 'pointer'};
      `
    ),
  };
});
