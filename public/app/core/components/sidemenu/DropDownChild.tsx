import React, { FC } from 'react';
import { css } from 'emotion';
import { Icon, IconName, useTheme } from '@grafana/ui';
import { textUtil } from '@grafana/data';

export interface Props {
  child: any;
}

const DropDownChild: FC<Props> = (props) => {
  const { child } = props;
  const listItemClassName = child.divider ? 'divider' : '';
  const theme = useTheme();
  const iconClassName = css`
    margin-right: ${theme.spacing.sm};
  `;
  const sanitizedUrl = textUtil.sanitizeAngularInterpolation(child.url ?? '');

  return (
    <li className={listItemClassName}>
      <a href={sanitizedUrl}>
        {child.icon && <Icon name={child.icon as IconName} className={iconClassName} />}
        {child.text}
      </a>
    </li>
  );
};

export default DropDownChild;
