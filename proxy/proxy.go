package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"reservoir/cache"
	"reservoir/config"
	"reservoir/metrics"
	"reservoir/proxy/certs"
	"reservoir/proxy/headers"
	"reservoir/proxy/responder"
	"reservoir/utils/httplistener"
	"reservoir/utils/typeutils"
	"strconv"
	"strings"
	"time"
)

var (
	ErrCacheGetFailed       = errors.New("error getting cache for key")
	ErrNoCachedResponse304  = errors.New("received 304 Not Modified but no cached response found")
	ErrUpdateCacheMetadata  = errors.New("error updating cache metadata")
	ErrCacheResponseFailed  = errors.New("error caching response")
	ErrTLSCertFailed        = errors.New("error getting TLS certificate")
	ErrClientResponseFailed = errors.New("failed to write HTTP OK response to client")
	ErrReadRequestFailed    = errors.New("error reading request from client")
	ErrRangeNotSatisfiable  = errors.New("range not satisfiable")
	ErrIfRangeMismatch      = errors.New("If-Range header mismatch")
	ErrBadGateway           = errors.New("bad gateway. Error when sending request to upstream")
)

type cachedRequestInfo struct {
	ETag         string
	LastModified time.Time
	Header       http.Header
}

type Proxy struct {
	ca    certs.CertAuthority
	cache *cache.FileCache[cachedRequestInfo]
	fetch fetcher
}

func (p *Proxy) Listen(address string, errChan chan error, ctx context.Context) {
	listener := httplistener.New(address, p)
	listener.ListenWithCancel(errChan, ctx)
}

// Creates a new MITM proxy. It should be passed the filenames
// for the certificate and private key of a certificate authority trusted by the
// client's machine.
func NewProxy(cacheDir string, ca certs.CertAuthority, ctx context.Context) (*Proxy, error) {
	cacheCleanupInterval := config.Global.CacheCleanupInterval.Read().Cast()
	cache := cache.NewFileCache[cachedRequestInfo](cacheDir, cacheCleanupInterval, ctx)
	return &Proxy{
		ca:    ca,
		cache: cache,
		fetch: newFetcher(cache),
	}, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, proxyReq *http.Request) {
	r := responder.NewHTTPResponder(w)

	if proxyReq.Method != http.MethodConnect && p.isPurgeRequest(proxyReq) {
		if err := p.handlePurgeEndpoint(r, proxyReq); err != nil {
			slog.Error("Error handling purge request", "error", err)
		}
		return
	}

	if proxyReq.Method == http.MethodConnect {
		if err := p.handleCONNECT(r, proxyReq); err != nil {
			slog.Error("Error handling CONNECT request", "error", err)
			return
		}
	} else {
		if err := p.handleHTTP(r, proxyReq); err != nil {
			slog.Error("Error handling HTTP request", "error", err)
			return
		}
	}
}

func finalizeAndRespond(r responder.Responder, resp io.Reader, status int, req *http.Request) error {
	body := resp
	if req.Method == http.MethodHead {
		body = http.NoBody
	}

	switch {
	case status >= 200 && status < 300:
		metrics.Global.Requests.StatusOKResponses.Increment()
	case status >= 400 && status < 500:
		metrics.Global.Requests.StatusClientErrorResponses.Increment()
	case status >= 500 && status < 600:
		metrics.Global.Requests.StatusServerErrorResponses.Increment()
	}

	written, err := r.Write(status, body)
	if err != nil {
		slog.Error("Error writing response", "url", req.URL, "error", err)
		return err
	}

	metrics.Global.Requests.BytesServed.Add(written)
	slog.Info("Response sent", "url", req.URL, "bytes_written", written)
	return nil
}

