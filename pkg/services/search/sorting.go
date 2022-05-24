package search

import (
	"sort"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/sqlstore/searchstore"
)

var (
	SortAlphaAsc = models.SortOption{
		Name:        "alpha-asc",
		DisplayName: "Alphabetically (A–Z)",
		Description: "Sort results in an alphabetically ascending order",
		Index:       0,
		Filter: []models.SortOptionFilter{
			searchstore.TitleSorter{},
		},
	}
	SortAlphaDesc = models.SortOption{
		Name:        "alpha-desc",
		DisplayName: "Alphabetically (Z–A)",
		Description: "Sort results in an alphabetically descending order",
		Index:       0,
		Filter: []models.SortOptionFilter{
			searchstore.TitleSorter{Descending: true},
		},
	}
)

// RegisterSortOption allows for hooking in more search options from
// other services.
func (s *SearchService) RegisterSortOption(option models.SortOption) {
	s.sortOptions[option.Name] = option
}

func (s *SearchService) SortOptions() []models.SortOption {
	opts := make([]models.SortOption, 0, len(s.sortOptions))
	for _, o := range s.sortOptions {
		opts = append(opts, o)
	}
	sort.Slice(opts, func(i, j int) bool {
		return opts[i].Index < opts[j].Index || (opts[i].Index == opts[j].Index && opts[i].Name < opts[j].Name)
	})
	return opts
}
