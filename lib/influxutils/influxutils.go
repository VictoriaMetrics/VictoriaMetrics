package influxutils

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

var influxDatabaseNames = flagutil.NewArrayString("influx.databaseNames", "Comma-separated list of database names to return from /query and /influx/query API. "+
	"This can be needed for accepting data from Telegraf plugins such as https://github.com/fangli/fluent-plugin-influxdb")

// WriteDatabaseNames writes influxDatabaseNames to w.
func WriteDatabaseNames(w http.ResponseWriter) {
	// Emulate fake response for influx query.
	// This is required for TSBS benchmark and some Telegraf plugins.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1124
	w.Header().Set("Content-Type", "application/json")
	dbNames := *influxDatabaseNames
	if len(dbNames) == 0 {
		dbNames = []string{"_internal"}
	}
	dbs := make([]string, len(dbNames))
	for i := range dbNames {
		dbs[i] = fmt.Sprintf(`[%q]`, dbNames[i])
	}
	fmt.Fprintf(w, `{"results":[{"statement_id":0,"series":[{"name":"databases","columns":["name"],"values":[%s]}]}]}`, strings.Join(dbs, ","))
}
