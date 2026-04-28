package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

var blocked = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),

	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
}

func main() {
	port := getenv("PORT", "9517")

	client := &http.Client{
		Transport: &http.Transport{
			DialContext:           safeDial,
			ForceAttemptHTTP2:     true,
			ResponseHeaderTimeout: 30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return checkURL(req.URL)
		},
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		rawPath := strings.TrimPrefix(r.URL.EscapedPath(), "/")

		raw, err := url.PathUnescape(rawPath)
		if err != nil {
			http.Error(w, "bad url", http.StatusBadRequest)
			return
		}

		if r.URL.RawQuery != "" {
			raw += "?" + r.URL.RawQuery
		}

		if raw == "" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
			return
		}

		target, err := url.Parse(raw)
		if err != nil {
			http.Error(w, "bad url", http.StatusBadRequest)
			return
		}

		if err := checkURL(target); err != nil {
			http.Error(w, "blocked url: "+err.Error(), http.StatusBadRequest)
			return
		}

		req, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), nil)
		if err != nil {
			http.Error(w, "request failed", http.StatusInternalServerError)
			return
		}

		req.Header.Set("User-Agent", "private-download-proxy/1.0")

		if h := r.Header.Get("Range"); h != "" {
			req.Header.Set("Range", h)
		}

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		copyHeader(w.Header(), resp.Header)

		if w.Header().Get("Content-Disposition") == "" {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename(target, resp.Header)))
		}

		w.WriteHeader(resp.StatusCode)

		if r.Method != http.MethodHead {
			_, _ = io.Copy(w, resp.Body)
		}
	}

	log.Println("listening on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, http.HandlerFunc(handler)))
}

func checkURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("only http/https allowed")
	}

	host := u.Hostname()
	if host == "" {
		return errors.New("empty host")
	}

	if strings.EqualFold(host, "localhost") {
		return errors.New("localhost blocked")
	}

	ips, err := resolve(host)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		if isBlocked(ip) {
			return fmt.Errorf("private ip blocked: %s", ip)
		}
	}

	return nil
}

func safeDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	ips, err := resolve(host)
	if err != nil {
		return nil, err
	}

	d := net.Dialer{
		Timeout: 15 * time.Second,
	}

	for _, ip := range ips {
		if isBlocked(ip) {
			continue
		}

		conn, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
	}

	return nil, errors.New("no safe ip available")
}

func resolve(host string) ([]netip.Addr, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		return []netip.Addr{ip.Unmap()}, nil
	}

	records, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return nil, err
	}

	var ips []netip.Addr

	for _, r := range records {
		ip, ok := netip.AddrFromSlice(r.IP)
		if ok {
			ips = append(ips, ip.Unmap())
		}
	}

	return ips, nil
}

func isBlocked(ip netip.Addr) bool {
	ip = ip.Unmap()

	if !ip.IsValid() {
		return true
	}

	if !ip.IsGlobalUnicast() {
		return true
	}

	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}

	for _, p := range blocked {
		if p.Contains(ip) {
			return true
		}
	}

	return false
}

func copyHeader(dst, src http.Header) {
	keys := []string{
		"Content-Type",
		"Content-Length",
		"Content-Disposition",
		"Accept-Ranges",
		"Content-Range",
		"ETag",
		"Last-Modified",
	}

	for _, k := range keys {
		if v := src.Get(k); v != "" {
			dst.Set(k, v)
		}
	}
}

func filename(u *url.URL, h http.Header) string {
	name := path.Base(u.Path)

	if name == "" || name == "." || name == "/" {
		name = "download"
	}

	name = cleanFilename(name)

	if path.Ext(name) != "" {
		return name
	}

	format := strings.ToLower(u.Query().Get("format"))

	switch format {
	case "jpg", "jpeg":
		return name + ".jpg"
	case "png":
		return name + ".png"
	case "webp":
		return name + ".webp"
	case "gif":
		return name + ".gif"
	case "mp4":
		return name + ".mp4"
	}

	// 根据 Content-Type 自动补后缀
	ct := h.Get("Content-Type")
	ct = strings.Split(ct, ";")[0]
	ct = strings.TrimSpace(strings.ToLower(ct))

	switch ct {
	case "image/jpeg":
		return name + ".jpg"
	case "image/png":
		return name + ".png"
	case "image/webp":
		return name + ".webp"
	case "image/gif":
		return name + ".gif"
	case "video/mp4":
		return name + ".mp4"
	case "application/pdf":
		return name + ".pdf"
	case "application/zip":
		return name + ".zip"
	case "application/x-7z-compressed":
		return name + ".7z"
	case "application/x-rar-compressed":
		return name + ".rar"
	case "application/octet-stream":
		return name
	}

	if exts, err := mime.ExtensionsByType(ct); err == nil && len(exts) > 0 {
		return name + exts[0]
	}

	return name
}

func cleanFilename(name string) string {
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")

	if strings.TrimSpace(name) == "" {
		return "download"
	}

	return name
}

func getenv(k, fallback string) string {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	return v
}