func (p *Proxy) handleRangeRequest(r responder.Responder, req *http.Request, cached *cache.Entry[cachedRequestInfo], key cache.CacheKey, clientHd *headers.HeaderDirectives) error {
	rangeHeader := clientHd.Range.Value()
	start, end, err := rangeHeader.SliceSize(cached.Metadata.FileSize)
	if err != nil {
		slog.Error("Error slicing Range header", "url", req.URL, "key", key, "error", err, "range_header", rangeHeader, "file_size", cached.Metadata.FileSize)

		if !config.Global.RetryOnInvalidRange.Read() {
			slog.Error("Sending 416 Range Not Satisfiable due to invalid Range header from client.", "url", req.URL, "key", key, "range_header", rangeHeader)

			r.SetHeader("Accept-Ranges", "bytes")
			r.SetHeader("Content-Range", fmt.Sprintf("bytes */%d", cached.Metadata.FileSize))
			r.WriteError("invalid Range header", http.StatusRequestedRangeNotSatisfiable)

			return ErrRangeNotSatisfiable
		}

		slog.Info("Retrying request without Range header due to invalid Range header from client.", "url", req.URL, "key", key, "range_header", rangeHeader)

		clientHd.Range.SyncRemove(req.Header)
		fetched, err := p.fetch.dedupFetch(req, key, clientHd)
		if err != nil {
			slog.Error("Error fetching resource without Range header", "url", req.URL, "key", key, "error", err)
			return err
		}

		data, header, status := fetched.getResponse()
		defer data.Close()

		r.SetHeaders(header)
		return finalizeAndRespond(r, data, status, req)
	}

	if clientHd.IfRange.IsPresent() {
		ifRange := clientHd.IfRange.Value()
		if ifRange.IsLeft() {
			// IfRange is ETag
			etagIfRange := ifRange.ForceUnwrapLeft()
			if etagIfRange != cached.Metadata.Object.ETag {
				slog.Info("If-Range does not match cached ETag. Sending full 200 response.", "url", req.URL, "key", key)
				return ErrIfRangeMismatch
			}
		} else {
			// IfRange is Time
			timeIfRange := ifRange.ForceUnwrapRight()
			if timeIfRange.Before(cached.Metadata.Object.LastModified) {
				slog.Info("If-Range does not match cached Last-Modified. Sending full 200 response.", "url", req.URL, "key", key)
				return ErrIfRangeMismatch
			}
		}

	}

	length := end - start + 1
	slog.Info("Serving Range request from cache", "url", req.URL, "key", key, "range_header", rangeHeader, "start", start, "end", end, "length", length)

	r.SetHeaders(cached.Metadata.Object.Header)
	r.SetHeader("Accept-Ranges", "bytes")
	r.SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, cached.Metadata.FileSize))
	r.SetHeader("Content-Length", fmt.Sprintf("%d", length))
	r.SetHeader("ETag", cached.Metadata.Object.ETag)
	r.SetHeader("Last-Modified", cached.Metadata.Object.LastModified.Format(http.TimeFormat))

	sections := io.NewSectionReader(cached.Data, start, length)
	return finalizeAndRespond(r, sections, http.StatusPartialContent, req)
}

func (p *Proxy) processRequest(r responder.Responder, req *http.Request, key cache.CacheKey, clientHd *headers.HeaderDirectives) error {
	slog.Info("Processing HTTP request", "remote_addr", req.RemoteAddr, "method", req.Method, "url", req.URL)

	startTime := time.Now()

	defer func() {
		metrics.Global.Requests.ClientRequestLatency.Add(time.Since(startTime).Nanoseconds())
	}()

	fetched, err := p.fetch.dedupFetch(req, key, clientHd)
	latency := time.Since(startTime)

	if err != nil {
		slog.Error("Error fetching resource", "url", req.URL, "key", key, "error", err)
		return err
	}

	fetchInfo := fetched.getFetchInfo()
	if fetchInfo.Status == hitStatusMiss {
		metrics.Global.Cache.CacheMisses.Increment()
		metrics.Global.Cache.CacheMissLatency.Add(latency.Nanoseconds())
	} else {
		metrics.Global.Cache.CacheHits.Increment()
		metrics.Global.Cache.CacheHitLatency.Add(latency.Nanoseconds())
	}

	switch fetched.Type {
	case fetchTypeDirect:
		defer fetched.Direct.Response.Body.Close()

		r.SetHeaders(fetched.Direct.Response.Header)
		if fetched.Direct.UpstreamStatus >= 200 && fetched.Direct.UpstreamStatus < 300 {
			r.SetHeader("Accept-Ranges", "bytes")
			addCacheHeaders(r, req, typeutils.None[*cache.Entry[cachedRequestInfo]](), fetchResultToCacheStatus(fetched))
		}
		return finalizeAndRespond(r, fetched.Direct.Response.Body, fetched.Direct.UpstreamStatus, req)

	case fetchTypeCached:
		defer fetched.Cached.Entry.Data.Close()

		if clientHd.Range.IsPresent() {
			if err := p.handleRangeRequest(r, req, fetched.Cached.Entry, key, clientHd); err != nil {
				slog.Error("Error handling Range request", "url", req.URL, "key", key, "error", err)
				if errors.Is(err, ErrIfRangeMismatch) {
					// If the If-Range is mismatched we just move on to send the full 200 cached response.
				} else if errors.Is(err, ErrRangeNotSatisfiable) {
					return err
				} else {
					r.WriteError("error handling Range request", http.StatusInternalServerError)
					return err
				}
			} else {
				return nil
			}
		}

		r.SetHeaders(fetched.Cached.Entry.Metadata.Object.Header)
		r.SetHeader("Accept-Ranges", "bytes")
		r.SetHeader("ETag", fetched.Cached.Entry.Metadata.Object.ETag)
		r.SetHeader("Last-Modified", fetched.Cached.Entry.Metadata.Object.LastModified.Format(http.TimeFormat))
		addCacheHeaders(r, req, typeutils.Some(fetched.Cached.Entry), fetchResultToCacheStatus(fetched))

		slog.Info("Serving cached response", "url", req.URL, "key", key)
		return finalizeAndRespond(r, fetched.Cached.Entry.Data, http.StatusOK, req)

	default:
		// This should not be possible
		return fmt.Errorf("unknown fetch type: %v", fetched.Type)
	}
}

