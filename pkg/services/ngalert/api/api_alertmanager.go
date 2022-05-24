package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/grafana/grafana/pkg/api/response"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	apimodels "github.com/grafana/grafana/pkg/services/ngalert/api/tooling/definitions"
	"github.com/grafana/grafana/pkg/services/ngalert/notifier"
	"github.com/grafana/grafana/pkg/services/ngalert/store"
	"github.com/grafana/grafana/pkg/util"
	"github.com/grafana/grafana/pkg/web"
)

const (
	defaultTestReceiversTimeout = 15 * time.Second
	maxTestReceiversTimeout     = 30 * time.Second
)

type AlertmanagerSrv struct {
	log    log.Logger
	ac     accesscontrol.AccessControl
	mam    *notifier.MultiOrgAlertmanager
	crypto notifier.Crypto
}

type UnknownReceiverError struct {
	UID string
}

func (e UnknownReceiverError) Error() string {
	return fmt.Sprintf("unknown receiver: %s", e.UID)
}

func (srv AlertmanagerSrv) RouteGetAMStatus(c *models.ReqContext) response.Response {
	am, errResp := srv.AlertmanagerFor(c.OrgId)
	if errResp != nil {
		return errResp
	}

	return response.JSON(http.StatusOK, am.GetStatus())
}

func (srv AlertmanagerSrv) RouteCreateSilence(c *models.ReqContext, postableSilence apimodels.PostableSilence) response.Response {
	err := postableSilence.Validate(strfmt.Default)
	if err != nil {
		srv.log.Error("silence failed validation", "err", err)
		return ErrResp(http.StatusBadRequest, err, "silence failed validation")
	}

	am, errResp := srv.AlertmanagerFor(c.OrgId)
	if errResp != nil {
		return errResp
	}

	action := accesscontrol.ActionAlertingInstanceUpdate
	if postableSilence.ID == "" {
		action = accesscontrol.ActionAlertingInstanceCreate
	}
	if !accesscontrol.HasAccess(srv.ac, c)(accesscontrol.ReqOrgAdminOrEditor, accesscontrol.EvalPermission(action)) {
		errAction := "update"
		if postableSilence.ID == "" {
			errAction = "create"
		}
		return ErrResp(http.StatusUnauthorized, fmt.Errorf("user is not authorized to %s silences", errAction), "")
	}

	silenceID, err := am.CreateSilence(&postableSilence)
	if err != nil {
		if errors.Is(err, notifier.ErrSilenceNotFound) {
			return ErrResp(http.StatusNotFound, err, "")
		}

		if errors.Is(err, notifier.ErrCreateSilenceBadPayload) {
			return ErrResp(http.StatusBadRequest, err, "")
		}

		return ErrResp(http.StatusInternalServerError, err, "failed to create silence")
	}
	return response.JSON(http.StatusAccepted, util.DynMap{"message": "silence created", "id": silenceID})
}

func (srv AlertmanagerSrv) RouteDeleteAlertingConfig(c *models.ReqContext) response.Response {
	am, errResp := srv.AlertmanagerFor(c.OrgId)
	if errResp != nil {
		return errResp
	}

	if err := am.SaveAndApplyDefaultConfig(c.Req.Context()); err != nil {
		srv.log.Error("unable to save and apply default alertmanager configuration", "err", err)
		return ErrResp(http.StatusInternalServerError, err, "failed to save and apply default Alertmanager configuration")
	}

	return response.JSON(http.StatusAccepted, util.DynMap{"message": "configuration deleted; the default is applied"})
}

func (srv AlertmanagerSrv) RouteDeleteSilence(c *models.ReqContext) response.Response {
	am, errResp := srv.AlertmanagerFor(c.OrgId)
	if errResp != nil {
		return errResp
	}

	silenceID := web.Params(c.Req)[":SilenceId"]
	if err := am.DeleteSilence(silenceID); err != nil {
		if errors.Is(err, notifier.ErrSilenceNotFound) {
			return ErrResp(http.StatusNotFound, err, "")
		}
		return ErrResp(http.StatusInternalServerError, err, "")
	}
	return response.JSON(http.StatusOK, util.DynMap{"message": "silence deleted"})
}

func (srv AlertmanagerSrv) RouteGetAlertingConfig(c *models.ReqContext) response.Response {
	config, err := srv.mam.GetAlertmanagerConfiguration(c.Req.Context(), c.OrgId)
	if err != nil {
		if errors.Is(err, store.ErrNoAlertmanagerConfiguration) {
			return ErrResp(http.StatusNotFound, err, "")
		}
		return ErrResp(http.StatusInternalServerError, err, err.Error())
	}
	return response.JSON(http.StatusOK, config)
}

