define([
  'angular',
  'lodash',
  'jquery',
],
function (angular, _, $) {
  'use strict';

  var module = angular.module('grafana.services');

  module.factory('dashboardViewStateSrv', function($location, $timeout, templateSrv, contextSrv, timeSrv) {

    // represents the transient view state
    // like fullscreen panel & edit
    function DashboardViewState($scope) {
      var self = this;
      self.state = {};
      self.panelScopes = [];
      self.$scope = $scope;
      self.dashboard = $scope.dashboard;

      $scope.exitFullscreen = function() {
        if (self.state.fullscreen) {
          self.update({ fullscreen: false });
        }
      };

      // update url on time range change
      $scope.onAppEvent('time-range-changed', function() {
        var urlParams = $location.search();
        var urlRange = timeSrv.timeRangeForUrl();
        urlParams.from = urlRange.from;
        urlParams.to = urlRange.to;
        $location.search(urlParams);
      });

      $scope.onAppEvent('$routeUpdate', function() {
        var urlState = self.getQueryStringState();
        if (self.needsSync(urlState)) {
          self.update(urlState, true);
        }
      });

      $scope.onAppEvent('panel-change-view', function(evt, payload) {
        self.update(payload);
      });

      $scope.onAppEvent('panel-initialized', function(evt, payload) {
        self.registerPanel(payload.scope);
      });

      this.update(this.getQueryStringState());
      this.expandRowForPanel();
    }

    DashboardViewState.prototype.expandRowForPanel = function() {
      if (!this.state.panelId) { return; }

      var panelInfo = this.$scope.dashboard.getPanelInfoById(this.state.panelId);
      if (panelInfo) {
        panelInfo.row.collapse = false;
      }
    };

    DashboardViewState.prototype.needsSync = function(urlState) {
      return _.isEqual(this.state, urlState) === false;
    };

    DashboardViewState.prototype.getQueryStringState = function() {
      var state = $location.search();
      state.panelId = parseInt(state.panelId) || null;
      state.fullscreen = state.fullscreen ? true : null;
      state.edit =  (state.edit === "true" || state.edit === true) || null;
      state.editview = state.editview || null;
      return state;
    };

    DashboardViewState.prototype.serializeToUrl = function() {
      var urlState = _.clone(this.state);
      urlState.fullscreen = this.state.fullscreen ? true : null;
      urlState.edit = this.state.edit ? true : null;
      return urlState;
    };

    DashboardViewState.prototype.update = function(state) {
      // implement toggle logic
      if (state.toggle) {
        delete state.toggle;
        if (this.state.fullscreen && state.fullscreen) {
          if (this.state.edit === state.edit) {
            state.fullscreen = !state.fullscreen;
          }
        }
      }

      // remember if editStateChanged
      this.editStateChanged = state.edit !== this.state.edit;

      _.extend(this.state, state);
      this.dashboard.meta.fullscreen = this.state.fullscreen;

      if (!this.state.fullscreen) {
        this.state.fullscreen = null;
        this.state.edit = null;
        // clear panel id unless in solo mode
        if (!this.dashboard.meta.soloMode) {
          this.state.panelId = null;
        }
      }

      // if no edit state cleanup tab parm
      if (!this.state.edit) {
        delete this.state.tab;
      }

      $location.search(this.serializeToUrl());
      this.syncState();
    };

    DashboardViewState.prototype.syncState = function() {
      if (this.panelScopes.length === 0) { return; }

      if (this.dashboard.meta.fullscreen) {
        var panelScope = this.getPanelScope(this.state.panelId);
        if (!panelScope) {
          return;
        }

        if (this.fullscreenPanel) {
          // if already fullscreen
          if (this.fullscreenPanel === panelScope && this.editStateChanged === false) {
            return;
          } else {
            this.leaveFullscreen(false);
          }
        }

        if (!panelScope.ctrl.editModeInitiated) {
          panelScope.ctrl.initEditMode();
        }

        if (!panelScope.ctrl.fullscreen) {
          this.enterFullscreen(panelScope);
        }
      } else if (this.fullscreenPanel) {
        this.leaveFullscreen(true);
      }
    };

    DashboardViewState.prototype.getPanelScope = function(id) {
      return _.find(this.panelScopes, function(panelScope) {
        return panelScope.ctrl.panel.id === id;
      });
    };

    DashboardViewState.prototype.leaveFullscreen = function(render) {
      var self = this;
      var ctrl = self.fullscreenPanel.ctrl;

      ctrl.editMode = false;
      ctrl.fullscreen = false;
      ctrl.dashboard.editMode = this.oldDashboardEditMode;

      this.$scope.appEvent('panel-fullscreen-exit', {panelId: ctrl.panel.id});

      if (!render) { return false;}

      $timeout(function() {
        if (self.oldTimeRange !== ctrl.range) {
          self.$scope.broadcastRefresh();
        } else {
          self.$scope.$broadcast('render');
        }
        delete self.fullscreenPanel;
      });
    };

    DashboardViewState.prototype.enterFullscreen = function(panelScope) {
      var ctrl = panelScope.ctrl;

      ctrl.editMode = this.state.edit && this.dashboard.meta.canEdit;
      ctrl.fullscreen = true;

      this.oldDashboardEditMode = this.dashboard.editMode;
      this.oldTimeRange = ctrl.range;
      this.fullscreenPanel = panelScope;
      this.dashboard.editMode = false;

      $(window).scrollTop(0);

      this.$scope.appEvent('panel-fullscreen-enter', {panelId: ctrl.panel.id});

      $timeout(function() {
        ctrl.render();
      });
    };

    DashboardViewState.prototype.registerPanel = function(panelScope) {
      var self = this;
      self.panelScopes.push(panelScope);

      if (!self.dashboard.meta.soloMode) {
        if (self.state.panelId === panelScope.ctrl.panel.id) {
          if (self.state.edit) {
            panelScope.ctrl.editPanel();
          } else {
            panelScope.ctrl.viewPanel();
          }
        }
      }

      var unbind = panelScope.$on('$destroy', function() {
        self.panelScopes = _.without(self.panelScopes, panelScope);
        unbind();
      });
    };

    return {
      create: function($scope) {
        return new DashboardViewState($scope);
      }
    };

  });
});
