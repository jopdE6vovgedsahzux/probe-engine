// Package webconnectivity implements OONI's Web Connectivity experiment.
//
// See https://github.com/ooni/spec/blob/master/nettests/ts-017-web-connectivity.md
package webconnectivity

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/ooni/probe-engine/experiment/webconnectivity/internal"
	"github.com/ooni/probe-engine/internal/httpheader"
	"github.com/ooni/probe-engine/model"
	"github.com/ooni/probe-engine/netx/archival"
)

const (
	testName    = "web_connectivity"
	testVersion = "0.2.0"
)

// Config contains the experiment config.
type Config struct{}

// TestKeys contains webconnectivity test keys.
type TestKeys struct {
	Agent          string  `json:"agent"`
	ClientResolver string  `json:"client_resolver"`
	Retries        *int64  `json:"retries"`    // unused
	SOCKSProxy     *string `json:"socksproxy"` // unused

	// DNS experiment
	Queries              []archival.DNSQueryEntry `json:"queries"`
	DNSExperimentFailure *string                  `json:"dns_experiment_failure"`
	DNSAnalysisResult

	// Control experiment
	ControlFailure *string         `json:"control_failure"`
	ControlRequest ControlRequest  `json:"-"`
	Control        ControlResponse `json:"control"`

	// TCP connect experiment
	TCPConnect          []archival.TCPConnectEntry `json:"tcp_connect"`
	TCPConnectSuccesses int                        `json:"-"`
	TCPConnectAttempts  int                        `json:"-"`

	// HTTP experiment
	Requests              []archival.RequestEntry `json:"requests"`
	HTTPExperimentFailure *string                 `json:"http_experiment_failure"`
	HTTPAnalysisResult

	// Top-level analysis
	Summary
}

// Measurer performs the measurement.
type Measurer struct {
	Config Config
}

// NewExperimentMeasurer creates a new ExperimentMeasurer.
func NewExperimentMeasurer(config Config) model.ExperimentMeasurer {
	return Measurer{Config: config}
}

// ExperimentName implements ExperimentMeasurer.ExperExperimentName.
func (m Measurer) ExperimentName() string {
	return testName
}

// ExperimentVersion implements ExperimentMeasurer.ExperExperimentVersion.
func (m Measurer) ExperimentVersion() string {
	return testVersion
}

var (
	// ErrNoAvailableTestHelpers is emitted when there are no available test helpers.
	ErrNoAvailableTestHelpers = errors.New("no available helpers")

	// ErrNoInput indicates that no input was provided
	ErrNoInput = errors.New("no input provided")

	// ErrInputIsNotAnURL indicates that the input is not an URL.
	ErrInputIsNotAnURL = errors.New("input is not an URL")

	// ErrUnsupportedInput indicates that the input URL scheme is unsupported.
	ErrUnsupportedInput = errors.New("unsupported input scheme")
)

