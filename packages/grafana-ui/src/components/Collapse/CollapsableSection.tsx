import { css, cx } from '@emotion/css';
import { uniqueId } from 'lodash';
import React, { FC, ReactNode, useRef, useState } from 'react';

import { GrafanaTheme2 } from '@grafana/data';

import { Icon, Spinner } from '..';
import { useStyles2 } from '../../themes';
import { getFocusStyles } from '../../themes/mixins';

export interface Props {
  label: ReactNode;
  isOpen: boolean;
  /** Callback for the toggle functionality */
  onToggle?: (isOpen: boolean) => void;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  loading?: boolean;
  labelId?: string;
}

export const CollapsableSection: FC<Props> = ({
  label,
  isOpen,
  onToggle,
  className,
  contentClassName,
  children,
  labelId,
  loading = false,
}) => {
  const [open, toggleOpen] = useState<boolean>(isOpen);
  const styles = useStyles2(collapsableSectionStyles);

  const onClick = (e: React.MouseEvent) => {
    if (e.target instanceof HTMLElement && e.target.tagName === 'A') {
      return;
    }

    e.preventDefault();
    e.stopPropagation();

    onToggle?.(!open);
    toggleOpen(!open);
  };
  const { current: id } = useRef(uniqueId());

  const buttonLabelId = labelId ?? `collapse-label-${id}`;

  return (
    <>
      <div onClick={onClick} className={cx(styles.header, className)}>
        <button
          id={`collapse-button-${id}`}
          className={styles.button}
          onClick={onClick}
          aria-expanded={open && !loading}
          aria-controls={`collapse-content-${id}`}
          aria-labelledby={buttonLabelId}
        >
          {loading ? (
            <Spinner className={styles.spinner} />
          ) : (
            <Icon name={open ? 'angle-up' : 'angle-down'} className={styles.icon} />
          )}
        </button>
        <div className={styles.label} id={`collapse-label-${id}`}>
          {label}
        </div>
      </div>
      {open && (
        <div id={`collapse-content-${id}`} className={cx(styles.content, contentClassName)}>
          {children}
        </div>
      )}
    </>
  );
};

const collapsableSectionStyles = (theme: GrafanaTheme2) => ({
  header: css({
    display: 'flex',
    cursor: 'pointer',
    boxSizing: 'border-box',
    flexDirection: 'row-reverse',
    position: 'relative',
    justifyContent: 'space-between',
    fontSize: theme.typography.size.lg,
    padding: `${theme.spacing(0.5)} 0`,
    '&:focus-within': getFocusStyles(theme),
  }),
  button: css({
    all: 'unset',
    '&:focus-visible': {
      outline: 'none',
      outlineOffset: 'unset',
      transition: 'none',
      boxShadow: 'none',
    },
  }),
  icon: css({
    color: theme.colors.text.secondary,
  }),
  content: css({
    padding: `${theme.spacing(2)} 0`,
  }),
  spinner: css({
    display: 'flex',
    alignItems: 'center',
    width: theme.v1.spacing.md,
  }),
  label: css({
    display: 'flex',
  }),
});
