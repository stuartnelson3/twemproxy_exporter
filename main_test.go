package main

import (
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnmarshalSimple(t *testing.T) {
	l, err := net.Listen("tcp", "0.0.0.0:")
	if err != nil {
		t.Fatalf("failed to create tcp listener: %v", err)
	}
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()

		f, err := os.Open("testdata/simple.json")
		require.NoError(t, err)
		defer f.Close()

		_, err = io.Copy(conn, f)
		require.NoError(t, err)
	}()

	e := newExporter(l.Addr().String(), time.Second)

	st, err := e.collect()
	require.NoError(t, err)

	require.Equal(t, 5.0, st.CurrConnections)
	require.Equal(t, "nutcracker", st.Service)
	proxied, ok := st.Pools["proxied"]
	require.True(t, ok)
	require.Equal(t, 65.0, proxied.ClientEOF)
	require.Equal(t, 3, len(proxied.Servers))
	require.Equal(t, 1495.0, proxied.Servers["memcached-1"].RequestBytes)
}

func TestUnmarshalFull(t *testing.T) {
	l, err := net.Listen("tcp", "0.0.0.0:")
	if err != nil {
		t.Fatalf("failed to create tcp listener: %v", err)
	}
	defer l.Close()

	go func() {
		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()

		f, err := os.Open("testdata/full.json")
		require.NoError(t, err)
		defer f.Close()

		_, err = io.Copy(conn, f)
		require.NoError(t, err)
	}()

	e := newExporter(l.Addr().String(), time.Second)

	st, err := e.collect()
	require.NoError(t, err)

	require.Equal(t, 5, len(st.Pools))
	beta, ok := st.Pools["beta"]
	require.True(t, ok)
	require.Equal(t, 33.0, beta.Servers["server1"].Requests)
}
