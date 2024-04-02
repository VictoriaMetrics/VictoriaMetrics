package graphite

import (
	"container/heap"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/graphiteql"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
)

// nextSeriesFunc must return the next series to process.
//
// nextSeriesFunc must release all the occupied resources before returning non-nil error.
// drainAllSeries can be used for releasing occupied resources.
//
// When there are no more series to return, (nil, nil) must be returned.
type nextSeriesFunc func() (*series, error)

type transformFunc func(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error)

var transformFuncs = map[string]transformFunc{}

func init() {
	// A workaround for https://github.com/golang/go/issues/43741
	transformFuncs = map[string]transformFunc{
		"absolute":                    transformAbsolute,
		"add":                         transformAdd,
		"aggregate":                   transformAggregate,
		"aggregateLine":               transformAggregateLine,
		"aggregateSeriesLists":        transformAggregateSeriesLists,
		"aggregateWithWildcards":      transformAggregateWithWildcards,
		"alias":                       transformAlias,
		"aliasByMetric":               transformAliasByMetric,
		"aliasByNode":                 transformAliasByNode,
		"aliasByTags":                 transformAliasByNode,
		"aliasQuery":                  transformAliasQuery,
		"aliasSub":                    transformAliasSub,
		"alpha":                       transformAlpha,
		"applyByNode":                 transformApplyByNode,
		"areaBetween":                 transformAreaBetween,
		"asPercent":                   transformAsPercent,
		"averageAbove":                transformAverageAbove,
		"averageBelow":                transformAverageBelow,
		"averageOutsidePercentile":    transformAverageOutsidePercentile,
		"averageSeries":               transformAverageSeries,
		"averageSeriesWithWildcards":  transformAverageSeriesWithWildcards,
		"avg":                         transformAverageSeries,
		"cactiStyle":                  transformTODO,
		"changed":                     transformChanged,
		"color":                       transformColor,
		"consolidateBy":               transformConsolidateBy,
		"constantLine":                transformConstantLine,
		"countSeries":                 transformCountSeries,
		"cumulative":                  transformCumulative,
		"currentAbove":                transformCurrentAbove,
		"currentBelow":                transformCurrentBelow,
		"dashed":                      transformDashed,
		"delay":                       transformDelay,
		"derivative":                  transformDerivative,
		"diffSeries":                  transformDiffSeries,
		"diffSeriesLists":             transformDiffSeriesLists,
		"divideSeries":                transformDivideSeries,
		"divideSeriesLists":           transformDivideSeriesLists,
		"drawAsInfinite":              transformDrawAsInfinite,
		"events":                      transformEvents,
		"exclude":                     transformExclude,
		"exp":                         transformExp,
		"exponentialMovingAverage":    transformExponentialMovingAverage,
		"fallbackSeries":              transformFallbackSeries,
		"filterSeries":                transformFilterSeries,
		"grep":                        transformGrep,
		"group":                       transformGroup,
		"groupByNode":                 transformGroupByNode,
		"groupByNodes":                transformGroupByNodes,
		"groupByTags":                 transformGroupByTags,
		"highest":                     transformHighest,
		"highestAverage":              transformHighestAverage,
		"highestCurrent":              transformHighestCurrent,
		"highestMax":                  transformHighestMax,
		"hitcount":                    transformHitcount,
		"holtWintersAberration":       transformHoltWintersAberration,
		"holtWintersConfidenceArea":   transformHoltWintersConfidenceArea,
		"holtWintersConfidenceBands":  transformHoltWintersConfidenceBands,
		"holtWintersForecast":         transformHoltWintersForecast,
		"identity":                    transformIdentity,
		"integral":                    transformIntegral,
		"integralByInterval":          transformIntegralByInterval,
		"interpolate":                 transformInterpolate,
		"invert":                      transformInvert,
		"isNonNull":                   transformIsNonNull,
		"keepLastValue":               transformKeepLastValue,
		"legendValue":                 transformTODO,
		"limit":                       transformLimit,
		"lineWidth":                   transformLineWidth,
		"linearRegression":            transformLinearRegression,
		"log":                         transformLogarithm,
		"logarithm":                   transformLogarithm,
		"logit":                       transformLogit,
		"lowest":                      transformLowest,
		"lowestAverage":               transformLowestAverage,
		"lowestCurrent":               transformLowestCurrent,
		"map":                         transformTODO,
		"mapSeries":                   transformTODO,
		"max":                         transformMaxSeries,
		"maxSeries":                   transformMaxSeries,
		"maximumAbove":                transformMaximumAbove,
		"maximumBelow":                transformMaximumBelow,
		"minMax":                      transformMinMax,
		"min":                         transformMinSeries,
		"minSeries":                   transformMinSeries,
		"minimumAbove":                transformMinimumAbove,
		"minimumBelow":                transformMinimumBelow,
		"mostDeviant":                 transformMostDeviant,
		"movingAverage":               transformMovingAverage,
		"movingMax":                   transformMovingMax,
		"movingMedian":                transformMovingMedian,
		"movingMin":                   transformMovingMin,
		"movingSum":                   transformMovingSum,
		"movingWindow":                transformMovingWindow,
		"multiplySeries":              transformMultiplySeries,
		"multiplySeriesLists":         transformMultiplySeriesLists,
		"multiplySeriesWithWildcards": transformMultiplySeriesWithWildcards,
		"nPercentile":                 transformNPercentile,
		"nonNegativeDerivative":       transformNonNegativeDerivative,
		"offset":                      transformOffset,
		"offsetToZero":                transformOffsetToZero,
		"perSecond":                   transformPerSecond,
		"percentileOfSeries":          transformPercentileOfSeries,
		// It looks like pie* functions aren't needed for Graphite render API
		//		"pieAverage":                  transformTODO,
		//		"pieMaximum":                  transformTODO,
		//		"pieMinimum":                  transformTODO,
		"pow":                     transformPow,
		"powSeries":               transformPowSeries,
		"randomWalk":              transformRandomWalk,
		"randomWalkFunction":      transformRandomWalk,
		"rangeOfSeries":           transformRangeOfSeries,
		"reduce":                  transformTODO,
		"reduceSeries":            transformTODO,
		"removeAbovePercentile":   transformRemoveAbovePercentile,
		"removeAboveValue":        transformRemoveAboveValue,
		"removeBelowPercentile":   transformRemoveBelowPercentile,
		"removeBelowValue":        transformRemoveBelowValue,
		"removeBetweenPercentile": transformRemoveBetweenPercentile,
		"removeEmptySeries":       transformRemoveEmptySeries,
		"round":                   transformRoundFunction,
		"roundFunction":           transformRoundFunction,
		"scale":                   transformScale,
		"scaleToSeconds":          transformScaleToSeconds,
		"secondYAxis":             transformSecondYAxis,
		"seriesByTag":             transformSeriesByTag,
		"setXFilesFactor":         transformSetXFilesFactor,
		"sigmoid":                 transformSigmoid,
		"sin":                     transformSinFunction,
		"sinFunction":             transformSinFunction,
		"smartSummarize":          transformSmartSummarize,
		"sortBy":                  transformSortBy,
		"sortByMaxima":            transformSortByMaxima,
		"sortByMinima":            transformSortByMinima,
		"sortByName":              transformSortByName,
		"sortByTotal":             transformSortByTotal,
		"squareRoot":              transformSquareRoot,
		"stacked":                 transformStacked,
		"stddevSeries":            transformStddevSeries,
		"stdev":                   transformStdev,
		"substr":                  transformSubstr,
		"sum":                     transformSumSeries,
		"sumSeries":               transformSumSeries,
		"sumSeriesLists":          transformSumSeriesLists,
		"sumSeriesWithWildcards":  transformSumSeriesWithWildcards,
		"summarize":               transformSummarize,
		"threshold":               transformThreshold,
		"time":                    transformTimeFunction,
		"timeFunction":            transformTimeFunction,
		"timeShift":               transformTimeShift,
		"timeSlice":               transformTimeSlice,
		"timeStack":               transformTimeStack,
		"transformNull":           transformTransformNull,
		"unique":                  transformUnique,
		"useSeriesAbove":          transformUseSeriesAbove,
		"verticalLine":            transformVerticalLine,
		"weightedAverage":         transformWeightedAverage,
		"xFilesFactor":            transformSetXFilesFactor,
	}
}

func transformTODO(_ *evalConfig, _ *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return nil, fmt.Errorf("TODO: implement this function")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.absolute
func transformAbsolute(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			values[i] = math.Abs(v)
		}
		s.Name = fmt.Sprintf("absolute(%s)", s.Name)
		s.Tags["absolute"] = "1"
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.add
func transformAdd(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "constant", 1)
	if err != nil {
		return nil, err
	}
	nString := fmt.Sprintf("%g", n)
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i := range values {
			values[i] += n
		}
		s.Tags["add"] = nString
		s.Name = fmt.Sprintf("add(%s,%s)", s.Name, nString)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.aggregate
func transformAggregate(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 && len(args) != 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2 or 3", len(args))
	}
	funcName, err := getString(args, "func", 1)
	if err != nil {
		return nil, err
	}
	funcName = strings.TrimSuffix(funcName, "Series")
	xFilesFactor, err := getOptionalNumber(args, "xFilesFactor", 2, ec.xFilesFactor)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return aggregateSeries(ec, fe, nextSeries, funcName, xFilesFactor)
}

func aggregateSeries(ec *evalConfig, expr graphiteql.Expr, nextSeries nextSeriesFunc, funcName string, xFilesFactor float64) (nextSeriesFunc, error) {
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, err
	}
	as, err := newAggrState(ec.pointsLen(step), funcName)
	if err != nil {
		_, _ = drainAllSeries(nextSeries)
		return nil, err
	}
	nextSeriesWrapper := getNextSeriesWrapperForAggregateFunc(funcName)
	var seriesTags []map[string]string
	var seriesExpressions []string
	var mu sync.Mutex
	f := nextSeriesWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidate(ec, step)
		mu.Lock()
		as.Update(s.Values)
		seriesTags = append(seriesTags, s.Tags)
		seriesExpressions = append(seriesExpressions, s.pathExpression)
		mu.Unlock()
		return s, nil
	})
	if _, err := drainAllSeries(f); err != nil {
		return nil, err
	}
	if len(seriesTags) == 0 {
		return newZeroSeriesFunc(), nil
	}
	tags := seriesTags[0]
	for _, m := range seriesTags[1:] {
		for k, v := range tags {
			if m[k] != v {
				delete(tags, k)
			}
		}
	}
	name := formatAggrFuncForSeriesNames(funcName, seriesExpressions)
	tags["aggregatedBy"] = funcName
	if tags["name"] == "" {
		tags["name"] = name
	}
	s := &series{
		Name:           name,
		Tags:           tags,
		Timestamps:     ec.newTimestamps(step),
		Values:         as.Finalize(xFilesFactor),
		pathExpression: name,
		expr:           expr,
		step:           step,
	}
	return singleSeriesFunc(s), nil
}

func aggregateSeriesGeneric(ec *evalConfig, fe *graphiteql.FuncExpr, funcName string) (nextSeriesFunc, error) {
	nextSeries, err := groupSeriesLists(ec, fe.Args, fe)
	if err != nil {
		return nil, err
	}
	return aggregateSeries(ec, fe, nextSeries, funcName, ec.xFilesFactor)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.aggregateLine
func transformAggregateLine(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1, 2 or 3", len(args))
	}
	funcName, err := getOptionalString(args, "func", 1, "avg")
	if err != nil {
		return nil, err
	}
	aggrFunc, err := getAggrFunc(funcName)
	if err != nil {
		return nil, err
	}
	keepStep, err := getOptionalBool(args, "keepStep", 2, false)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		v := aggrFunc(values)
		if keepStep {
			for i := range values {
				values[i] = v
			}
		} else {
			s.Timestamps = []int64{ec.startTime, (ec.endTime + ec.startTime) / 2, ec.endTime}
			s.Values = []float64{v, v, v}
		}
		vString := "None"
		if !math.IsNaN(v) {
			vString = fmt.Sprintf("%g", v)
		}
		s.Name = fmt.Sprintf("aggregateLine(%s,%s)", s.Name, vString)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.aggregateWithWildcards
func transformAggregateWithWildcards(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want at least 2", len(args))
	}
	funcName, err := getString(args, "func", 1)
	if err != nil {
		return nil, err
	}
	positions, err := getInts(args[2:], "positions")
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return aggregateSeriesWithWildcards(ec, fe, nextSeries, funcName, positions)
}

