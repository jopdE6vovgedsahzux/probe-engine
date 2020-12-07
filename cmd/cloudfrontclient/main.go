// This package contains a simpler cloudfront client.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/apex/log"
	"github.com/ooni/probe-engine/netx"
)

const defaultBody = `{"net-tests": [{"test-helpers": ["tcp-echo"], "version": "0.2", "name": "http_invalid_request_line", "input-hashes": []}]}`

func fatalOnError(err error, msg string) {
	if err != nil {
		log.WithError(err).Fatal(msg)
	}
}

const usage = `

Usage: cloudfrontclient [-v] -url <cloudfront-url> -host-header <host-header>

For example:

1. ./cloudfrontclient -v -url https://d3kr4emv7f56qa.cloudfront.net/bouncer/net-tests -host-header d3kr4emv7f56qa.cloudfront.net

2. ./cloudfrontclient -v -url https://a0.awsstatic.com/bouncer/net-tests -host-header d3kr4emv7f56qa.cloudfront.net
`

func main() {
	URL := flag.String("url", "", "Cloudfront URL")
	hostHeader := flag.String("host-header", "", "Host header")
	verbose := flag.Bool("v", false, "Be verbose")
	flag.Parse()
	if *URL == "" || *hostHeader == "" {
		log.Fatal(usage)
	}
	logmap := map[bool]log.Level{
		true:  log.DebugLevel,
		false: log.InfoLevel,
	}
	log.SetLevel(logmap[*verbose])
	txp := netx.NewHTTPTransport(netx.Config{
		Logger: log.Log,
	})
	client := &http.Client{Transport: txp}
	body := ioutil.NopCloser(strings.NewReader(defaultBody))
	req, err := http.NewRequest("POST", *URL, body)
	fatalOnError(err, "http.NewRequest failed")
	resp, err := client.Do(req)
	fatalOnError(err, "client.Do failed")
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	fatalOnError(err, "ioutil.ReadAll failed")
	fmt.Printf("%s\n", string(data))
}
