import { contextSrv } from 'app/core/services/context_srv';
import impressionSrv from 'app/core/services/impression_srv';
import { SearchSrv } from 'app/core/services/search_srv';
import { DashboardSearchItem } from 'app/features/search/types';

import { backendSrv } from '../services/backend_srv';

jest.mock('app/core/store', () => {
  return {
    getBool: jest.fn(),
    set: jest.fn(),
    getObject: jest.fn(),
  };
});

jest.mock('app/core/services/impression_srv', () => {
  return {
    getDashboardOpened: jest.fn,
  };
});

describe('SearchSrv', () => {
  let searchSrv: SearchSrv;
  const searchMock = jest.spyOn(backendSrv, 'search'); // will use the mock in __mocks__

  beforeEach(() => {
    searchSrv = new SearchSrv();

    contextSrv.isSignedIn = true;
    impressionSrv.getDashboardOpened = jest.fn().mockResolvedValue([]);
    jest.clearAllMocks();
  });

  describe('With recent dashboards', () => {
    let results: any;

    beforeEach(() => {
      searchMock.mockImplementation((options) => {
        if (options.dashboardUIDs) {
          return Promise.resolve([
            { uid: 'DSNdW0gVk', title: 'second but first' },
            { uid: 'srx16xR4z', title: 'first but second' },
          ] as DashboardSearchItem[]);
        }
        return Promise.resolve([]);
      });

      impressionSrv.getDashboardOpened = jest.fn().mockResolvedValue(['srx16xR4z', 'DSNdW0gVk']);

      return searchSrv.search({ query: '' }).then((res) => {
        results = res;
      });
    });

    it('should include recent dashboards section', () => {
      expect(results[0].title).toBe('Recent');
    });

    it('should return order decided by impressions store not api', () => {
      expect(results[0].items[0].title).toBe('first but second');
      expect(results[0].items[1].title).toBe('second but first');
    });

    describe('and 3 recent dashboards removed in backend', () => {
      let results: any;

      beforeEach(() => {
        searchMock.mockImplementation((options) => {
          if (options.dashboardUIDs) {
            return Promise.resolve([
              { uid: 'DSNdW0gVk', title: 'two' },
              { uid: 'srx16xR4z', title: 'one' },
            ] as DashboardSearchItem[]);
          }
          return Promise.resolve([]);
        });

        impressionSrv.getDashboardOpened = jest
          .fn()
          .mockResolvedValue(['Xrx16x4z', 'CSxdW0gYA', 'srx16xR4z', 'DSNdW0gVk', 'xSxdW0gYA']);

        return searchSrv.search({ query: '' }).then((res) => {
          results = res;
        });
      });

      it('should return 2 dashboards', () => {
        expect(results[0].items.length).toBe(2);
        expect(results[0].items[0].uid).toBe('srx16xR4z');
        expect(results[0].items[1].uid).toBe('DSNdW0gVk');
      });
    });
  });

  describe('With starred dashboards', () => {
    let results: any;

    beforeEach(() => {
      searchMock.mockImplementation((options) => {
        if (options.starred) {
          return Promise.resolve([{ uid: '1', title: 'starred' }] as DashboardSearchItem[]);
        }
        return Promise.resolve([]);
      });

      return searchSrv.search({ query: '' }).then((res) => {
        results = res;
      });
    });

    it('should include starred dashboards section', () => {
      expect(results[0].title).toBe('Starred');
      expect(results[0].items.length).toBe(1);
    });
  });

  describe('With starred dashboards and recent', () => {
    let results: any;

    beforeEach(() => {
      searchMock.mockImplementation((options) => {
        if (options.dashboardUIDs) {
          return Promise.resolve([
            { uid: 'srx16xR4z', title: 'starred and recent', isStarred: true },
            { uid: 'DSNdW0gVk', title: 'recent' },
          ] as DashboardSearchItem[]);
        }
        return Promise.resolve([{ uid: 'srx16xR4z', title: 'starred and recent' }] as DashboardSearchItem[]);
      });

      impressionSrv.getDashboardOpened = jest.fn().mockResolvedValue(['srx16xR4z', 'DSNdW0gVk']);
      return searchSrv.search({ query: '' }).then((res) => {
        results = res;
      });
    });

    it('should not show starred in recent', () => {
      expect(results[1].title).toBe('Recent');
      expect(results[1].items[0].title).toBe('recent');
    });

    it('should show starred', () => {
      expect(results[0].title).toBe('Starred');
      expect(results[0].items[0].title).toBe('starred and recent');
    });
  });

  describe('with no query string and dashboards with folders returned', () => {
    let results: any;

    beforeEach(() => {
      searchMock.mockImplementation(
        jest
          .fn()
          .mockResolvedValueOnce(Promise.resolve([]))
          .mockResolvedValue(
            Promise.resolve([
              {
                title: 'folder1',
                type: 'dash-folder',
                uid: 'folder-1',
              },
              {
                title: 'dash with no folder',
                type: 'dash-db',
                uid: '2',
              },
              {
                title: 'dash in folder1 1',
                type: 'dash-db',
                uid: '3',
                folderUid: 'folder-1',
              },
              {
                title: 'dash in folder1 2',
                type: 'dash-db',
                uid: '4',
                folderUid: 'folder-1',
              },
            ])
          )
      );

      return searchSrv.search({ query: '' }).then((res) => {
        results = res;
      });
    });

    it('should create sections for each folder and root', () => {
      expect(results).toHaveLength(2);
    });

    it('should place folders first', () => {
      expect(results[0].title).toBe('folder1');
    });
  });

  describe('with query string and dashboards with folders returned', () => {
    let results: any;

    beforeEach(() => {
      searchMock.mockImplementation(
        jest.fn().mockResolvedValue([
          {
            folderUid: 'dash-with-no-folder-uid',
            title: 'dash with no folder',
            type: 'dash-db',
          },
          {
            title: 'dash in folder1 1',
            type: 'dash-db',
            folderUid: 'uid',
            folderTitle: 'folder1',
            folderUrl: '/dashboards/f/uid/folder1',
          },
        ])
      );

      return searchSrv.search({ query: 'search' }).then((res) => {
        results = res;
      });
    });

    it('should not specify folder ids', () => {
      expect(searchMock.mock.calls[0][0].folderIds).toHaveLength(0);
    });

    it('should group results by folder', () => {
      expect(results).toHaveLength(2);
      expect(results[0].uid).toEqual('dash-with-no-folder-uid');
      expect(results[1].uid).toEqual('uid');
      expect(results[1].title).toEqual('folder1');
      expect(results[1].url).toEqual('/dashboards/f/uid/folder1');
    });
  });

  describe('with tags', () => {
    beforeEach(() => {
      searchMock.mockImplementation(jest.fn().mockResolvedValue(Promise.resolve([])));

      return searchSrv.search({ tag: ['atag'] }).then(() => {});
    });

    it('should send tags query to backend search', () => {
      expect(searchMock.mock.calls[0][0].tag).toHaveLength(1);
    });
  });

  describe('with starred', () => {
    beforeEach(() => {
      searchMock.mockImplementation(jest.fn().mockResolvedValue(Promise.resolve([])));

      return searchSrv.search({ starred: true }).then(() => {});
    });

    it('should send starred query to backend search', () => {
      expect(searchMock.mock.calls[0][0].starred).toEqual(true);
    });
  });

  describe('when skipping recent dashboards', () => {
    let getRecentDashboardsCalled = false;

    beforeEach(() => {
      searchMock.mockImplementation(jest.fn().mockResolvedValue(Promise.resolve([])));

      searchSrv['getRecentDashboards'] = () => {
        getRecentDashboardsCalled = true;
        return Promise.resolve();
      };

      return searchSrv.search({ skipRecent: true }).then(() => {});
    });

    it('should not fetch recent dashboards', () => {
      expect(getRecentDashboardsCalled).toBeFalsy();
    });
  });

  describe('when skipping starred dashboards', () => {
    let getStarredCalled = false;

    beforeEach(() => {
      searchMock.mockImplementation(jest.fn().mockResolvedValue(Promise.resolve([])));
      impressionSrv.getDashboardOpened = jest.fn().mockResolvedValue([]);

      searchSrv['getStarred'] = () => {
        getStarredCalled = true;
        return Promise.resolve({});
      };

      return searchSrv.search({ skipStarred: true }).then(() => {});
    });

    it('should not fetch starred dashboards', () => {
      expect(getStarredCalled).toBeFalsy();
    });
  });
});
