package proxy

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidTargetHost = errors.New("invalid target host")
	ErrURLParseFailed    = errors.New("URL parse failed")
	ErrSendRequestFailed = errors.New("error sending request to target")
)

// staticAssetExtensions defines the file extensions allowed for caching.
var staticAssetExtensions = map[string]bool{
	".js":    true,
	".css":   true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".svg":   true,
	".ico":   true,
	".webp":  true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".eot":   true,
	".json":  true,
}

func removeHopByHopHeaders(header http.Header) {
	for _, v := range header.Values("Connection") {
		for raw := range strings.SplitSeq(v, ",") {
			token := http.CanonicalHeaderKey(strings.TrimSpace(raw))
			if token == "" {
				continue
			}

			header.Del(token)
			slog.Debug("Removed header referenced in Connection header:", "header", token)
		}
	}

	hopHeaders := []string{
		"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
		"Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade",
	}
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func addrToUrl(addr string, httpsDefault bool) (*url.URL, error) {
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		if httpsDefault {
			addr = "https://" + addr
		} else {
			addr = "http://" + addr
		}
	}
	u, err := url.Parse(addr)
	if err != nil {
		slog.Error("Failed to parse URL", "addr", addr, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrURLParseFailed, err)
	}
	return u, nil
}

func changeRequestToTarget(req *http.Request, httpsDefault bool) error {
	// Infer scheme from request URI if present
	isHttp := strings.HasPrefix(req.RequestURI, "http://")
	httpsDefault = !isHttp
	targetHost := req.Host
	targetUrl, err := addrToUrl(targetHost, httpsDefault)
	if err != nil {
		slog.Error("Invalid target host", "host", targetHost, "error", err)
		return fmt.Errorf("%w: %v", ErrInvalidTargetHost, err)
	}

	targetUrl.Path = req.URL.Path
	targetUrl.RawQuery = req.URL.RawQuery
	targetUrl.Fragment = req.URL.Fragment
	req.URL = targetUrl
	// Make sure this is unset for sending the request through a client
	req.RequestURI = ""

	return nil
}

func sendRequestToTarget(req *http.Request, httpsDefault bool) (*http.Response, error) {
	// Change request URL to point to the target server.
	changeRequestToTarget(req, httpsDefault)
	// Remove hop-by-hop headers in the request that should not be forwarded to the target server.
	removeHopByHopHeaders(req.Header)

	slog.Debug("Sending request", "url", req.URL, "method", req.Method)
	var client *http.Client
	var err error
	if httpsDefault {
		// Custom TLS config: skip upstream cert verification
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			ResponseHeaderTimeout: 10 * time.Second,
			DisableCompression:    true,
		}

		client = &http.Client{
			Transport: tr,
			Timeout:   15 * time.Second,
		}
	} else {
		tr := &http.Transport{
			DisableCompression: true,
		}
		client = &http.Client{
			Transport: tr,
			Timeout:   15 * time.Second,
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Error sending request to target", "url", req.URL, "error", err)
		return nil, fmt.Errorf("%w: %v", ErrSendRequestFailed, err)
	}
	slog.Debug("Sent request to target", "url", req.URL, "status", resp.Status)

	// Remove any hop-by-hop headers in the response that should not be forwarded to the client.
	removeHopByHopHeaders(resp.Header)

	return resp, nil
}
