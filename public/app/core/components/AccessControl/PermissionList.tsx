import React from 'react';
import { ResourcePermission } from './types';
import { PermissionListItem } from './PermissionListItem';

interface Props {
  title: string;
  items: ResourcePermission[];
  permissionLevels: string[];
  canSet: boolean;
  onRemove: (item: ResourcePermission) => void;
  onChange: (resourcePermission: ResourcePermission, permission: string) => void;
}

export const PermissionList = ({ title, items, permissionLevels, canSet, onRemove, onChange }: Props) => {
  if (items.length === 0) {
    return null;
  }

  return (
    <div>
      <h5>{title}</h5>
      <table className="filter-table gf-form-group">
        <tbody>
          {items.map((item, index) => (
            <PermissionListItem
              item={item}
              onRemove={onRemove}
              onChange={onChange}
              canSet={canSet}
              key={`${index}-${item.userId}`}
              permissionLevels={permissionLevels}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
};
