package geolocate_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"

	"github.com/apex/log"
	"github.com/ooni/probe-engine/geolocate"
	"github.com/ooni/probe-engine/model"
)

func TestIPLookupGood(t *testing.T) {
	ip, err := (&geolocate.IPLookupClient{
		HTTPClient: http.DefaultClient,
		Logger:     log.Log,
		UserAgent:  "ooniprobe-engine/0.1.0",
	}).Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if net.ParseIP(ip) == nil {
		t.Fatal("not an IP address")
	}
}

func TestIPLookupAllFailed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel to cause Do() to fail
	ip, err := (&geolocate.IPLookupClient{
		HTTPClient: http.DefaultClient,
		Logger:     log.Log,
		UserAgent:  "ooniprobe-engine/0.1.0",
	}).Do(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("expected an error here")
	}
	if ip != model.DefaultProbeIP {
		t.Fatal("expected the default IP here")
	}
}

func TestIPLookupInvalidIP(t *testing.T) {
	ctx := context.Background()
	ip, err := (&geolocate.IPLookupClient{
		HTTPClient: http.DefaultClient,
		Logger:     log.Log,
		UserAgent:  "ooniprobe-engine/0.1.0",
	}).DoWithCustomFunc(ctx, geolocate.InvalidIPLookup)
	if !errors.Is(err, geolocate.ErrInvalidIPAddress) {
		t.Fatal("expected an error here")
	}
	if ip != model.DefaultProbeIP {
		t.Fatal("expected the default IP here")
	}
}
