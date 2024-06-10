package graphite

import (
	// embed functions.json file
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
)

// FunctionsHandler implements /functions handler.
//
// See https://graphite.readthedocs.io/en/latest/functions.html#function-api
func FunctionsHandler(w http.ResponseWriter, r *http.Request) error {
	grouped := httputils.GetBool(r, "grouped")
	group := r.FormValue("group")
	result := make(map[string]interface{})
	for funcName, fi := range funcs {
		if group != "" && fi.Group != group {
			continue
		}
		if grouped {
			v := result[fi.Group]
			if v == nil {
				v = make(map[string]*funcInfo)
				result[fi.Group] = v
			}
			m := v.(map[string]*funcInfo)
			m[funcName] = fi
		} else {
			result[funcName] = fi
		}
	}
	return writeJSON(result, w, r)
}

// FunctionDetailsHandler implements /functions/<func_name> handler.
//
// See https://graphite.readthedocs.io/en/latest/functions.html#function-api
func FunctionDetailsHandler(funcName string, w http.ResponseWriter, r *http.Request) error {
	result := funcs[funcName]
	if result == nil {
		return fmt.Errorf("cannot find function %q", funcName)
	}
	return writeJSON(result, w, r)
}

func writeJSON(result interface{}, w http.ResponseWriter, r *http.Request) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("cannot marshal response to JSON: %w", err)
	}
	jsonp := r.FormValue("jsonp")
	contentType := getContentType(jsonp)
	w.Header().Set("Content-Type", contentType)
	if jsonp != "" {
		fmt.Fprintf(w, "%s(", jsonp)
	}
	w.Write(data)
	if jsonp != "" {
		fmt.Fprintf(w, ")")
	}
	return nil
}

//go:embed functions.json
var funcsJSON []byte

type funcInfo struct {
	Name        string          `json:"name"`
	Function    string          `json:"function"`
	Description string          `json:"description"`
	Module      string          `json:"module"`
	Group       string          `json:"group"`
	Params      json.RawMessage `json:"params"`
}

var funcs = func() map[string]*funcInfo {
	var m map[string]*funcInfo
	if err := json.Unmarshal(funcsJSON, &m); err != nil {
		// Do not use logger.Panicf, since it isn't ready yet.
		panic(fmt.Errorf("cannot parse funcsJSON: %w", err))
	}
	return m
}()
