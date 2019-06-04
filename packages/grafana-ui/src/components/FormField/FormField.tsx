import React, { InputHTMLAttributes, FunctionComponent } from 'react';
import { FormLabel } from '../FormLabel/FormLabel';
import { PopperContent } from '../Tooltip/PopperController';

export interface Props extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  tooltip?: PopperContent<any>;
  labelWidth?: number;
  inputWidth?: number;
  inputEl?: React.ReactNode;
}

const defaultProps = {
  labelWidth: 6,
  inputWidth: 12,
};

/**
 * Default form field including label used in Grafana UI. Default input element is simple <input />. You can also pass
 * custom inputEl if required in which case inputWidth and inputProps are ignored.
 */
export const FormField: FunctionComponent<Props> = ({
  label,
  tooltip,
  labelWidth,
  inputWidth,
  inputEl,
  ...inputProps
}) => {
  return (
    <div className="form-field">
      <FormLabel width={labelWidth} tooltip={tooltip}>
        {label}
      </FormLabel>
      {inputEl || <input type="text" className={`gf-form-input width-${inputWidth}`} {...inputProps} />}
    </div>
  );
};

FormField.displayName = 'FormField';
FormField.defaultProps = defaultProps;
