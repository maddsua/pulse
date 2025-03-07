package socks

import (
	"net/url"

	"golang.org/x/net/proxy"
)

func NewSocksProxyDialer(host string, user *url.Userinfo) (proxy.ContextDialer, error) {

	var proxyAuth *proxy.Auth
	if user.Username() != "" {

		proxyAuth = &proxy.Auth{User: user.Username()}

		if pass, has := user.Password(); has {
			proxyAuth.Password = pass
		}
	}

	dialer, err := proxy.SOCKS5("tcp", host, proxyAuth, proxy.Direct)
	if err != nil {
		return nil, err
	}

	return dialer.(proxy.ContextDialer), nil
}
