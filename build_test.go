package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/cretz/bine/process/embedded"
	"github.com/cretz/bine/tor"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context/ctxhttp"
)

var torExePath string
var torVerbose bool
var globalEnabledNetworkContext *TestContext

type TestContext struct {
	context.Context
	*testing.T
	*tor.Tor
	Require         *require.Assertions
	CloseTorOnClose bool
}

func TestMain(m *testing.M) {
	flag.StringVar(&torExePath, "tor.path", "tor", "The Tor exe path")
	flag.BoolVar(&torVerbose, "tor.verbose", false, "Show verbose test info")
	flag.Parse()
	exitCode := m.Run()
	if globalEnabledNetworkContext != nil {
		globalEnabledNetworkContext.CloseTorOnClose = true
		globalEnabledNetworkContext.Close()
	}
	os.Exit(exitCode)
}

func TestDialerSimpleHTTP(t *testing.T) {
	ctx := GlobalEnabledNetworkContext(t)
	httpClient := httpClient(ctx, nil)
	// IsTor check
	byts := httpGet(ctx, httpClient, "https://check.torproject.org/api/ip")
	jsn := map[string]interface{}{}
	ctx.Require.NoError(json.Unmarshal(byts, &jsn))
	ctx.Require.True(jsn["IsTor"].(bool))
}

func httpClient(ctx *TestContext, conf *tor.DialConf) *http.Client {
	// 15 seconds max to dial
	dialCtx, dialCancel := context.WithTimeout(ctx, 15*time.Second)
	defer dialCancel()
	// Make connection
	dialer, err := ctx.Dialer(dialCtx, conf)
	ctx.Require.NoError(err)
	return &http.Client{Transport: &http.Transport{DialContext: dialer.DialContext}}
}

func httpGet(ctx *TestContext, client *http.Client, url string) []byte {
	// We'll give it 30 seconds to respond
	callCtx, callCancel := context.WithTimeout(ctx, 30*time.Second)
	defer callCancel()
	resp, err := ctxhttp.Get(callCtx, client, url)
	ctx.Require.NoError(err)
	defer resp.Body.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	ctx.Require.NoError(err)
	return respBytes
}

func GlobalEnabledNetworkContext(t *testing.T) *TestContext {
	if globalEnabledNetworkContext == nil {
		ctx := NewTestContext(t, nil)
		ctx.CloseTorOnClose = false
		// 45 second wait for enable network
		enableCtx, enableCancel := context.WithTimeout(ctx, 45*time.Second)
		defer enableCancel()
		ctx.Require.NoError(ctx.EnableNetwork(enableCtx, true))
		globalEnabledNetworkContext = ctx
	} else {
		globalEnabledNetworkContext.T = t
		globalEnabledNetworkContext.Require = require.New(t)
	}
	return globalEnabledNetworkContext
}

func NewTestContext(t *testing.T, conf *tor.StartConf) *TestContext {
	// Build start conf
	if conf == nil {
		conf = &tor.StartConf{ProcessCreator: embedded.NewCreator()}
	}
	conf.ExePath = torExePath
	if torVerbose {
		conf.DebugWriter = os.Stdout
		conf.NoHush = true
	} else {
		conf.ExtraArgs = append(conf.ExtraArgs, "--quiet")
	}
	ret := &TestContext{Context: context.Background(), T: t, Require: require.New(t), CloseTorOnClose: true}
	// Start tor
	var err error
	if ret.Tor, err = tor.Start(ret.Context, conf); err != nil {
		defer ret.Close()
		t.Fatal(err)
	}
	return ret
}

func (t *TestContext) Close() {
	if t.CloseTorOnClose {
		if err := t.Tor.Close(); err != nil {
			if t.Failed() {
				t.Logf("Failure on close: %v", err)
			} else {
				t.Errorf("Failure on close: %v", err)
			}
		}
	}
}
