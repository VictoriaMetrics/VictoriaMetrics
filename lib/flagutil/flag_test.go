package flagutil

import (
	"flag"
	"os"
	"testing"
)

func TestSetFlagsFromEnvironment(t *testing.T) {
	tests := []struct {
		name       string
		flagName   string
		flagSetter func(flagName string) interface{}
		envName    string
		envValue   string
		wantErr    bool
	}{
		{
			name:     "strings flag",
			flagName: "string-env",
			flagSetter: func(flagName string) interface{} {
				return flag.String(flagName, "", "")
			},
			envName:  "STRING_ENV",
			envValue: "1",
			wantErr:  false,
		},
		{
			name:     "bool flag",
			flagName: "bool-env",
			flagSetter: func(flagName string) interface{} {
				return flag.Bool(flagName, true, "")
			},
			envName:  "BOOL_ENV",
			envValue: "true",
			wantErr:  false,
		},
		{
			name:     "bool flag wrong",
			flagName: "bool-env-wrong",
			flagSetter: func(flagName string) interface{} {
				return flag.Bool(flagName, true, "")
			},
			envName:  "BOOL_ENV_WRONG",
			envValue: "123",
			wantErr:  true,
		},
		{
			name:     "wrong set value",
			flagName: "wrong-env",
			flagSetter: func(flagName string) interface{} {
				return flag.Int(flagName, 0, "")
			},
			envName:  "WRONG_ENV",
			envValue: "2147483648298412365817364817635871635876",
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.Setenv(tt.envName, tt.envValue)
			if err != nil {
				t.Fatalf("error set environment variable: %s, error: %s", tt.envName, err)
			}
			defer func() { _ = os.Unsetenv(tt.envName) }()

			tt.flagSetter(tt.flagName)

			flag.Parse()

			if err := SetFlagsFromEnvironment(); (err != nil) != tt.wantErr {
				t.Errorf("SetFlagsFromEnvironment() error = %v, wantErr %v", err, tt.wantErr)
			}

			env := os.Getenv(tt.envName)

			flag.Visit(func(f *flag.Flag) {
				if tt.flagName == f.Name && f.Value.String() != env {
					t.Fatalf("Expected flag value %v, got %v for flag: %q", tt.envValue, f.Value.String(), tt.flagName)
				}
			})
		})
	}
}
