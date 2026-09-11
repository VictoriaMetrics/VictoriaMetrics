package ecs

import (
	"testing"
)

func TestBuildListBody(t *testing.T) {
	f := func(clusterARN, nextToken, want string) {
		t.Helper()
		got := string(buildListBody(clusterARN, nextToken))
		if got != want {
			t.Fatalf("buildListBody(%q, %q)\ngot  %s\nwant %s", clusterARN, nextToken, got, want)
		}
	}
	// ListClusters first page: no cluster, no token
	f("", "", `{"maxResults":100}`)
	// ListClusters next page: no cluster, with token
	f("", "tok123", `{"maxResults":100,"nextToken":"tok123"}`)
	// ListTasks/ListServices first page: with cluster, no token
	f("arn:aws:ecs:us-east-1:123:cluster/my-cluster", "", `{"maxResults":100,"cluster":"arn:aws:ecs:us-east-1:123:cluster/my-cluster"}`)
	// ListTasks/ListServices next page: with cluster and token
	f("arn:aws:ecs:us-east-1:123:cluster/my-cluster", "tok456", `{"maxResults":100,"cluster":"arn:aws:ecs:us-east-1:123:cluster/my-cluster","nextToken":"tok456"}`)
}

func TestBuildDescribeBody(t *testing.T) {
	f := func(clusterARN string, includeTags bool, itemsKey string, items []string, want string) {
		t.Helper()
		got := string(buildDescribeBody(clusterARN, includeTags, itemsKey, items))
		if got != want {
			t.Fatalf("buildDescribeBody(%q, %v, %q, %v)\ngot  %s\nwant %s", clusterARN, includeTags, itemsKey, items, got, want)
		}
	}
	// DescribeClusters: no cluster, with tags
	f("", true, "clusters",
		[]string{"arn:aws:ecs:us-east-1:123:cluster/a", "arn:aws:ecs:us-east-1:123:cluster/b"},
		`{"include":["TAGS"],"clusters":["arn:aws:ecs:us-east-1:123:cluster/a","arn:aws:ecs:us-east-1:123:cluster/b"]}`)
	// DescribeServices: with cluster, with tags
	f("arn:aws:ecs:us-east-1:123:cluster/my-cluster", true, "services",
		[]string{"arn:aws:ecs:us-east-1:123:service/svc1"},
		`{"cluster":"arn:aws:ecs:us-east-1:123:cluster/my-cluster","include":["TAGS"],"services":["arn:aws:ecs:us-east-1:123:service/svc1"]}`)
	// DescribeTasks: with cluster, with tags, multiple items
	f("arn:aws:ecs:us-east-1:123:cluster/my-cluster", true, "tasks",
		[]string{"arn:aws:ecs:us-east-1:123:task/t1", "arn:aws:ecs:us-east-1:123:task/t2"},
		`{"cluster":"arn:aws:ecs:us-east-1:123:cluster/my-cluster","include":["TAGS"],"tasks":["arn:aws:ecs:us-east-1:123:task/t1","arn:aws:ecs:us-east-1:123:task/t2"]}`)
	// DescribeContainerInstances: with cluster, no tags
	f("arn:aws:ecs:us-east-1:123:cluster/my-cluster", false, "containerInstances",
		[]string{"arn:aws:ecs:us-east-1:123:container-instance/ci1"},
		`{"cluster":"arn:aws:ecs:us-east-1:123:cluster/my-cluster","containerInstances":["arn:aws:ecs:us-east-1:123:container-instance/ci1"]}`)
	// empty items list
	f("", false, "tasks", []string{}, `{"tasks":[]}`)
}
