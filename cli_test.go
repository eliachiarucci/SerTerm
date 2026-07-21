package main

import (
	"testing"
	"time"
)

func TestIsUpToDate(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "same version", current: "1.2.3", latest: "v1.2.3", want: true},
		{name: "tag without v prefix", current: "1.2.3", latest: "1.2.3", want: true},
		{name: "older than latest", current: "1.2.2", latest: "v1.2.3", want: false},
		{name: "dev build", current: "dev", latest: "v1.2.3", want: false},
		{name: "empty tag", current: "1.2.3", latest: "", want: false},
		{name: "bare v tag", current: "1.2.3", latest: "v", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUpToDate(tt.current, tt.latest); got != tt.want {
				t.Fatalf("isUpToDate(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
			}
		})
	}
}

func TestParseOpenArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		device   string
		duration time.Duration
		wantErr  bool
	}{
		{name: "device only", args: []string{"/dev/cu.usb"}, device: "/dev/cu.usb"},
		{name: "device and seconds", args: []string{"/dev/cu.usb", "5"}, device: "/dev/cu.usb", duration: 5 * time.Second},
		{name: "fractional seconds", args: []string{"/dev/cu.usb", "0.5"}, device: "/dev/cu.usb", duration: 500 * time.Millisecond},
		{name: "no device", args: nil, wantErr: true},
		{name: "negative seconds", args: []string{"/dev/cu.usb", "-3"}, wantErr: true},
		{name: "zero seconds", args: []string{"/dev/cu.usb", "0"}, wantErr: true},
		{name: "non-numeric seconds", args: []string{"/dev/cu.usb", "soon"}, wantErr: true},
		{name: "max seconds", args: []string{"/dev/cu.usb", "60"}, device: "/dev/cu.usb", duration: 60 * time.Second},
		{name: "over max seconds", args: []string{"/dev/cu.usb", "61"}, wantErr: true},
		{name: "too many args", args: []string{"/dev/cu.usb", "5", "extra"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			device, duration, err := parseOpenArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseOpenArgs(%v): expected error, got none", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOpenArgs(%v): unexpected error: %v", tt.args, err)
			}
			if device != tt.device || duration != tt.duration {
				t.Fatalf("parseOpenArgs(%v) = (%q, %v), want (%q, %v)",
					tt.args, device, duration, tt.device, tt.duration)
			}
		})
	}
}
