package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"time"
	"net/url"
)

const (
	ISE_HTTP_LOG = "ISE_HTTP_LOG"
)

func CreateTimeoutTransport(timeout time.Duration) *http.Transport {
	certPool := RootCAsGlobal()
	cfg := &tls.Config{RootCAs: certPool}
	
	// 配置自定义DNS解析器，避免Android DNS问题
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Resolver: &net.Resolver{
			PreferGo: true, // 使用Go内置的DNS解析器
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				// 强制使用IPv4进行DNS查询
				if network == "tcp" || network == "udp" {
					network = network + "4"
				}
				// 使用公共DNS服务器作为备选
				d := net.Dialer{
					Timeout: timeout,
				}
				// 尝试多个DNS服务器
				dnsServers := []string{
					"8.8.8.8:53",    // Google DNS
					"8.8.4.4:53",    // Google DNS备用
					"1.1.1.1:53",    // Cloudflare DNS
					"114.114.114.114:53", // 114 DNS
				}
				
				var lastErr error
				for _, dnsServer := range dnsServers {
					conn, err := d.DialContext(ctx, network, dnsServer)
					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				return nil, fmt.Errorf("所有DNS服务器均失败: %v", lastErr)
			},
		},
	}
	
	return &http.Transport{
		TLSClientConfig: cfg,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// 强制使用IPv4，避免Android IPv6 DNS问题
			if network == "tcp" {
				network = "tcp4"
			}
			return dialer.DialContext(ctx, network, addr)
		},
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			// 强制使用IPv4，避免Android IPv6 DNS问题
			if network == "tcp" {
				network = "tcp4"
			}
			tconn, err := dialer.DialContext(ctx, network, addr)
			if err != nil {
				return nil, err
			}
			return NewTimeoutConn(tls.Client(tconn, &tls.Config{
				ServerName: host,
				RootCAs:    certPool,
			}), timeout), nil
		},
		ForceAttemptHTTP2: true,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
	}
}

type LogTransport struct {
	lower http.RoundTripper
}

func (lt *LogTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqd, err0 := httputil.DumpRequestOut(req, false)
	if err0 != nil {
		return nil, err0
	}
	timeStart := time.Now().Unix()
	resp, err := lt.lower.RoundTrip(req)
	if err != nil {
		fmt.Println(string(reqd))
		fmt.Println("Failed ", err)
		fmt.Println()
		return resp, err
	}
	respd, err0 := httputil.DumpResponse(resp, false)
	if err0 != nil {
		return nil, err0
	}
	fmt.Println(string(reqd))
	fmt.Println(string(respd))
	fmt.Println("Success in ", time.Now().Unix()-timeStart, " seconds")
	return resp, err
}

func CreateLogTransport(lower http.RoundTripper) http.RoundTripper {
	if os.Getenv(ISE_HTTP_LOG) == "1" {
		return &LogTransport{
			lower: lower,
		}
	} else {
		return lower
	}
}