func (p *Proxy) handleHTTP(r responder.Responder, proxyReq *http.Request) error {
	slog.Info("Handling HTTP request", "host", proxyReq.Host, "remote_addr", proxyReq.RemoteAddr)
	metrics.Global.Requests.HTTPProxyRequests.Increment()

	if p.isPurgeRequest(proxyReq) {
		return p.handlePurgeEndpoint(r, proxyReq)
	}

	clientHd := headers.ParseHeaderDirective(proxyReq.Header)
	clientHd.StripRegularConditionals(proxyReq.Header)

	key := cache.MakeFromRequest(proxyReq)

	return p.processRequest(r, proxyReq, key, clientHd)
}

func (p *Proxy) handleCONNECT(r responder.Responder, proxyReq *http.Request) error {
	slog.Info("Handling CONNECT request", "url", proxyReq.URL, "remote_addr", proxyReq.RemoteAddr)

	metrics.Global.Requests.HTTPSProxyRequests.Increment()

	clientConn, _, err := r.Hijack()
	if err != nil {
		r.WriteError("Unable to take over socket.", http.StatusInternalServerError)
		return err
	}
	defer clientConn.Close() // Ensure we always close the hijacked connection

	intermediateResponder := responder.NewRawHTTPResponder(clientConn)

	tlsCert, err := p.ca.GetCertForHost(proxyReq.Host)
	if err != nil {
		// can't use http.Error after hijacking, so we write directly
		slog.Error("Error getting TLS certificate", "host", proxyReq.Host, "error", err)
		intermediateResponder.WriteError("Error getting TLS certificate", http.StatusInternalServerError)
		return fmt.Errorf("%w: %v", ErrTLSCertFailed, err)
	}

	// Send an HTTP OK response back to the client. This initiates the CONNECT
	// tunnel. From this point on the client will assume it's connected directly
	// to the target.
	if err := intermediateResponder.WriteEmpty(http.StatusOK); err != nil {
		slog.Error("Failed to write HTTP OK response to client", "error", err)
		return fmt.Errorf("%w: %v", ErrClientResponseFailed, err)
	}
	slog.Debug("Sent HTTP 200 OK response to client, established CONNECT tunnel")

	// Configure a new TLS server, pointing it at the client connection, using
	// our certificate. This server will now pretend being the target.
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{*tlsCert},
	}
	tlsConn := tls.Server(clientConn, tlsConfig)
	defer tlsConn.Close()

	// Create a buffered reader for the client connection. This is required to
	// use http package functions with this connection.
	connReader := bufio.NewReader(tlsConn)
	responder := responder.NewRawHTTPResponder(tlsConn)

	slog.Debug("Entering request loop for CONNECT tunnel")
	for {
		// Read next HTTP request from client.
		req, err := http.ReadRequest(connReader)
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			slog.Error("Error reading request from client", "remote_addr", proxyReq.RemoteAddr, "error", err)
			return fmt.Errorf("%w: %v ", ErrReadRequestFailed, err)
		}

		tlsState := tlsConn.ConnectionState()
		req.TLS = &tlsState

		if err := p.handleHTTP(responder, req); err != nil {
			slog.Error("Error processing HTTP request", "remote_addr", proxyReq.RemoteAddr, "error", err)
		}
	}

	return nil
}

