package dbutils

import (
	"regexp"
	"strings"

	"github.com/pocketbase/pocketbase/tools/tokenizer"
)

var (
	indexRegex       = regexp.MustCompile(`(?im)create\s+(unique\s+)?\s*index\s*(if\s+not\s+exists\s+)?(\S*)\s+on\s+(\S*)\s*\(([\s\S]*)\)(?:\s*where\s+([\s\S]*))?`)
	indexColumnRegex = regexp.MustCompile(`(?im)^([\s\S]+?)(?:\s+collate\s+([\w]+))?(?:\s+(asc|desc))?$`)
)

// IndexColumn represents a single parsed SQL index column.
type IndexColumn struct {
	Name    string `json:"name"` // identifier or expression
	Collate string `json:"collate"`
	Sort    string `json:"sort"`
}

// Index represents a single parsed SQL CREATE INDEX expression.
type Index struct {
	SchemaName string        `json:"schemaName"`
	IndexName  string        `json:"indexName"`
	TableName  string        `json:"tableName"`
	Where      string        `json:"where"`
	Columns    []IndexColumn `json:"columns"`
	Unique     bool          `json:"unique"`
	Optional   bool          `json:"optional"`
}

// IsValid checks if the current Index contains the minimum required fields to be considered valid.
func (idx Index) IsValid() bool {
	return idx.IndexName != "" && idx.TableName != "" && len(idx.Columns) > 0
}

// Build returns a "CREATE INDEX" SQL string from the current index parts.
//
// Returns empty string if idx.IsValid() is false.
func (idx Index) Build() string {
	if !idx.IsValid() {
		return ""
	}

	var str strings.Builder

	str.WriteString("CREATE ")

	if idx.Unique {
		str.WriteString("UNIQUE ")
	}

	str.WriteString("INDEX ")

	if idx.Optional {
		str.WriteString("IF NOT EXISTS ")
	}

	if idx.SchemaName != "" {
		// str.WriteString("`"), // SQLite
		str.WriteString(`"`) // PostgreSQL
		str.WriteString(idx.SchemaName)
		// str.WriteString("`.") // SQLite
		str.WriteString(`".`) // PostgreSQL
	}

	// str.WriteString("`") // SQLite
	str.WriteString(`"`) // PostgreSQL
	str.WriteString(idx.IndexName)
	// str.WriteString("` ") // SQLite
	str.WriteString(`" `) // PostgreSQL

	// str.WriteString("ON `") // SQLite
	str.WriteString(`ON "`) // PostgreSQL
	str.WriteString(idx.TableName)
	// str.WriteString("` (") // SQLite
	str.WriteString(`" (`) // PostgreSQL

	if len(idx.Columns) > 1 {
		str.WriteString("\n  ")
	}

	var hasCol bool
	for _, col := range idx.Columns {
		trimmedColName := strings.TrimSpace(col.Name)
		if trimmedColName == "" {
			continue
		}

		if hasCol {
			str.WriteString(",\n  ")
		}

		if strings.Contains(col.Name, "(") || strings.Contains(col.Name, " ") {
			// most likely an expression
			str.WriteString(normalizeMySQLIdentifierQuotes(trimmedColName))
		} else {
			// regular identifier
			// str.WriteString("`") // SQLite
			str.WriteString(`"`) // PostgreSQL
			str.WriteString(trimmedColName)
			// str.WriteString("`") // SQLite
			str.WriteString(`"`) // PostgreSQL
		}

		if col.Collate != "" {
			str.WriteString(" COLLATE ")
			str.WriteString(col.Collate)
		}

		if col.Sort != "" {
			str.WriteString(" ")
			str.WriteString(strings.ToUpper(col.Sort))
		}

		hasCol = true
	}

	if hasCol && len(idx.Columns) > 1 {
		str.WriteString("\n")
	}

	str.WriteString(")")

	if idx.Where != "" {
		str.WriteString(" WHERE ")
		str.WriteString(normalizeMySQLIdentifierQuotes(idx.Where))
	}

	return str.String()
}