func (srv AlertmanagerSrv) RouteGetAMAlertGroups(c *models.ReqContext) response.Response {
	am, errResp := srv.AlertmanagerFor(c.OrgId)
	if errResp != nil {
		return errResp
	}

	groups, err := am.GetAlertGroups(
		c.QueryBoolWithDefault("active", true),
		c.QueryBoolWithDefault("silenced", true),
		c.QueryBoolWithDefault("inhibited", true),
		c.QueryStrings("filter"),
		c.Query("receiver"),
	)
	if err != nil {
		if errors.Is(err, notifier.ErrGetAlertGroupsBadPayload) {
			return ErrResp(http.StatusBadRequest, err, "")
		}
		// any other error here should be an unexpected failure and thus an internal error
		return ErrResp(http.StatusInternalServerError, err, "")
	}

	return response.JSON(http.StatusOK, groups)
}

func (srv AlertmanagerSrv) RouteGetAMAlerts(c *models.ReqContext) response.Response {
	am, errResp := srv.AlertmanagerFor(c.OrgId)
	if errResp != nil {
		return errResp
	}

	alerts, err := am.GetAlerts(
		c.QueryBoolWithDefault("active", true),
		c.QueryBoolWithDefault("silenced", true),
		c.QueryBoolWithDefault("inhibited", true),
		c.QueryStrings("filter"),
		c.Query("receiver"),
	)
	if err != nil {
		if errors.Is(err, notifier.ErrGetAlertsBadPayload) {
			return ErrResp(http.StatusBadRequest, err, "")
		}
		if errors.Is(err, notifier.ErrGetAlertsUnavailable) {
			return ErrResp(http.StatusServiceUnavailable, err, "")
		}
		// any other error here should be an unexpected failure and thus an internal error
		return ErrResp(http.StatusInternalServerError, err, "")
	}

	return response.JSON(http.StatusOK, alerts)
}

func (srv AlertmanagerSrv) RouteGetSilence(c *models.ReqContext) response.Response {
	am, errResp := srv.AlertmanagerFor(c.OrgId)
	if errResp != nil {
		return errResp
	}

	silenceID := web.Params(c.Req)[":SilenceId"]
	gettableSilence, err := am.GetSilence(silenceID)
	if err != nil {
		if errors.Is(err, notifier.ErrSilenceNotFound) {
			return ErrResp(http.StatusNotFound, err, "")
		}
		// any other error here should be an unexpected failure and thus an internal error
		return ErrResp(http.StatusInternalServerError, err, "")
	}
	return response.JSON(http.StatusOK, gettableSilence)
}

func (srv AlertmanagerSrv) RouteGetSilences(c *models.ReqContext) response.Response {
	am, errResp := srv.AlertmanagerFor(c.OrgId)
	if errResp != nil {
		return errResp
	}

	gettableSilences, err := am.ListSilences(c.QueryStrings("filter"))
	if err != nil {
		if errors.Is(err, notifier.ErrListSilencesBadPayload) {
			return ErrResp(http.StatusBadRequest, err, "")
		}
		// any other error here should be an unexpected failure and thus an internal error
		return ErrResp(http.StatusInternalServerError, err, "")
	}
	return response.JSON(http.StatusOK, gettableSilences)
}

func (srv AlertmanagerSrv) RoutePostAlertingConfig(c *models.ReqContext, body apimodels.PostableUserConfig) response.Response {
	err := srv.mam.ApplyAlertmanagerConfiguration(c.Req.Context(), c.OrgId, body)
	if err == nil {
		return response.JSON(http.StatusAccepted, util.DynMap{"message": "configuration created"})
	}
	var unknownReceiverError notifier.UnknownReceiverError
	if errors.As(err, &unknownReceiverError) {
		return ErrResp(http.StatusBadRequest, unknownReceiverError, "")
	}
	var configRejectedError notifier.AlertmanagerConfigRejectedError
	if errors.As(err, &configRejectedError) {
		return ErrResp(http.StatusBadRequest, configRejectedError, "")
	}
	if errors.Is(err, notifier.ErrNoAlertmanagerForOrg) {
		return response.Error(http.StatusNotFound, err.Error(), err)
	}
	if errors.Is(err, notifier.ErrAlertmanagerNotReady) {
		return response.Error(http.StatusConflict, err.Error(), err)
	}

	return ErrResp(http.StatusInternalServerError, err, "")
}

func (srv AlertmanagerSrv) RoutePostAMAlerts(_ *models.ReqContext, _ apimodels.PostableAlerts) response.Response {
	return NotImplementedResp
}

