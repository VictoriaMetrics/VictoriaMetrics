package netutil

import (
	"reflect"
	"testing"
)

func TestCipherSuitesFromNames(t *testing.T) {
	type args struct {
		definedCipherSuites []string
	}
	tests := []struct {
		name    string
		args    args
		want    []uint16
		wantErr bool
	}{
		{
			name: "empty cipher suites",
			args: args{definedCipherSuites: []string{}},
			want: nil,
		},
		{
			name:    "got wrong string",
			args:    args{definedCipherSuites: []string{"word"}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "got wrong number",
			args:    args{definedCipherSuites: []string{"123"}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "got correct string cipher suite",
			args:    args{definedCipherSuites: []string{"TLS_RSA_WITH_AES_128_CBC_SHA", "TLS_RSA_WITH_AES_256_CBC_SHA"}},
			want:    []uint16{0x2f, 0x35},
			wantErr: false,
		},
		{
			name:    "got correct string with different cases (upper and lower) cipher suite",
			args:    args{definedCipherSuites: []string{"tls_rsa_with_aes_128_cbc_sha", "TLS_RSA_WITH_AES_256_CBC_SHA"}},
			want:    []uint16{0x2f, 0x35},
			wantErr: false,
		},
		{
			name:    "got correct number cipher suite",
			args:    args{definedCipherSuites: []string{"0x2f", "0x35"}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "got insecure number cipher suite",
			args:    args{definedCipherSuites: []string{"0x0005", "0x000a"}},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "got insecure string cipher suite",
			args:    args{definedCipherSuites: []string{"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA", "TLS_ECDHE_RSA_WITH_RC4_128_SHA"}},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cipherSuitesFromNames(tt.args.definedCipherSuites)
			if (err != nil) != tt.wantErr {
				t.Errorf("cipherSuitesFromNames() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("validateCipherSuites() got = %v, want %v", got, tt.want)
			}
		})
	}
}