func aggregateSeriesWithWildcards(ec *evalConfig, expr graphiteql.Expr, nextSeries nextSeriesFunc, funcName string, positions []int) (nextSeriesFunc, error) {
	positionsMap := make(map[int]struct{})
	for _, pos := range positions {
		positionsMap[pos] = struct{}{}
	}
	keyFunc := func(name string, _ map[string]string) string {
		parts := strings.Split(getPathFromName(name), ".")
		dstParts := parts[:0]
		for i, part := range parts {
			if _, ok := positionsMap[i]; ok {
				continue
			}
			dstParts = append(dstParts, part)
		}
		return strings.Join(dstParts, ".")
	}
	return groupByKeyFunc(ec, expr, nextSeries, funcName, keyFunc)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.alias
func transformAlias(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	newName, err := getString(args, "newName", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		s.Name = newName
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.aliasByMetric
func transformAliasByMetric(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		path := getPathFromName(s.Name)
		n := strings.LastIndexByte(path, '.')
		if n > 0 {
			path = path[n+1:]
		}
		s.Name = path
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.aliasByNode
func transformAliasByNode(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want at least 1", len(args))
	}
	nodes, err := getNodes(args[1:])
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		s.Name = getNameFromNodes(s.Name, s.Tags, nodes)
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.aliasQuery
func transformAliasQuery(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 4 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 4", len(args))
	}
	re, err := getRegexp(args, "search", 1)
	if err != nil {
		return nil, err
	}
	replace, err := getRegexpReplacement(args, "replace", 2)
	if err != nil {
		return nil, err
	}
	newName, err := getString(args, "newName", 3)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		query := re.ReplaceAllString(s.Name, replace)
		next, err := execExpr(ec, query)
		if err != nil {
			return nil, fmt.Errorf("cannot evaluate query %q: %w", query, err)
		}
		ss, err := fetchAllSeries(next)
		if err != nil {
			return nil, fmt.Errorf("cannot fetch series for query %q: %w", query, err)
		}
		if len(ss) == 0 {
			return nil, fmt.Errorf("cannot find series for query %q", query)
		}
		v := aggrLast(ss[0].Values)
		if math.IsNaN(v) {
			return nil, fmt.Errorf("cannot find values for query %q", query)
		}
		name := strings.ReplaceAll(newName, "%d", fmt.Sprintf("%d", int(v)))
		name = strings.ReplaceAll(name, "%g", fmt.Sprintf("%g", v))
		name = strings.ReplaceAll(name, "%f", fmt.Sprintf("%f", v))
		s.Name = name
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.aliasSub
func transformAliasSub(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 3", len(args))
	}
	re, err := getRegexp(args, "search", 1)
	if err != nil {
		return nil, err
	}
	replace, err := getRegexpReplacement(args, "replace", 2)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		s.Name = re.ReplaceAllString(s.Name, replace)
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.alpha
func transformAlpha(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	_, err := getNumber(args, "alpha", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.applyByNode
func transformApplyByNode(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 3 || len(args) > 4 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want from 3 to 4", len(args))
	}
	nn, err := getNumber(args, "nodeNum", 1)
	if err != nil {
		return nil, err
	}
	nodeNum := int(nn)
	templateFunction, err := getString(args, "templateFunction", 2)
	if err != nil {
		return nil, err
	}
	newName, err := getOptionalString(args, "newName", 3, "")
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	nextTemplateSeries := newZeroSeriesFunc()
	prefix := ""
	visitedPrefixes := make(map[string]struct{})
	f := func() (*series, error) {
		for {
			ts, err := nextTemplateSeries()
			if err != nil {
				_, _ = drainAllSeries(nextSeries)
				return nil, err
			}
			if ts != nil {
				if newName != "" {
					ts.Name = strings.ReplaceAll(newName, "%", prefix)
				}
				ts.expr = fe
				ts.pathExpression = prefix
				return ts, nil
			}
			for {
				s, err := nextSeries()
				if err != nil {
					return nil, err
				}
				if s == nil {
					return nil, nil
				}
				prefix = getPathFromName(s.Name)
				nodes := strings.Split(prefix, ".")
				if nodeNum >= 0 && nodeNum < len(nodes) {
					prefix = strings.Join(nodes[:nodeNum+1], ".")
				}
				if _, ok := visitedPrefixes[prefix]; !ok {
					visitedPrefixes[prefix] = struct{}{}
					break
				}
			}
			query := strings.ReplaceAll(templateFunction, "%", prefix)
			next, err := execExpr(ec, query)
			if err != nil {
				_, _ = drainAllSeries(nextSeries)
				return nil, err
			}
			nextTemplateSeries = next
		}
	}
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.areaBetween
func transformAreaBetween(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	seriesFound := 0
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		seriesFound++
		if seriesFound > 2 {
			return nil, fmt.Errorf("expecting exactly two series; got more series")
		}
		s.Tags["areaBetween"] = "1"
		s.Name = fmt.Sprintf("areaBetween(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.asPercent
func transformAsPercent(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want at least 1", len(args))
	}
	totalArg := getOptionalArg(args, "total", 1)
	if totalArg == nil {
		totalArg = &graphiteql.ArgExpr{
			Expr: &graphiteql.NoneExpr{},
		}
	}
	var nodes []graphiteql.Expr
	if len(args) > 2 {
		ns, err := getNodes(args[2:])
		if err != nil {
			return nil, err
		}
		nodes = ns
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	switch t := totalArg.Expr.(type) {
	case *graphiteql.NoneExpr:
		if len(nodes) == 0 {
			ss, step, err := fetchNormalizedSeries(ec, nextSeries, true)
			if err != nil {
				return nil, err
			}
			inplacePercentForMultiSeries(ec, fe, ss, step)
			return multiSeriesFunc(ss), nil
		}
		m, step, err := fetchNormalizedSeriesByNodes(ec, nextSeries, nodes)
		if err != nil {
			return nil, err
		}
		var ssAll []*series
		for _, ss := range m {
			inplacePercentForMultiSeries(ec, fe, ss, step)
			ssAll = append(ssAll, ss...)
		}
		return multiSeriesFunc(ssAll), nil
	case *graphiteql.NumberExpr:
		if len(nodes) > 0 {
			_, _ = drainAllSeries(nextSeries)
			return nil, fmt.Errorf("unexpected non-empty nodes for numeric total")
		}
		total := t.N
		f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
			values := s.Values
			for i, v := range values {
				values[i] = v / total * 100
			}
			s.Name = fmt.Sprintf("asPercent(%s,%g)", s.Name, total)
			s.expr = fe
			s.pathExpression = s.Name
			return s, nil
		})
		return f, nil
	default:
		nextTotal, err := evalExpr(ec, t)
		if err != nil {
			_, _ = drainAllSeries(nextSeries)
			return nil, err
		}
		if len(nodes) == 0 {
			// Fetch series serially in order to preserve the original order of series returned by nextTotal,
			// so the returned series could be matched against series returned by nextSeries.
			ssTotal, stepTotal, err := fetchNormalizedSeries(ec, nextTotal, false)
			if err != nil {
				_, _ = drainAllSeries(nextSeries)
				return nil, err
			}
			if len(ssTotal) == 0 {
				_, _ = drainAllSeries(nextSeries)
				// The `total` expression matches zero series. Return empty response in this case.
				return multiSeriesFunc(nil), nil
			}
			if len(ssTotal) == 1 {
				sTotal := ssTotal[0]
				f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
					s.consolidate(ec, stepTotal)
					inplacePercentForSingleSeries(fe, s, sTotal)
					return s, nil
				})
				return f, nil
			}
			// Fetch series serially in order to preserve the original order of series returned by nextSeries
			// and match these series to ssTotal
			ss, step, err := fetchNormalizedSeries(ec, nextSeries, false)
			if err != nil {
				return nil, err
			}
			if len(ss) != len(ssTotal) {
				return nil, fmt.Errorf("unexpected number of series returned by total expression; got %d; want %d", len(ssTotal), len(ss))
			}
			if step != stepTotal {
				return nil, fmt.Errorf("step mismatch for series and total series: %d vs %d", step, stepTotal)
			}
			for i, s := range ss {
				inplacePercentForSingleSeries(fe, s, ssTotal[i])
			}
			return multiSeriesFunc(ss), nil
		}
		m, step, err := fetchNormalizedSeriesByNodes(ec, nextSeries, nodes)
		if err != nil {
			_, _ = drainAllSeries(nextTotal)
			return nil, err
		}
		mTotal, stepTotal, err := fetchNormalizedSeriesByNodes(ec, nextTotal, nodes)
		if err != nil {
			return nil, err
		}
		if step != stepTotal {
			return nil, fmt.Errorf("step mismatch for series and total series: %d vs %d", step, stepTotal)
		}
		var ssAll []*series
		for key, ssTotal := range mTotal {
			seriesExpressions := make([]string, 0, len(ssTotal))
			as := newAggrStateSum(ec.pointsLen(step))
			for _, s := range ssTotal {
				seriesExpressions = append(seriesExpressions, s.pathExpression)
				as.Update(s.Values)
			}
			totalValues := as.Finalize(ec.xFilesFactor)
			totalName := formatAggrFuncForPercentSeriesNames("sum", seriesExpressions)
			ss := m[key]
			if ss == nil {
				s := newNaNSeries(ec, step)
				newName := fmt.Sprintf("asPercent(MISSING,%s)", totalName)
				s.Name = newName
				s.Tags["name"] = newName
				s.expr = fe
				s.pathExpression = s.Name
				ssAll = append(ssAll, s)
				continue
			}
			for _, s := range ss {
				values := s.Values
				for i, v := range values {
					values[i] = v / totalValues[i] * 100
				}
				newName := fmt.Sprintf("asPercent(%s,%s)", s.Name, totalName)
				s.Name = newName
				s.Tags["name"] = newName
				s.expr = fe
				s.pathExpression = s.Name
				ssAll = append(ssAll, s)
			}
		}
		for key, ss := range m {
			ssTotal := mTotal[key]
			if ssTotal != nil {
				continue
			}
			for _, s := range ss {
				values := s.Values
				for i := range values {
					values[i] = nan
				}
				newName := fmt.Sprintf("asPercent(%s,MISSING)", s.Name)
				s.Name = newName
				s.Tags["name"] = newName
				s.expr = fe
				s.pathExpression = s.Name
				ssAll = append(ssAll, s)
			}
		}
		return multiSeriesFunc(ssAll), nil
	}
}

func inplacePercentForSingleSeries(expr graphiteql.Expr, s, sTotal *series) {
	values := s.Values
	totalValues := sTotal.Values
	for i, v := range values {
		values[i] = v / totalValues[i] * 100
	}
	newName := fmt.Sprintf("asPercent(%s,%s)", s.Name, sTotal.Name)
	s.Name = newName
	s.Tags["name"] = newName
	s.expr = expr
	s.pathExpression = s.Name
}

func inplacePercentForMultiSeries(ec *evalConfig, expr graphiteql.Expr, ss []*series, step int64) {
	seriesExpressions := make([]string, 0, len(ss))
	as := newAggrStateSum(ec.pointsLen(step))
	for _, s := range ss {
		seriesExpressions = append(seriesExpressions, s.pathExpression)
		as.Update(s.Values)
	}
	totalValues := as.Finalize(ec.xFilesFactor)
	totalName := formatAggrFuncForPercentSeriesNames("sum", seriesExpressions)
	for _, s := range ss {
		values := s.Values
		for i, v := range values {
			values[i] = v / totalValues[i] * 100
		}
		newName := fmt.Sprintf("asPercent(%s,%s)", s.Name, totalName)
		s.Name = newName
		s.Tags["name"] = newName
		s.expr = expr
		s.pathExpression = s.Name
	}
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.averageAbove
func transformAverageAbove(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return filterSeriesGeneric(fe, nextSeries, "average", ">", n)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.averageBelow
func transformAverageBelow(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return filterSeriesGeneric(fe, nextSeries, "average", "<", n)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.averageOutsidePercentile
func transformAverageOutsidePercentile(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	var sws []seriesWithWeight
	var lock sync.Mutex
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		avg := aggrAvg(s.Values)
		lock.Lock()
		sws = append(sws, seriesWithWeight{
			s: s,
			v: avg,
		})
		lock.Unlock()
		return s, nil
	})
	if _, err := drainAllSeries(f); err != nil {
		return nil, err
	}
	avgs := make([]float64, len(sws))
	for i, sw := range sws {
		avgs[i] = sw.v
	}
	if n > 50 {
		n = 100 - n
	}
	lowPercentile := n
	highPercentile := 100 - n
	lowValue := newAggrFuncPercentile(lowPercentile)(avgs)
	highValue := newAggrFuncPercentile(highPercentile)(avgs)
	var ss []*series
	for _, sw := range sws {
		if sw.v < lowValue || sw.v > highValue {
			s := sw.s
			s.expr = fe
			ss = append(ss, s)
		}
	}
	return multiSeriesFunc(ss), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.averageSeries
func transformAverageSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesGeneric(ec, fe, "average")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.averageSeriesWithWildcards
func transformAverageSeriesWithWildcards(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesWithWildcardsGeneric(ec, fe, "average")
}

func aggregateSeriesWithWildcardsGeneric(ec *evalConfig, fe *graphiteql.FuncExpr, funcName string) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; must be at least 1", len(args))
	}
	positions, err := getInts(args[1:], "position")
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return aggregateSeriesWithWildcards(ec, fe, nextSeries, funcName, positions)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.changed
func transformChanged(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("expecting a single arg; got %d args", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		prevValue := nan
		for i, v := range values {
			if math.IsNaN(prevValue) {
				prevValue = v
				values[i] = 0
			} else if !math.IsNaN(v) && prevValue != v {
				prevValue = v
				values[i] = 1
			} else {
				values[i] = 0
			}
		}
		s.Name = fmt.Sprintf("changed(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.color
func transformColor(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	_, err := getString(args, "theColor", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.countSeries
func transformCountSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesGeneric(ec, fe, "count")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.cumulative
func transformCumulative(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return consolidateBy(fe, nextSeries, "sum")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.consolidateBy
func transformConsolidateBy(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	funcName, err := getString(args, "consolidationFunc", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return consolidateBy(fe, nextSeries, funcName)
}

func consolidateBy(expr graphiteql.Expr, nextSeries nextSeriesFunc, funcName string) (nextSeriesFunc, error) {
	consolidateFunc, err := getAggrFunc(funcName)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidateFunc = consolidateFunc
		s.Name = fmt.Sprintf("consolidateBy(%s,%s)", s.Name, graphiteql.QuoteString(funcName))
		s.Tags["consolidateBy"] = funcName
		s.expr = expr
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.constantLine
func transformConstantLine(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("expecting a single arg; got %d", len(args))
	}
	n, err := getNumber(args, "value", 0)
	if err != nil {
		return nil, err
	}
	return constantLine(ec, fe, n), nil
}

func constantLine(ec *evalConfig, expr graphiteql.Expr, n float64) nextSeriesFunc {
	name := fmt.Sprintf("%g", n)
	step := (ec.endTime - ec.startTime) / 2
	s := &series{
		Name:           name,
		Tags:           unmarshalTags(name),
		Timestamps:     []int64{ec.startTime, ec.startTime + step, ec.startTime + 2*step},
		Values:         []float64{n, n, n},
		expr:           expr,
		pathExpression: string(expr.AppendString(nil)),
		step:           step,
	}
	return singleSeriesFunc(s)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.currentAbove
func transformCurrentAbove(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return filterSeriesGeneric(fe, nextSeries, "current", ">", n)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.currentBelow
func transformCurrentBelow(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return filterSeriesGeneric(fe, nextSeries, "current", "<", n)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.dashed
func transformDashed(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1 or 2", len(args))
	}
	dashLength, err := getOptionalNumber(args, "dashLength", 1, 5)
	if err != nil {
		return nil, err
	}
	dashLengthStr := fmt.Sprintf("%g", dashLength)
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		s.Name = fmt.Sprintf("dashed(%s,%s)", s.Name, dashLengthStr)
		s.Tags["dashed"] = dashLengthStr
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.delay
func transformDelay(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	stepsFloat, err := getNumber(args, "steps", 1)
	if err != nil {
		return nil, err
	}
	steps := int(stepsFloat)
	stepsStr := fmt.Sprintf("%d", steps)
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		stepsLocal := steps
		if stepsLocal < 0 {
			stepsLocal = -stepsLocal
			if stepsLocal > len(values) {
				stepsLocal = len(values)
			}
			copy(values, values[stepsLocal:])
			for i := len(values) - 1; i >= len(values)-stepsLocal; i-- {
				values[i] = nan
			}
		} else {
			if stepsLocal > len(values) {
				stepsLocal = len(values)
			}
			copy(values[stepsLocal:], values[:len(values)-stepsLocal])
			for i := 0; i < stepsLocal; i++ {
				values[i] = nan
			}
		}
		s.Tags["delay"] = stepsStr
		s.Name = fmt.Sprintf("delay(%s,%d)", s.Name, steps)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.derivative
func transformDerivative(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		prevValue := nan
		for i, v := range values {
			if math.IsNaN(prevValue) || math.IsNaN(v) {
				values[i] = nan
			} else {
				values[i] = v - prevValue
			}
			prevValue = v
		}
		s.Tags["derivative"] = "1"
		s.Name = fmt.Sprintf("derivative(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.diffSeries
func transformDiffSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesGeneric(ec, fe, "diff")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.divideSeries
func transformDivideSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	nextDivisor, err := evalSeriesList(ec, args, "divisorSeries", 1)
	if err != nil {
		return nil, err
	}
	ssDivisors, stepDivisor, err := fetchNormalizedSeries(ec, nextDivisor, false)
	if err != nil {
		return nil, err
	}
	if len(ssDivisors) > 1 {
		return nil, fmt.Errorf("unexpected number of divisorSeries; got %d; want 1", len(ssDivisors))
	}
	nextDividend, err := evalSeriesList(ec, args, "dividendSeriesList", 0)
	if err != nil {
		return nil, err
	}
	if len(ssDivisors) == 0 {
		f := nextSeriesConcurrentWrapper(nextDividend, func(s *series) (*series, error) {
			values := s.Values
			for i := range values {
				values[i] = nan
			}
			s.Name = fmt.Sprintf("divideSeries(%s,MISSING)", s.Name)
			s.expr = fe
			s.pathExpression = s.Name
			return s, nil
		})
		return f, nil
	}
	sDivisor := ssDivisors[0]
	divisorName := sDivisor.Name
	divisorValues := sDivisor.Values
	f := nextSeriesSerialWrapper(nextDividend, func(s *series) (*series, error) {
		s.consolidate(ec, stepDivisor)
		values := s.Values
		for i, v := range values {
			values[i] = v / divisorValues[i]
		}
		s.Name = fmt.Sprintf("divideSeries(%s,%s)", s.Name, divisorName)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

func aggregateSeriesListsGeneric(ec *evalConfig, fe *graphiteql.FuncExpr, funcName string) (nextSeriesFunc, error) {
	args := fe.Args
	agg, err := getAggrFunc(funcName)
	if err != nil {
		return nil, err
	}
	nextSeriesFirst, err := evalSeriesList(ec, args, "seriesListFirstPos", 0)
	if err != nil {
		return nil, err
	}
	nextSeriesSecond, err := evalSeriesList(ec, args, "seriesListSecondPos", 1)
	if err != nil {
		_, _ = drainAllSeries(nextSeriesFirst)
		return nil, err
	}
	return aggregateSeriesList(ec, fe, nextSeriesFirst, nextSeriesSecond, agg, funcName)
}

// See https://graphite.readthedocs.io/en/latest/functions.html#graphite.render.functions.aggregateSeriesLists
func transformAggregateSeriesLists(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 3 && len(args) != 4 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 3 or 4", len(args))
	}

	funcName, err := getString(args, "func", 2)
	if err != nil {
		return nil, err
	}

	return aggregateSeriesListsGeneric(ec, fe, funcName)
}

// See https://graphite.readthedocs.io/en/latest/functions.html#graphite.render.functions.sumSeriesLists
func transformSumSeriesLists(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesListsGeneric(ec, fe, "sum")
}

// See https://graphite.readthedocs.io/en/latest/functions.html#graphite.render.functions.multiplySeriesLists
func transformMultiplySeriesLists(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesListsGeneric(ec, fe, "multiply")
}

// See https://graphite.readthedocs.io/en/latest/functions.html#graphite.render.functions.diffSeriesLists
func transformDiffSeriesLists(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesListsGeneric(ec, fe, "diff")
}

func aggregateSeriesList(ec *evalConfig, fe *graphiteql.FuncExpr, nextSeriesFirst, nextSeriesSecond nextSeriesFunc, agg aggrFunc, funcName string) (nextSeriesFunc, error) {
	ssFirst, stepFirst, err := fetchNormalizedSeries(ec, nextSeriesFirst, false)
	if err != nil {
		_, _ = drainAllSeries(nextSeriesSecond)
		return nil, err
	}
	ssSecond, stepSecond, err := fetchNormalizedSeries(ec, nextSeriesSecond, false)
	if err != nil {
		return nil, err
	}

	if len(ssFirst) != len(ssSecond) {
		return nil, fmt.Errorf("First and second lists must have equal number of series; got %d vs %d series", len(ssFirst), len(ssSecond))
	}
	if stepFirst != stepSecond {
		return nil, fmt.Errorf("step mismatch for first and second: %d vs %d", stepFirst, stepSecond)
	}

	valuePair := make([]float64, 2)
	for i, s := range ssFirst {
		sSecond := ssSecond[i]
		values := s.Values
		secondValues := sSecond.Values
		for j, v := range values {
			valuePair[0], valuePair[1] = v, secondValues[j]
			values[j] = agg(valuePair)
		}
		s.Name = fmt.Sprintf("%sSeries(%s,%s)", funcName, s.Name, sSecond.Name)
		s.expr = fe
		s.pathExpression = s.Name
	}
	return multiSeriesFunc(ssFirst), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.divideSeriesLists
func transformDivideSeriesLists(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	nextDividend, err := evalSeriesList(ec, args, "dividendSeriesList", 0)
	if err != nil {
		return nil, err
	}
	nextDivisor, err := evalSeriesList(ec, args, "divisorSeriesList", 1)
	if err != nil {
		return nil, err
	}

	return aggregateSeriesList(ec, fe, nextDividend, nextDivisor, func(values []float64) float64 {
		return values[0] / values[1]
	}, "divide")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.drawAsInfinite
func transformDrawAsInfinite(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		s.Tags["drawAsInfinite"] = "1"
		s.Name = fmt.Sprintf("drawAsInfinite(%s)", s.Name)
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.events
func transformEvents(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	var tags []string
	for _, arg := range args {
		se, ok := arg.Expr.(*graphiteql.StringExpr)
		if !ok {
			return nil, fmt.Errorf("expecting string tag; got %T", arg.Expr)
		}
		tags = append(tags, graphiteql.QuoteString(se.S))
	}
	s := newNaNSeries(ec, ec.storageStep)
	events := fmt.Sprintf("events(%s)", strings.Join(tags, ","))
	s.Name = events
	s.Tags = map[string]string{"name": events}
	s.expr = fe
	s.pathExpression = s.Name
	return singleSeriesFunc(s), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.exclude
func transformExclude(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("expecting two args; got %d args", len(args))
	}
	pattern, err := getRegexp(args, "pattern", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		if pattern.MatchString(s.Name) {
			return nil, nil
		}
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.exp
func transformExp(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("expecting one arg; got %d args", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			values[i] = math.Exp(v)
		}
		s.Tags["exp"] = "e"
		s.Name = fmt.Sprintf("exp(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.exponentialMovingAverage
func transformExponentialMovingAverage(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	windowSizeArg, err := getArg(args, "windowSize", 1)
	if err != nil {
		return nil, err
	}
	windowSizeStr := string(windowSizeArg.Expr.AppendString(nil))
	var c float64
	var windowSize int64
	switch t := windowSizeArg.Expr.(type) {
	case *graphiteql.StringExpr:
		ws, err := parseInterval(t.S)
		if err != nil {
			return nil, fmt.Errorf("cannot parse windowSize: %w", err)
		}
		c = 2 / (float64(ws)/1000 + 1)
		windowSize = ws
	case *graphiteql.NumberExpr:
		c = 2 / (t.N + 1)
		windowSize = int64(t.N * float64(ec.storageStep))
	default:
		return nil, fmt.Errorf("windowSize must be either string or number; got %T", t)
	}
	if windowSize < 0 {
		windowSize = -windowSize
	}

	ecCopy := *ec
	ecCopy.startTime -= windowSize
	nextSeries, err := evalSeriesList(&ecCopy, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		timestamps := s.Timestamps
		i := 0
		for i < len(timestamps) && timestamps[i] < ec.startTime {
			i++
		}
		ema := aggrAvg(s.Values[:i])
		if math.IsNaN(ema) {
			ema = 0
		}
		values := s.Values[i:]
		timestamps = timestamps[i:]
		for i, v := range values {
			ema = c*v + (1-c)*ema
			values[i] = ema
		}
		s.Timestamps = append([]int64{}, timestamps...)
		s.Values = append([]float64{}, values...)
		s.Tags["exponentialMovingAverage"] = windowSizeStr
		s.Name = fmt.Sprintf("exponentialMovingAverage(%s,%s)", s.Name, windowSizeStr)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.fallbackSeries
func transformFallbackSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of arg; got %d; want 2", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	seriesFetched := 0
	fallbackUsed := false
	f := func() (*series, error) {
		for {
			s, err := nextSeries()
			if err != nil {
				return nil, err
			}
			if s != nil {
				seriesFetched++
				s.expr = fe
				return s, nil
			}
			if fallbackUsed || seriesFetched > 0 {
				return nil, nil
			}
			fallback, err := evalSeriesList(ec, args, "fallback", 1)
			if err != nil {
				return nil, err
			}
			nextSeries = fallback
			fallbackUsed = true
		}
	}
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.filterSeries
func transformFilterSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 4 {
		return nil, fmt.Errorf("unexpected number of arg; got %d; want 4", len(args))
	}
	funcName, err := getString(args, "func", 1)
	if err != nil {
		return nil, err
	}
	operator, err := getString(args, "operator", 2)
	if err != nil {
		return nil, err
	}
	threshold, err := getNumber(args, "threshold", 3)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return filterSeriesGeneric(fe, nextSeries, funcName, operator, threshold)
}

func filterSeriesGeneric(expr graphiteql.Expr, nextSeries nextSeriesFunc, funcName, operator string, threshold float64) (nextSeriesFunc, error) {
	aggrFunc, err := getAggrFunc(funcName)
	if err != nil {
		_, _ = drainAllSeries(nextSeries)
		return nil, err
	}
	operatorFunc, err := getOperatorFunc(operator)
	if err != nil {
		_, _ = drainAllSeries(nextSeries)
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		v := aggrFunc(s.Values)
		if !operatorFunc(v, threshold) {
			return nil, nil
		}
		s.expr = expr
		return s, nil
	})
	return f, nil
}

func getOperatorFunc(operator string) (operatorFunc, error) {
	switch operator {
	case "=":
		return operatorFuncEqual, nil
	case "!=":
		return operatorFuncNotEqual, nil
	case ">":
		return operatorFuncAbove, nil
	case ">=":
		return operatorFuncAboveEqual, nil
	case "<":
		return operatorFuncBelow, nil
	case "<=":
		return operatorFuncBelowEqual, nil
	default:
		return nil, fmt.Errorf("unknown operator %q", operator)
	}
}

type operatorFunc func(v, threshold float64) bool

func operatorFuncEqual(v, threshold float64) bool {
	return v == threshold
}

func operatorFuncNotEqual(v, threshold float64) bool {
	return v != threshold
}

func operatorFuncAbove(v, threshold float64) bool {
	return v > threshold
}

func operatorFuncAboveEqual(v, threshold float64) bool {
	return v >= threshold
}

func operatorFuncBelow(v, threshold float64) bool {
	return v < threshold
}

func operatorFuncBelowEqual(v, threshold float64) bool {
	return v <= threshold
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.grep
func transformGrep(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("expecting two args; got %d args", len(args))
	}
	pattern, err := getRegexp(args, "pattern", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		if !pattern.MatchString(s.Name) {
			return nil, nil
		}
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.group
func transformGroup(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return groupSeriesLists(ec, fe.Args, fe)
}

func groupSeriesLists(ec *evalConfig, args []*graphiteql.ArgExpr, expr graphiteql.Expr) (nextSeriesFunc, error) {
	var nextSeriess []nextSeriesFunc
	for i := 0; i < len(args); i++ {
		nextSeries, err := evalSeriesList(ec, args, "seriesList", i)
		if err != nil {
			for _, f := range nextSeriess {
				_, _ = drainAllSeries(f)
			}
			return nil, err
		}
		nextSeriess = append(nextSeriess, nextSeries)
	}
	return nextSeriesGroup(nextSeriess, expr), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.groupByNode
func transformGroupByNode(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2 or 3", len(args))
	}
	nodes, err := getNodes(args[1:2])
	if err != nil {
		return nil, err
	}
	callback, err := getOptionalString(args, "callback", 2, "average")
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return groupByNodesGeneric(ec, fe, nextSeries, nodes, callback)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.groupByNodes
func transformGroupByNodes(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want at least 2", len(args))
	}
	callback, err := getString(args, "callback", 1)
	if err != nil {
		return nil, err
	}
	nodes, err := getNodes(args[2:])
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return groupByNodesGeneric(ec, fe, nextSeries, nodes, callback)
}

func groupByNodesGeneric(ec *evalConfig, expr graphiteql.Expr, nextSeries nextSeriesFunc, nodes []graphiteql.Expr, callback string) (nextSeriesFunc, error) {
	keyFunc := func(name string, tags map[string]string) string {
		return getNameFromNodes(name, tags, nodes)
	}
	return groupByKeyFunc(ec, expr, nextSeries, callback, keyFunc)
}

func groupByKeyFunc(ec *evalConfig, expr graphiteql.Expr, nextSeries nextSeriesFunc, aggrFuncName string,
	keyFunc func(name string, tags map[string]string) string) (nextSeriesFunc, error) {
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, err
	}
	nextSeriesWrapper := getNextSeriesWrapperForAggregateFunc(aggrFuncName)
	type x struct {
		as                aggrState
		tags              map[string]string
		seriesExpressions []string
	}
	m := make(map[string]*x)
	var mLock sync.Mutex
	f := nextSeriesWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidate(ec, step)
		key := keyFunc(s.Name, s.Tags)
		mLock.Lock()
		defer mLock.Unlock()
		e := m[key]
		if e == nil {
			as, err := newAggrState(ec.pointsLen(step), aggrFuncName)
			if err != nil {
				return nil, err
			}
			e = &x{
				as:   as,
				tags: s.Tags,
			}
			m[key] = e
		} else {
			for k, v := range e.tags {
				if v != s.Tags[k] {
					delete(e.tags, k)
				}
			}
		}
		e.as.Update(s.Values)
		e.seriesExpressions = append(e.seriesExpressions, s.pathExpression)
		return s, nil
	})
	if _, err := drainAllSeries(f); err != nil {
		return nil, err
	}
	var ss []*series
	for key, e := range m {
		tags := e.tags
		if tags["name"] == "" {
			funcName := strings.TrimSuffix(aggrFuncName, "Series")
			tags["name"] = fmt.Sprintf("%sSeries(%s)", funcName, formatPathsFromSeriesExpressions(e.seriesExpressions, true))
		}
		tags["aggregatedBy"] = aggrFuncName
		s := &series{
			Name:           key,
			Tags:           tags,
			Timestamps:     ec.newTimestamps(step),
			Values:         e.as.Finalize(ec.xFilesFactor),
			expr:           expr,
			pathExpression: tags["name"],
			step:           step,
		}
		ss = append(ss, s)
	}
	return multiSeriesFunc(ss), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.groupByTags
func transformGroupByTags(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want at least 2", len(args))
	}
	callback, err := getString(args, "callback", 1)
	if err != nil {
		return nil, err
	}
	tagKeys := make(map[string]struct{})
	for _, arg := range args[2:] {
		se, ok := arg.Expr.(*graphiteql.StringExpr)
		if !ok {
			return nil, fmt.Errorf("unexpected tag type: %T; expecting string", arg.Expr)
		}
		tagKeys[se.S] = struct{}{}
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	keyFunc := func(_ string, tags map[string]string) string {
		return formatKeyFromTags(tags, tagKeys, callback)
	}
	return groupByKeyFunc(ec, fe, nextSeries, callback, keyFunc)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.highest
func transformHighest(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want from 1 to 3", len(args))
	}
	n, err := getOptionalNumber(args, "n", 1, 1)
	if err != nil {
		return nil, err
	}
	funcName, err := getOptionalString(args, "func", 2, "average")
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return highestGeneric(fe, nextSeries, n, funcName)
}

func highestGeneric(expr graphiteql.Expr, nextSeries nextSeriesFunc, n float64, funcName string) (nextSeriesFunc, error) {
	aggrFunc, err := getAggrFunc(funcName)
	if err != nil {
		_, _ = drainAllSeries(nextSeries)
		return nil, err
	}
	nextSeriesWrapper := getNextSeriesWrapperForAggregateFunc(funcName)
	var topSeries maxSeriesHeap
	var topSeriesLock sync.Mutex
	f := nextSeriesWrapper(nextSeries, func(s *series) (*series, error) {
		v := aggrFunc(s.Values)
		topSeriesLock.Lock()
		defer topSeriesLock.Unlock()
		if len(topSeries) < int(n) {
			heap.Push(&topSeries, &seriesWithWeight{
				v: v,
				s: s,
			})
		} else if v > topSeries[0].v {
			topSeries[0] = &seriesWithWeight{
				v: v,
				s: s,
			}
			heap.Fix(&topSeries, 0)
		}
		return s, nil
	})
	if _, err := drainAllSeries(f); err != nil {
		return nil, err
	}
	sort.Slice(topSeries, func(i, j int) bool {
		return topSeries[i].v < topSeries[j].v
	})
	var ss []*series
	for _, x := range topSeries {
		s := x.s
		s.expr = expr
		ss = append(ss, s)
	}
	return multiSeriesFunc(ss), nil
}

type seriesWithWeight struct {
	v float64
	s *series
}

type minSeriesHeap []*seriesWithWeight

func (h *minSeriesHeap) Len() int { return len(*h) }
func (h *minSeriesHeap) Less(i, j int) bool {
	a := *h
	return a[i].v > a[j].v
}
func (h *minSeriesHeap) Swap(i, j int) {
	a := *h
	a[i], a[j] = a[j], a[i]
}
func (h *minSeriesHeap) Push(x interface{}) {
	*h = append(*h, x.(*seriesWithWeight))
}
func (h *minSeriesHeap) Pop() interface{} {
	a := *h
	x := a[len(a)-1]
	*h = a[:len(a)-1]
	return x
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.highestAverage
func transformHighestAverage(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return highestGeneric(fe, nextSeries, n, "average")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.highestCurrent
func transformHighestCurrent(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return highestGeneric(fe, nextSeries, n, "current")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.highestMax
func transformHighestMax(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return highestGeneric(fe, nextSeries, n, "max")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.hitcount
func transformHitcount(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2 or 3", len(args))
	}
	intervalString, err := getString(args, "intervalString", 1)
	if err != nil {
		return nil, err
	}
	interval, err := parseInterval(intervalString)
	if err != nil {
		return nil, err
	}
	if interval <= 0 {
		return nil, fmt.Errorf("interval must be positive; got %dms", interval)
	}
	alignToInterval, err := getOptionalBool(args, "alignToInterval", 2, false)
	if err != nil {
		return nil, err
	}
	ecCopy := *ec
	if alignToInterval {
		startTime := ecCopy.startTime
		tz := ecCopy.currentTime.Location()
		t := time.Unix(startTime/1e3, (startTime%1000)*1e6).In(tz)
		if interval >= 24*3600*1000 {
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz)
		} else if interval >= 3600*1000 {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, tz)
		} else if interval >= 60*1000 {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, tz)
		}
		ecCopy.startTime = t.UnixNano() / 1e6
	}
	nextSeries, err := evalSeriesList(&ecCopy, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		ts := ecCopy.startTime
		timestamps := s.Timestamps
		values := s.Values
		var dstTimestamps []int64
		var dstValues []float64
		i := 0
		vPrev := float64(0)
		for ts < ecCopy.endTime {
			tsPrev := ts
			hitcount := float64(0)
			if i < len(timestamps) && !math.IsNaN(vPrev) {
				hitcount = vPrev * float64(timestamps[i]-tsPrev) / 1000
			}
			tsEnd := ts + interval
			for i < len(timestamps) {
				tsCurr := timestamps[i]
				if tsCurr >= tsEnd {
					break
				}
				v := values[i]
				if !math.IsNaN(v) {
					hitcount += v * (float64(tsCurr-tsPrev) / 1000)
				}
				tsPrev = tsCurr
				vPrev = v
				i++
			}
			if hitcount == 0 {
				hitcount = nan
			}
			dstValues = append(dstValues, hitcount)
			dstTimestamps = append(dstTimestamps, ts)
			ts = tsEnd
		}
		s.Timestamps = dstTimestamps
		s.Values = dstValues
		s.Tags["hitcount"] = intervalString
		if alignToInterval {
			s.Name = fmt.Sprintf("hitcount(%s,%s,true)", s.Name, graphiteql.QuoteString(intervalString))
		} else {
			s.Name = fmt.Sprintf("hitcount(%s,%s)", s.Name, graphiteql.QuoteString(intervalString))
		}
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.identity
func transformIdentity(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	name, err := getString(args, "name", 0)
	if err != nil {
		return nil, err
	}
	const step = 60e3
	var dstValues []float64
	var dstTimestamps []int64
	ts := ec.startTime
	for ts < ec.endTime {
		dstValues = append(dstValues, float64(ts)/1000)
		dstTimestamps = append(dstTimestamps, ts)
		ts += step
	}
	s := &series{
		Name:           name,
		Tags:           unmarshalTags(name),
		Timestamps:     dstTimestamps,
		Values:         dstValues,
		expr:           fe,
		pathExpression: name,
		step:           step,
	}
	return singleSeriesFunc(s), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.integral
func transformIntegral(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		sum := float64(0)
		for i, v := range values {
			if math.IsNaN(v) {
				continue
			}
			sum += v
			values[i] = sum
		}
		s.Tags["integral"] = "1"
		s.Name = fmt.Sprintf("integral(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.integralByInterval
func transformIntegralByInterval(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	intervalUnit, err := getString(args, "intervalUnit", 1)
	if err != nil {
		return nil, err
	}
	interval, err := parseInterval(intervalUnit)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		timestamps := s.Timestamps
		sum := float64(0)
		dtPrev := int64(0)
		for i, v := range values {
			if math.IsNaN(v) {
				continue
			}
			dt := timestamps[i] / interval
			if dt != dtPrev {
				sum = 0
				dtPrev = dt
			}
			sum += v
			values[i] = sum
		}
		s.Tags["integralByInterval"] = "1"
		s.Name = fmt.Sprintf("integralByInterval(%s,%s)", s.Name, graphiteql.QuoteString(intervalUnit))
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.interpolate
func transformInterpolate(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1 or 2", len(args))
	}
	limit, err := getOptionalNumber(args, "limit", 1, math.Inf(1))
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		nansCount := float64(0)
		prevValue := nan
		for i, v := range values {
			if math.IsNaN(v) {
				nansCount++
				continue
			}
			if nansCount > 0 && nansCount <= limit {
				delta := (v - prevValue) / (nansCount + 1)
				for j := i - int(nansCount); j < i; j++ {
					prevValue += delta
					values[j] = prevValue
				}
			}
			nansCount = 0
			prevValue = v
		}
		s.Name = fmt.Sprintf("interpolate(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.invert
func transformInvert(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			values[i] = 1 / v
		}
		s.Tags["invert"] = "1"
		s.Name = fmt.Sprintf("invert(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.keepLastValue
func transformKeepLastValue(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1 or 2", len(args))
	}
	limit, err := getOptionalNumber(args, "limit", 1, math.Inf(1))
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "serieslList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		nansCount := float64(0)
		prevValue := nan
		for i, v := range values {
			if !math.IsNaN(v) {
				nansCount = 0
				prevValue = v
				continue
			}
			nansCount++
			if nansCount <= limit {
				values[i] = prevValue
			}
		}
		s.Name = fmt.Sprintf("keepLastValue(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.limit
func transformLimit(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	seriesFetched := 0
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		if seriesFetched >= int(n) {
			return nil, nil
		}
		seriesFetched++
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.lineWidth
func transformLineWidth(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	_, err := getNumber(args, "width", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.logarithm
func transformLogarithm(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1 or 2", len(args))
	}
	base, err := getOptionalNumber(args, "base", 1, 10)
	if err != nil {
		return nil, err
	}
	baseStr := fmt.Sprintf("%g", base)
	baseLog := math.Log(base)
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			values[i] = math.Log(v) / baseLog
		}
		s.Tags["log"] = baseStr
		s.Name = fmt.Sprintf("log(%s,%s)", s.Name, baseStr)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.logit
func transformLogit(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			values[i] = math.Log(v / (1 - v))
		}
		s.Tags["logit"] = "logit"
		s.Name = fmt.Sprintf("logit(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.lowest
func transformLowest(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want from 1 to 3", len(args))
	}
	n, err := getOptionalNumber(args, "n", 1, 1)
	if err != nil {
		return nil, err
	}
	funcName, err := getOptionalString(args, "func", 2, "average")
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return lowestGeneric(fe, nextSeries, n, funcName)
}

func lowestGeneric(expr graphiteql.Expr, nextSeries nextSeriesFunc, n float64, funcName string) (nextSeriesFunc, error) {
	aggrFunc, err := getAggrFunc(funcName)
	if err != nil {
		_, _ = drainAllSeries(nextSeries)
		return nil, err
	}
	nextSeriesWrapper := getNextSeriesWrapperForAggregateFunc(funcName)
	var minSeries minSeriesHeap
	var minSeriesLock sync.Mutex
	f := nextSeriesWrapper(nextSeries, func(s *series) (*series, error) {
		v := aggrFunc(s.Values)
		minSeriesLock.Lock()
		defer minSeriesLock.Unlock()
		if len(minSeries) < int(n) {
			heap.Push(&minSeries, &seriesWithWeight{
				v: v,
				s: s,
			})
		} else if v < minSeries[0].v {
			minSeries[0] = &seriesWithWeight{
				v: v,
				s: s,
			}
			heap.Fix(&minSeries, 0)
		}
		return s, nil
	})
	if _, err := drainAllSeries(f); err != nil {
		return nil, err
	}
	sort.Slice(minSeries, func(i, j int) bool {
		return minSeries[i].v > minSeries[j].v
	})
	var ss []*series
	for _, x := range minSeries {
		s := x.s
		s.expr = expr
		ss = append(ss, s)
	}
	return multiSeriesFunc(ss), nil
}

type maxSeriesHeap []*seriesWithWeight

func (h *maxSeriesHeap) Len() int { return len(*h) }
func (h *maxSeriesHeap) Less(i, j int) bool {
	a := *h
	return a[i].v < a[j].v
}
func (h *maxSeriesHeap) Swap(i, j int) {
	a := *h
	a[i], a[j] = a[j], a[i]
}
func (h *maxSeriesHeap) Push(x interface{}) {
	*h = append(*h, x.(*seriesWithWeight))
}
func (h *maxSeriesHeap) Pop() interface{} {
	a := *h
	x := a[len(a)-1]
	*h = a[:len(a)-1]
	return x
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.lowestAverage
func transformLowestAverage(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return lowestGeneric(fe, nextSeries, n, "average")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.lowestCurrent
func transformLowestCurrent(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return lowestGeneric(fe, nextSeries, n, "current")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.maxSeries
func transformMaxSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesGeneric(ec, fe, "max")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.maximumAbove
func transformMaximumAbove(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return filterSeriesGeneric(fe, nextSeries, "max", ">", n)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.maximumBelow
func transformMaximumBelow(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return filterSeriesGeneric(fe, nextSeries, "max", "<", n)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.minMax
func transformMinMax(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		min := aggrMin(values)
		if math.IsNaN(min) {
			min = 0
		}
		max := aggrMax(values)
		if math.IsNaN(max) {
			max = 0
		}
		vRange := max - min
		for i, v := range values {
			v = (v - min) / vRange
			if math.IsInf(v, 0) {
				v = 0
			}
			values[i] = v
		}
		s.Name = fmt.Sprintf("minMax(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.minSeries
func transformMinSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesGeneric(ec, fe, "min")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.minimumAbove
func transformMinimumAbove(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return filterSeriesGeneric(fe, nextSeries, "min", ">", n)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.minimumBelow
func transformMinimumBelow(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return filterSeriesGeneric(fe, nextSeries, "min", "<", n)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.mostDeviant
func transformMostDeviant(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return highestGeneric(fe, nextSeries, n, "stddev")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.movingAverage
func transformMovingAverage(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return movingWindowGeneric(ec, fe, "average")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.movingMax
func transformMovingMax(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return movingWindowGeneric(ec, fe, "max")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.movingMedian
func transformMovingMedian(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return movingWindowGeneric(ec, fe, "median")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.movingMin
func transformMovingMin(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return movingWindowGeneric(ec, fe, "min")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.movingSum
func transformMovingSum(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return movingWindowGeneric(ec, fe, "sum")
}

func movingWindowGeneric(ec *evalConfig, fe *graphiteql.FuncExpr, funcName string) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2 or 3", len(args))
	}
	windowSizeArg, err := getArg(args, "windowSize", 1)
	if err != nil {
		return nil, err
	}
	xFilesFactor, err := getOptionalNumber(args, "xFilesFactor", 2, ec.xFilesFactor)
	if err != nil {
		return nil, err
	}
	seriesListArg, err := getArg(args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return movingWindow(ec, fe, seriesListArg, windowSizeArg, funcName, xFilesFactor)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.movingWindow
func transformMovingWindow(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 || len(args) > 4 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want from 2 to 4", len(args))
	}
	windowSizeArg, err := getArg(args, "windowSize", 1)
	if err != nil {
		return nil, err
	}
	funcName, err := getOptionalString(args, "func", 2, "avg")
	if err != nil {
		return nil, err
	}
	xFilesFactor, err := getOptionalNumber(args, "xFilesFactor", 3, ec.xFilesFactor)
	if err != nil {
		return nil, err
	}
	seriesListArg, err := getArg(args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return movingWindow(ec, fe, seriesListArg, windowSizeArg, funcName, xFilesFactor)
}

func movingWindow(ec *evalConfig, fe *graphiteql.FuncExpr, seriesListArg, windowSizeArg *graphiteql.ArgExpr, funcName string, xFilesFactor float64) (nextSeriesFunc, error) {
	windowSize, stepsCount, err := getWindowSize(ec, windowSizeArg)
	if err != nil {
		return nil, err
	}
	windowSizeStr := string(windowSizeArg.Expr.AppendString(nil))
	aggrFunc, err := getAggrFunc(funcName)
	if err != nil {
		return nil, err
	}
	ecCopy := *ec
	ecCopy.startTime -= windowSize
	nextSeries, err := evalExpr(&ecCopy, seriesListArg.Expr)
	if err != nil {
		return nil, err
	}
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, err
	}
	if stepsCount > 0 && step != ec.storageStep {
		// The inner function call changes the step and the moving* function refers to it.
		// Adjust the startTime and re-calculate the inner function on the adjusted time range.
		if _, err := drainAllSeries(nextSeries); err != nil {
			return nil, err
		}
		windowSize = int64(stepsCount * float64(step))
		ecCopy = *ec
		ecCopy.startTime -= windowSize
		nextSeries, err = evalExpr(&ecCopy, seriesListArg.Expr)
		if err != nil {
			return nil, err
		}
	}
	tagName := "moving" + strings.Title(funcName)
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		timestamps := s.Timestamps
		values := s.Values
		var dstTimestamps []int64
		var dstValues []float64
		tsEnd := ecCopy.startTime + windowSize
		i := 0
		j := 0
		for tsEnd <= ecCopy.endTime {
			tsStart := tsEnd - windowSize
			for i < len(timestamps) && timestamps[i] < tsStart {
				i++
			}
			if i > j {
				j = i
			}
			for j < len(timestamps) && timestamps[j] < tsEnd {
				j++
			}
			v := aggrFunc.apply(xFilesFactor, values[i:j])
			dstTimestamps = append(dstTimestamps, tsEnd)
			dstValues = append(dstValues, v)
			tsEnd += step
		}
		s.Timestamps = dstTimestamps
		s.Values = dstValues
		s.Tags[tagName] = windowSizeStr
		s.Name = fmt.Sprintf("%s(%s,%s)", tagName, s.Name, windowSizeStr)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.multiplySeries
func transformMultiplySeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesGeneric(ec, fe, "multiply")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.multiplySeriesWithWildcards
func transformMultiplySeriesWithWildcards(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesWithWildcardsGeneric(ec, fe, "multiply")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.percentileOfSeries
func transformPercentileOfSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2 or 3", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	// TODO: properly use interpolate
	if _, err := getOptionalBool(args, "interpolate", 2, false); err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, err
	}
	as := newAggrStatePercentile(ec.pointsLen(step), n)
	var lock sync.Mutex
	var seriesExpressions []string
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidate(ec, step)
		lock.Lock()
		as.Update(s.Values)

		seriesExpressions = append(seriesExpressions, s.pathExpression)
		lock.Unlock()
		return s, nil
	})
	if _, err := drainAllSeries(f); err != nil {
		return nil, err
	}
	if len(seriesExpressions) == 0 {
		return multiSeriesFunc(nil), nil
	}
	// peek first expr as graphite does.
	sort.Strings(seriesExpressions)
	name := fmt.Sprintf("percentileOfSeries(%s,%g)", seriesExpressions[0], n)
	s := &series{
		Name:           name,
		Tags:           map[string]string{"name": name},
		Timestamps:     ec.newTimestamps(step),
		Values:         as.Finalize(ec.xFilesFactor),
		expr:           fe,
		pathExpression: name,
		step:           step,
	}
	return singleSeriesFunc(s), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.pow
func transformPow(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	factor, err := getNumber(args, "factor", 1)
	if err != nil {
		return nil, err
	}
	factorStr := fmt.Sprintf("%g", factor)
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			values[i] = math.Pow(v, factor)
		}
		s.Tags["pow"] = factorStr
		s.Name = fmt.Sprintf("pow(%s,%s)", s.Name, factorStr)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.powSeries
func transformPowSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesGeneric(ec, fe, "pow")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.randomWalk
func transformRandomWalk(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1 or 2", len(args))
	}
	name, err := getString(args, "name", 0)
	if err != nil {
		return nil, err
	}
	step, err := getOptionalNumber(args, "step", 1, 60)
	if err != nil {
		return nil, err
	}
	if step <= 0 {
		return nil, fmt.Errorf("step must be positive; got %g", step)
	}
	stepMsecs := int64(step * 1000)
	var dstValues []float64
	var dstTimestamps []int64
	ts := ec.startTime
	v := float64(0)
	for ts < ec.endTime {
		dstValues = append(dstValues, v)
		dstTimestamps = append(dstTimestamps, ts)
		v += rand.Float64() - 0.5
		ts += stepMsecs
	}
	s := &series{
		Name:           name,
		Tags:           unmarshalTags(name),
		Timestamps:     dstTimestamps,
		Values:         dstValues,
		expr:           fe,
		pathExpression: name,
		step:           stepMsecs,
	}
	return singleSeriesFunc(s), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.rangeOfSeries
func transformRangeOfSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesGeneric(ec, fe, "rangeOf")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.removeAbovePercentile
func transformRemoveAbovePercentile(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	aggrFunc := newAggrFuncPercentile(n)
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		max := aggrFunc(values)
		for i, v := range values {
			if v > max {
				values[i] = nan
			}
		}
		s.Name = fmt.Sprintf("removeAbovePercentile(%s,%g)", s.Name, n)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.removeAboveValue
func transformRemoveAboveValue(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			if v > n {
				values[i] = nan
			}
		}
		s.Name = fmt.Sprintf("removeAboveValue(%s,%g)", s.Name, n)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.removeBelowPercentile
func transformRemoveBelowPercentile(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	aggrFunc := newAggrFuncPercentile(n)
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		min := aggrFunc(values)
		for i, v := range values {
			if v < min {
				values[i] = nan
			}
		}
		s.Name = fmt.Sprintf("removeBelowPercentile(%s,%g)", s.Name, n)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.removeBelowValue
func transformRemoveBelowValue(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			if v < n {
				values[i] = nan
			}
		}
		s.Name = fmt.Sprintf("removeBelowValue(%s,%g)", s.Name, n)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.removeBetweenPercentile
func transformRemoveBetweenPercentile(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	if n > 50 {
		n = 100 - n
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, err
	}
	var ss []*series
	asLow := newAggrStatePercentile(ec.pointsLen(step), n)
	asHigh := newAggrStatePercentile(ec.pointsLen(step), 100-n)
	var lock sync.Mutex
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidate(ec, step)
		lock.Lock()
		asLow.Update(s.Values)
		asHigh.Update(s.Values)
		ss = append(ss, s)
		lock.Unlock()
		return s, nil
	})
	if _, err := drainAllSeries(f); err != nil {
		return nil, err
	}
	lows := asLow.Finalize(ec.xFilesFactor)
	highs := asHigh.Finalize(ec.xFilesFactor)
	var ssDst []*series
	for _, s := range ss {
		values := s.Values
		for i, v := range values {
			if v < lows[i] || v > highs[i] {
				s.expr = fe
				ssDst = append(ssDst, s)
				break
			}
		}
	}
	return multiSeriesFunc(ssDst), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.removeEmptySeries
func transformRemoveEmptySeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1 or 2", len(args))
	}
	xFilesFactor, err := getOptionalNumber(args, "xFilesFactor", 1, ec.xFilesFactor)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		xff := s.xFilesFactor
		if xff == 0 {
			xff = xFilesFactor
		}
		n := aggrCount(s.Values)
		if n/float64(len(s.Values)) < xff {
			return nil, nil
		}
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.roundFunction
func transformRoundFunction(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1 or 2", len(args))
	}
	precision, err := getOptionalNumber(args, "precision", 1, 0)
	if err != nil {
		return nil, err
	}
	precisionProduct := math.Pow10(int(precision))
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			values[i] = math.Round(v*precisionProduct) / precisionProduct
		}
		if precision == 0 {
			s.Name = fmt.Sprintf("round(%s)", s.Name)
		} else {
			s.Name = fmt.Sprintf("round(%s,%g)", s.Name, precision)
		}
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.scale
func transformScale(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	factor, err := getNumber(args, "factor", 1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			values[i] = v * factor
		}
		s.Name = fmt.Sprintf("scale(%s,%g)", s.Name, factor)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.seriesByTag
func transformSeriesByTag(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) == 0 {
		return nil, fmt.Errorf("at least one tagExpression must be passed to seriesByTag")
	}
	var tagExpressions []string
	for i := 0; i < len(args); i++ {
		te, err := getString(args, "tagExpressions", i)
		if err != nil {
			return nil, err
		}
		tagExpressions = append(tagExpressions, te)
	}
	sq, err := getSearchQueryForExprs(ec.currentTime, ec.etfs, tagExpressions, *maxGraphiteSeries)
	if err != nil {
		return nil, err
	}
	sq.MinTimestamp = ec.startTime
	sq.MaxTimestamp = ec.endTime
	return newNextSeriesForSearchQuery(ec, sq, fe)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.setXFilesFactor
func transformSetXFilesFactor(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	xFilesFactor, err := getNumber(args, "xFilesFactor", 1)
	if err != nil {
		return nil, err
	}
	ecCopy := *ec
	ecCopy.xFilesFactor = xFilesFactor
	nextSeries, err := evalSeriesList(&ecCopy, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	xFilesFactorStr := fmt.Sprintf("%g", xFilesFactor)
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		s.xFilesFactor = xFilesFactor
		s.Tags["xFilesFactor"] = xFilesFactorStr
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.sumSeriesWithWildcards
func transformSumSeriesWithWildcards(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesWithWildcardsGeneric(ec, fe, "sum")
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.summarize
func transformSummarize(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 || len(args) > 4 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want from 2 to 4", len(args))
	}
	intervalString, err := getString(args, "intervalString", 1)
	if err != nil {
		return nil, err
	}
	interval, err := parseInterval(intervalString)
	if err != nil {
		return nil, fmt.Errorf("cannot parse intervalString: %w", err)
	}
	if interval <= 0 {
		return nil, fmt.Errorf("interval must be positive; got %dms", interval)
	}
	funcName, err := getOptionalString(args, "func", 2, "sum")
	if err != nil {
		return nil, err
	}
	aggrFunc, err := getAggrFunc(funcName)
	if err != nil {
		return nil, err
	}
	alignToFrom, err := getOptionalBool(args, "alignToFrom", 3, false)
	if err != nil {
		return nil, err
	}
	ecCopy := *ec
	if !alignToFrom {
		ecCopy.startTime -= ecCopy.startTime % interval
		ecCopy.endTime += interval - ecCopy.endTime%interval
	}
	nextSeries, err := evalSeriesList(&ecCopy, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.summarize(aggrFunc, ecCopy.startTime, ecCopy.endTime, interval, s.xFilesFactor)
		s.Tags["summarize"] = intervalString
		s.Tags["summarizeFunction"] = funcName
		if alignToFrom {
			s.Name = fmt.Sprintf("summarize(%s,%s,%s,true)", s.Name, graphiteql.QuoteString(intervalString), graphiteql.QuoteString(funcName))
		} else {
			s.Name = fmt.Sprintf("summarize(%s,%s,%s)", s.Name, graphiteql.QuoteString(intervalString), graphiteql.QuoteString(funcName))
		}
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.weightedAverage
func transformWeightedAverage(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2 at least", len(args))
	}
	nodes, err := getNodes(args[2:])
	if err != nil {
		return nil, err
	}
	avgSeries, err := evalSeriesList(ec, args, "seriesListAvg", 0)
	if err != nil {
		return nil, err
	}
	ss, stepAvg, err := fetchNormalizedSeries(ec, avgSeries, false)
	if err != nil {
		return nil, err
	}
	weightSeries, err := evalSeriesList(ec, args, "seriesListWeight", 1)
	if err != nil {
		return nil, err
	}
	ssWeight, stepWeight, err := fetchNormalizedSeries(ec, weightSeries, false)
	if err != nil {
		return nil, err
	}
	if len(ss) != len(ssWeight) {
		return nil, fmt.Errorf("series len mismatch, got seriesListAvg: %d,seriesListWeight: %d ", len(ss), len(ssWeight))
	}
	if stepAvg != stepWeight {
		return nil, fmt.Errorf("step mismatch for seriesListAvg and seriesListWeight: %d vs %d", stepAvg, stepWeight)
	}
	mAvg := groupSeriesByNodes(ss, nodes)
	mWeight := groupSeriesByNodes(ssWeight, nodes)
	var ssProduct []*series
	for k, ss := range mAvg {
		wss := mWeight[k]
		if len(wss) == 0 {
			continue
		}
		s := ss[len(ss)-1]
		ws := wss[len(wss)-1]
		values := s.Values
		valuesWeight := ws.Values
		for i, v := range values {
			values[i] = v * valuesWeight[i]
		}
		ssProduct = append(ssProduct, s)
	}
	if len(ssProduct) == 0 {
		return multiSeriesFunc(nil), nil
	}

	step := stepAvg
	as := newAggrStateSum(ec.pointsLen(step))
	for _, s := range ssProduct {
		as.Update(s.Values)
	}
	values := as.Finalize(ec.xFilesFactor)

	asWeight := newAggrStateSum(ec.pointsLen(step))
	for _, s := range ssWeight {
		asWeight.Update(s.Values)
	}
	valuesWeight := asWeight.Finalize(ec.xFilesFactor)

	for i, v := range values {
		values[i] = v / valuesWeight[i]
	}

	var nodesStr []string
	for _, node := range nodes {
		nodesStr = append(nodesStr, string(node.AppendString(nil)))
	}
	name := fmt.Sprintf("weightedAverage(%s,%s,%s)",
		formatPathsFromSeries(ss),
		formatPathsFromSeries(ssWeight),
		strings.Join(nodesStr, ","),
	)
	sResult := &series{
		Name:           name,
		Tags:           map[string]string{"name": name},
		Timestamps:     ec.newTimestamps(step),
		Values:         values,
		expr:           fe,
		pathExpression: name,
		step:           step,
	}
	return singleSeriesFunc(sResult), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.timeFunction
func transformTimeFunction(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting 1 or 2 args", len(args))
	}
	name, err := getString(args, "name", 0)
	if err != nil {
		return nil, err
	}
	step, err := getOptionalNumber(args, "step", 1, 60)
	if err != nil {
		return nil, err
	}
	stepMsecs := int64(step * 1000)
	var values []float64
	var timestamps []int64
	ts := ec.startTime
	for ts <= ec.endTime {
		timestamps = append(timestamps, ts)
		values = append(values, float64(ts/1000))
		ts += stepMsecs
	}
	s := &series{
		Name:           name,
		Tags:           unmarshalTags(name),
		Timestamps:     timestamps,
		Values:         values,
		expr:           fe,
		pathExpression: name,
		step:           stepMsecs,
	}
	return singleSeriesFunc(s), nil
}

func getWindowSize(ec *evalConfig, windowSizeArg *graphiteql.ArgExpr) (windowSize int64, stepsCount float64, err error) {
	switch t := windowSizeArg.Expr.(type) {
	case *graphiteql.NumberExpr:
		stepsCount = t.N
		windowSize = int64(t.N * float64(ec.storageStep))
	case *graphiteql.StringExpr:
		ws, err := parseInterval(t.S)
		if err != nil {
			return 0, 0, fmt.Errorf("cannot parse windowSize: %w", err)
		}
		windowSize = ws
	default:
		return 0, 0, fmt.Errorf("unexpected type for windowSize arg: %T; expecting number or string", windowSizeArg.Expr)
	}
	if windowSize <= 0 {
		return 0, 0, fmt.Errorf("windowSize must be positive; got %dms", windowSize)
	}
	return windowSize, stepsCount, nil
}

func getArg(args []*graphiteql.ArgExpr, name string, index int) (*graphiteql.ArgExpr, error) {
	for _, arg := range args {
		if arg.Name == name {
			return arg, nil
		}
	}
	if index >= len(args) {
		return nil, fmt.Errorf("missing arg %q at position %d", name, index)
	}
	arg := args[index]
	if arg.Name != "" {
		return nil, fmt.Errorf("unexpected named arg at position %d: %q", index, arg.Name)
	}
	return arg, nil
}

func getOptionalArg(args []*graphiteql.ArgExpr, name string, index int) *graphiteql.ArgExpr {
	for _, arg := range args {
		if arg.Name == name {
			return arg
		}
	}
	if index >= len(args) {
		return nil
	}
	arg := args[index]
	if arg.Name != "" {
		return nil
	}
	return arg
}

func evalSeriesList(ec *evalConfig, args []*graphiteql.ArgExpr, name string, index int) (nextSeriesFunc, error) {
	arg, err := getArg(args, name, index)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalExpr(ec, arg.Expr)
	if err != nil {
		return nil, fmt.Errorf("cannot evaluate arg %q at position %d: %w", name, index, err)
	}
	return nextSeries, nil
}

func getInts(args []*graphiteql.ArgExpr, name string) ([]int, error) {
	var ns []int
	for i := range args {
		n, err := getNumber(args, name, i)
		if err != nil {
			return nil, err
		}
		ns = append(ns, int(n))
	}
	return ns, nil
}

func getNumber(args []*graphiteql.ArgExpr, name string, index int) (float64, error) {
	arg, err := getArg(args, name, index)
	if err != nil {
		return 0, err
	}
	ne, ok := arg.Expr.(*graphiteql.NumberExpr)
	if !ok {
		return 0, fmt.Errorf("arg %q at position %d must be a number; got %T", name, index, arg.Expr)
	}
	return ne.N, nil
}

func getOptionalNumber(args []*graphiteql.ArgExpr, name string, index int, defaultValue float64) (float64, error) {
	arg := getOptionalArg(args, name, index)
	if arg == nil {
		return defaultValue, nil
	}
	if _, ok := arg.Expr.(*graphiteql.NoneExpr); ok {
		return defaultValue, nil
	}
	ne, ok := arg.Expr.(*graphiteql.NumberExpr)
	if !ok {
		return 0, fmt.Errorf("arg %q at position %d must be a number; got %T", name, index, arg.Expr)
	}
	return ne.N, nil
}

func getString(args []*graphiteql.ArgExpr, name string, index int) (string, error) {
	arg, err := getArg(args, name, index)
	if err != nil {
		return "", err
	}
	se, ok := arg.Expr.(*graphiteql.StringExpr)
	if !ok {
		return "", fmt.Errorf("arg %q at position %d must be a string; got %T", name, index, arg.Expr)
	}
	return se.S, nil
}

func getOptionalString(args []*graphiteql.ArgExpr, name string, index int, defaultValue string) (string, error) {
	arg := getOptionalArg(args, name, index)
	if arg == nil {
		return defaultValue, nil
	}
	if _, ok := arg.Expr.(*graphiteql.NoneExpr); ok {
		return defaultValue, nil
	}
	se, ok := arg.Expr.(*graphiteql.StringExpr)
	if !ok {
		return "", fmt.Errorf("arg %q at position %d must be a string; got %T", name, index, arg.Expr)
	}
	return se.S, nil
}

func getOptionalBool(args []*graphiteql.ArgExpr, name string, index int, defaultValue bool) (bool, error) {
	arg := getOptionalArg(args, name, index)
	if arg == nil {
		return defaultValue, nil
	}
	if _, ok := arg.Expr.(*graphiteql.NoneExpr); ok {
		return defaultValue, nil
	}
	be, ok := arg.Expr.(*graphiteql.BoolExpr)
	if !ok {
		return false, fmt.Errorf("arg %q at position %d must be a bool; got %T", name, index, arg.Expr)
	}
	return be.B, nil
}

func getRegexp(args []*graphiteql.ArgExpr, name string, index int) (*regexp.Regexp, error) {
	search, err := getString(args, name, index)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(search)
	if err != nil {
		return nil, fmt.Errorf("cannot compile search regexp %q: %w", search, err)
	}
	return re, nil
}

func getRegexpReplacement(args []*graphiteql.ArgExpr, name string, index int) (string, error) {
	replace, err := getString(args, name, index)
	if err != nil {
		return "", err
	}
	return graphiteToGolangRegexpReplace(replace), nil
}

func graphiteToGolangRegexpReplace(replace string) string {
	return graphiteToGolangRe.ReplaceAllString(replace, "$$${1}")
}

var graphiteToGolangRe = regexp.MustCompile(`\\(\d+)`)

func getNodes(args []*graphiteql.ArgExpr) ([]graphiteql.Expr, error) {
	var nodes []graphiteql.Expr
	for i := 0; i < len(args); i++ {
		expr := args[i].Expr
		switch expr.(type) {
		case *graphiteql.NumberExpr, *graphiteql.StringExpr:
		default:
			return nil, fmt.Errorf("unexpected arg type for `nodes`; got %T; expecting number or string", expr)
		}
		nodes = append(nodes, expr)
	}
	return nodes, nil
}

func fetchNormalizedSeriesByNodes(ec *evalConfig, nextSeries nextSeriesFunc, nodes []graphiteql.Expr) (map[string][]*series, int64, error) {
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, 0, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidate(ec, step)
		return s, nil
	})
	ss, err := fetchAllSeries(f)
	if err != nil {
		return nil, 0, err
	}
	return groupSeriesByNodes(ss, nodes), step, nil
}

func groupSeriesByNodes(ss []*series, nodes []graphiteql.Expr) map[string][]*series {
	m := make(map[string][]*series)
	for _, s := range ss {
		key := getNameFromNodes(s.Name, s.Tags, nodes)
		m[key] = append(m[key], s)
	}
	return m
}

func getAbsoluteNodeIndex(index, size int) int {
	// Handle the negative index case as Python does
	if index < 0 {
		index = size + index
	}
	if index < 0 || index >= size {
		return -1
	}
	return index
}

func getNameFromNodes(name string, tags map[string]string, nodes []graphiteql.Expr) string {
	if len(nodes) == 0 {
		return ""
	}
	path := getPathFromName(name)
	parts := strings.Split(path, ".")
	var dstParts []string
	for _, node := range nodes {
		switch t := node.(type) {
		case *graphiteql.NumberExpr:
			if n := getAbsoluteNodeIndex(int(t.N), len(parts)); n >= 0 {
				dstParts = append(dstParts, parts[n])
			}
		case *graphiteql.StringExpr:
			if v := tags[t.S]; v != "" {
				dstParts = append(dstParts, v)
			}
		}
	}
	return strings.Join(dstParts, ".")
}

func getPathFromName(s string) string {
	expr, err := graphiteql.Parse(s)
	if err != nil {
		return s
	}
	for {
		switch t := expr.(type) {
		case *graphiteql.MetricExpr:
			return t.Query
		case *graphiteql.FuncExpr:
			for _, arg := range t.Args {
				if me, ok := arg.Expr.(*graphiteql.MetricExpr); ok {
					return me.Query
				}
			}
			if len(t.Args) == 0 {
				return s
			}
			expr = t.Args[0].Expr
		case *graphiteql.StringExpr:
			return t.S
		case *graphiteql.NumberExpr:
			return string(t.AppendString(nil))
		case *graphiteql.BoolExpr:
			return strconv.FormatBool(t.B)
		default:
			return s
		}
	}
}

func fetchNormalizedSeries(ec *evalConfig, nextSeries nextSeriesFunc, isConcurrent bool) ([]*series, int64, error) {
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, 0, err
	}
	nextSeriesWrapper := getNextSeriesWrapper(isConcurrent)
	f := nextSeriesWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidate(ec, step)
		return s, nil
	})
	ss, err := fetchAllSeries(f)
	if err != nil {
		return nil, 0, err
	}
	return ss, step, nil
}

func fetchAllSeries(nextSeries nextSeriesFunc) ([]*series, error) {
	var ss []*series
	for {
		s, err := nextSeries()
		if err != nil {
			return nil, err
		}
		if s == nil {
			return ss, nil
		}
		ss = append(ss, s)
	}
}

func drainAllSeries(nextSeries nextSeriesFunc) (int, error) {
	seriesCount := 0
	for {
		s, err := nextSeries()
		if err != nil {
			return seriesCount, err
		}
		if s == nil {
			return seriesCount, nil
		}
		seriesCount++
	}
}

func singleSeriesFunc(s *series) nextSeriesFunc {
	return multiSeriesFunc([]*series{s})
}

func multiSeriesFunc(ss []*series) nextSeriesFunc {
	for _, s := range ss {
		if s == nil {
			panic(fmt.Errorf("BUG: all the series passed to multiSeriesFunc must be non-nil"))
		}
	}
	f := func() (*series, error) {
		if len(ss) == 0 {
			return nil, nil
		}
		s := ss[0]
		ss = ss[1:]
		return s, nil
	}
	return f
}

func nextSeriesGroup(nextSeriess []nextSeriesFunc, expr graphiteql.Expr) nextSeriesFunc {
	f := func() (*series, error) {
		for {
			if len(nextSeriess) == 0 {
				return nil, nil
			}
			nextSeries := nextSeriess[0]
			s, err := nextSeries()
			if err != nil {
				for _, f := range nextSeriess[1:] {
					_, _ = drainAllSeries(f)
				}
				nextSeriess = nil
				return nil, err
			}
			if s != nil {
				if expr != nil {
					s.expr = expr
				}
				return s, nil
			}
			nextSeriess = nextSeriess[1:]
		}
	}
	return f
}

func getNextSeriesWrapperForAggregateFunc(funcName string) func(nextSeriesFunc, func(s *series) (*series, error)) nextSeriesFunc {
	isConcurrent := !isSerialFunc(funcName)
	return getNextSeriesWrapper(isConcurrent)
}

func isSerialFunc(funcName string) bool {
	switch funcName {
	case "diff", "first", "last", "current", "pow":
		return true
	}
	return false
}

func getNextSeriesWrapper(isConcurrent bool) func(nextSeriesFunc, func(s *series) (*series, error)) nextSeriesFunc {
	if isConcurrent {
		return nextSeriesConcurrentWrapper
	}
	return nextSeriesSerialWrapper
}

// nextSeriesSerialWrapper serially fetches series from nextSeries and passes them to f.
//
// see nextSeriesConcurrentWrapper for CPU-bound f.
//
// If f returns (nil, nil), then the current series is skipped.
// If f returns non-nil error, then nextSeries is drained with drainAllSeries.
func nextSeriesSerialWrapper(nextSeries nextSeriesFunc, f func(s *series) (*series, error)) nextSeriesFunc {
	wrapper := func() (*series, error) {
		for {
			s, err := nextSeries()
			if err != nil {
				return nil, err
			}
			if s == nil {
				return nil, nil
			}
			sNew, err := f(s)
			if err != nil {
				_, _ = drainAllSeries(nextSeries)
				return nil, err
			}
			if sNew != nil {
				return sNew, nil
			}
		}
	}
	return wrapper
}

// nextSeriesConcurrentWrapper fetches multiple series from nextSeries and calls f on these series from concurrent goroutines.
//
// This function is useful for parallelizing CPU-bound f across available CPU cores.
// f must be goroutine-safe, since it is called from multiple concurrent goroutines.
// See nextSeriesSerialWrapper for serial calls to f.
//
// If f returns (nil, nil), then the current series is skipped.
// If f returns non-nil error, then nextSeries is drained.
//
// nextSeries is called serially.
func nextSeriesConcurrentWrapper(nextSeries nextSeriesFunc, f func(s *series) (*series, error)) nextSeriesFunc {
	goroutines := cgroup.AvailableCPUs()
	type result struct {
		s   *series
		err error
	}
	resultCh := make(chan *result, goroutines)
	seriesCh := make(chan *series, goroutines)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	go func() {
		var err error
		for {
			s, e := nextSeries()
			if e != nil || s == nil {
				err = e
				break
			}
			seriesCh <- s
		}
		close(seriesCh)
		wg.Wait()
		close(resultCh)
		errCh <- err
		close(errCh)
	}()
	var skipProcessing atomic.Bool
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for s := range seriesCh {
				if skipProcessing.Load() {
					continue
				}
				sNew, err := f(s)
				if err != nil {
					// Drain the rest of series and do not call f for them in order to conserve CPU time.
					skipProcessing.Store(true)
					resultCh <- &result{
						err: err,
					}
				} else if sNew != nil {
					resultCh <- &result{
						s: sNew,
					}
				}
			}
		}()
	}
	wrapper := func() (*series, error) {
		r := <-resultCh
		if r == nil {
			err := <-errCh
			return nil, err
		}
		if r.err != nil {
			// Drain the rest of series before returning the error.
			for {
				_, ok := <-resultCh
				if !ok {
					break
				}
			}
			<-errCh
			return nil, r.err
		}
		if r.s == nil {
			panic(fmt.Errorf("BUG: r.s must be non-nil"))
		}
		return r.s, nil
	}
	return wrapper
}

func newZeroSeriesFunc() nextSeriesFunc {
	f := func() (*series, error) {
		return nil, nil
	}
	return f
}

func unmarshalTags(s string) map[string]string {
	if len(s) == 0 {
		return make(map[string]string)
	}
	tmp := strings.Split(s, ";")
	m := make(map[string]string, len(tmp))
	m["name"] = tmp[0]
	for _, x := range tmp[1:] {
		kv := strings.SplitN(x, "=", 2)
		if len(kv) == 2 {
			m[kv[0]] = kv[1]
		}
	}
	return m
}

func marshalTags(m map[string]string) string {
	parts := make([]string, 0, len(m))
	parts = append(parts, m["name"])
	for k, v := range m {
		if k != "name" {
			parts = append(parts, k+"="+v)
		}
	}
	sort.Strings(parts[1:])
	return strings.Join(parts, ";")
}

func formatKeyFromTags(tags map[string]string, tagKeys map[string]struct{}, defaultName string) string {
	newTags := make(map[string]string)
	for key := range tagKeys {
		newTags[key] = tags[key]
	}
	if _, ok := tagKeys["name"]; !ok {
		newTags["name"] = defaultName
	}
	return marshalTags(newTags)
}

func formatPathsFromSeries(ss []*series) string {
	seriesExpressions := make([]string, len(ss))
	for i, s := range ss {
		seriesExpressions[i] = s.pathExpression
	}
	return formatPathsFromSeriesExpressions(seriesExpressions, true)
}

func formatAggrFuncForPercentSeriesNames(funcName string, seriesNames []string) string {
	if len(seriesNames) == 0 {
		return "None"
	}
	if len(seriesNames) == 1 {
		return seriesNames[0]
	}
	return formatAggrFuncForSeriesNames(funcName, seriesNames)
}

func formatAggrFuncForSeriesNames(funcName string, seriesNames []string) string {
	if len(seriesNames) == 0 {
		return "None"
	}
	sortPaths := !isSerialFunc(funcName)
	return fmt.Sprintf("%sSeries(%s)", funcName, formatPathsFromSeriesExpressions(seriesNames, sortPaths))
}

func formatPathsFromSeriesExpressions(seriesExpressions []string, sortPaths bool) string {
	if len(seriesExpressions) == 0 {
		return ""
	}
	paths := make([]string, 0, len(seriesExpressions))
	visitedPaths := make(map[string]struct{})
	for _, path := range seriesExpressions {
		if _, ok := visitedPaths[path]; ok {
			continue
		}
		visitedPaths[path] = struct{}{}
		paths = append(paths, path)
	}
	if sortPaths {
		sort.Strings(paths)
	}
	return strings.Join(paths, ",")
}

func newNaNSeries(ec *evalConfig, step int64) *series {
	values := make([]float64, ec.pointsLen(step))
	for i := 0; i < len(values); i++ {
		values[i] = nan
	}
	return &series{
		Tags:       map[string]string{},
		Timestamps: ec.newTimestamps(step),
		Values:     values,
		step:       step,
	}
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.verticalLine
func transformVerticalLine(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting 1, 2 or 3 args", len(args))
	}
	tsArg, err := getString(args, "ts", 0)
	if err != nil {
		return nil, err
	}
	ts, err := parseTime(ec.currentTime, tsArg)
	if err != nil {
		return nil, err
	}
	name, err := getOptionalString(args, "label", 1, "")
	if err != nil {
		return nil, err
	}
	start := ec.startTime
	if ts < start {
		return nil, fmt.Errorf("verticalLine(): timestamp %d exists before start of range: %d", ts, start)
	}
	end := ec.endTime
	if ts > end {
		return nil, fmt.Errorf("verticalLine(): timestamp %d exists after end of range: %d", ts, end)
	}
	s := &series{
		Name:           name,
		Tags:           unmarshalTags(name),
		Timestamps:     []int64{ts, ts},
		Values:         []float64{1.0, 1.0},
		expr:           fe,
		pathExpression: name,
		step:           ec.endTime - ec.startTime,
	}
	return singleSeriesFunc(s), nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.useSeriesAbove
func transformUseSeriesAbove(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 4 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting 4 args", len(args))
	}
	value, err := getNumber(args, "value", 1)
	if err != nil {
		return nil, err
	}
	searchRe, err := getRegexp(args, "search", 2)
	if err != nil {
		return nil, err
	}
	replace, err := getRegexpReplacement(args, "replace", 3)
	if err != nil {
		return nil, err
	}
	ss, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}

	var seriesNames []string
	var lock sync.Mutex
	f := nextSeriesConcurrentWrapper(ss, func(s *series) (*series, error) {
		for _, v := range s.Values {
			if v <= value {
				continue
			}
			newName := searchRe.ReplaceAllString(s.Name, replace)
			lock.Lock()
			seriesNames = append(seriesNames, newName)
			lock.Unlock()
			break
		}
		return s, nil
	})
	if _, err = drainAllSeries(f); err != nil {
		return nil, err
	}
	query := fmt.Sprintf("group(%s)", strings.Join(seriesNames, ","))
	return execExpr(ec, query)
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.unique
func transformUnique(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	var uniqSeries []nextSeriesFunc
	uniq := make(map[string]struct{})
	for i := range args {
		nextSS, err := evalSeriesList(ec, args, "seriesList", i)
		if err != nil {
			for _, s := range uniqSeries {
				_, _ = drainAllSeries(s)
			}
			return nil, err
		}
		// Use nextSeriesSerialWrapper in order to guarantee that the first series among duplicate series is returned.
		nextUniq := nextSeriesSerialWrapper(nextSS, func(s *series) (*series, error) {
			name := s.Name
			if _, ok := uniq[name]; !ok {
				uniq[name] = struct{}{}
				return s, nil
			}
			return nil, nil
		})
		uniqSeries = append(uniqSeries, nextUniq)
	}
	return nextSeriesGroup(uniqSeries, fe), nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.transformNull
func transformTransformNull(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting 1,2 or 3 args", len(args))
	}
	defaultValue, err := getOptionalNumber(args, "default", 1, 0)
	if err != nil {
		return nil, err
	}
	defaultStr := fmt.Sprintf("%g", defaultValue)
	referenceSeries := getOptionalArg(args, "referenceSeries", 2)
	if referenceSeries == nil {
		// referenceSeries isn't set. Replace all NaNs with defaultValue.
		nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
		if err != nil {
			return nil, err
		}
		f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
			values := s.Values
			for i, v := range values {
				if math.IsNaN(v) {
					values[i] = defaultValue
				}
			}
			s.Tags["transformNull"] = defaultStr
			s.Name = fmt.Sprintf("transformNull(%s,%s)", s.Name, defaultStr)
			s.expr = fe
			s.pathExpression = s.Name
			return s, nil
		})
		return f, nil
	}

	// referenceSeries is set. Replace NaNs with defaultValue only if referenceSeries has non-NaN value at the given point.
	// Series must be normalized in order to match referenceSeries points.
	nextRefSeries, err := evalExpr(ec, referenceSeries.Expr)
	if err != nil {
		return nil, fmt.Errorf("cannot evaluate referenceSeries: %w", err)
	}
	ssRef, step, err := fetchNormalizedSeries(ec, nextRefSeries, true)
	if err != nil {
		return nil, err
	}
	replaceNan := make([]bool, ec.pointsLen(step))
	for i := range replaceNan {
		for _, sRef := range ssRef {
			if !math.IsNaN(sRef.Values[i]) {
				replaceNan[i] = true
				break
			}
		}
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidate(ec, step)
		values := s.Values
		for i, v := range values {
			if replaceNan[i] && math.IsNaN(v) {
				values[i] = defaultValue
			}
		}
		s.Tags["transformNull"] = defaultStr
		s.Tags["referenceSeries"] = "1"
		s.Name = fmt.Sprintf("transformNull(%s,%s,referenceSeries)", s.Name, defaultStr)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.timeStack
func transformTimeStack(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 4 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 4 args", len(args))
	}
	timeShiftUnit, err := getOptionalString(args, "timeShiftUnit", 1, "1d")
	if err != nil {
		return nil, err
	}
	delta, err := parseInterval(timeShiftUnit)
	if err != nil {
		return nil, err
	}
	if delta > 0 && !strings.HasPrefix(timeShiftUnit, "+") {
		delta = -delta
	}
	start, err := getOptionalNumber(args, "timeShiftStart", 2, 0)
	if err != nil {
		return nil, err
	}
	end, err := getOptionalNumber(args, "timeShiftEnd", 3, 7)
	if err != nil {
		return nil, err
	}
	if end < start {
		return nil, fmt.Errorf("timeShiftEnd=%g cannot be smaller than timeShiftStart=%g", end, start)
	}
	var allSeries []nextSeriesFunc
	for shift := int64(start); shift <= int64(end); shift++ {
		innerDelta := delta * shift
		ecCopy := *ec
		ecCopy.startTime = ecCopy.startTime + innerDelta
		ecCopy.endTime = ecCopy.endTime + innerDelta
		nextSS, err := evalSeriesList(&ecCopy, args, "seriesList", 0)
		if err != nil {
			for _, f := range allSeries {
				_, _ = drainAllSeries(f)
			}
			return nil, err
		}
		shiftStr := fmt.Sprintf("%d", shift)
		f := nextSeriesConcurrentWrapper(nextSS, func(s *series) (*series, error) {
			timestamps := s.Timestamps
			for i := range timestamps {
				timestamps[i] -= innerDelta
			}
			s.Tags["timeShiftUnit"] = timeShiftUnit
			s.Tags["timeShift"] = shiftStr
			s.Name = fmt.Sprintf("timeShift(%s,%s,%s)", s.Name, timeShiftUnit, shiftStr)
			s.expr = fe
			s.pathExpression = s.Name

			return s, nil
		})
		allSeries = append(allSeries, f)
	}
	return nextSeriesGroup(allSeries, fe), nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.timeSlice
func transformTimeSlice(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting 2 or 3 args", len(args))
	}
	startStr, err := getString(args, "startSliceAt", 1)
	if err != nil {
		return nil, err
	}
	start, err := parseTime(ec.currentTime, startStr)
	if err != nil {
		return nil, err
	}
	endStr, err := getOptionalString(args, "endSliceAt", 2, "now")
	if err != nil {
		return nil, err
	}
	end, err := parseTime(ec.currentTime, endStr)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	startSecsStr := fmt.Sprintf("%d", start/1000)
	endSecsStr := fmt.Sprintf("%d", end/1000)
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		timestamps := s.Timestamps
		for i := range values {
			if timestamps[i] < start || timestamps[i] > end {
				values[i] = nan
			}
		}
		s.Tags["timeSliceStart"] = startSecsStr
		s.Tags["timeSliceEnd"] = endSecsStr
		s.Name = fmt.Sprintf("timeSlice(%s,%s,%s)", s.Name, startSecsStr, endSecsStr)

		s.expr = fe
		return s, nil
	})
	return f, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.timeShift
func transformTimeShift(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 || len(args) > 4 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 2 to 4 args", len(args))
	}
	timeShiftStr, err := getString(args, "timeShift", 1)
	if err != nil {
		return nil, err
	}
	timeShift, err := parseInterval(timeShiftStr)
	if err != nil {
		return nil, err
	}
	if timeShift > 0 && !strings.HasPrefix(timeShiftStr, "+") {
		timeShift = -timeShift
	}
	resetEnd, err := getOptionalBool(args, "resetEnd", 2, true)
	if err != nil {
		return nil, err
	}
	_, err = getOptionalBool(args, "alignDST", 3, false)
	if err != nil {
		return nil, err
	}
	// TODO: properly use alignDST

	ecCopy := *ec
	ecCopy.startTime += timeShift
	ecCopy.endTime += timeShift
	nextSeries, err := evalSeriesList(&ecCopy, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		if resetEnd {
			for i, ts := range s.Timestamps {
				if ts > ec.endTime {
					s.Timestamps = s.Timestamps[:i]
					s.Values = s.Values[:i]
					break
				}
			}
		}
		timestamps := s.Timestamps
		for i := range timestamps {
			timestamps[i] -= timeShift
		}
		s.Tags["timeShift"] = timeShiftStr
		s.Name = fmt.Sprintf(`timeShift(%s,%s)`, s.Name, graphiteql.QuoteString(timeShiftStr))
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.nPercentile
func transformNPercentile(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	n, err := getNumber(args, "n", 1)
	if err != nil {
		return nil, err
	}
	nStr := fmt.Sprintf("%g", n)
	aggrFunc := newAggrFuncPercentile(n)
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		percentile := aggrFunc(values)
		for i := range values {
			values[i] = percentile
		}
		s.Tags["nPercentile"] = nStr
		s.Name = fmt.Sprintf("nPercentile(%s,%s)", s.Name, nStr)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.nonNegativeDerivative
func transformNonNegativeDerivative(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want from 1 to 3", len(args))
	}
	maxValue, err := getOptionalNumber(args, "maxValue", 1, nan)
	if err != nil {
		return nil, err
	}
	minValue, err := getOptionalNumber(args, "minValue", 2, nan)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		prev := nan
		var delta float64
		values := s.Values
		for i, v := range values {
			delta, prev = nonNegativeDelta(v, prev, maxValue, minValue)
			values[i] = delta
		}
		s.Tags["nonNegativeDerivative"] = "1"
		s.Name = fmt.Sprintf(`nonNegativeDerivative(%s)`, s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.offset
func transformOffset(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	factor, err := getNumber(args, "factor", 1)
	if err != nil {
		return nil, err
	}
	factorStr := fmt.Sprintf("%g", factor)
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			if !math.IsNaN(v) {
				values[i] = v + factor
			}
		}
		s.Tags["offset"] = factorStr
		s.Name = fmt.Sprintf("offset(%s,%s)", s.Name, factorStr)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.offsetToZero
func transformOffsetToZero(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		min := aggrMin(values)
		for i, v := range values {
			values[i] = v - min
		}
		s.Tags["offsetToZero"] = fmt.Sprintf("%g", min)
		s.Name = fmt.Sprintf("offsetToZero(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.perSecond
func transformPerSecond(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 3 args", len(args))
	}
	maxValue, err := getOptionalNumber(args, "maxValue", 1, nan)
	if err != nil {
		return nil, err
	}
	minValue, err := getOptionalNumber(args, "minValue", 2, nan)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		prev := nan
		var delta float64
		values := s.Values
		timestamps := s.Timestamps
		for i, v := range values {
			delta, prev = nonNegativeDelta(v, prev, maxValue, minValue)
			stepSecs := nan
			if i > 0 {
				stepSecs = float64(timestamps[i]-timestamps[i-1]) / 1000
			}
			values[i] = delta / stepSecs
		}
		s.Tags["perSecond"] = "1"
		s.Name = fmt.Sprintf(`perSecond(%s)`, s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

func nonNegativeDelta(curr, prev, max, min float64) (float64, float64) {
	if !math.IsNaN(max) && curr > max {
		return nan, nan
	}
	if !math.IsNaN(min) && curr < min {
		return nan, nan
	}
	if math.IsNaN(curr) || math.IsNaN(prev) {
		return nan, curr
	}
	if curr >= prev {
		return curr - prev, curr
	}
	if !math.IsNaN(max) {
		if math.IsNaN(min) {
			min = float64(0)
		}
		return max + 1 + curr - prev - min, curr
	}
	if !math.IsNaN(min) {
		return curr - min, curr
	}
	return nan, curr
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.threshold
func transformThreshold(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 3 args", len(args))
	}
	value, err := getNumber(args, "value", 0)
	if err != nil {
		return nil, err
	}
	label, err := getOptionalString(args, "label", 1, "")
	if err != nil {
		return nil, err
	}
	_, err = getOptionalString(args, "color", 2, "")
	if err != nil {
		return nil, err
	}
	nextSeries := constantLine(ec, fe, value)
	if label == "" {
		return nextSeries, nil
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		s.Name = label
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.sumSeries
func transformSumSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesGeneric(ec, fe, "sum")
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.substr
func transformSubstr(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 3 args", len(args))
	}
	startf, err := getOptionalNumber(args, "start", 1, 0)
	if err != nil {
		return nil, err
	}
	stopf, err := getOptionalNumber(args, "stop", 2, 0)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		path := getPathFromName(s.Name)
		splitName := strings.Split(path, ".")
		start := int(startf)
		stop := int(stopf)
		if start > len(splitName) {
			start = len(splitName)
		} else if start < 0 {
			start = len(splitName) + start
			if start < 0 {
				start = 0
			}
		}
		if stop == 0 {
			stop = len(splitName)
		} else if stop > len(splitName) {
			stop = len(splitName)
		} else if stop < 0 {
			stop = len(splitName) + stop
			if stop < 0 {
				stop = 0
			}
		}
		if stop < start {
			stop = start
		}
		s.Name = strings.Join(splitName[start:stop], ".")
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.stdev
func transformStdev(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 2 to 3 args", len(args))
	}
	pointsf, err := getNumber(args, "points", 1)
	if err != nil {
		return nil, err
	}
	points := int(pointsf)
	pointsStr := fmt.Sprintf("%d", points)
	windowTolerance, err := getOptionalNumber(args, "windowTolerance", 2, 0.1)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		var sum, sum2 float64
		var n int
		values := s.Values
		dstValues := make([]float64, len(values))
		for i, v := range values {
			if !math.IsNaN(v) {
				n++
				sum += v
				sum2 += v * v
			}
			if i >= points {
				v = values[i-points]
				if !math.IsNaN(v) {
					n--
					sum -= v
					sum2 -= v * v
				}
			}
			stddev := nan
			if n > 0 && float64(n)/pointsf >= windowTolerance {
				stddev = math.Sqrt(float64(n)*sum2-sum*sum) / float64(n)
			}
			dstValues[i] = stddev
		}
		s.Values = dstValues
		s.Tags["stdev"] = pointsStr
		s.Name = fmt.Sprintf("stdev(%s,%s)", s.Name, pointsStr)
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.stddevSeries
func transformStddevSeries(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	return aggregateSeriesGeneric(ec, fe, "stddev")
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.stacked
func transformStacked(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 2 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 2 args", len(args))
	}
	stackName, err := getOptionalString(args, "stackName", 1, "__DEFAULT__")
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, err
	}
	totalStack := make([]float64, ec.pointsLen(step))
	// Use nextSeriesSerialWrapper instead of nextSeriesConcurrentWrapper for preserving the original order of series.
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		// Consolidation is needed in order to align points in time. Otherwise stacking has little sense.
		s.consolidate(ec, step)
		values := s.Values
		for i, v := range values {
			if !math.IsNaN(v) {
				totalStack[i] += v
				values[i] = totalStack[i]
			}
		}
		if stackName == "__DEFAULT__" {
			s.Tags["stacked"] = stackName
			s.Name = fmt.Sprintf("stacked(%s)", s.Name)
		}
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.squareRoot
func transformSquareRoot(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting 1 arg", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			values[i] = math.Pow(v, 0.5)
		}
		s.Tags["squareRoot"] = "1"
		s.Name = fmt.Sprintf("squareRoot(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.sortByTotal
func transformSortByTotal(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting 1 arg", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return sortByGeneric(fe, nextSeries, "sum", true)
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.sortBy
func transformSortBy(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 3 args", len(args))
	}
	funcName, err := getOptionalString(args, "func", 1, "average")
	if err != nil {
		return nil, err
	}
	reverse, err := getOptionalBool(args, "reverse", 2, false)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return sortByGeneric(fe, nextSeries, funcName, reverse)
}

func sortByGeneric(fe *graphiteql.FuncExpr, nextSeries nextSeriesFunc, funcName string, reverse bool) (nextSeriesFunc, error) {
	aggrFunc, err := getAggrFunc(funcName)
	if err != nil {
		_, _ = drainAllSeries(nextSeries)
		return nil, err
	}
	var sws []seriesWithWeight
	var ssLock sync.Mutex
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		v := aggrFunc(s.Values)
		if math.IsNaN(v) {
			v = math.Inf(-1)
		}
		s.expr = fe
		ssLock.Lock()
		sws = append(sws, seriesWithWeight{
			s: s,
			v: v,
		})
		ssLock.Unlock()
		return s, nil
	})
	if _, err := drainAllSeries(f); err != nil {
		return nil, err
	}
	sort.Slice(sws, func(i, j int) bool {
		left := sws[i].v
		right := sws[j].v
		if reverse {
			left, right = right, left
		}
		return left < right
	})
	ss := make([]*series, len(sws))
	for i, sw := range sws {
		ss[i] = sw.s
	}
	return multiSeriesFunc(ss), nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.sortByName
func transformSortByName(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 3 args", len(args))
	}
	natural, err := getOptionalBool(args, "natural", 1, false)
	if err != nil {
		return nil, err
	}
	reverse, err := getOptionalBool(args, "reverse", 2, false)
	if err != nil {
		return nil, err
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	type seriesWithName struct {
		s    *series
		name string
	}
	var sns []seriesWithName
	f := nextSeriesSerialWrapper(nextSeries, func(s *series) (*series, error) {
		name := s.Name
		sns = append(sns, seriesWithName{
			s:    s,
			name: name,
		})
		s.expr = fe
		return s, nil
	})
	if _, err := drainAllSeries(f); err != nil {
		return nil, err
	}
	sort.Slice(sns, func(i, j int) bool {
		left := sns[i].name
		right := sns[j].name
		if reverse {
			left, right = right, left
		}
		if natural {
			return naturalLess(left, right)
		}
		return left < right
	})
	ss := make([]*series, len(sns))
	for i, sn := range sns {
		ss[i] = sn.s
	}
	return multiSeriesFunc(ss), nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.sortByMinima
func transformSortByMinima(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting 1 arg", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	// Filter out series with all the values smaller than 0
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		max := aggrMax(s.Values)
		if math.IsNaN(max) || max <= 0 {
			return nil, nil
		}
		return s, nil
	})
	return sortByGeneric(fe, f, "min", false)
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.sortByMaxima
func transformSortByMaxima(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting 1 arg", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	return sortByGeneric(fe, nextSeries, "max", true)
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.smartSummarize
func transformSmartSummarize(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 2 || len(args) > 4 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want from 2 to 4", len(args))
	}
	intervalString, err := getString(args, "intervalString", 1)
	if err != nil {
		return nil, err
	}
	interval, err := parseInterval(intervalString)
	if err != nil {
		return nil, fmt.Errorf("cannot parse intervalString: %w", err)
	}
	if interval <= 0 {
		return nil, fmt.Errorf("interval must be positive; got %dms", interval)
	}
	funcName, err := getOptionalString(args, "func", 2, "sum")
	if err != nil {
		return nil, err
	}
	aggrFunc, err := getAggrFunc(funcName)
	if err != nil {
		return nil, err
	}
	alignTo, err := getOptionalString(args, "alignTo", 3, "")
	if err != nil {
		return nil, err
	}
	ecCopy := *ec
	if alignTo != "" {
		ecCopy.startTime, err = alignTimeUnit(ecCopy.startTime, alignTo, ec.currentTime.Location())
		if err != nil {
			return nil, err
		}
	}
	nextSeries, err := evalSeriesList(&ecCopy, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.summarize(aggrFunc, ecCopy.startTime, ecCopy.endTime, interval, s.xFilesFactor)
		s.Tags["smartSummarize"] = intervalString
		s.Tags["smartSummarizeFunction"] = funcName
		s.Name = fmt.Sprintf("smartSummarize(%s,%s,%s)", s.Name, graphiteql.QuoteString(intervalString), graphiteql.QuoteString(funcName))
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

func alignTimeUnit(startTime int64, s string, tz *time.Location) (int64, error) {
	t := time.Unix(startTime/1e3, (startTime%1000)*1e6).In(tz)
	switch {
	case strings.HasPrefix(s, "ms"):
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), (t.Nanosecond()/1e6)*1e6, tz)
	case strings.HasPrefix(s, "s"):
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, tz)
	case strings.HasPrefix(s, "min"):
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, tz)
	case strings.HasPrefix(s, "h"):
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, tz)
	case strings.HasPrefix(s, "d"):
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz)
	case strings.HasPrefix(s, "w"):
		// check for week day to align.
		weekday := s[len(s)-1]
		isoWeekDayAlignTo := 1
		if weekday >= '0' && weekday <= '9' {
			isoWeekDayAlignTo = int(weekday - '0')
		}
		daysToSubtract := int(t.Weekday()) - isoWeekDayAlignTo
		if daysToSubtract < 0 {
			daysToSubtract += 7
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz).Add(-time.Hour * 24 * time.Duration(daysToSubtract))
	case strings.HasPrefix(s, "mon"):
		t = time.Date(t.Year(), t.Month(), 0, 0, 0, 0, 0, tz)
	case strings.HasPrefix(s, "y"):
		t = time.Date(t.Year(), 0, 0, 0, 0, 0, 0, tz)
	default:
		return 0, fmt.Errorf("unsupported interval %q", s)
	}
	return t.UnixNano() / 1e6, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.sinFunction
func transformSinFunction(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 3 args", len(args))
	}
	name, err := getString(args, "name", 0)
	if err != nil {
		return nil, err
	}
	amplitude, err := getOptionalNumber(args, "amplitude", 1, 1)
	if err != nil {
		return nil, err
	}
	step, err := getOptionalNumber(args, "step", 2, 60)
	if err != nil {
		return nil, err
	}
	if step <= 0 {
		return nil, fmt.Errorf("step must be positive; got %g", step)
	}
	stepMsecs := int64(step * 1000)
	values := make([]float64, 0, ec.pointsLen(stepMsecs))
	timestamps := make([]int64, 0, ec.pointsLen(stepMsecs))
	ts := ec.startTime
	for ts < ec.endTime {
		v := amplitude * math.Sin(float64(ts)/1000)
		values = append(values, v)
		timestamps = append(timestamps, ts)
		ts += stepMsecs
	}
	s := &series{
		Name:           name,
		Tags:           unmarshalTags(name),
		Timestamps:     timestamps,
		Values:         values,
		expr:           fe,
		pathExpression: name,
		step:           stepMsecs,
	}
	return singleSeriesFunc(s), nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.sigmoid
func transformSigmoid(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting 1 arg", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			values[i] = 1 / (1 + math.Exp(-v))
		}
		s.Tags["sigmoid"] = "sigmoid"
		s.Name = fmt.Sprintf("sigmoid(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.scaleToSeconds
func transformScaleToSeconds(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 2 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 2", len(args))
	}
	seconds, err := getNumber(args, "seconds", 1)
	if err != nil {
		return nil, err
	}
	secondsStr := fmt.Sprintf("%g", seconds)
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		timestamps := s.Timestamps
		values := s.Values
		step := nan
		if len(timestamps) > 1 {
			step = float64(timestamps[1]-timestamps[0]) / 1000
		}
		for i, v := range values {
			if i > 0 {
				step = float64(timestamps[i]-timestamps[i-1]) / 1000
			}
			values[i] = v * (seconds / step)
		}
		s.Tags["scaleToSeconds"] = secondsStr
		s.Name = fmt.Sprintf("scaleToSeconds(%s,%s)", s.Name, secondsStr)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.secondYAxis
func transformSecondYAxis(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.Tags["secondYAxis"] = "1"
		s.Name = fmt.Sprintf("secondYAxis(%s)", s.Name)
		s.expr = fe
		return s, nil
	})
	return f, nil
}

// See https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.isNonNull
func transformIsNonNull(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected number of args; got %d; want 1", len(args))
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		values := s.Values
		for i, v := range values {
			if math.IsNaN(v) {
				values[i] = 0
			} else {
				values[i] = 1
			}
		}
		s.Tags["isNonNull"] = "1"
		s.Name = fmt.Sprintf("isNonNull(%s)", s.Name)
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.sinFunction
func transformLinearRegression(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 3 args", len(args))
	}

	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	ss, _, err := fetchNormalizedSeries(ec, nextSeries, false)
	if err != nil {
		return nil, err
	}
	startSourceAt := getOptionalArg(args, "startSourceAt", 1)
	endSourceAt := getOptionalArg(args, "endSourceAt", 2)

	if startSourceAt == nil && endSourceAt == nil {
		// fast path, calculate for series with the same time range.
		return linearRegressionForSeries(ec, fe, ss, ss)
	}
	ecCopy := *ec
	ecCopy.startTime, err = getTimeFromArgExpr(ecCopy.startTime, ecCopy.currentTime, startSourceAt)
	if err != nil {
		return nil, err
	}
	ecCopy.endTime, err = getTimeFromArgExpr(ecCopy.endTime, ecCopy.currentTime, endSourceAt)
	if err != nil {
		return nil, err
	}
	nextSourceSeries, err := evalSeriesList(&ecCopy, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	sourceSeries, _, err := fetchNormalizedSeries(&ecCopy, nextSourceSeries, false)
	if err != nil {
		return nil, err
	}
	return linearRegressionForSeries(&ecCopy, fe, ss, sourceSeries)
}

func linearRegressionForSeries(ec *evalConfig, fe *graphiteql.FuncExpr, ss, sourceSeries []*series) (nextSeriesFunc, error) {
	var resp []*series
	for i := 0; i < len(ss); i++ {
		source := sourceSeries[i]
		s := ss[i]
		s.Tags["linearRegressions"] = fmt.Sprintf("%d, %d", ec.startTime/1e3, ec.endTime/1e3)
		s.Tags["name"] = s.Name
		s.Name = fmt.Sprintf("linearRegression(%s, %d, %d)", s.Name, ec.startTime/1e3, ec.endTime/1e3)
		s.expr = fe
		s.pathExpression = s.Name
		ok, factor, offset := linearRegressionAnalysis(source, float64(s.step))
		// skip
		if !ok {
			continue
		}
		values := s.Values
		for j := 0; j < len(values); j++ {
			values[j] = offset + (float64(int(s.Timestamps[0])+j*int(s.step)))*factor
		}
		resp = append(resp, s)
	}
	return multiSeriesFunc(resp), nil
}

func getTimeFromArgExpr(originT int64, currentT time.Time, expr *graphiteql.ArgExpr) (int64, error) {
	if expr == nil {
		return originT, nil
	}
	switch data := expr.Expr.(type) {
	case *graphiteql.NoneExpr:
	case *graphiteql.StringExpr:
		t, err := parseTime(currentT, data.S)
		if err != nil {
			return originT, err
		}
		originT = t
	case *graphiteql.NumberExpr:
		originT = int64(data.N * 1e3)
	}
	return originT, nil
}

// Returns is_not_none, factor and offset of linear regression function by least squares method.
// https://en.wikipedia.org/wiki/Linear_least_squares
// https://github.com/graphite-project/graphite-web/blob/master/webapp/graphite/render/functions.py#L4158
func linearRegressionAnalysis(s *series, step float64) (bool, float64, float64) {
	if step == 0 {
		return false, 0, 0
	}
	var sumI, sumII int
	var sumV, sumIV float64
	values := s.Values
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		sumI += i
		sumII += i * i
		sumIV += float64(i) * v
		sumV += v
	}
	denominator := float64(len(values)*sumII - sumI*sumI)
	if denominator == 0 {
		return false, 0.0, 0.0
	}
	factor := (float64(len(values))*sumIV - float64(sumI)*sumV) / denominator / step
	// safe to take index, denominator cannot be non zero in case of empty array.
	offset := (float64(sumII)*sumV-sumIV*float64(sumI))/denominator - factor*float64(s.Timestamps[0])
	return true, factor, offset
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.holtWintersConfidenceBands
func transformHoltWintersConfidenceBands(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 4 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 4 args", len(args))
	}
	resultSeries, err := holtWinterConfidenceBands(ec, fe, args)
	if err != nil {
		return nil, err
	}
	return multiSeriesFunc(resultSeries), nil
}

func holtWinterConfidenceBands(ec *evalConfig, fe *graphiteql.FuncExpr, args []*graphiteql.ArgExpr) ([]*series, error) {
	delta, err := getOptionalNumber(args, "delta", 1, 3)
	if err != nil {
		return nil, err
	}
	bootstrapInterval, err := getOptionalString(args, "bootstrapInterval", 2, "7d")
	if err != nil {
		return nil, err
	}
	bootstrapMs, err := parseInterval(bootstrapInterval)
	if err != nil {
		return nil, err
	}
	seasonality, err := getOptionalString(args, "seasonality", 3, "1d")
	if err != nil {
		return nil, err
	}

	seasonalityMs, err := parseInterval(seasonality)
	if err != nil {
		return nil, err
	}
	ecCopy := *ec
	ecCopy.startTime = ecCopy.startTime - bootstrapMs
	nextSeries, err := evalSeriesList(&ecCopy, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, err
	}
	trimWindowPoints := ecCopy.pointsLen(step) - ec.pointsLen(step)
	var resultSeries []*series
	var resultSeriesLock sync.Mutex
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidate(&ecCopy, step)
		timeStamps := s.Timestamps[trimWindowPoints:]
		analysis := holtWintersAnalysis(s, seasonalityMs)
		forecastValues := analysis.predictions.Values[trimWindowPoints:]
		deviationValues := analysis.deviations.Values[trimWindowPoints:]
		valuesLen := len(forecastValues)
		upperBand := make([]float64, 0, valuesLen)
		lowerBand := make([]float64, 0, valuesLen)
		for i := 0; i < valuesLen; i++ {
			forecastItem := forecastValues[i]
			deviationItem := deviationValues[i]
			if math.IsNaN(forecastItem) || math.IsNaN(deviationItem) {
				upperBand = append(upperBand, nan)
				lowerBand = append(lowerBand, nan)
			} else {
				scaledDeviation := delta * deviationItem
				upperBand = append(upperBand, forecastItem+scaledDeviation)
				lowerBand = append(lowerBand, forecastItem-scaledDeviation)
			}
		}
		name := fmt.Sprintf("holtWintersConfidenceUpper(%s)", s.Name)
		upperSeries := &series{
			Timestamps:     timeStamps,
			Values:         upperBand,
			Name:           name,
			Tags:           map[string]string{"holtWintersConfidenceUpper": "1", "name": s.Name},
			expr:           fe,
			pathExpression: name,
			step:           step,
		}
		name = fmt.Sprintf("holtWintersConfidenceLower(%s)", s.Name)
		lowerSeries := &series{
			Timestamps:     timeStamps,
			Values:         lowerBand,
			Name:           name,
			Tags:           map[string]string{"holtWintersConfidenceLower": "1", "name": s.Name},
			expr:           fe,
			pathExpression: name,
			step:           step,
		}
		resultSeriesLock.Lock()
		resultSeries = append(resultSeries, upperSeries, lowerSeries)
		resultSeriesLock.Unlock()
		return s, nil
	})
	if _, err := drainAllSeries(f); err != nil {
		return nil, err
	}
	return resultSeries, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.holtWintersConfidenceArea
func transformHoltWintersConfidenceArea(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 4 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 4 args", len(args))
	}
	bands, err := holtWinterConfidenceBands(ec, fe, args)
	if err != nil {
		return nil, err
	}
	if len(bands) != 2 {
		return nil, fmt.Errorf("expecting exactly two series; got more series")
	}
	for _, s := range bands {
		s.Name = fmt.Sprintf("areaBetween(%s)", s.Name)
		s.Tags["areaBetween"] = "1"
	}
	return multiSeriesFunc(bands), nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.holtWintersAberration
func transformHoltWintersAberration(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 4 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 4 args", len(args))
	}
	bands, err := holtWinterConfidenceBands(ec, fe, args)
	if err != nil {
		return nil, err
	}
	confidenceBands := make(map[string][]float64)
	for _, s := range bands {
		confidenceBands[s.Name] = s.Values
	}
	nextSeries, err := evalSeriesList(ec, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, err
	}
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidate(ec, step)
		values := s.Values
		lowerBand := confidenceBands[fmt.Sprintf("holtWintersConfidenceLower(%s)", s.Name)]
		upperBand := confidenceBands[fmt.Sprintf("holtWintersConfidenceUpper(%s)", s.Name)]
		if len(values) != len(lowerBand) || len(values) != len(upperBand) {
			return nil, fmt.Errorf("bug, len mismatch for series: %d and upperBand values: %d or lowerBand values: %d", len(values), len(upperBand), len(lowerBand))
		}
		aberration := make([]float64, 0, len(values))
		for i := 0; i < len(values); i++ {
			v := values[i]
			upperValue := upperBand[i]
			lowerValue := lowerBand[i]
			if math.IsNaN(v) {
				aberration = append(aberration, 0)
				continue
			}
			if !math.IsNaN(upperValue) && v > upperValue {
				aberration = append(aberration, v-upperValue)
				continue
			}
			if !math.IsNaN(lowerValue) && v < lowerValue {
				aberration = append(aberration, v-lowerValue)
				continue
			}
			aberration = append(aberration, 0)
		}
		s.Tags["holtWintersAberration"] = "1"
		s.Name = fmt.Sprintf("holtWintersAberration(%s)", s.Name)
		s.Values = aberration
		s.expr = fe
		s.pathExpression = s.Name
		return s, nil
	})
	return f, nil
}

// https://graphite.readthedocs.io/en/stable/functions.html#graphite.render.functions.holtWintersForecast
func transformHoltWintersForecast(ec *evalConfig, fe *graphiteql.FuncExpr) (nextSeriesFunc, error) {
	args := fe.Args
	if len(args) < 1 || len(args) > 3 {
		return nil, fmt.Errorf("unexpected number of args: %d; expecting from 1 to 3 args", len(args))
	}
	bootstrapInterval, err := getOptionalString(args, "bootstrapInterval", 1, "7d")
	if err != nil {
		return nil, err
	}
	bootstrapMs, err := parseInterval(bootstrapInterval)
	if err != nil {
		return nil, err
	}
	seasonality, err := getOptionalString(args, "seasonality", 2, "1d")
	if err != nil {
		return nil, err
	}
	seasonalityMs, err := parseInterval(seasonality)
	if err != nil {
		return nil, err
	}

	ecCopy := *ec
	ecCopy.startTime = ecCopy.startTime - bootstrapMs
	nextSeries, err := evalSeriesList(&ecCopy, args, "seriesList", 0)
	if err != nil {
		return nil, err
	}
	step, err := nextSeries.peekStep(ec.storageStep)
	if err != nil {
		return nil, err
	}
	trimWindowPoints := ecCopy.pointsLen(step) - ec.pointsLen(step)
	f := nextSeriesConcurrentWrapper(nextSeries, func(s *series) (*series, error) {
		s.consolidate(&ecCopy, step)
		analysis := holtWintersAnalysis(s, seasonalityMs)
		predictions := analysis.predictions

		s.Tags["holtWintersForecast"] = "1"
		s.Values = predictions.Values[trimWindowPoints:]
		s.Timestamps = predictions.Timestamps[trimWindowPoints:]
		newName := fmt.Sprintf("holtWintersForecast(%s)", s.Name)
		s.Name = newName
		s.Tags["name"] = newName
		s.expr = fe
		s.pathExpression = s.Name

		return s, nil
	})
	return f, nil

}

func holtWintersAnalysis(s *series, seasonality int64) holtWintersAnalysisResult {
	alpha := 0.1
	gamma := alpha
	beta := 0.0035

	seasonLength := seasonality / s.step

	var intercept, seasonal, deviation, slope float64

	intercepts := make([]float64, 0, len(s.Values))
	predictions := make([]float64, 0, len(s.Values))
	slopes := make([]float64, 0, len(s.Values))
	seasonals := make([]float64, 0, len(s.Values))
	deviations := make([]float64, 0, len(s.Values))

	getlastSeasonal := func(i int64) float64 {
		j := i - seasonLength
		if j >= 0 {
			return seasonals[j]
		}
		return 0
	}

	getlastDeviation := func(i int64) float64 {
		j := i - seasonLength
		if j >= 0 {
			return deviations[j]
		}
		return 0
	}
	var lastSeasonal, lastSeasonalDev, nextLastSeasonal float64
	nextPred := nan

	for i, v := range s.Values {
		if math.IsNaN(v) {
			intercepts = append(intercepts, 0)
			slopes = append(slopes, 0)
			seasonals = append(seasonals, 0)
			predictions = append(predictions, nextPred)
			deviations = append(deviations, 0)
			nextPred = nan
			continue
		}

		var lastIntercept, lastSlope, prediction float64
		if i == 0 {
			lastIntercept = v
			lastSlope = 0
			prediction = v
		} else {
			lastIntercept = intercepts[len(intercepts)-1]
			lastSlope = slopes[len(slopes)-1]
			if math.IsNaN(lastIntercept) {
				lastIntercept = v
			}
			prediction = nextPred
		}

		lastSeasonal = getlastSeasonal(int64(i))
		nextLastSeasonal = getlastSeasonal(int64(i + 1))
		lastSeasonalDev = getlastDeviation(int64(i))

		intercept = holtWintersIntercept(alpha, v, lastSeasonal, lastIntercept, lastSlope)
		slope = holtWintersSlope(beta, intercept, lastIntercept, lastSlope)
		seasonal = holtWintersSeasonal(gamma, v, intercept, lastSeasonal)

		nextPred = intercept + slope + nextLastSeasonal
		deviation = holtWintersDeviation(gamma, v, prediction, lastSeasonalDev)

		intercepts = append(intercepts, intercept)
		slopes = append(slopes, slope)
		seasonals = append(seasonals, seasonal)
		predictions = append(predictions, prediction)
		deviations = append(deviations, deviation)
	}
	forecastSeries := &series{
		Timestamps: s.Timestamps,
		Values:     predictions,
		Name:       fmt.Sprintf("holtWintersForecast(%s)", s.Name),
		step:       s.step,
	}
	deviationsSS := &series{
		Timestamps: s.Timestamps,
		Values:     deviations,
		Name:       fmt.Sprintf("holtWintersDeviation(%s)", s.Name),
		step:       s.step,
	}

	return holtWintersAnalysisResult{
		deviations:  deviationsSS,
		predictions: forecastSeries,
	}
}

type holtWintersAnalysisResult struct {
	predictions *series
	deviations  *series
}

func holtWintersIntercept(alpha, actual, lastReason, lastIntercept, lastSlope float64) float64 {
	return alpha*(actual-lastReason) + (1-alpha)*(lastIntercept+lastSlope)
}

func holtWintersSlope(beta, intercept, lastIntercept, lastSlope float64) float64 {
	return beta*(intercept-lastIntercept) + (1-beta)*lastSlope
}
func holtWintersSeasonal(gamma, actual, intercept, lastSeason float64) float64 {
	return gamma*(actual-intercept) + (1-gamma)*lastSeason
}

func holtWintersDeviation(gamma, actual, prediction, lastSeasonalDev float64) float64 {
	if math.IsNaN(prediction) {
		prediction = 0
	}
	return gamma*math.Abs(actual-prediction) + (1-gamma)*lastSeasonalDev
}

func (nsf *nextSeriesFunc) peekStep(step int64) (int64, error) {
	nextSeries := *nsf
	s, err := nextSeries()
	if err != nil {
		return 0, err
	}
	if s != nil {
		step = s.step
	}
	var calls atomic.Uint64
	*nsf = func() (*series, error) {
		if calls.Add(1) == 1 {
			return s, nil
		}
		return nextSeries()
	}
	return step, nil
}
