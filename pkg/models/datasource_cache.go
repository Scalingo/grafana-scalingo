package models

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"sync"
	"time"
)

type proxyTransportCache struct {
	cache map[int64]cachedTransport
	sync.Mutex
}

type cachedTransport struct {
	updated time.Time

	*http.Transport
}

var ptc = proxyTransportCache{
	cache: make(map[int64]cachedTransport),
}

func (ds *DataSource) GetHttpClient() (*http.Client, error) {
	transport, err := ds.GetHttpTransport()

	if err != nil {
		return nil, err
	}

	return &http.Client{
		Timeout:   time.Duration(30 * time.Second),
		Transport: transport,
	}, nil
}

func (ds *DataSource) GetHttpTransport() (*http.Transport, error) {
	ptc.Lock()
	defer ptc.Unlock()

	if t, present := ptc.cache[ds.Id]; present && ds.Updated.Equal(t.updated) {
		return t.Transport, nil
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
	}

	var tlsAuth, tlsAuthWithCACert bool
	if ds.JsonData != nil {
		tlsAuth = ds.JsonData.Get("tlsAuth").MustBool(false)
		tlsAuthWithCACert = ds.JsonData.Get("tlsAuthWithCACert").MustBool(false)
	}

	if tlsAuth {
		transport.TLSClientConfig.InsecureSkipVerify = false

		decrypted := ds.SecureJsonData.Decrypt()

		if tlsAuthWithCACert && len(decrypted["tlsCACert"]) > 0 {
			caPool := x509.NewCertPool()
			ok := caPool.AppendCertsFromPEM([]byte(decrypted["tlsCACert"]))
			if ok {
				transport.TLSClientConfig.RootCAs = caPool
			}
		}

		cert, err := tls.X509KeyPair([]byte(decrypted["tlsClientCert"]), []byte(decrypted["tlsClientKey"]))
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}

	ptc.cache[ds.Id] = cachedTransport{
		Transport: transport,
		updated:   ds.Updated,
	}

	return transport, nil
}