func (srv AlertmanagerSrv) RoutePostTestReceivers(c *models.ReqContext, body apimodels.TestReceiversConfigBodyParams) response.Response {
	if err := srv.crypto.LoadSecureSettings(c.Req.Context(), c.OrgId, body.Receivers); err != nil {
		var unknownReceiverError UnknownReceiverError
		if errors.As(err, &unknownReceiverError) {
			return ErrResp(http.StatusBadRequest, err, "")
		}
		return ErrResp(http.StatusInternalServerError, err, "")
	}

	if err := body.ProcessConfig(srv.crypto.Encrypt); err != nil {
		return ErrResp(http.StatusInternalServerError, err, "failed to post process Alertmanager configuration")
	}

	ctx, cancelFunc, err := contextWithTimeoutFromRequest(
		c.Req.Context(),
		c.Req,
		defaultTestReceiversTimeout,
		maxTestReceiversTimeout)
	if err != nil {
		return ErrResp(http.StatusBadRequest, err, "")
	}
	defer cancelFunc()

	am, errResp := srv.AlertmanagerFor(c.OrgId)
	if errResp != nil {
		return errResp
	}

	result, err := am.TestReceivers(ctx, body)
	if err != nil {
		if errors.Is(err, notifier.ErrNoReceivers) {
			return response.Error(http.StatusBadRequest, "", err)
		}
		return response.Error(http.StatusInternalServerError, "", err)
	}

	return response.JSON(statusForTestReceivers(result.Receivers), newTestReceiversResult(result))
}

// contextWithTimeoutFromRequest returns a context with a deadline set from the
// Request-Timeout header in the HTTP request. If the header is absent then the
// context will use the default timeout. The timeout in the Request-Timeout
// header cannot exceed the maximum timeout.
func contextWithTimeoutFromRequest(ctx context.Context, r *http.Request, defaultTimeout, maxTimeout time.Duration) (context.Context, context.CancelFunc, error) {
	timeout := defaultTimeout
	if s := strings.TrimSpace(r.Header.Get("Request-Timeout")); s != "" {
		// the timeout is measured in seconds
		v, err := strconv.ParseInt(s, 10, 16)
		if err != nil {
			return nil, nil, err
		}
		if d := time.Duration(v) * time.Second; d < maxTimeout {
			timeout = d
		} else {
			return nil, nil, fmt.Errorf("exceeded maximum timeout of %d seconds", maxTimeout)
		}
	}
	ctx, cancelFunc := context.WithTimeout(ctx, timeout)
	return ctx, cancelFunc, nil
}

func newTestReceiversResult(r *notifier.TestReceiversResult) apimodels.TestReceiversResult {
	v := apimodels.TestReceiversResult{
		Alert: apimodels.TestReceiversConfigAlertParams{
			Annotations: r.Alert.Annotations,
			Labels:      r.Alert.Labels,
		},
		Receivers:  make([]apimodels.TestReceiverResult, len(r.Receivers)),
		NotifiedAt: r.NotifedAt,
	}
	for ix, next := range r.Receivers {
		configs := make([]apimodels.TestReceiverConfigResult, len(next.Configs))
		for jx, config := range next.Configs {
			configs[jx].Name = config.Name
			configs[jx].UID = config.UID
			configs[jx].Status = config.Status
			if config.Error != nil {
				configs[jx].Error = config.Error.Error()
			}
		}
		v.Receivers[ix].Configs = configs
		v.Receivers[ix].Name = next.Name
	}
	return v
}

// statusForTestReceivers returns the appropriate status code for the response
// for the results.
//
// It returns an HTTP 200 OK status code if notifications were sent to all receivers,
// an HTTP 400 Bad Request status code if all receivers contain invalid configuration,
// an HTTP 408 Request Timeout status code if all receivers timed out when sending
// a test notification or an HTTP 207 Multi Status.
func statusForTestReceivers(v []notifier.TestReceiverResult) int {
	var (
		numBadRequests   int
		numTimeouts      int
		numUnknownErrors int
	)
	for _, receiver := range v {
		for _, next := range receiver.Configs {
			if next.Error != nil {
				var (
					invalidReceiverErr notifier.InvalidReceiverError
					receiverTimeoutErr notifier.ReceiverTimeoutError
				)
				if errors.As(next.Error, &invalidReceiverErr) {
					numBadRequests += 1
				} else if errors.As(next.Error, &receiverTimeoutErr) {
					numTimeouts += 1
				} else {
					numUnknownErrors += 1
				}
			}
		}
	}
	if numBadRequests == len(v) {
		// if all receivers contain invalid configuration
		return http.StatusBadRequest
	} else if numTimeouts == len(v) {
		// if all receivers contain valid configuration but timed out
		return http.StatusRequestTimeout
	} else if numBadRequests+numTimeouts+numUnknownErrors > 0 {
		return http.StatusMultiStatus
	} else {
		// all receivers were sent a notification without error
		return http.StatusOK
	}
}

func (srv AlertmanagerSrv) AlertmanagerFor(orgID int64) (Alertmanager, *response.NormalResponse) {
	am, err := srv.mam.AlertmanagerFor(orgID)
	if err == nil {
		return am, nil
	}

	if errors.Is(err, notifier.ErrNoAlertmanagerForOrg) {
		return nil, response.Error(http.StatusNotFound, err.Error(), err)
	}
	if errors.Is(err, notifier.ErrAlertmanagerNotReady) {
		return am, response.Error(http.StatusConflict, err.Error(), err)
	}

	srv.log.Error("unable to obtain the org's Alertmanager", "err", err)
	return nil, response.Error(http.StatusInternalServerError, "unable to obtain org's Alertmanager", err)
}
