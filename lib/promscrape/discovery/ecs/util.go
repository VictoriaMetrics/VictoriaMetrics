package ecs

import (
	"bytes"
	"net/url"
	"strconv"
	"strings"
)

// Tag represents a tag from https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_Tag.html
type Tag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func buildListBody(clusterARN, nextToken string) []byte {
	var bb bytes.Buffer
	bb.WriteString(`{"maxResults":100`)
	if clusterARN != "" {
		bb.WriteString(`,"cluster":`)
		bb.WriteString(strconv.Quote(clusterARN))
	}
	if nextToken != "" {
		bb.WriteString(`,"nextToken":`)
		bb.WriteString(strconv.Quote(nextToken))
	}
	bb.WriteByte('}')
	return bb.Bytes()
}

func buildDescribeBody(clusterARN string, includeTags bool, itemsKey string, items []string) []byte {
	var bb bytes.Buffer
	bb.WriteByte('{')
	if clusterARN != "" {
		bb.WriteString(`"cluster":`)
		bb.WriteString(strconv.Quote(clusterARN))
		bb.WriteByte(',')
	}
	if includeTags {
		bb.WriteString(`"include":["TAGS"],`)
	}
	bb.WriteByte('"')
	bb.WriteString(itemsKey)
	bb.WriteString(`":[`)
	for i, item := range items {
		if i > 0 {
			bb.WriteByte(',')
		}
		bb.WriteString(strconv.Quote(item))
	}
	bb.WriteString(`]}`)
	return bb.Bytes()
}

func buildIDFilterQueryString(paramName string, ids []string) string {
	var sb strings.Builder
	for i, id := range ids {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(paramName)
		sb.WriteByte('.')
		sb.WriteString(strconv.Itoa(i + 1))
		sb.WriteByte('=')
		sb.WriteString(url.QueryEscape(id))
	}
	return sb.String()
}
