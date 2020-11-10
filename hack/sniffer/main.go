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

	if err := http.ListenAndServe(":23765", nil); err != nil {
		log.Fatalf("listen and serve: %v", err)
	}
}

func sniffer(res http.ResponseWriter, req *http.Request) {
	err := serveReverseProxy("http://127.0.0.1:2376", res, req)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func serveReverseProxy(target string, res http.ResponseWriter, req *http.Request) error {
	_, err := printRequest(os.Stdout, req)
	if err != nil {
		return err
	}

	url, err := url.Parse(target)
	if err != nil {
		// TODO Avoid panicing during runtime, parse url during start up once.
		return fmt.Errorf("parse url %s: %v", target, err)
	}

	req.URL.Host = url.Host
	req.URL.Scheme = url.Scheme
	req.Host = url.Host

	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))

	proxy := httputil.NewSingleHostReverseProxy(url)

	proxy.ModifyResponse = func(resp *http.Response) error {
		return printRespose(os.Stdout, resp)
	}

	proxy.ServeHTTP(res, req)

	return nil
}

func printRequest(w io.Writer, req *http.Request) (*bytes.Reader, error) {
	// TODO To avoid OOM, check body size and use disk to cache.

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %v", err)
	}

	w.Write([]byte(fmt.Sprintf("req: %v\n\n", req)))

	req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
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