func (p *Proxy) isPurgeRequest(req *http.Request) bool {
	if req.Method == "PURGE" {
		return true
	}
	cleanPath := path.Clean(req.URL.Path)
	if cleanPath == "/__purge" || cleanPath == "/__cache/purge" || cleanPath == "/__stats" {
		return true
	}
	if cleanPath == "/purge" || cleanPath == "/dashboard" || cleanPath == "/admin" {
		return true
	}
	// Direct request to proxy root (e.g. http://localhost:9999/ or http://<proxy-ip>:9999/)
	if cleanPath == "/" && !strings.HasPrefix(req.RequestURI, "http://") && !strings.HasPrefix(req.RequestURI, "https://") {
		return true
	}
	return false
}

func (p *Proxy) writeJSON(r responder.Responder, status int, data any) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	r.SetHeader("Content-Type", "application/json")
	r.SetHeader("Content-Length", strconv.Itoa(len(jsonBytes)))
	_, err = r.Write(status, strings.NewReader(string(jsonBytes)))
	return err
}

func (p *Proxy) purgeByTargetURL(rawURL string) (bool, error) {
	targetURL := strings.TrimSpace(rawURL)
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "https://" + targetURL
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return false, fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(u.Host)
	cleanPath := path.Clean(u.Path)
	rawQuery := u.RawQuery

	schemes := []string{"https", "http"}
	methods := []string{"GET", "HEAD"}

	anyDeleted := false
	for _, scheme := range schemes {
		for _, method := range methods {
			stringKey := fmt.Sprintf("%s|%s|%s|%s|%s", scheme, method, host, cleanPath, rawQuery)
			key := cache.FromString(stringKey)
			deleted, err := p.cache.Delete(key)
			if err != nil {
				return false, err
			}
			if deleted {
				anyDeleted = true
			}
		}
	}
	return anyDeleted, nil
}

func (p *Proxy) purgeRequest(req *http.Request) (bool, error) {
	reqCopy := *req
	reqCopy.Method = http.MethodGet
	key := cache.MakeFromRequest(&reqCopy)
	purged, err := p.cache.Delete(key)
	if err != nil {
		return false, err
	}
	if purged {
		return true, nil
	}

	// Toggle scheme in case it was cached under the other protocol
	altReq := reqCopy
	if altReq.TLS != nil {
		altReq.TLS = nil
	} else {
		altReq.TLS = &tls.ConnectionState{}
	}
	altKey := cache.MakeFromRequest(&altReq)
	return p.cache.Delete(altKey)
}

