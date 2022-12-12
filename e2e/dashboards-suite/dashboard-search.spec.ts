import { e2e } from '@grafana/e2e';

import testDashboard from '../dashboards/TestDashboard.json';

e2e.scenario({
  describeName: 'Dashboard search',
  itName: 'Basic folder view test',
  addScenarioDataSource: false,
  addScenarioDashBoard: false,
  skipScenario: false,
  scenario: () => {
    e2e.flows.importDashboard(testDashboard, 1000, true);

    e2e.pages.Search.FolderView.visit();

    // folder view is collapsed - verify its content does not exist
    e2e.components.Search.folderContent('General').should('not.exist');
    e2e.components.Search.dashboardItem('E2E Test - Dashboard Search').should('not.exist');

    e2e.components.Search.folderHeader('General').click({ force: true });

    e2e.components.Search.folderContent('General').should('be.visible');
    e2e.components.Search.dashboardItem('E2E Test - Import Dashboard').should('be.visible');

    // verify the imported dashboard is the only dashboard present in the folder
    e2e().get('[data-testid^="data-testid Dashboard search item "]').should('have.length', 1);
  },
});
