package parser

import (
	"fmt"
	"strings"

	"github.com/transisidb/transisidb/internal/config"
	"github.com/xwb1989/sqlparser"
)

// QueryType represents the type of SQL query
type QueryType int

const (
	QueryTypeUnknown QueryType = iota
	QueryTypeSelect
	QueryTypeInsert
	QueryTypeUpdate
	QueryTypeDelete
)

// ParsedQuery represents a parsed SQL query with metadata
type ParsedQuery struct {
	Original        string
	Type            QueryType
	Statement       sqlparser.Statement
	TableName       string
	CurrencyColumns []string
	Values          map[string]interface{}
	NeedsTransform  bool
}

// Parser handles SQL query parsing and analysis
type Parser struct {
	tableConfig config.TablesConfig
}

// NewParser creates a new SQL parser
func NewParser(tableConfig config.TablesConfig) *Parser {
	return &Parser{
		tableConfig: tableConfig,
	}
}

// Parse parses a SQL query and returns metadata
func (p *Parser) Parse(query string) (*ParsedQuery, error) {
	// Parse SQL using sqlparser
	stmt, err := sqlparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	pq := &ParsedQuery{
		Original:  query,
		Statement: stmt,
		Values:    make(map[string]interface{}),
	}

	// Detect query type and extract info
	switch stmt := stmt.(type) {
	case *sqlparser.Select:
		pq.Type = QueryTypeSelect
		if err := p.analyzeSelect(stmt, pq); err != nil {
			return nil, err
		}

	case *sqlparser.Insert:
		pq.Type = QueryTypeInsert
		if err := p.analyzeInsert(stmt, pq); err != nil {
			return nil, err
		}

	case *sqlparser.Update:
		pq.Type = QueryTypeUpdate
		if err := p.analyzeUpdate(stmt, pq); err != nil {
			return nil, err
		}

	case *sqlparser.Delete:
		pq.Type = QueryTypeDelete
		if err := p.analyzeDelete(stmt, pq); err != nil {
			return nil, err
		}

	default:
		pq.Type = QueryTypeUnknown
	}

	return pq, nil
}

// analyzeInsert analyzes an INSERT statement
func (p *Parser) analyzeInsert(stmt *sqlparser.Insert, pq *ParsedQuery) error {
	// Extract table name
	tableName := sqlparser.String(stmt.Table)
	pq.TableName = tableName

	// Check if this table is configured for transformation
	tableConfig, exists := p.tableConfig[tableName]
	if !exists || !tableConfig.Enabled {
		pq.NeedsTransform = false
		return nil
	}

	// Extract column names
	var columns []string
	if stmt.Columns != nil {
		for _, col := range stmt.Columns {
			columns = append(columns, col.String())
		}
	}

	// Find currency columns in the INSERT
	for _, col := range columns {
		if _, exists := tableConfig.Columns[col]; exists {
			pq.CurrencyColumns = append(pq.CurrencyColumns, col)
			pq.NeedsTransform = true
		}
	}

	// Extract values (for simple INSERT VALUES)
	if rows, ok := stmt.Rows.(sqlparser.Values); ok {
		if len(rows) > 0 && len(columns) > 0 {
			row := rows[0]
			for i, val := range row {
				if i < len(columns) {
					pq.Values[columns[i]] = extractValue(val)
				}
			}
		}
	}

	return nil
}

// analyzeUpdate analyzes an UPDATE statement
func (p *Parser) analyzeUpdate(stmt *sqlparser.Update, pq *ParsedQuery) error {
	// Extract table name (handle multi-table updates)
	if len(stmt.TableExprs) > 0 {
		if aliasedTable, ok := stmt.TableExprs[0].(*sqlparser.AliasedTableExpr); ok {
			if tableName, ok := aliasedTable.Expr.(sqlparser.TableName); ok {
				pq.TableName = tableName.Name.String()
			}
		}
	}

	// Check if this table is configured
	tableConfig, exists := p.tableConfig[pq.TableName]
	if !exists || !tableConfig.Enabled {
		pq.NeedsTransform = false
		return nil
	}

	// Extract SET columns and values
	for _, expr := range stmt.Exprs {
		colName := expr.Name.Name.String()

		// Check if this is a currency column
		if _, exists := tableConfig.Columns[colName]; exists {
			pq.CurrencyColumns = append(pq.CurrencyColumns, colName)
			pq.Values[colName] = extractValue(expr.Expr)
			pq.NeedsTransform = true
		}
	}

	return nil
}

// analyzeSelect analyzes a SELECT statement
func (p *Parser) analyzeSelect(stmt *sqlparser.Select, pq *ParsedQuery) error {
	// Extract table name from FROM clause
	if len(stmt.From) > 0 {
		if aliasedTable, ok := stmt.From[0].(*sqlparser.AliasedTableExpr); ok {
			if tableName, ok := aliasedTable.Expr.(sqlparser.TableName); ok {
				pq.TableName = tableName.Name.String()
			}
		}
	}

	// For SELECT, we might need to transform response (simulation mode)
	// but not the query itself
	pq.NeedsTransform = false

	return nil
}

