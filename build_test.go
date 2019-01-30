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
)

var torVerbose bool

func TestMain(m *testing.M) {
	flag.BoolVar(&torVerbose, "tor.verbose", false, "Show verbose test info")
	flag.Parse()
	os.Exit(m.Run())
}

func TestDialierSimpleHTTP(t *testing.T) {
	// Give the whole thing a minute
	ctx, cancelFn := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelFn()
	// Start embedded Tor
	conf := &tor.StartConf{ProcessCreator: embedded.NewCreator()}
	if torVerbose {
		conf.DebugWriter = os.Stdout
		conf.NoHush = true
	} else {
		conf.ExtraArgs = append(conf.ExtraArgs, "--quiet")
	}
	tr, err := tor.Start(ctx, conf)
	fatalIfErr(t, err)
	defer tr.Close()
	// Create HTTP client
	dialer, err := tr.Dialer(ctx, nil)
	fatalIfErr(t, err)
	client := &http.Client{Transport: &http.Transport{DialContext: dialer.DialContext}}
	// Make simple Get call to check if we're on Tor
	req, err := http.NewRequest("GET", "https://check.torproject.org/api/ip", nil)
	fatalIfErr(t, err)
	resp, err := client.Do(req.WithContext(ctx))
	fatalIfErr(t, err)
	respBytes, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	fatalIfErr(t, err)
	// Parse out the JSON and confirm the response
	jsn := map[string]interface{}{}
	fatalIfErr(t, json.Unmarshal(respBytes, &jsn))
	if !jsn["IsTor"].(bool) {
		t.Fatal("IsTor returned false")
	}
}

func fatalIfErr(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
