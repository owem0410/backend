package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Staging struct {
	Table    StagingTable  `json:"table"`
	SearchBy StagingFields `json:"searchBy"`
	Fields   StagingFields `json:"fields"`

	CreatedAt time.Time `json:"createdAt"`
}

func (s Staging) Valid() (bool, error) {
	if !s.Table.Valid() {
		return false, errors.New(fmt.Sprintf("invalid table name: %s", s.Table))
	}

	for k, v := range s.SearchBy {
		if !s.Table.isField(k) {
			return false, errors.New(fmt.Sprintf("invalid searchBy key: %s", k))
		}

		switch v.(type) {
		case float64:
		case bool:
		case string:
		default:
			return false, errors.New(fmt.Sprintf("invalid searchBy value: %v", v))
		}
	}

	if len(s.Fields) == 0 {
		return false, errors.New("fields is empty")
	}

	for k, v := range s.Fields {
		if !s.Table.isField(k) {
			return false, errors.New(fmt.Sprintf("invalid fields key: %s", k))
		}

		switch v.(type) {
		case map[string]any:
			// check if it's nested search
			nestedSearchBy, ok := mapToNestedSearchBy(v.(map[string]any))
			if !ok {
				return false, errors.New(fmt.Sprintf("invalid nested searchBy: %v", v))
			}

			return nestedSearchBy.Valid()
		case float64:
		case bool:
		case string:
		default:
			return false, errors.New(fmt.Sprintf("invalid fields value: %v", v))
		}
	}

	return true, nil
}

func (s Staging) Query() ([]string, []any, string, []any) {
	return searchQuery(s.Table, s.SearchBy)
}

func (s Staging) KeyString() string {
	strs := []string{}
	for _, pk := range s.Table.PkNames() {
		v, ok := s.Fields[pk]
		if !ok {
			panic("Staging.KeyString: missing pk")
		}
		strs = append(strs, fmt.Sprintf("%v", v))
	}

	return strings.Join(strs, "-")
}

type StagingFields map[string]any

func (sf StagingFields) Equal(fields StagingFields) bool {
	if len(sf) != len(fields) {
		return false
	}

	for k, v := range sf {
		if fields[k] != v {
			return false
		}
	}

	return true
}

func (sf StagingFields) ExistIn(fields StagingFields) bool {
	for k, v := range sf {
		if fields[k] != v {
			return false
		}
	}

	return true
}

type StagingNestedSearch struct {
	Table    StagingTable
	SearchBy StagingFields
}

func (ns *StagingNestedSearch) Query() ([]string, []any, string, []any) {
	return searchQuery(ns.Table, ns.SearchBy)
}

func (ns *StagingNestedSearch) Valid() (bool, error) {
	if !ns.Table.Valid() {
		return false, errors.New(fmt.Sprintf("invalid nested search table name: %s", ns.Table))
	}

	for k, v := range ns.SearchBy {
		if !ns.Table.isField(k) {
			return false, errors.New(fmt.Sprintf("invalid nested search searchBy key: %s", k))
		}

		switch v.(type) {
		case float64:
		case bool:
		case string:
		default:
			return false, errors.New(fmt.Sprintf("invalid nested search searchBy value: %v", v))
		}
	}

	return true, nil
}

type StagingResultStatus string

const (
	StagingResultStatusCreate   StagingResultStatus = "create"
	StagingResultStatusUpdate   StagingResultStatus = "update"
	StagingResultStatusConflict StagingResultStatus = "conflict"
)

type StagingResult struct {
	Fields []StagingResultField `json:"fields"`
	Status StagingResultStatus  `json:"status"`
}

type StagingResultFieldType string

const (
	StagingResultFieldTypeCompare StagingResultFieldType = "compare"
	StagingResultFieldTypeValue   StagingResultFieldType = "value"
)

type StagingResultField struct {
	Type  StagingResultFieldType `json:"type"`
	Field string                 `json:"field"`
	Value any                    `json:"value"`
}

type StagingFieldCompare struct {
	Changed bool `json:"changed"`
	Old     any  `json:"old"`
	New     any  `json:"new"`
}

func searchQuery(table StagingTable, searchBy StagingFields) ([]string, []any, string, []any) {
	where := []string{}
	args := []any{}
	i := 1
	keys := []string{}
	for k := range searchBy {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		where = append(where, fmt.Sprintf("%s = $%d", k, i))
		args = append(args, searchBy[k])
		i += 1
	}

	pks := table.PkNames()
	fieldVars := table.PkVars()
	query := fmt.Sprintf("SELECT %s FROM %s WHERE "+strings.Join(where, " AND "), strings.Join(pks, ", "), table)
	return pks, fieldVars.Vars, query, args
}

func mapToNestedSearchBy(m map[string]any) (*StagingNestedSearch, bool) {
	table, ok := m["table"]
	if !ok {
		return nil, false
	}

	searchByMap, ok := m["searchBy"]
	if !ok {
		return nil, false
	}

	searchBy, ok := searchByMap.(map[string]any)
	if !ok {
		return nil, false
	}

	return &StagingNestedSearch{
		Table:    StagingTable(table.(string)),
		SearchBy: searchBy,
	}, true
}
