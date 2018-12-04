package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnmarshal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := os.Open("testdata/example.json")
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to open example.json: %v", err), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if _, err := io.Copy(w, f); err != nil {
			t.Fatalf("failed to write to conn: %v", err)
		}
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	e := newExporter(u, time.Second)

	st, err := e.collect()
	require.NoError(t, err)

	require.Equal(t, 5.0, st.CurrConnections)
	require.Equal(t, "nutcracker", st.Service)
	proxied := st.Proxied
	require.Equal(t, 65.0, proxied.ClientEOF)
	require.Equal(t, 3, len(proxied.Servers))
	require.Equal(t, 1495.0, proxied.Servers["memcached-1"].RequestBytes)
}
