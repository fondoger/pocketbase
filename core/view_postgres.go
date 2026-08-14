package core

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"

	"github.com/pocketbase/pocketbase/tools/sort"
)

// loadPostgresViewGraph loads the PostgreSQL view dependency graph and DDL.
func loadPostgresViewGraph(app App) (map[string]viewDef, map[string][]string, error) {
	const query = `
		select
			u.view_name,
			u.table_name referenced_table_name,
			v.view_definition
		from information_schema.view_table_usage u
		join information_schema.views v on u.view_schema = v.table_schema
			and u.view_name = v.table_name
		where u.table_schema = current_schema()
		order by u.view_name
	`

	var rows []struct {
		ViewName            string `db:"view_name"`
		ReferencedTableName string `db:"referenced_table_name"`
		ViewDefinition      string `db:"view_definition"`
	}
	if err := app.DB().NewQuery(query).All(&rows); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return map[string]viewDef{}, map[string][]string{}, nil
		}
		return nil, nil, err
	}

	nodes := make(map[string]viewDef)
	graph := make(map[string][]string)
	for _, row := range rows {
		if def, ok := nodes[row.ViewName]; !ok || def.SQL == "[TABLE]" {
			nodes[row.ViewName] = viewDef{
				Name: row.ViewName,
				SQL:  fmt.Sprintf(`CREATE VIEW "%s" AS %s`, row.ViewName, row.ViewDefinition),
			}
		}
		if !slices.Contains(graph[row.ReferencedTableName], row.ViewName) {
			graph[row.ReferencedTableName] = append(graph[row.ReferencedTableName], row.ViewName)
			if _, ok := nodes[row.ReferencedTableName]; !ok {
				nodes[row.ReferencedTableName] = viewDef{Name: row.ReferencedTableName, SQL: "[TABLE]"}
			}
		}
	}

	return nodes, graph, nil
}

func findDependentViews(app App, tableOrViewName string) ([]viewDef, error) {
	nodes, graph, err := loadPostgresViewGraph(app)
	if err != nil {
		return nil, err
	}
	if _, ok := graph[tableOrViewName]; !ok {
		return nil, nil
	}

	ordered, err := sort.TopologicalSortReachable(nodes, graph, tableOrViewName)
	if err != nil {
		return nil, err
	}
	return ordered[1:], nil
}

func findAllViewsInDependencyOrder(app App) ([]viewDef, error) {
	nodes, graph, err := loadPostgresViewGraph(app)
	if err != nil {
		return nil, err
	}

	ordered, err := sort.TopologicalSortAll(nodes, graph)
	if err != nil {
		return nil, err
	}

	views := make([]viewDef, 0, len(ordered))
	for _, node := range ordered {
		if node.SQL != "[TABLE]" {
			views = append(views, node)
		}
	}
	return views, nil
}