// Run implements ExperimentMeasurer.Run.
func (m Measurer) Run(
	ctx context.Context,
	sess model.ExperimentSession,
	measurement *model.Measurement,
	callbacks model.ExperimentCallbacks,
) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	tk := new(TestKeys)
	measurement.TestKeys = tk
	tk.Agent = "redirect"
	tk.ClientResolver = sess.ResolverIP()
	if measurement.Input == "" {
		return ErrNoInput
	}
	URL, err := url.Parse(string(measurement.Input))
	if err != nil {
		return ErrInputIsNotAnURL
	}
	if URL.Scheme != "http" && URL.Scheme != "https" {
		return ErrUnsupportedInput
	}
	// 1. find test helper
	testhelpers, _ := sess.GetTestHelpersByName("web-connectivity")
	var testhelper *model.Service
	for _, th := range testhelpers {
		if th.Type == "https" {
			testhelper = &th
			break
		}
	}
	if testhelper == nil {
		return ErrNoAvailableTestHelpers
	}
	measurement.TestHelpers = map[string]interface{}{
		"backend": testhelper,
	}
	// 2. perform the DNS lookup step
	dnsResult := DNSLookup(ctx, DNSLookupConfig{Session: sess, URL: URL})
	tk.Queries = append(tk.Queries, dnsResult.TestKeys.Queries...)
	tk.DNSExperimentFailure = dnsResult.Failure
	epnts := NewEndpoints(URL, dnsResult.Addresses())
	// 3. perform the control measurement
	tk.Control, err = Control(ctx, sess, testhelper.Address, ControlRequest{
		HTTPRequest: URL.String(),
		HTTPRequestHeaders: map[string][]string{
			"Accept":          {httpheader.Accept()},
			"Accept-Language": {httpheader.AcceptLanguage()},
			"User-Agent":      {httpheader.UserAgent()},
		},
		TCPConnect: epnts.Endpoints(),
	})
	tk.ControlFailure = archival.NewFailure(err)
	// 4. analyze DNS results
	if tk.ControlFailure == nil {
		tk.DNSAnalysisResult = DNSAnalysis(URL, dnsResult, tk.Control)
	}
	sess.Logger().Infof("DNS analysis result: %+v", internal.StringPointerToString(
		tk.DNSAnalysisResult.DNSConsistency))
	// 5. perform TCP/TLS connects
	connectsResult := Connects(ctx, ConnectsConfig{
		Session:       sess,
		TargetURL:     URL,
		URLGetterURLs: epnts.URLs(),
	})
	sess.Logger().Infof(
		"TCP/TLS endpoints: %d/%d reachable", connectsResult.Successes, connectsResult.Total)
	for _, tcpkeys := range connectsResult.AllKeys {
		// rewrite TCPConnect to include blocking information - it is very
		// sad that we're storing analysis result inside the measurement
		tk.TCPConnect = append(tk.TCPConnect, ComputeTCPBlocking(
			tcpkeys.TCPConnect, tk.Control.TCPConnect)...)
	}
	tk.TCPConnectAttempts = connectsResult.Total
	tk.TCPConnectSuccesses = connectsResult.Successes
	// 6. perform HTTP/HTTPS measurement
	httpResult := HTTPGet(ctx, HTTPGetConfig{
		Addresses: dnsResult.Addresses(),
		Session:   sess,
		TargetURL: URL,
	})
	tk.HTTPExperimentFailure = httpResult.Failure
	tk.Requests = append(tk.Requests, httpResult.TestKeys.Requests...)
	// 7. compare HTTP measurement to control
	tk.HTTPAnalysisResult = HTTPAnalysis(httpResult.TestKeys, tk.Control)
	tk.HTTPAnalysisResult.Log(sess.Logger())
	tk.Summary = Summarize(tk)
	tk.Summary.Log(sess.Logger())
	return nil
}

// ComputeTCPBlocking will return a copy of the input TCPConnect structure
// where we set the Blocking value depending on the control results.
func ComputeTCPBlocking(measurement []archival.TCPConnectEntry,
	control map[string]ControlTCPConnectResult) (out []archival.TCPConnectEntry) {
	out = []archival.TCPConnectEntry{}
	for _, me := range measurement {
		epnt := net.JoinHostPort(me.IP, strconv.Itoa(me.Port))
		if ce, ok := control[epnt]; ok {
			v := ce.Failure == nil && me.Status.Failure != nil
			me.Status.Blocked = &v
		}
		out = append(out, me)
	}
	return
}

// SummaryKeys contains summary keys for this experiment.
//
// Note that this structure is part of the ABI contract with probe-cli
// therefore we should be careful when changing it.
type SummaryKeys struct {
	Accessible bool   `json:"accessible"`
	Blocking   string `json:"blocking"`
	IsAnomaly  bool   `json:"-"`
}

// GetSummaryKeys implements model.ExperimentMeasurer.GetSummaryKeys.
func (m Measurer) GetSummaryKeys(measurement *model.Measurement) (interface{}, error) {
	sk := SummaryKeys{IsAnomaly: false}
	tk, ok := measurement.TestKeys.(*TestKeys)
	if !ok {
		return sk, errors.New("invalid test keys type")
	}
	sk.IsAnomaly = tk.BlockingReason != nil
	if tk.BlockingReason != nil {
		sk.Blocking = *tk.BlockingReason
	}
	sk.Accessible = tk.Accessible != nil && *tk.Accessible
	return sk, nil
}
