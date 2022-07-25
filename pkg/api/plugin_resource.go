package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins/backendplugin"
	"github.com/grafana/grafana/pkg/util/errutil"
	"github.com/grafana/grafana/pkg/util/proxyutil"
	"github.com/grafana/grafana/pkg/web"
)

// CallResource passes a resource call from a plugin to the backend plugin.
//
// /api/plugins/:pluginId/resources/*
func (hs *HTTPServer) CallResource(c *models.ReqContext) {
	hs.callPluginResource(c, web.Params(c.Req)[":pluginId"])
}

func (hs *HTTPServer) callPluginResource(c *models.ReqContext, pluginID string) {
	pCtx, found, err := hs.PluginContextProvider.Get(c.Req.Context(), pluginID, c.SignedInUser)
	if err != nil {
		c.JsonApiErr(500, "Failed to get plugin settings", err)
		return
	}
	if !found {
		c.JsonApiErr(404, "Plugin not found", nil)
		return
	}

	req, err := hs.pluginResourceRequest(c)
	if err != nil {
		c.JsonApiErr(http.StatusBadRequest, "Failed for create plugin resource request", err)
		return
	}

	if err = hs.makePluginResourceRequest(c.Resp, req, pCtx); err != nil {
		handleCallResourceError(err, c)
	}
}

func (hs *HTTPServer) callPluginResourceWithDataSource(c *models.ReqContext, pluginID string, ds *models.DataSource) {
	pCtx, found, err := hs.PluginContextProvider.GetWithDataSource(c.Req.Context(), pluginID, c.SignedInUser, ds)
	if err != nil {
		c.JsonApiErr(500, "Failed to get plugin settings", err)
		return
	}
	if !found {
		c.JsonApiErr(404, "Plugin not found", nil)
		return
	}

	var dsURL string
	if pCtx.DataSourceInstanceSettings != nil {
		dsURL = pCtx.DataSourceInstanceSettings.URL
	}

	err = hs.PluginRequestValidator.Validate(dsURL, c.Req)
	if err != nil {
		c.JsonApiErr(http.StatusForbidden, "Access denied", err)
		return
	}

	req, err := hs.pluginResourceRequest(c)
	if err != nil {
		c.JsonApiErr(http.StatusBadRequest, "Failed for create plugin resource request", err)
		return
	}

	if hs.DataProxy.OAuthTokenService.IsOAuthPassThruEnabled(ds) {
		if token := hs.DataProxy.OAuthTokenService.GetCurrentOAuthToken(c.Req.Context(), c.SignedInUser); token != nil {
			req.Header.Add("Authorization", fmt.Sprintf("%s %s", token.Type(), token.AccessToken))

			idToken, ok := token.Extra("id_token").(string)
			if ok && idToken != "" {
				req.Header.Add("X-ID-Token", idToken)
			}
		}
	}

	if err = hs.makePluginResourceRequest(c.Resp, req, pCtx); err != nil {
		handleCallResourceError(err, c)
	}
}

func (hs *HTTPServer) pluginResourceRequest(c *models.ReqContext) (*http.Request, error) {
	clonedReq := c.Req.Clone(c.Req.Context())
	rawURL := web.Params(c.Req)["*"]
	if clonedReq.URL.RawQuery != "" {
		rawURL += "?" + clonedReq.URL.RawQuery
	}
	urlPath, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	clonedReq.URL = urlPath

	return clonedReq, nil
}

