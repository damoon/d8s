package main

import "testing"

func Test_extractAddress(t *testing.T) {
	tests := []struct {
		name string
		ln   string
		want string
	}{
		{
			name: "empty",
			ln:   "",
			want: "",
		},
		{
			name: "random text",
			ln:   "random text",
			want: "",
		},
		{
			name: "ipv4",
			ln:   "Forwarding from 127.0.0.1:37175 -> 9000",
			want: "127.0.0.1:37175",
		},
		{
			name: "ipv6",
			ln:   "Forwarding from [::1]:37175 -> 9000",
			want: "[::1]:37175",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractAddress(tt.ln); got != tt.want {
				t.Errorf("extractAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}
