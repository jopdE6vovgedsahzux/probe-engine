package dialer_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/ooni/probe-engine/legacy/netx/handlers"
	"github.com/ooni/probe-engine/legacy/netx/modelx"
	"github.com/ooni/probe-engine/netx/dialer"
	"github.com/ooni/probe-engine/netx/errorx"
)

func TestDNSDialerNoPort(t *testing.T) {
	dialer := dialer.DNSDialer{Dialer: new(net.Dialer), Resolver: new(net.Resolver)}
	conn, err := dialer.DialContext(context.Background(), "tcp", "antani.ooni.nu")
	if err == nil {
		t.Fatal("expected an error here")
	}
	if conn != nil {
		t.Fatal("expected a nil conn here")
	}
}

func TestDNSDialerLookupHostAddress(t *testing.T) {
	dialer := dialer.DNSDialer{Dialer: new(net.Dialer), Resolver: MockableResolver{
		Err: errors.New("mocked error"),
	}}
	addrs, err := dialer.LookupHost(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 || addrs[0] != "1.1.1.1" {
		t.Fatal("not the result we expected")
	}
}

func TestDNSDialerLookupHostFailure(t *testing.T) {
	expected := errors.New("mocked error")
	dialer := dialer.DNSDialer{Dialer: new(net.Dialer), Resolver: MockableResolver{
		Err: expected,
	}}
	conn, err := dialer.DialContext(context.Background(), "tcp", "dns.google.com:853")
	if !errors.Is(err, expected) {
		t.Fatal("not the error we expected")
	}
	if conn != nil {
		t.Fatal("expected nil conn")
	}
}

type MockableResolver struct {
	Addresses []string
	Err       error
}

func (r MockableResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return r.Addresses, r.Err
}

func TestDNSDialerDialForSingleIPFails(t *testing.T) {
	dialer := dialer.DNSDialer{Dialer: dialer.EOFDialer{}, Resolver: new(net.Resolver)}
	conn, err := dialer.DialContext(context.Background(), "tcp", "1.1.1.1:853")
	if !errors.Is(err, io.EOF) {
		t.Fatal("not the error we expected")
	}
	if conn != nil {
		t.Fatal("expected nil conn")
	}
}

func TestDNSDialerDialForManyIPFails(t *testing.T) {
	dialer := dialer.DNSDialer{Dialer: dialer.EOFDialer{}, Resolver: MockableResolver{
		Addresses: []string{"1.1.1.1", "8.8.8.8"},
	}}
	conn, err := dialer.DialContext(context.Background(), "tcp", "dot.dns:853")
	if !errors.Is(err, io.EOF) {
		t.Fatal("not the error we expected")
	}
	if conn != nil {
		t.Fatal("expected nil conn")
	}
}

func TestDNSDialerDialForManyIPSuccess(t *testing.T) {
	dialer := dialer.DNSDialer{Dialer: dialer.EOFConnDialer{}, Resolver: MockableResolver{
		Addresses: []string{"1.1.1.1", "8.8.8.8"},
	}}
	conn, err := dialer.DialContext(context.Background(), "tcp", "dot.dns:853")
	if err != nil {
		t.Fatal("expected nil error here")
	}
	if conn == nil {
		t.Fatal("expected non-nil conn")
	}
	conn.Close()
}

func TestDNSDialerDialSetsDialID(t *testing.T) {
	saver := &handlers.SavingHandler{}
	ctx := modelx.WithMeasurementRoot(context.Background(), &modelx.MeasurementRoot{
		Beginning: time.Now(),
		Handler:   saver,
	})
	dialer := dialer.DNSDialer{Dialer: dialer.EmitterDialer{
		Dialer: dialer.EOFConnDialer{},
	}, Resolver: MockableResolver{
		Addresses: []string{"1.1.1.1", "8.8.8.8"},
	}}
	conn, err := dialer.DialContext(ctx, "tcp", "dot.dns:853")
	if err != nil {
		t.Fatal("expected nil error here")
	}
	if conn == nil {
		t.Fatal("expected non-nil conn")
	}
	conn.Close()
	events := saver.Read()
	if len(events) != 2 {
		t.Fatal("unexpected number of events")
	}
	for _, ev := range events {
		if ev.Connect != nil && ev.Connect.DialID == 0 {
			t.Fatal("unexpected DialID")
		}
	}
}

func TestReduceErrors(t *testing.T) {
	t.Run("no errors", func(t *testing.T) {
		result := dialer.ReduceErrors(nil)
		if result != nil {
			t.Fatal("wrong result")
		}
	})

	t.Run("single error", func(t *testing.T) {
		err := errors.New("mocked error")
		result := dialer.ReduceErrors([]error{err})
		if result != err {
			t.Fatal("wrong result")
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		err1 := errors.New("mocked error #1")
		err2 := errors.New("mocked error #2")
		result := dialer.ReduceErrors([]error{err1, err2})
		if result.Error() != "mocked error #1" {
			t.Fatal("wrong result")
		}
	})

	t.Run("multiple errors with meaningful ones", func(t *testing.T) {
		err1 := errors.New("mocked error #1")
		err2 := &errorx.ErrWrapper{
			Failure: "unknown_failure: antani",
		}
		err3 := &errorx.ErrWrapper{
			Failure: errorx.FailureConnectionRefused,
		}
		err4 := errors.New("mocked error #3")
		result := dialer.ReduceErrors([]error{err1, err2, err3, err4})
		if result.Error() != errorx.FailureConnectionRefused {
			t.Fatal("wrong result")
		}
	})
}