func (p *Proxy) handlePurgeEndpoint(r responder.Responder, req *http.Request) error {
	cleanPath := path.Clean(req.URL.Path)

	// API: Stats endpoint
	if cleanPath == "/__stats" {
		memUsed := p.cache.GetMemByteSize()
		maxMem := p.cache.GetMaxMemCacheSize()
		diskUsed := p.cache.GetDiskByteSize()
		maxDisk := int64(0)
		if config.Global != nil {
			maxDisk = config.Global.MaxCacheSize.Read().Bytes()
		}
		entries := p.cache.GetEntriesCount()
		return p.writeJSON(r, http.StatusOK, map[string]any{
			"status":          "ok",
			"mem_used_bytes":  memUsed,
			"mem_max_bytes":   maxMem,
			"mem_enabled":     maxMem > 0,
			"disk_used_bytes": diskUsed,
			"disk_max_bytes":  maxDisk,
			"entries_count":   entries,
			"cache_hits":      metrics.Global.Cache.CacheHits.Get(),
			"mem_cache_hits":  metrics.Global.Cache.MemCacheHits.Get(),
		})
	}

	query := req.URL.Query()
	allParam := query.Get("all")
	urlParam := query.Get("url")
	memParam := query.Get("mem")

	// 1. Purge all cache (RAM + Disk)
	if allParam == "1" || strings.ToLower(allParam) == "true" {
		count, err := p.cache.PurgeAll()
		if err != nil {
			slog.Error("Error purging all cache entries", "error", err)
			return p.writeJSON(r, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": err.Error(),
			})
		}
		slog.Info("Purged all cache via endpoint", "purged_count", count)
		return p.writeJSON(r, http.StatusOK, map[string]any{
			"status":       "ok",
			"action":       "purge_all",
			"purged_count": count,
			"message":      fmt.Sprintf("成功清空全部 %d 个静态资源缓存（内存 + 磁盘）", count),
		})
	}

	// 2. Purge memory cache only
	if memParam == "1" || strings.ToLower(memParam) == "true" {
		count := p.cache.PurgeMemOnly()
		slog.Info("Purged memory cache via endpoint", "purged_count", count)
		return p.writeJSON(r, http.StatusOK, map[string]any{
			"status":       "ok",
			"action":       "purge_mem",
			"purged_count": count,
			"message":      fmt.Sprintf("成功释放内存缓存，清空了 %d 个内存对象（磁盘文件仍保留）", count),
		})
	}

	// 3. Purge by target URL parameter
	if urlParam != "" {
		purged, err := p.purgeByTargetURL(urlParam)
		if err != nil {
			slog.Error("Error purging target URL", "url", urlParam, "error", err)
			return p.writeJSON(r, http.StatusBadRequest, map[string]any{
				"status":  "error",
				"action":  "purge_url",
				"url":     urlParam,
				"message": err.Error(),
			})
		}

		if purged {
			slog.Info("Purged target URL from cache", "url", urlParam)
			return p.writeJSON(r, http.StatusOK, map[string]any{
				"status":  "ok",
				"action":  "purge_url",
				"url":     urlParam,
				"purged":  true,
				"message": fmt.Sprintf("已成功清除目标资源缓存: %s", urlParam),
			})
		}

		slog.Info("Target URL not found in cache for purge", "url", urlParam)
		return p.writeJSON(r, http.StatusNotFound, map[string]any{
			"status":  "not_found",
			"action":  "purge_url",
			"url":     urlParam,
			"purged":  false,
			"message": fmt.Sprintf("未在缓存中找到该资源: %s", urlParam),
		})
	}

	// 4. HTTP PURGE method
	if req.Method == "PURGE" {
		purged, err := p.purgeRequest(req)
		if err != nil {
			slog.Error("Error handling PURGE method", "url", req.URL, "error", err)
			return p.writeJSON(r, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": err.Error(),
			})
		}

		if purged {
			return p.writeJSON(r, http.StatusOK, map[string]any{
				"status":  "ok",
				"action":  "purge",
				"url":     req.URL.String(),
				"purged":  true,
				"message": "Cache entry deleted",
			})
		}

		return p.writeJSON(r, http.StatusNotFound, map[string]any{
			"status":  "not_found",
			"action":  "purge",
			"url":     req.URL.String(),
			"purged":  false,
			"message": "No matching cache entry found",
		})
	}

	// 5. Interactive HTML Web UI
	if strings.Contains(req.Header.Get("Accept"), "text/html") || cleanPath == "/" || cleanPath == "/dashboard" || cleanPath == "/admin" || cleanPath == "/purge" || cleanPath == "/__purge" {
		r.SetHeader("Content-Type", "text/html; charset=utf-8")
		r.SetHeader("Content-Length", strconv.Itoa(len(dashboardHTML)))
		_, _ = r.Write(http.StatusOK, strings.NewReader(dashboardHTML))
		return nil
	}

	// 6. Default JSON instructions & stats
	memUsed := p.cache.GetMemByteSize()
	maxMem := p.cache.GetMaxMemCacheSize()
	return p.writeJSON(r, http.StatusOK, map[string]any{
		"status": "info",
		"name":   "reservoir-lite",
		"memory_cache": map[string]any{
			"enabled":        maxMem > 0,
			"used_bytes":     memUsed,
			"max_bytes":      maxMem,
			"mem_cache_hits": metrics.Global.Cache.MemCacheHits.Get(),
		},
		"instructions": map[string]string{
			"dashboard":   "GET / (Browser Web UI)",
			"purge_all":   "GET /__purge?all=1",
			"purge_mem":   "GET /__purge?mem=1",
			"purge_url":   "GET /__purge?url=<target_url>",
			"http_method": "PURGE <target_url> (via proxy)",
		},
	})
}
