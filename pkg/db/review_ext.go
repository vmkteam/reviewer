package db

// WithEnabledAndIssueFilters adds StatusEnabledFilter for all entities
// except Issues, which already have IssueStatusFilter as base filter
// that correctly includes statuses 1 (enabled), 4 (valid), 5 (falsePositive), 6 (ignored).
func (rr ReviewRepo) WithEnabledAndIssueFilters() ReviewRepo {
	f := make(map[string][]Filter, len(rr.filters))
	for table := range rr.filters {
		f[table] = make([]Filter, len(rr.filters[table]))
		copy(f[table], rr.filters[table])
		if table != Tables.Issue.Name {
			f[table] = append(f[table], StatusEnabledFilter)
		}
	}
	rr.filters = f

	return rr
}
