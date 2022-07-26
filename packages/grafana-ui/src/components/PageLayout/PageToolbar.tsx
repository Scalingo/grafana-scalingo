import { css, cx } from '@emotion/css';
import React, { FC, ReactNode } from 'react';

import { GrafanaTheme2 } from '@grafana/data';
import { selectors } from '@grafana/e2e-selectors';

import { Link } from '..';
import { styleMixins } from '../../themes';
import { useStyles2 } from '../../themes/ThemeContext';
import { getFocusStyles } from '../../themes/mixins';
import { IconName } from '../../types';
import { Icon } from '../Icon/Icon';
import { IconButton } from '../IconButton/IconButton';

export interface Props {
  pageIcon?: IconName;
  title?: string;
  section?: string;
  parent?: string;
  onGoBack?: () => void;
  titleHref?: string;
  parentHref?: string;
  leftItems?: ReactNode[];
  children?: ReactNode;
  className?: string;
  isFullscreen?: boolean;
  'aria-label'?: string;
}

/** @alpha */
export const PageToolbar: FC<Props> = React.memo(
  ({
    title,
    section,
    parent,
    pageIcon,
    onGoBack,
    children,
    titleHref,
    parentHref,
    leftItems,
    isFullscreen,
    className,
    /** main nav-container aria-label **/
    'aria-label': ariaLabel,
  }) => {
    const styles = useStyles2(getStyles);

    /**
     * .page-toolbar css class is used for some legacy css view modes (TV/Kiosk) and
     * media queries for mobile view when toolbar needs left padding to make room
     * for mobile menu icon. This logic hopefylly can be changed when we move to a full react
     * app and change how the app side menu & mobile menu is rendered.
     */
    const mainStyle = cx(
      'page-toolbar',
      styles.toolbar,
      {
        ['page-toolbar--fullscreen']: isFullscreen,
      },
      className
    );

    const leftItemChildren = leftItems?.map((child, index) => (
      <div className={styles.leftActionItem} key={index}>
        {child}
      </div>
    ));

    const titleEl = (
      <>
        <span className={styles.noLinkTitle}>{title}</span>
        {section && <span className={styles.pre}> / {section}</span>}
      </>
    );

    return (
      <nav className={mainStyle} aria-label={ariaLabel}>
        <div className={styles.leftWrapper}>
          {pageIcon && !onGoBack && (
            <div className={styles.pageIcon}>
              <Icon name={pageIcon} size="lg" aria-hidden />
            </div>
          )}
          {onGoBack && (
            <div className={styles.pageIcon}>
              <IconButton
                name="arrow-left"
                tooltip="Go back (Esc)"
                tooltipPlacement="bottom"
                size="xxl"
                aria-label={selectors.components.BackButton.backArrow}
                onClick={onGoBack}
              />
            </div>
          )}
          <nav aria-label="Search links" className={styles.navElement}>
            {parent && parentHref && (
              <>
                <Link
                  aria-label={`Search dashboard in the ${parent} folder`}
                  className={cx(styles.titleText, styles.parentLink, styles.titleLink)}
                  href={parentHref}
                >
                  {parent} <span className={styles.parentIcon}></span>
                </Link>
                {titleHref && (
                  <span className={cx(styles.titleText, styles.titleDivider, styles.parentLink)} aria-hidden>
                    /
                  </span>
                )}
              </>
            )}

            {title && (
              <div className={styles.titleWrapper}>
                <h1 className={styles.h1Styles}>
                  {titleHref ? (
                    <Link
                      aria-label="Search dashboard by name"
                      className={cx(styles.titleText, styles.titleLink)}
                      href={titleHref}
                    >
                      {titleEl}
                    </Link>
                  ) : (
                    <div className={styles.titleText}>{titleEl}</div>
                  )}
                </h1>
                {leftItemChildren}
              </div>
            )}
          </nav>
        </div>
        {React.Children.toArray(children)
          .filter(Boolean)
          .map((child, index) => {
            return (
              <div className={styles.actionWrapper} key={index}>
                {child}
              </div>
            );
          })}
      </nav>
    );
  }
);

PageToolbar.displayName = 'PageToolbar';

const getStyles = (theme: GrafanaTheme2) => {
  const { spacing, typography } = theme;

  const focusStyle = getFocusStyles(theme);

  return {
    pre: css`
      white-space: pre;
    `,
    toolbar: css`
      align-items: center;
      background: ${theme.colors.background.canvas};
      display: flex;
      flex-wrap: wrap;
      justify-content: flex-end;
      padding: ${theme.spacing(1.5, 2)};
    `,
    leftWrapper: css`
      display: flex;
      flex-wrap: nowrap;
      flex-grow: 1;
    `,
    pageIcon: css`
      display: none;
      @media ${styleMixins.mediaUp(theme.v1.breakpoints.md)} {
        display: flex;
        padding-right: ${theme.spacing(1)};
        align-items: center;
      }
    `,
    noLinkTitle: css`
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    `,
    titleWrapper: css`
      display: flex;
      flex-grow: 1;
      margin: 0;
    `,
    navElement: css`
      display: flex;
      flex-grow: 1;
      align-items: center;
      max-width: calc(100vw - 78px);
    `,
    h1Styles: css`
      margin: 0;
      line-height: inherit;
      width: 300px;
      max-width: min-content;
      flex-grow: 1;
    `,
    parentIcon: css`
      margin-left: ${theme.spacing(0.5)};
    `,
    titleText: css`
      display: flex;
      font-size: ${typography.size.lg};
      margin: 0;
      border-radius: 2px;
    `,
    titleLink: css`
      &:focus-visible {
        ${focusStyle}
      }
    `,
    titleDivider: css`
      padding: ${spacing(0, 0.5, 0, 0.5)};
    `,
    parentLink: css`
      display: none;
      @media ${styleMixins.mediaUp(theme.v1.breakpoints.md)} {
        display: unset;
      }
    `,
    actionWrapper: css`
      padding: ${spacing(0.5, 0, 0.5, 1)};
    `,
    leftActionItem: css`
      display: none;
      @media ${styleMixins.mediaUp(theme.v1.breakpoints.md)} {
        align-items: center;
        display: flex;
        padding-left: ${spacing(0.5)};
      }
    `,
  };
};
