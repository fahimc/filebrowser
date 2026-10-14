// Photos is a small local frontend and same-origin API proxy. FileBrowser owns
// authentication, files, quotas and metadata; this process owns no photo store.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

//go:embed web/*
var assets embed.FS
var version = "development"

func main() {
	exe, _ := os.Executable()
	logFile, logErr := os.OpenFile(filepath.Join(filepath.Dir(exe), "Photos.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if logErr == nil {
		defer logFile.Close()
		log.SetOutput(logFile)
	}
	defaults := struct {
		Server string `json:"server"`
		Listen string `json:"listen"`
	}{"http://127.0.0.1:8080", "127.0.0.1:8860"}
	if data, e := os.ReadFile(filepath.Join(filepath.Dir(exe), "Photos.json")); e == nil {
		if e = json.Unmarshal([]byte(strings.TrimPrefix(string(data), "\ufeff")), &defaults); e != nil {
			log.Fatal("Invalid Photos.json: ", e)
		}
	}
	serverURL := flag.String("server", defaults.Server, "Existing FileBrowser URL, including its base path")
	listen := flag.String("listen", defaults.Listen, "Local Photos address (loopback only)")
	noBrowser := flag.Bool("no-browser", false, "Do not open a browser on launch")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("Photos", version)
		return
	}
	target, err := url.Parse(*serverURL)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		log.Fatal("Use an http(s) FileBrowser URL without credentials or query parameters")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || (host != "localhost" && (net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback())) {
		log.Fatal("Photos must listen on a loopback address")
	}
	root, err := fs.Sub(assets, "web")
	if err != nil {
		log.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		r.Host = target.Host
		for _, header := range []string{"X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-Port", "Forwarded"} {
			r.Header.Del(header)
		}
		if r.Header.Get("Origin") != "" {
			r.Header.Set("Origin", target.Scheme+"://"+target.Host)
		}
	}
	proxy.ModifyResponse = func(res *http.Response) error {
		cookies := res.Cookies()
		res.Header.Del("Set-Cookie")
		for _, cookie := range cookies {
			cookie.Domain = ""
			cookie.Path = "/"
			cookie.Secure = false
			res.Header.Add("Set-Cookie", cookie.String())
		}
		if location := res.Header.Get("Location"); location != "" {
			u, e := url.Parse(location)
			if e == nil && (u.Host == "" || u.Host == target.Host) {
				u.Scheme = ""
				u.Host = ""
				u.Path = "/" + strings.TrimPrefix(strings.TrimPrefix(u.Path, strings.TrimRight(target.Path, "/")), "/")
				res.Header.Set("Location", u.String())
			}
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "FileBrowser is unavailable. Start it or check the --server address.", 502)
	}
	static := http.StripPrefix("/photos-static/", http.FileServer(http.FS(root)))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != *listen && r.Host != "localhost:"+strings.Split(*listen, ":")[len(strings.Split(*listen, ":"))-1] {
			http.Error(w, "Invalid host", 400)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
				http.Error(w, "Cross-origin requests are not allowed", 403)
				return
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		switch {
		case r.URL.Path == "/photos-companion.json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"version": version, "filebrowserURL": target.String()})
		case r.URL.Path == "/" || r.URL.Path == "/photos" || r.URL.Path == "/photos/":
			body, e := fs.ReadFile(root, "index.html")
			if e != nil {
				http.Error(w, "Photos assets missing", 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Write(body)
		case strings.HasPrefix(r.URL.Path, "/photos-static/"):
			static.ServeHTTP(w, r)
		default:
			proxy.ServeHTTP(w, r)
		}
	})
	srv := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 15 * time.Second, IdleTimeout: 90 * time.Second}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Photos %s listening at http://%s", version, *listen)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdown)
	}()
	if !*noBrowser {
		openBrowser("http://" + *listen + "/photos")
	}
	if err = srv.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
