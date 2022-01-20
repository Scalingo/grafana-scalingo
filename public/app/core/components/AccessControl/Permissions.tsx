import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { sortBy } from 'lodash';
import { getBackendSrv } from 'app/core/services/backend_srv';

import { Button } from '@grafana/ui';
import { SlideDown } from 'app/core/components/Animations/SlideDown';
import { AddPermission } from './AddPermission';
import { PermissionList } from './PermissionList';
import { PermissionTarget, ResourcePermission, SetPermission, Description } from './types';

const EMPTY_PERMISSION = '';

const INITIAL_DESCRIPTION: Description = {
  permissions: [],
  assignments: {
    teams: false,
    users: false,
    builtInRoles: false,
  },
};

export type Props = {
  resource: string;
  resourceId: number;

  canListUsers: boolean;
  canSetPermissions: boolean;
};

export const Permissions = ({ resource, resourceId, canListUsers, canSetPermissions }: Props) => {
  const [isAdding, setIsAdding] = useState(false);
  const [items, setItems] = useState<ResourcePermission[]>([]);
  const [desc, setDesc] = useState(INITIAL_DESCRIPTION);

  const fetchItems = useCallback(() => {
    return getPermissions(resource, resourceId).then((r) => setItems(r));
  }, [resource, resourceId]);

  useEffect(() => {
    getDescription(resource).then((r) => {
      setDesc(r);
      return fetchItems();
    });
  }, [resource, resourceId, fetchItems]);

  const onAdd = (state: SetPermission) => {
    let promise: Promise<void> | null = null;
    if (state.target === PermissionTarget.User) {
      promise = setUserPermission(resource, resourceId, state.userId!, state.permission);
    } else if (state.target === PermissionTarget.Team) {
      promise = setTeamPermission(resource, resourceId, state.teamId!, state.permission);
    } else if (state.target === PermissionTarget.BuiltInRole) {
      promise = setBuiltInRolePermission(resource, resourceId, state.builtInRole!, state.permission);
    }

    if (promise !== null) {
      promise.then(fetchItems);
    }
  };

  const onRemove = (item: ResourcePermission) => {
    let promise: Promise<void> | null = null;
    if (item.userId) {
      promise = setUserPermission(resource, resourceId, item.userId, EMPTY_PERMISSION);
    } else if (item.teamId) {
      promise = setTeamPermission(resource, resourceId, item.teamId, EMPTY_PERMISSION);
    } else if (item.builtInRole) {
      promise = setBuiltInRolePermission(resource, resourceId, item.builtInRole, EMPTY_PERMISSION);
    }

    if (promise !== null) {
      promise.then(fetchItems);
    }
  };

  const onChange = (item: ResourcePermission, permission: string) => {
    if (item.permission === permission) {
      return;
    }
    if (item.userId) {
      onAdd({ permission, userId: item.userId, target: PermissionTarget.User });
    } else if (item.teamId) {
      onAdd({ permission, teamId: item.teamId, target: PermissionTarget.Team });
    } else if (item.builtInRole) {
      onAdd({ permission, builtInRole: item.builtInRole, target: PermissionTarget.BuiltInRole });
    }
  };

  const teams = useMemo(
    () =>
      sortBy(
        items.filter((i) => i.teamId),
        ['team']
      ),
    [items]
  );
  const users = useMemo(
    () =>
      sortBy(
        items.filter((i) => i.userId),
        ['userLogin']
      ),
    [items]
  );
  const builtInRoles = useMemo(
    () =>
      sortBy(
        items.filter((i) => i.builtInRole),
        ['builtInRole']
      ),
    [items]
  );

  return (
    <div>
      <div className="page-action-bar">
        <h3 className="page-sub-heading">Permissions</h3>
        <div className="page-action-bar__spacer" />
        {canSetPermissions && (
          <Button variant={'primary'} key="add-permission" onClick={() => setIsAdding(true)}>
            Add a permission
          </Button>
        )}
      </div>

      <div>
        <SlideDown in={isAdding}>
          <AddPermission
            onAdd={onAdd}
            permissions={desc.permissions}
            assignments={desc.assignments}
            canListUsers={canListUsers}
            onCancel={() => setIsAdding(false)}
          />
        </SlideDown>
        <PermissionList
          title="Roles"
          items={builtInRoles}
          permissionLevels={desc.permissions}
          onChange={onChange}
          onRemove={onRemove}
          canSet={canSetPermissions}
        />
        <PermissionList
          title="Users"
          items={users}
          permissionLevels={desc.permissions}
          onChange={onChange}
          onRemove={onRemove}
          canSet={canSetPermissions}
        />
        <PermissionList
          title="Teams"
          items={teams}
          permissionLevels={desc.permissions}
          onChange={onChange}
          onRemove={onRemove}
          canSet={canSetPermissions}
        />
      </div>
    </div>
  );
};

const getDescription = async (resource: string): Promise<Description> => {
  try {
    return await getBackendSrv().get(`/api/access-control/${resource}/description`);
  } catch (e) {
    console.error('failed to load resource description: ', e);
    return INITIAL_DESCRIPTION;
  }
};

const getPermissions = (resource: string, datasourceId: number): Promise<ResourcePermission[]> =>
  getBackendSrv().get(`/api/access-control/${resource}/${datasourceId}`);

const setUserPermission = (resource: string, resourceId: number, userId: number, permission: string) =>
  setPermission(resource, resourceId, 'users', userId, permission);

const setTeamPermission = (resource: string, resourceId: number, teamId: number, permission: string) =>
  setPermission(resource, resourceId, 'teams', teamId, permission);

const setBuiltInRolePermission = (resource: string, resourceId: number, builtInRole: string, permission: string) =>
  setPermission(resource, resourceId, 'builtInRoles', builtInRole, permission);

const setPermission = (
  resource: string,
  resourceId: number,
  type: 'users' | 'teams' | 'builtInRoles',
  typeId: number | string,
  permission: string
): Promise<void> =>
  getBackendSrv().post(`/api/access-control/${resource}/${resourceId}/${type}/${typeId}/`, { permission });
