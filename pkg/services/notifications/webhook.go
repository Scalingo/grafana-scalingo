package notifications

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/context/ctxhttp"

	"github.com/grafana/grafana/pkg/util"
)

type Webhook struct {
	Url        string
	User       string
	Password   string
	Body       string
	HttpMethod string
	HttpHeader map[string]string
}

var netTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	Dial: (&net.Dialer{
		Timeout:   30 * time.Second,
		DualStack: true,
	}).Dial,
	TLSHandshakeTimeout: 5 * time.Second,
}
var netClient = &http.Client{
	Timeout:   time.Second * 30,
	Transport: netTransport,
}

func (ns *NotificationService) sendWebRequestSync(ctx context.Context, webhook *Webhook) error {
	ns.log.Debug("Sending webhook", "url", webhook.Url, "http method", webhook.HttpMethod)

	if webhook.HttpMethod == "" {
		webhook.HttpMethod = http.MethodPost
	}

	request, err := http.NewRequest(webhook.HttpMethod, webhook.Url, bytes.NewReader([]byte(webhook.Body)))
	if err != nil {
		return err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("User-Agent", "Grafana")
	if webhook.User != "" && webhook.Password != "" {
		request.Header.Add("Authorization", util.GetBasicAuthHeader(webhook.User, webhook.Password))
	}

	for k, v := range webhook.HttpHeader {
		request.Header.Set(k, v)
	}

	resp, err := ctxhttp.Do(ctx, netClient, request)
	if err != nil {
		return err
	}

	if resp.StatusCode/100 == 2 {
		return nil
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	ns.log.Debug("Webhook failed", "statuscode", resp.Status, "body", string(body))
	return fmt.Errorf("Webhook response status %v", resp.Status)
}
