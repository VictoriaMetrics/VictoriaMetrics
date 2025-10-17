package logstorage

import (
	"fmt"
)

func parsePipeTotalStats(lex *lexer) (pipe, error) {
	if !lex.isKeyword("total_stats") {
		return nil, fmt.Errorf("expecting `total_stats`; got %q", lex.token)
	}
	lex.nextToken()

	return parsePipeRunningStatsExt(lex, "total_stats")
}
