package datasource

// Router dispatches query building to the appropriate client
// based on the DataSourceType parameter.
type Router struct {
	defaultClient *Client
	sqlClient     *Client
}

// BuildWithParams dispatches to the SQL client for "sql" type,
// otherwise to the default client.
func (r *Router) BuildWithParams(params QuerierParams) Querier {
	if params.DataSourceType == string(datasourceSQL) {
		return r.sqlClient.BuildWithParams(params)
	}
	return r.defaultClient.BuildWithParams(params)
}
