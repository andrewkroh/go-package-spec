package sqlgen

// SortTables returns table names in dependency order (parents before children)
// using topological sort on FK relationships.
func SortTables(tables map[string]*TableDef) []string {
	// Build adjacency: child → parent.
	deps := make(map[string][]string, len(tables))
	for name, td := range tables {
		deps[name] = nil
		if td.Parent != "" {
			deps[name] = append(deps[name], td.Parent)
		}
		for _, col := range td.Columns {
			if col.FK != "" && col.FK != td.Parent {
				deps[name] = append(deps[name], col.FK)
			}
		}
	}

	// Kahn's algorithm for topological sort.
	inDegree := make(map[string]int, len(tables))
	for name := range tables {
		inDegree[name] = 0
	}
	for _, parents := range deps {
		for _, p := range parents {
			inDegree[p] += 0 // ensure parent exists in map
		}
	}
	for name, parents := range deps {
		_ = name
		for range parents {
			inDegree[name]++
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	// Sort queue for deterministic output.
	sortStrings(queue)

	var result []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		// Find all nodes that depend on this node.
		for name, parents := range deps {
			for _, p := range parents {
				if p == node {
					inDegree[name]--
					if inDegree[name] == 0 {
						queue = insertSorted(queue, name)
					}
				}
			}
		}
	}

	return result
}

// sortStrings sorts a string slice in place.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// insertSorted inserts s into a sorted slice maintaining sort order.
func insertSorted(sorted []string, s string) []string {
	i := 0
	for i < len(sorted) && sorted[i] < s {
		i++
	}
	sorted = append(sorted, "")
	copy(sorted[i+1:], sorted[i:])
	sorted[i] = s
	return sorted
}
