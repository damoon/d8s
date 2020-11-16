package wedding

import (
	"encoding/base64"
	"reflect"
	"testing"
)

func Test_xRegistryAuth_toDockerConfig(t *testing.T) {
	tests := []struct {
		name    string
		x       xRegistryAuth
		want    dockerConfig
		wantErr bool
	}{
		{
			name: "default",
			x: xRegistryAuth(base64.StdEncoding.
				EncodeToString([]byte(`{"username":"user", "password":"pass123", "serveraddress":"reg.domain.tld"}`))),
			want: dockerConfig{
				Auths: map[string]dockerAuth{
					"reg.domain.tld": {
						Auth: base64.StdEncoding.EncodeToString([]byte("user:pass123")),
					},
				},
			},
		},
		{
			name: "null",
			x:    xRegistryAuth(base64.StdEncoding.EncodeToString([]byte(`null`))),
			want: dockerConfig{
				Auths: map[string]dockerAuth{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.x.toDockerConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("xRegistryAuth.toDockerConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("xRegistryAuth.toDockerConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_dockerConfig_mustToJSON(t *testing.T) {
	tests := []struct {
		name  string
		auths map[string]dockerAuth
		want  []byte
	}{
		{
			name:  "empty",
			auths: map[string]dockerAuth{},
			want:  []byte(`{"auths":{}}`),
		},
		{
			name: "logged in",
			auths: map[string]dockerAuth{"reg": {
				Auth: base64.StdEncoding.EncodeToString([]byte("user:pass123")),
			}},
			want: []byte(`{"auths":{"reg":{"auth":"dXNlcjpwYXNzMTIz"}}}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dockerConfig{
				Auths: tt.auths,
			}
			if got := d.mustToJSON(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("dockerConfig.mustToJSON() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}
