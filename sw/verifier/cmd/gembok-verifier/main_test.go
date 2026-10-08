package main

import (
	"testing"
	"time"
)

func TestParseFlags(t *testing.T) {
	opts, err := parseFlags(nil)
	if err != nil {
		t.Fatal(err)
	}
	if opts.backend != "sim" || opts.base != 0xFF240000 || opts.data != "data" || opts.listen != "127.0.0.1:8080" || opts.k != 3 || opts.thresh != 64 || opts.attempts != 5 {
		t.Fatalf("nilai bawaan %+v", opts)
	}
	opts, err = parseFlags([]string{"--backend", "mmio", "--base", "0xFF210000", "--data", "/var/lib/gembok", "--listen", "0.0.0.0:80", "--k", "4", "--ambang", "65535", "--coba-nyala", "20"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.backend != "mmio" || opts.base != 0xFF210000 || opts.data != "/var/lib/gembok" || opts.listen != "0.0.0.0:80" || opts.k != 4 || opts.thresh != 65535 || opts.attempts != 20 {
		t.Fatalf("nilai pilihan %+v", opts)
	}
	if opts, err := parseFlags([]string{"--ambang", "1", "--coba-nyala", "1"}); err != nil || opts.thresh != 1 || opts.attempts != 1 {
		t.Fatalf("batas bawah %+v (%v)", opts, err)
	}
	bad := [][]string{
		{"--backend", "fpga"},
		{"--k", "2"},
		{"--k", "5"},
		{"--base", "dasar"},
		{"--data", ""},
		{"--ambang", "0"},
		{"--ambang", "65536"},
		{"--ambang", "-3"},
		{"--coba-nyala", "0"},
		{"--coba-nyala", "21"},
		{"lebih"},
		{"--tidak-ada"},
	}
	for _, args := range bad {
		if _, err := parseFlags(args); err == nil {
			t.Fatalf("%v diterima", args)
		}
	}
}

func TestRequestTimeout(t *testing.T) {
	cases := []struct {
		puf      time.Duration
		attempts int
		want     time.Duration
	}{
		{5 * time.Second, 1, 20 * time.Second},
		{5 * time.Second, 5, 40 * time.Second},
		{41_289_634_560 * time.Nanosecond, 5, 221_448_172_800 * time.Nanosecond},
		{41_289_634_560 * time.Nanosecond, 1, 56_289_634_560 * time.Nanosecond},
	}
	for _, c := range cases {
		if got := requestTimeout(c.puf, c.attempts); got != c.want {
			t.Fatalf("PUF %v x %d: %v, harap %v", c.puf, c.attempts, got, c.want)
		}
	}
}

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8080": true,
		"localhost:8080": true,
		"[::1]:8080":     true,
		"127.1.2.3:80":   true,
		"0.0.0.0:8080":   false,
		":8080":          false,
		"192.168.1.9:80": false,
		"tanpa-port":     false,
	}
	for in, want := range cases {
		if got := isLoopback(in); got != want {
			t.Fatalf("%s: %v", in, got)
		}
	}
}
