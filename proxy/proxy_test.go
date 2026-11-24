package proxy

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reservoir/cache"
	"reservoir/proxy/certs"
	"testing"
	"time"
)

func generateTestCA(t *testing.T, certPath, keyPath string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate ecdsa key: %v", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("failed to generate serial: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "reservoir-test-ca",
			Organization: []string{"reservoir-test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create ca cert: %v", err)
	}

	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("failed to create cert file: %v", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		t.Fatalf("failed to encode cert pem: %v", err)
	}

	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}
	defer keyFile.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("failed to marshal pkcs8 private key: %v", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		t.Fatalf("failed to encode key pem: %v", err)
	}
}

func TestStaticAssetExtensionsWhitelist(t *testing.T) {
	testCases := []struct {
		urlPath  string
		expected bool
	}{
		{"/static/bundle.js", true},
		{"/css/style.css", true},
		{"/images/logo.png", true},
		{"/assets/font.woff2", true},
		{"/data/config.json", true},
		{"/assets/module.wasm", true},
		{"/media/video.webm", true},
		{"/api/v1/users", false},
		{"/index.php", false},
		{"/search?q=test", false},
		{"/setup.exe", false},
	}

	for _, tc := range testCases {
		ext := filepath.Ext(tc.urlPath)
		actual := staticAssetExtensions[ext]
		if actual != tc.expected {
			t.Errorf("urlPath %s (ext %s): expected %v, got %v", tc.urlPath, ext, tc.expected, actual)
		}
	}
}

func TestPrivateCA_IPSANSupport(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := filepath.Join(tmpDir, "ca.crt")
	caKeyPath := filepath.Join(tmpDir, "ca.key")

	generateTestCA(t, caCertPath, caKeyPath)

	// Generate a temporary CA
	ca, err := certs.NewPrivateCA(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("failed to create PrivateCA: %v", err)
	}

	// 1. Test IP SAN certificate generation
	targetIPHost := "192.168.1.100:443"
	targetIP := "192.168.1.100"
	tlsCert, err := ca.GetCertForHost(targetIPHost)
	if err != nil {
		t.Fatalf("failed to get cert for IP: %v", err)
	}

	if len(tlsCert.Certificate) == 0 {
		t.Fatalf("certificate chain is empty")
	}

	parsedCert, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	// Verify that the target IP is in IPAddresses
	foundIP := false
	for _, ip := range parsedCert.IPAddresses {
		if ip.Equal(net.ParseIP(targetIP)) {
			foundIP = true
			break
		}
	}
	if !foundIP {
		t.Fatalf("target IP %s not found in certificate IPAddresses: %v", targetIP, parsedCert.IPAddresses)
	}

	// 2. Test domain SAN
	targetDomainHost := "example.com:443"
	targetDomain := "example.com"
	domainCert, err := ca.GetCertForHost(targetDomainHost)
	if err != nil {
		t.Fatalf("failed to get cert for domain: %v", err)
	}
	parsedDomainCert, err := x509.ParseCertificate(domainCert.Certificate[0])
	if err != nil {
		t.Fatalf("failed to parse domain cert: %v", err)
	}
	foundDomain := false
	for _, name := range parsedDomainCert.DNSNames {
		if name == targetDomain {
			foundDomain = true
			break
		}
	}
	if !foundDomain {
		t.Fatalf("target domain %s not found in DNSNames: %v", targetDomain, parsedDomainCert.DNSNames)
	}
}

func TestAddrToUrl(t *testing.T) {
	// Test IP address conversion
	u, err := addrToUrl("127.0.0.1:8080", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Scheme != "http" || u.Host != "127.0.0.1:8080" {
		t.Errorf("expected http://127.0.0.1:8080, got %s", u.String())
	}

	uHttps, err := addrToUrl("127.0.0.1:8443", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uHttps.Scheme != "https" {
		t.Errorf("expected https scheme, got %s", uHttps.Scheme)
	}
}

func TestProxy_PurgeEndpoint_And_TargetURL(t *testing.T) {
	tmpDir := t.TempDir()
	caCertPath := filepath.Join(tmpDir, "ca.crt")
	caKeyPath := filepath.Join(tmpDir, "ca.key")
	generateTestCA(t, caCertPath, caKeyPath)

	ca, err := certs.NewPrivateCA(caCertPath, caKeyPath)
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p, err := NewProxy(tmpDir, ca, ctx)
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}
	defer p.cache.Destroy()

	// Seed cache with a resource: https://121.29.88.190:39092/static/js/chunk.js
	keyStr := "https|GET|121.29.88.190:39092|/static/js/chunk.js|"
	key := cache.FromString(keyStr)
	entry, err := p.cache.Cache(key, bytes.NewReader([]byte("js payload")), time.Now().Add(1*time.Hour), cachedRequestInfo{})
	if err != nil {
		t.Fatalf("failed to cache test entry: %v", err)
	}
	entry.Data.Close()

	// 1. Purge by specific URL via GET /__purge?url=...
	req := httptest.NewRequest(http.MethodGet, "/__purge?url=https://121.29.88.190:39092/static/js/chunk.js", nil)
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var purgeResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &purgeResp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if purgeResp["status"] != "ok" || purgeResp["purged"] != true {
		t.Fatalf("unexpected purge response: %v", purgeResp)
	}

	// Verify it's no longer in cache
	_, err = p.cache.Get(key)
	if err != cache.ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}

	// 2. Querying purge again should yield 404 not_found
	rec2 := httptest.NewRecorder()
	p.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found on already purged URL, got %d", rec2.Code)
	}

	// 3. Seed another entry and test PurgeAll via /__purge?all=1
	key2Str := "https|GET|121.29.88.190:39092|/static/css/style.css|"
	key2 := cache.FromString(key2Str)
	entry2, err := p.cache.Cache(key2, bytes.NewReader([]byte("css payload")), time.Now().Add(1*time.Hour), cachedRequestInfo{})
	if err != nil {
		t.Fatalf("failed to cache test entry: %v", err)
	}
	entry2.Data.Close()

	reqAll := httptest.NewRequest(http.MethodGet, "/__purge?all=1", nil)
	recAll := httptest.NewRecorder()
	p.ServeHTTP(recAll, reqAll)

	if recAll.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for purge all, got %d", recAll.Code)
	}

	var allResp map[string]any
	if err := json.Unmarshal(recAll.Body.Bytes(), &allResp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if allResp["status"] != "ok" || allResp["action"] != "purge_all" {
		t.Fatalf("unexpected purge all response: %v", allResp)
	}

	// 4. Test PURGE HTTP method
	entry3, err := p.cache.Cache(key, bytes.NewReader([]byte("js payload 2")), time.Now().Add(1*time.Hour), cachedRequestInfo{})
	if err != nil {
		t.Fatalf("failed to cache test entry: %v", err)
	}
	entry3.Data.Close()

	reqPurgeMethod := httptest.NewRequest("PURGE", "http://121.29.88.190:39092/static/js/chunk.js", nil)
	recPurgeMethod := httptest.NewRecorder()
	p.ServeHTTP(recPurgeMethod, reqPurgeMethod)

	if recPurgeMethod.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HTTP PURGE method, got %d", recPurgeMethod.Code)
	}
}
