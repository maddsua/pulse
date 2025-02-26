package main

import (
	"net/url"

	"golang.org/x/net/proxy"
)

func NewSocksProxyDialer(proxyUrl *url.URL) (proxy.ContextDialer, error) {

	var proxyAuth *proxy.Auth
	if proxyUrl.User.Username() != "" {

		proxyAuth = &proxy.Auth{User: proxyUrl.User.Username()}

		if pass, has := proxyUrl.User.Password(); has {
			proxyAuth.Password = pass
		}
	}

	dialer, err := proxy.SOCKS5("tcp", proxyUrl.Host, proxyAuth, proxy.Direct)
	if err != nil {
		return nil, err
	}

	return dialer.(proxy.ContextDialer), nil
}
