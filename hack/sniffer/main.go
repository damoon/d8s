package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
)

func main() {
	http.HandleFunc("/", sniffer)

	log.Println("proxy http traffic from :23765 to http://127.0.0.1:2376")

	if err := http.ListenAndServe(":23765", nil); err != nil {
		log.Fatalf("listen and serve: %v", err)
	}
}

func sniffer(w http.ResponseWriter, r *http.Request) {
	err := serveReverseProxy("http://127.0.0.1:2376", w, r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func serveReverseProxy(target string, w http.ResponseWriter, r *http.Request) error {
	_, err := printRequest(os.Stdout, r)
	if err != nil {
		return err
	}

	url, err := url.Parse(target)
	if err != nil {
		// TODO Avoid panicing during runtime, parse url during start up once.
		return fmt.Errorf("parse url %s: %v", target, err)
	}

	r.URL.Host = url.Host
	r.URL.Scheme = url.Scheme
	r.Host = url.Host

	r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))

	proxy := httputil.NewSingleHostReverseProxy(url)

	proxy.ModifyResponse = func(w *http.Response) error {
		return printRespose(os.Stdout, w)
	}

	proxy.ServeHTTP(w, r)

	return nil
}

func printRequest(w io.Writer, r *http.Request) (*bytes.Reader, error) {
	// TODO To avoid OOM, check body size and use disk to cache.

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %v", err)
	}

	w.Write([]byte(fmt.Sprintf("req: %v\n\n", r)))

	r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	reader := bytes.NewReader(body)

	return reader, nil
}

func printRespose(w io.Writer, resp *http.Response) error {
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = resp.Body.Close()
	if err != nil {
		return err
	}

	w.Write([]byte(fmt.Sprintf("%v\n%v\n", resp, string(b))))

	body := ioutil.NopCloser(bytes.NewReader(b))
	resp.Body = body
	resp.ContentLength = int64(len(b))
	resp.Header.Set("Content-Length", strconv.Itoa(len(b)))
	return nil
}
