package mergeset

import (
	"fmt"
	"sort"
	"testing"
	"time"
)

func TestBlockStreamReaderReadFromInmemoryPart(t *testing.T) {
	var items []string
	var ib inmemoryBlock
	for i := 0; i < 100; i++ {
		item := getRandomBytes()
		if !ib.Add(item) {
			break
		}
		items = append(items, string(item))
	}
	sort.Strings(items)
	var ip inmemoryPart
	ip.Init(&ib)

	// Make sure items may be read concurrently from the same inmemoryPart.
	ch := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			ch <- testBlockStreamReaderRead(&ip, items)
		}()
	}
	for i := 0; i < 5; i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}
}

func testBlockStreamReaderRead(ip *inmemoryPart, items []string) error {
	bsr := newTestBlockStreamReader(ip)
	i := 0
	for bsr.Next() {
		for _, item := range bsr.Block.items {
			if string(item) != items[i] {
				return fmt.Errorf("unexpected item[%d]; got %q; want %q", i, item, items[i])
			}
			i++
		}
	}
	if err := bsr.Error(); err != nil {
		return err
	}
	if i != len(items) {
		return fmt.Errorf("unexpected number of items read; got %d; want %d", i, len(items))
	}
	return nil
}
