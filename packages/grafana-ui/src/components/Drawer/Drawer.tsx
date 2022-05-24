import { css } from '@emotion/css';
import { useDialog } from '@react-aria/dialog';
import { FocusScope } from '@react-aria/focus';
import { useOverlay } from '@react-aria/overlays';
import RcDrawer from 'rc-drawer';
import React, { CSSProperties, ReactNode, useState, useEffect } from 'react';

import { GrafanaTheme2 } from '@grafana/data';
import { selectors } from '@grafana/e2e-selectors';

import { useStyles2 } from '../../themes';
import { CustomScrollbar } from '../CustomScrollbar/CustomScrollbar';
import { IconButton } from '../IconButton/IconButton';

export interface Props {
  children: ReactNode;
  /** Title shown at the top of the drawer */
  title?: ReactNode;
  /** Subtitle shown below the title */
  subtitle?: ReactNode;
  /** Should the Drawer be closable by clicking on the mask, defaults to true */
  closeOnMaskClick?: boolean;
  /** Render the drawer inside a container on the page */
  inline?: boolean;
  /** Either a number in px or a string with unit postfix */
  width?: number | string;
  /** Should the Drawer be expandable to full width */
  expandable?: boolean;
  /** Tabs */
  tabs?: React.ReactNode;
  /** Set to true if the component rendered within in drawer content has its own scroll */
  scrollableContent?: boolean;
  /** Callback for closing the drawer */
  onClose: () => void;
}

export function Drawer({
  children,
  inline = false,
  onClose,
  closeOnMaskClick = true,
  scrollableContent = false,
  title,
  subtitle,
  width = '40%',
  expandable = false,
  tabs,
}: Props) {
  const drawerStyles = useStyles2(getStyles);
  const [isExpanded, setIsExpanded] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const currentWidth = isExpanded ? '100%' : width;
  const overlayRef = React.useRef(null);
  const { dialogProps, titleProps } = useDialog({}, overlayRef);
  const { overlayProps } = useOverlay(
    {
      isDismissable: true,
    },
    overlayRef
  );

  // RcDrawer v4.x needs to be mounted in advance for animations to play.
  useEffect(() => {
    setIsOpen(true);
  }, []);

  const content = <div className={drawerStyles.content}>{children}</div>;

  return (
    <RcDrawer
      level={null}
      handler={false}
      open={isOpen}
      onClose={onClose}
      maskClosable={closeOnMaskClick}
      placement="right"
      width={currentWidth}
      getContainer={inline ? undefined : 'body'}
      style={{ position: `${inline && 'absolute'}` } as CSSProperties}
      className={drawerStyles.drawer}
      aria-label={
        typeof title === 'string'
          ? selectors.components.Drawer.General.title(title)
          : selectors.components.Drawer.General.title('no title')
      }
    >
      <FocusScope restoreFocus contain autoFocus>
        <div className={drawerStyles.container} {...overlayProps} {...dialogProps} ref={overlayRef}>
          {typeof title === 'string' && (
            <div className={drawerStyles.header}>
              <div className={drawerStyles.actions}>
                {expandable && !isExpanded && (
                  <IconButton
                    name="angle-left"
                    size="xl"
                    onClick={() => setIsExpanded(true)}
                    surface="header"
                    aria-label={selectors.components.Drawer.General.expand}
                  />
                )}
                {expandable && isExpanded && (
                  <IconButton
                    name="angle-right"
                    size="xl"
                    onClick={() => setIsExpanded(false)}
                    surface="header"
                    aria-label={selectors.components.Drawer.General.contract}
                  />
                )}
                <IconButton
                  name="times"
                  size="xl"
                  onClick={onClose}
                  surface="header"
                  aria-label={selectors.components.Drawer.General.close}
                />
              </div>
              <div className={drawerStyles.titleWrapper}>
                <h3 {...titleProps}>{title}</h3>
                {typeof subtitle === 'string' && <div className="muted">{subtitle}</div>}
                {typeof subtitle !== 'string' && subtitle}
                {tabs && <div className={drawerStyles.tabsWrapper}>{tabs}</div>}
              </div>
            </div>
          )}
          {typeof title !== 'string' && title}
          <div className={drawerStyles.contentScroll}>
            {!scrollableContent ? content : <CustomScrollbar autoHeightMin="100%">{content}</CustomScrollbar>}
          </div>
        </div>
      </FocusScope>
    </RcDrawer>
  );
}

const getStyles = (theme: GrafanaTheme2) => {
  return {
    container: css`
      display: flex;
      flex-direction: column;
      height: 100%;
      flex: 1 1 0;
    `,
    drawer: css`
      .drawer-content {
        background-color: ${theme.colors.background.primary};
        display: flex;
        flex-direction: column;
        overflow: hidden;
      }
      &.drawer-open .drawer-mask {
        background-color: ${theme.components.overlay.background};
        backdrop-filter: blur(1px);
        opacity: 1;
      }
      .drawer-mask {
        background-color: ${theme.components.overlay.background};
        backdrop-filter: blur(1px);
      }
      .drawer-open .drawer-content-wrapper {
        box-shadow: ${theme.shadows.z3};
      }
      z-index: ${theme.zIndex.dropdown};
    `,
    header: css`
      background-color: ${theme.colors.background.canvas};
      flex-grow: 0;
      padding-top: ${theme.spacing(0.5)};
    `,
    actions: css`
      display: flex;
      align-items: baseline;
      justify-content: flex-end;
    `,
    titleWrapper: css`
      margin-bottom: ${theme.spacing(3)};
      padding: ${theme.spacing(0, 1, 0, 3)};
      overflow-wrap: break-word;
    `,
    content: css({
      padding: theme.spacing(2),
      height: '100%',
      flexGrow: 1,
    }),
    contentScroll: css({
      minHeight: 0,
      flex: 1,
    }),
    tabsWrapper: css({
      paddingLeft: theme.spacing(2),
      margin: theme.spacing(3, -1, -3, -3),
    }),
  };
};