// analyzeDelete analyzes a DELETE statement
func (p *Parser) analyzeDelete(stmt *sqlparser.Delete, pq *ParsedQuery) error {
	// Extract table name
	if len(stmt.TableExprs) > 0 {
		if aliasedTable, ok := stmt.TableExprs[0].(*sqlparser.AliasedTableExpr); ok {
			if tableName, ok := aliasedTable.Expr.(sqlparser.TableName); ok {
				pq.TableName = tableName.Name.String()
			}
		}
	}

	// DELETE doesn't need transformation
	pq.NeedsTransform = false

	return nil
}

// extractValue extracts the actual value from a sqlparser expression
func extractValue(expr sqlparser.Expr) interface{} {
	switch v := expr.(type) {
	case *sqlparser.SQLVal:
		switch v.Type {
		case sqlparser.IntVal:
			return string(v.Val)
		case sqlparser.StrVal:
			return string(v.Val)
		case sqlparser.FloatVal:
			return string(v.Val)
		}
	case sqlparser.BoolVal:
		return bool(v)
	case *sqlparser.NullVal:
		return nil
	}
	return sqlparser.String(expr)
}

// RewriteForDualWrite rewrites a query to include shadow columns
func (p *Parser) RewriteForDualWrite(pq *ParsedQuery, convertedValues map[string]float64) (string, error) {
	if !pq.NeedsTransform {
		return pq.Original, nil
	}

	tableConfig := p.tableConfig[pq.TableName]

	switch stmt := pq.Statement.(type) {
	case *sqlparser.Insert:
		return p.rewriteInsert(stmt, pq, tableConfig, convertedValues)
	case *sqlparser.Update:
		return p.rewriteUpdate(stmt, pq, tableConfig, convertedValues)
	default:
		return pq.Original, nil
	}
}

// rewriteInsert rewrites an INSERT to include shadow columns
func (p *Parser) rewriteInsert(stmt *sqlparser.Insert, pq *ParsedQuery,
	tableConfig config.TableConfig, convertedValues map[string]float64) (string, error) {

	// Clone the statement
	newStmt := *stmt

	// Add shadow columns to column list
	var newColumns sqlparser.Columns
	for _, col := range stmt.Columns {
		newColumns = append(newColumns, col)
	}

	// Add shadow columns
	for _, currencyCol := range pq.CurrencyColumns {
		if colConfig, exists := tableConfig.Columns[currencyCol]; exists {
			shadowCol := sqlparser.NewColIdent(colConfig.TargetColumn)
			newColumns = append(newColumns, shadowCol)
		}
	}
	newStmt.Columns = newColumns

	// Add shadow values to VALUES clause
	if rows, ok := stmt.Rows.(sqlparser.Values); ok {
		var newRows sqlparser.Values
		for _, row := range rows {
			var newRow sqlparser.ValTuple
			for _, val := range row {
				newRow = append(newRow, val)
			}

			// Add converted values
			for _, currencyCol := range pq.CurrencyColumns {
				if convertedValue, exists := convertedValues[currencyCol]; exists {
					newRow = append(newRow, sqlparser.NewFloatVal([]byte(fmt.Sprintf("%.4f", convertedValue))))
				}
			}

			newRows = append(newRows, newRow)
		}
		newStmt.Rows = newRows
	}

	return sqlparser.String(&newStmt), nil
}

// rewriteUpdate rewrites an UPDATE to include shadow columns
func (p *Parser) rewriteUpdate(stmt *sqlparser.Update, pq *ParsedQuery,
	tableConfig config.TableConfig, convertedValues map[string]float64) (string, error) {

	// Clone the statement
	newStmt := *stmt

	// Add shadow column updates
	var newExprs sqlparser.UpdateExprs
	for _, expr := range stmt.Exprs {
		newExprs = append(newExprs, expr)
	}

	// Add converted values for shadow columns
	for _, currencyCol := range pq.CurrencyColumns {
		if colConfig, exists := tableConfig.Columns[currencyCol]; exists {
			if convertedValue, exists := convertedValues[currencyCol]; exists {
				shadowExpr := &sqlparser.UpdateExpr{
					Name: &sqlparser.ColName{
						Name: sqlparser.NewColIdent(colConfig.TargetColumn),
					},
					Expr: sqlparser.NewFloatVal([]byte(fmt.Sprintf("%.4f", convertedValue))),
				}
				newExprs = append(newExprs, shadowExpr)
			}
		}
	}
	newStmt.Exprs = newExprs

	return sqlparser.String(&newStmt), nil
}

// GetQueryType returns a string representation of query type
func (qt QueryType) String() string {
	switch qt {
	case QueryTypeSelect:
		return "SELECT"
	case QueryTypeInsert:
		return "INSERT"
	case QueryTypeUpdate:
		return "UPDATE"
	case QueryTypeDelete:
		return "DELETE"
	default:
		return "UNKNOWN"
	}
}

// IsMutation returns true if the query mutates data
func (qt QueryType) IsMutation() bool {
	return qt == QueryTypeInsert || qt == QueryTypeUpdate || qt == QueryTypeDelete
}

// NormalizeTableName removes backticks and quotes from table name
func NormalizeTableName(name string) string {
	name = strings.Trim(name, "`")
	name = strings.Trim(name, "\"")
	return name
}
