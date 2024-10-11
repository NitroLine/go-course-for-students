package tagcloud

import (
	"slices"
)

// TagCloud aggregates statistics about used tags
type TagCloud struct {
	tags map[string]int
}

// TagStat represents statistics regarding single tag
type TagStat struct {
	Tag             string
	OccurrenceCount int
}

// New should create a valid TagCloud instance
func New() *TagCloud {
	return &TagCloud{tags: map[string]int{}}
}

// AddTag should add a tag to the cloud if it wasn't present and increase tag occurrence count
// thread-safety is not needed
func (cloud *TagCloud) AddTag(tag string) {
	cloud.tags[tag]++
}

// TopN should return top N most frequent tags ordered in descending order by occurrence count
// if there are multiple tags with the same occurrence count then the order is defined by implementation
// if n is greater that TagCloud size then all elements should be returned
// thread-safety is not needed
// there are no restrictions on time complexity
func (cloud *TagCloud) TopN(n int) []TagStat {
	tags := make([]TagStat, 0, len(cloud.tags))
	for tag, count := range cloud.tags {
		tags = append(tags, TagStat{Tag: tag, OccurrenceCount: count})
	}
	slices.SortFunc(tags, func(a, b TagStat) int {
		return b.OccurrenceCount - a.OccurrenceCount
	})
	if len(tags) < n {
		n = len(tags)
	}
	return tags[:n]
}