// normalizeMySQLIdentifierQuotes converts MySQL-style backtick-quoted
// identifiers to PostgreSQL-compatible double-quoted identifiers.
// It keeps string literals untouched.
func normalizeMySQLIdentifierQuotes(expr string) string {
	if !strings.Contains(expr, "`") {
		return expr
	}

	var b strings.Builder
	b.Grow(len(expr))

	inSingle := false
	inDouble := false
	inBacktick := false

	for i := 0; i < len(expr); i++ {
		ch := expr[i]

		if inSingle {
			b.WriteByte(ch)
			if ch == '\'' {
				// Escaped single quote (SQL style): ''
				if i+1 < len(expr) && expr[i+1] == '\'' {
					b.WriteByte(expr[i+1])
					i++
				} else {
					inSingle = false
				}
			}
			continue
		}

		if inDouble {
			b.WriteByte(ch)
			if ch == '"' {
				inDouble = false
			}
			continue
		}

		if inBacktick {
			if ch == '`' {
				// Escaped backtick inside identifier (MySQL style): ``
				if i+1 < len(expr) && expr[i+1] == '`' {
					b.WriteByte('`')
					i++
					continue
				}

				b.WriteByte('"')
				inBacktick = false
				continue
			}

			if ch == '"' {
				b.WriteString(`""`)
				continue
			}

			b.WriteByte(ch)
			continue
		}

		switch ch {
		case '\'':
			inSingle = true
			b.WriteByte(ch)
		case '"':
			inDouble = true
			b.WriteByte(ch)
		case '`':
			inBacktick = true
			b.WriteByte('"')
		default:
			b.WriteByte(ch)
		}
	}

	// Keep the original expression if there is an unclosed backtick quote.
	if inBacktick {
		return expr
	}

	return b.String()
}

// ParseIndex parses the provided "CREATE INDEX" SQL string into Index struct.
func ParseIndex(createIndexExpr string) Index {
	result := Index{}

	matches := indexRegex.FindStringSubmatch(createIndexExpr)
	if len(matches) != 7 {
		return result
	}

	trimChars := "`\"'[]\r\n\t\f\v "

	// Unique
	// ---
	result.Unique = strings.TrimSpace(matches[1]) != ""

	// Optional (aka. "IF NOT EXISTS")
	// ---
	result.Optional = strings.TrimSpace(matches[2]) != ""

	// SchemaName and IndexName
	// ---
	nameTk := tokenizer.NewFromString(matches[3])
	nameTk.Separators('.')

	nameParts, _ := nameTk.ScanAll()
	if len(nameParts) == 2 {
		result.SchemaName = strings.Trim(nameParts[0], trimChars)
		result.IndexName = strings.Trim(nameParts[1], trimChars)
	} else {
		result.IndexName = strings.Trim(nameParts[0], trimChars)
	}

	// TableName
	// ---
	result.TableName = strings.Trim(matches[4], trimChars)

	// Columns
	// ---
	columnsTk := tokenizer.NewFromString(matches[5])
	columnsTk.Separators(',')

	rawColumns, _ := columnsTk.ScanAll()

	result.Columns = make([]IndexColumn, 0, len(rawColumns))

	for _, col := range rawColumns {
		colMatches := indexColumnRegex.FindStringSubmatch(col)
		if len(colMatches) != 4 {
			continue
		}

		trimmedName := strings.Trim(colMatches[1], trimChars)
		if trimmedName == "" {
			continue
		}

		result.Columns = append(result.Columns, IndexColumn{
			Name:    trimmedName,
			Collate: strings.TrimSpace(colMatches[2]),
			Sort:    strings.ToUpper(colMatches[3]),
		})
	}

	// WHERE expression
	// ---
	result.Where = strings.TrimSpace(matches[6])

	return result
}

// FindSingleColumnUniqueIndex returns the first matching single column unique index.
func FindSingleColumnUniqueIndex(indexes []string, column string) (Index, bool) {
	var index Index

	for _, idx := range indexes {
		index := ParseIndex(idx)
		if index.Unique && len(index.Columns) == 1 && strings.EqualFold(index.Columns[0].Name, column) {
			return index, true
		}
	}

	return index, false
}

// Deprecated: Use `_, ok := FindSingleColumnUniqueIndex(indexes, column)` instead.
//
// HasColumnUniqueIndex loosely checks whether the specified column has
// a single column unique index (WHERE statements are ignored).
func HasSingleColumnUniqueIndex(column string, indexes []string) bool {
	_, ok := FindSingleColumnUniqueIndex(indexes, column)
	return ok
}
