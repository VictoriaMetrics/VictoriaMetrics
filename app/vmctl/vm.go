package main

import (
	"flag"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

var (
	vmAddr = flag.String("vm-addr", "http://localhost:8428", "VictoriaMetrics address to perform import requests. \n"+
		"Should be the same as --httpListenAddr value for single-node version or vminsert component. \n"+
		"When importing into the clustered version do not forget to set additionally --vm-account-id flag. \n"+
		"Please note, that `vmctl` performs initial readiness check for the given address by checking `/health` endpoint.")
	vmUser      = flag.String("vm-user", "", "VictoriaMetrics username for basic auth")
	vmPassword  = flag.String("vm-password", "", "VictoriaMetrics password for basic auth")
	vmAccountID = flag.String("vm-account-id", "", "AccountID is an arbitrary 32-bit integer identifying namespace for data ingestion (aka tenant). \n"+
		"AccountID is required when importing into the clustered version of VictoriaMetrics. \n"+
		"It is possible to set it as accountID:projectID, where projectID is also arbitrary 32-bit integer. \n"+
		"If projectID isn't set, then it equals to 0")
	vmConcurrency        = flag.Uint("vm-concurrency", 2, "Number of workers concurrently performing import requests to VM")
	vmCompress           = flag.Bool("vm-compress", true, "Whether to apply gzip compression to import requests")
	vmBatchSize          = flag.Int("vm-batch-size", 200_000, "How many samples importer collects before sending the import request to VM")
	vmSignificantFigures = flag.Int("vm-significant-figures", 0, "The number of significant figures to leave in metric values before importing. "+
		"See https://en.wikipedia.org/wiki/Significant_figures. Zero value saves all the significant figures. "+
		"This option may be used for increasing on-disk compression level for the stored metrics. "+
		"See also --vm-round-digits option")
	vmRoundDigits = flag.Int("vm-round-digits", 100, "Round metric values to the given number of decimal digits after the point. "+
		"This option may be used for increasing on-disk compression level for the stored metrics")
	vmDisableProgressBar = flag.Bool("vm-disable-progress-bar", false, "Whether to disable progress bar per each worker during the import.")

	vmExtraLabel = flagutil.NewArrayString("vm-extra-label", "Extra labels, that will be added to imported timeseries. In case of collision, label value defined by flag"+
		"will have priority. Flag can be set multiple times, to add few additional labels.")
	vmRateLimit = flag.Int64("vm-rate-limit", 0, "Optional data transfer rate limit in bytes per second.\n"+
		"By default the rate limit is disabled. It can be useful for limiting load on configured via '--vmAddr' destination.")

	vmInterCluster = flag.Bool("vm-intercluster", false, "Enables cluster-to-cluster migration mode with automatic tenants data migration.\n"+
		" In this mode --vm-native-src-addr flag format is: 'http://vmselect:8481/'. --vm-native-dst-addr flag format is: http://vminsert:8480/. \n"+
		" TenantID will be appended automatically after discovering tenants from src.")
)

func initConfigVM() vm.Config {
	return vm.Config{
		Addr:               *vmAddr,
		User:               *vmUser,
		Password:           *vmPassword,
		Concurrency:        uint8(*vmConcurrency),
		Compress:           *vmCompress,
		AccountID:          *vmAccountID,
		BatchSize:          *vmBatchSize,
		SignificantFigures: *vmSignificantFigures,
		RoundDigits:        *vmRoundDigits,
		ExtraLabels:        *vmExtraLabel,
		RateLimit:          *vmRateLimit,
		DisableProgressBar: *vmDisableProgressBar,
	}
}
