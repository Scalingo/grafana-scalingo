import { cx, css } from '@emotion/css';
import React, { FC } from 'react';

import { GrafanaTheme2 } from '@grafana/data';

import { useTheme2 } from '../../themes';
import { getChildId } from '../../utils/reactUtils';
import { PopoverContent } from '../Tooltip';

import { FieldProps } from './Field';
import { FieldValidationMessage } from './FieldValidationMessage';
import { InlineLabel } from './InlineLabel';

export interface Props extends Omit<FieldProps, 'css' | 'horizontal' | 'description' | 'error'> {
  /** Content for the label's tooltip */
  tooltip?: PopoverContent;
  /** Custom width for the label as a multiple of 8px */
  labelWidth?: number | 'auto';
  /** Make the field's child to fill the width of the row. Equivalent to setting `flex-grow:1` on the field */
  grow?: boolean;
  /** Make the field's child shrink with width of the row. Equivalent to setting `flex-shrink:1` on the field */
  shrink?: boolean;
  /** Make field's background transparent */
  transparent?: boolean;
  /** Error message to display */
  error?: string | null;
  htmlFor?: string;
  /** Make tooltip interactive */
  interactive?: boolean;
}

export const InlineField: FC<Props> = ({
  children,
  label,
  tooltip,
  labelWidth = 'auto',
  invalid,
  loading,
  disabled,
  className,
  htmlFor,
  grow,
  shrink,
  error,
  transparent,
  interactive,
  ...htmlProps
}) => {
  const theme = useTheme2();
  const styles = getStyles(theme, grow, shrink);
  const inputId = htmlFor ?? getChildId(children);

  const labelElement =
    typeof label === 'string' ? (
      <InlineLabel
        interactive={interactive}
        width={labelWidth}
        tooltip={tooltip}
        htmlFor={inputId}
        transparent={transparent}
      >
        {label}
      </InlineLabel>
    ) : (
      label
    );

  return (
    <div className={cx(styles.container, className)} {...htmlProps}>
      {labelElement}
      <div className={styles.childContainer}>
        {React.cloneElement(children, { invalid, disabled, loading })}
        {invalid && error && (
          <div className={cx(styles.fieldValidationWrapper)}>
            <FieldValidationMessage>{error}</FieldValidationMessage>
          </div>
        )}
      </div>
    </div>
  );
};

InlineField.displayName = 'InlineField';

const getStyles = (theme: GrafanaTheme2, grow?: boolean, shrink?: boolean) => {
  return {
    container: css`
      display: flex;
      flex-direction: row;
      align-items: flex-start;
      text-align: left;
      position: relative;
      flex: ${grow ? 1 : 0} ${shrink ? 1 : 0} auto;
      margin: 0 ${theme.spacing(0.5)} ${theme.spacing(0.5)} 0;
    `,
    childContainer: css`
      flex: ${grow ? 1 : 0} ${shrink ? 1 : 0} auto;
    `,
    fieldValidationWrapper: css`
      margin-top: ${theme.spacing(0.5)};
    `,
  };
};