func (hs *HTTPServer) makePluginResourceRequest(w http.ResponseWriter, req *http.Request, pCtx backend.PluginContext) error {
	keepCookieModel := struct {
		KeepCookies []string `json:"keepCookies"`
	}{}
	if dis := pCtx.DataSourceInstanceSettings; dis != nil {
		err := json.Unmarshal(dis.JSONData, &keepCookieModel)
		if err != nil {
			hs.log.Warn("failed to unpack JSONData in datasource instance settings", "err", err)
		}
	}
	proxyutil.ClearCookieHeader(req, keepCookieModel.KeepCookies)
	proxyutil.PrepareProxyRequest(req)

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	crReq := &backend.CallResourceRequest{
		PluginContext: pCtx,
		Path:          req.URL.Path,
		Method:        req.Method,
		URL:           req.URL.String(),
		Headers:       req.Header,
		Body:          body,
	}

	childCtx, cancel := context.WithCancel(req.Context())
	defer cancel()
	stream := newCallResourceResponseStream(childCtx)

	var wg sync.WaitGroup
	wg.Add(1)

	defer func() {
		if err := stream.Close(); err != nil {
			hs.log.Warn("Failed to close plugin resource stream", "err", err)
		}
		wg.Wait()
	}()

	var flushStreamErr error
	go func() {
		flushStreamErr = hs.flushStream(stream, w)
		wg.Done()
	}()

	if err := hs.pluginClient.CallResource(req.Context(), crReq, stream); err != nil {
		return err
	}

	return flushStreamErr
}

func (hs *HTTPServer) flushStream(stream callResourceClientResponseStream, w http.ResponseWriter) error {
	processedStreams := 0

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			if processedStreams == 0 {
				return errors.New("received empty resource response")
			}
			return nil
		}
		if err != nil {
			if processedStreams == 0 {
				return errutil.Wrap("failed to receive response from resource call", err)
			}

			hs.log.Error("Failed to receive response from resource call", "err", err)
			return stream.Close()
		}

		// Expected that headers and status are only part of first stream
		if processedStreams == 0 && resp.Headers != nil {
			// Make sure a content type always is returned in response
			if _, exists := resp.Headers["Content-Type"]; !exists {
				resp.Headers["Content-Type"] = []string{"application/json"}
			}

			for k, values := range resp.Headers {
				// Due to security reasons we don't want to forward
				// cookies from a backend plugin to clients/browsers.
				if k == "Set-Cookie" {
					continue
				}

				for _, v := range values {
					// TODO: Figure out if we should use Set here instead
					// nolint:gocritic
					w.Header().Add(k, v)
				}
			}

			proxyutil.SetProxyResponseHeaders(w.Header())

			w.WriteHeader(resp.Status)
		}

		if _, err := w.Write(resp.Body); err != nil {
			hs.log.Error("Failed to write resource response", "err", err)
		}

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		processedStreams++
	}
}

func handleCallResourceError(err error, reqCtx *models.ReqContext) {
	if errors.Is(err, backendplugin.ErrPluginUnavailable) {
		reqCtx.JsonApiErr(503, "Plugin unavailable", err)
		return
	}

	if errors.Is(err, backendplugin.ErrMethodNotImplemented) {
		reqCtx.JsonApiErr(404, "Not found", err)
		return
	}

	reqCtx.JsonApiErr(500, "Failed to call resource", err)
}

// callResourceClientResponseStream is used for receiving resource call responses.
type callResourceClientResponseStream interface {
	Recv() (*backend.CallResourceResponse, error)
	Close() error
}

type callResourceResponseStream struct {
	ctx    context.Context
	stream chan *backend.CallResourceResponse
	closed bool
}

func newCallResourceResponseStream(ctx context.Context) *callResourceResponseStream {
	return &callResourceResponseStream{
		ctx:    ctx,
		stream: make(chan *backend.CallResourceResponse),
	}
}

func (s *callResourceResponseStream) Send(res *backend.CallResourceResponse) error {
	if s.closed {
		return errors.New("cannot send to a closed stream")
	}

	select {
	case <-s.ctx.Done():
		return errors.New("cancelled")
	case s.stream <- res:
		return nil
	}
}

func (s *callResourceResponseStream) Recv() (*backend.CallResourceResponse, error) {
	select {
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case res, ok := <-s.stream:
		if !ok {
			return nil, io.EOF
		}
		return res, nil
	}
}

func (s *callResourceResponseStream) Close() error {
	if s.closed {
		return errors.New("cannot close a closed stream")
	}

	close(s.stream)
	s.closed = true
	return nil
}
