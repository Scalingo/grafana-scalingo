import { within } from '@testing-library/dom';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React from 'react';
import { Provider } from 'react-redux';
import { BrowserRouter } from 'react-router-dom';
import { getGrafanaContextMock } from 'test/mocks/getGrafanaContextMock';

import { selectors } from '@grafana/e2e-selectors';
import { locationService, setAngularLoader, setDataSourceSrv } from '@grafana/runtime';
import { GrafanaContext } from 'app/core/context/GrafanaContext';
import { mockDataSource, MockDataSourceSrv } from 'app/features/alerting/unified/mocks';

import { configureStore } from '../../../../store/configureStore';
import { DashboardModel } from '../../state/DashboardModel';

import { AnnotationsSettings } from './AnnotationsSettings';

function setup(dashboard: DashboardModel, editIndex?: number) {
  const store = configureStore();
  const sectionNav = {
    main: { text: 'Dashboard' },
    node: {
      text: 'Annotations',
    },
  };

  return render(
    <GrafanaContext.Provider value={getGrafanaContextMock()}>
      <Provider store={store}>
        <BrowserRouter>
          <AnnotationsSettings sectionNav={sectionNav} dashboard={dashboard} editIndex={editIndex} />
        </BrowserRouter>
      </Provider>
    </GrafanaContext.Provider>
  );
}

