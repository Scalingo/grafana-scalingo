import React, { FC, useContext } from 'react';
import { css } from 'emotion';
import { Modal } from '../Modal/Modal';
import { IconType } from '../Icon/types';
import { Button } from '../Button/Button';
import { stylesFactory, ThemeContext } from '../../themes';
import { GrafanaTheme } from '@grafana/data';
import { HorizontalGroup } from '..';

const getStyles = stylesFactory((theme: GrafanaTheme) => ({
  modal: css`
    width: 500px;
  `,
  modalContent: css`
    text-align: center;
  `,
  modalText: css`
    font-size: ${theme.typography.heading.h4};
    color: ${theme.colors.link};
    margin-bottom: calc(${theme.spacing.d} * 2);
    padding-top: ${theme.spacing.d};
  `,
}));

const defaultIcon: IconType = 'exclamation-triangle';

interface Props {
  isOpen: boolean;
  title: string;
  body: React.ReactNode;
  confirmText: string;
  dismissText?: string;
  icon?: IconType;
  onConfirm(): void;
  onDismiss(): void;
}

export const ConfirmModal: FC<Props> = ({
  isOpen,
  title,
  body,
  confirmText,
  dismissText = 'Cancel',
  icon,
  onConfirm,
  onDismiss,
}) => {
  const theme = useContext(ThemeContext);
  const styles = getStyles(theme);

  return (
    <Modal className={styles.modal} title={title} icon={icon || defaultIcon} isOpen={isOpen} onDismiss={onDismiss}>
      <div className={styles.modalContent}>
        <div className={styles.modalText}>{body}</div>
        <HorizontalGroup justify="center">
          <Button variant="danger" onClick={onConfirm}>
            {confirmText}
          </Button>
          <Button variant="inverse" onClick={onDismiss}>
            {dismissText}
          </Button>
        </HorizontalGroup>
      </div>
    </Modal>
  );
};
