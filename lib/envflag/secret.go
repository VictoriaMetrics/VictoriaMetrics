package envflag

import "github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"

// secretFlagsList contains names of flags with secret values obtained from
// the `-secret.flags` command-line option.
var secretFlagsList = flagutil.NewArrayString("secret.flags",
	"Comma-separated list of flag names with secret values. Values for these flags are hidden in logs and on /metrics page")

// applySecretFlags registers flags from `-secret.flags` after they are parsed.
//
// The function must be called inside envflag.Parse after parsing flags.
func applySecretFlags() {
	for _, f := range *secretFlagsList {
		flagutil.RegisterSecretFlag(f)
	}
}