describe('AnnotationsSettings', () => {
  let dashboard: DashboardModel;

  const dataSources = {
    grafana: mockDataSource(
      {
        name: 'Grafana',
        uid: 'uid1',
        type: 'grafana',
      },
      { annotations: true }
    ),
    Testdata: mockDataSource(
      {
        name: 'Testdata',
        uid: 'uid2',
        type: 'testdata',
        isDefault: true,
      },
      { annotations: true }
    ),
    Prometheus: mockDataSource(
      {
        name: 'Prometheus',
        uid: 'uid3',
        type: 'prometheus',
      },
      { annotations: true }
    ),
  };

  setDataSourceSrv(new MockDataSourceSrv(dataSources));

  const getTableBody = () => screen.getAllByRole('rowgroup')[1];
  const getTableBodyRows = () => within(getTableBody()).getAllByRole('row');

  beforeAll(() => {
    setAngularLoader({
      load: () => ({
        destroy: jest.fn(),
        digest: jest.fn(),
        getScope: () => ({ $watch: () => {} }),
      }),
    });
  });

  beforeEach(() => {
    dashboard = new DashboardModel({
      id: 74,
      version: 7,
      annotations: {
        list: [
          {
            builtIn: 1,
            datasource: { uid: 'uid1', type: 'grafana' },
            enable: true,
            hide: true,
            iconColor: 'rgba(0, 211, 255, 1)',
            name: 'Annotations & Alerts',
            type: 'dashboard',
          },
        ],
      },
      links: [],
    });
  });

  test('it renders empty list cta if only builtIn annotation', async () => {
    setup(dashboard);

    expect(screen.queryByRole('grid')).toBeInTheDocument();
    expect(screen.getByRole('row', { name: /annotations & alerts \(built\-in\) grafana/i })).toBeInTheDocument();
    expect(
      screen.getByTestId(selectors.components.CallToActionCard.buttonV2('Add annotation query'))
    ).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /annotations documentation/i })).toBeInTheDocument();
  });

  test('it renders empty list if annotations', async () => {
    dashboard.annotations.list = [];
    setup(dashboard);

    expect(
      screen.getByTestId(selectors.components.CallToActionCard.buttonV2('Add annotation query'))
    ).toBeInTheDocument();
  });

  test('it renders the annotation names or uid if annotation doesnt exist', async () => {
    dashboard.annotations.list = [
      ...dashboard.annotations.list,
      {
        builtIn: 0,
        datasource: { uid: 'uid3', type: 'prometheus' },
        enable: true,
        hide: true,
        iconColor: 'rgba(0, 211, 255, 1)',
        name: 'Annotation 2',
        type: 'dashboard',
      },
      {
        builtIn: 0,
        datasource: { uid: 'deletedAnnotationId', type: 'prometheus' },
        enable: true,
        hide: true,
        iconColor: 'rgba(0, 211, 255, 1)',
        name: 'Annotation 2',
        type: 'dashboard',
      },
    ];
    setup(dashboard);
    // Check that we have the correct annotations
    expect(screen.queryByText(/prometheus/i)).toBeInTheDocument();
    expect(screen.queryByText(/deletedAnnotationId/i)).toBeInTheDocument();
  });

  test('it renders a sortable table of annotations', async () => {
    dashboard.annotations.list = [
      ...dashboard.annotations.list,
      {
        builtIn: 0,
        datasource: { uid: 'uid3', type: 'prometheus' },
        enable: true,
        hide: true,
        iconColor: 'rgba(0, 211, 255, 1)',
        name: 'Annotation 2',
        type: 'dashboard',
      },
      {
        builtIn: 0,
        datasource: { uid: 'uid3', type: 'prometheus' },
        enable: true,
        hide: true,
        iconColor: 'rgba(0, 211, 255, 1)',
        name: 'Annotation 3',
        type: 'dashboard',
      },
    ];

    setup(dashboard);

    // Check that we have sorting buttons
    expect(within(getTableBodyRows()[0]).queryByRole('button', { name: 'arrow-up' })).not.toBeInTheDocument();
    expect(within(getTableBodyRows()[0]).queryByRole('button', { name: 'arrow-down' })).toBeInTheDocument();

    expect(within(getTableBodyRows()[1]).queryByRole('button', { name: 'arrow-up' })).toBeInTheDocument();
    expect(within(getTableBodyRows()[1]).queryByRole('button', { name: 'arrow-down' })).toBeInTheDocument();

    expect(within(getTableBodyRows()[2]).queryByRole('button', { name: 'arrow-up' })).toBeInTheDocument();
    expect(within(getTableBodyRows()[2]).queryByRole('button', { name: 'arrow-down' })).not.toBeInTheDocument();

    // Check the original order
    expect(within(getTableBodyRows()[0]).queryByText(/annotations & alerts/i)).toBeInTheDocument();
    expect(within(getTableBodyRows()[1]).queryByText(/annotation 2/i)).toBeInTheDocument();
    expect(within(getTableBodyRows()[2]).queryByText(/annotation 3/i)).toBeInTheDocument();

    await userEvent.click(within(getTableBody()).getAllByRole('button', { name: 'arrow-down' })[0]);
    await userEvent.click(within(getTableBody()).getAllByRole('button', { name: 'arrow-down' })[1]);
    await userEvent.click(within(getTableBody()).getAllByRole('button', { name: 'arrow-up' })[0]);

    // Checking if it has changed the sorting accordingly
    expect(within(getTableBodyRows()[0]).queryByText(/annotation 3/i)).toBeInTheDocument();
    expect(within(getTableBodyRows()[1]).queryByText(/annotation 2/i)).toBeInTheDocument();
    expect(within(getTableBodyRows()[2]).queryByText(/annotations & alerts/i)).toBeInTheDocument();
  });

  test('Adding a new annotation', async () => {
    setup(dashboard);

    await userEvent.click(screen.getByTestId(selectors.components.CallToActionCard.buttonV2('Add annotation query')));

    expect(locationService.getSearchObject().editIndex).toBe('1');
    expect(dashboard.annotations.list.length).toBe(2);
  });

  test('Editing annotation', async () => {
    dashboard.annotations.list.push({
      name: 'New annotation query',
      datasource: { uid: 'uid2', type: 'testdata' },
      iconColor: 'red',
      enable: true,
    });

    setup(dashboard, 1);

    const nameInput = screen.getByRole('textbox', { name: /name/i });
    await userEvent.clear(nameInput);
    await userEvent.type(nameInput, 'My Prometheus Annotation');

    await userEvent.click(screen.getByText(/testdata/i));

    expect(await screen.findByText(/Prometheus/i)).toBeVisible();
    expect(screen.queryAllByText(/testdata/i)).toHaveLength(2);

    await userEvent.click(screen.getByText(/prometheus/i));

    expect(screen.getByRole('checkbox', { name: /hidden/i })).not.toBeChecked();
  });

  test('Deleting annotation', async () => {
    dashboard.annotations.list = [
      ...dashboard.annotations.list,
      {
        builtIn: 0,
        datasource: { uid: 'uid3', type: 'prometheus' },
        enable: true,
        hide: true,
        iconColor: 'rgba(0, 211, 255, 1)',
        name: 'Annotation 2',
        type: 'dashboard',
      },
    ];
    setup(dashboard, 1); // Edit the not built-in annotations

    await userEvent.click(screen.getByRole('button', { name: 'Delete' }));

    expect(locationService.getSearchObject().editIndex).toBe(undefined);
    expect(dashboard.annotations.list.length).toBe(1); // started with two
  });
});
