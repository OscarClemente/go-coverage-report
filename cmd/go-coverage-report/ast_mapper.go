package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

// StatementLineMapper maps statements to their line numbers using AST parsing
type StatementLineMapper struct {
	fset *token.FileSet
}

// NewStatementLineMapper creates a new statement line mapper
func NewStatementLineMapper() *StatementLineMapper {
	return &StatementLineMapper{
		fset: token.NewFileSet(),
	}
}

// GetStatementLines returns a map of line numbers that contain actual statements
// This can be used to determine if a changed line actually contains a statement
func (m *StatementLineMapper) GetStatementLines(filePath string) (map[int]bool, error) {
	// Read the source file
	src, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Parse the file
	file, err := parser.ParseFile(m.fset, filePath, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	statementLines := make(map[int]bool)

	// Walk the AST and collect statement positions
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			return false
		}

		// Check if this node is a statement
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			// Assignment: x := 5
			line := m.fset.Position(stmt.Pos()).Line
			statementLines[line] = true
		case *ast.ExprStmt:
			// Expression statement: fmt.Println("hello")
			line := m.fset.Position(stmt.Pos()).Line
			statementLines[line] = true
		case *ast.ReturnStmt:
			// Return statement
			line := m.fset.Position(stmt.Pos()).Line
			statementLines[line] = true
		case *ast.IfStmt:
			// If statement (the condition line)
			line := m.fset.Position(stmt.If).Line
			statementLines[line] = true
		case *ast.ForStmt:
			// For loop (the for line)
			line := m.fset.Position(stmt.For).Line
			statementLines[line] = true
		case *ast.RangeStmt:
			// Range loop
			line := m.fset.Position(stmt.For).Line
			statementLines[line] = true
		case *ast.SwitchStmt:
			// Switch statement
			line := m.fset.Position(stmt.Switch).Line
			statementLines[line] = true
		case *ast.CaseClause:
			// Case clause
			line := m.fset.Position(stmt.Case).Line
			statementLines[line] = true
		case *ast.SelectStmt:
			// Select statement
			line := m.fset.Position(stmt.Select).Line
			statementLines[line] = true
		case *ast.SendStmt:
			// Channel send
			line := m.fset.Position(stmt.Pos()).Line
			statementLines[line] = true
		case *ast.IncDecStmt:
			// Increment/decrement: i++
			line := m.fset.Position(stmt.Pos()).Line
			statementLines[line] = true
		case *ast.GoStmt:
			// Go statement
			line := m.fset.Position(stmt.Go).Line
			statementLines[line] = true
		case *ast.DeferStmt:
			// Defer statement
			line := m.fset.Position(stmt.Defer).Line
			statementLines[line] = true
		case *ast.BranchStmt:
			// Break, continue, goto, fallthrough
			line := m.fset.Position(stmt.Pos()).Line
			statementLines[line] = true
		case *ast.DeclStmt:
			// Declaration statement (var, const inside function)
			line := m.fset.Position(stmt.Pos()).Line
			statementLines[line] = true
		}

		return true
	})

	return statementLines, nil
}

// CountStatementsInLines counts how many statements are on the specified lines
func (m *StatementLineMapper) CountStatementsInLines(filePath string, lines map[int]bool) (int, error) {
	statementLines, err := m.GetStatementLines(filePath)
	if err != nil {
		return 0, err
	}

	count := 0
	for line := range lines {
		if statementLines[line] {
			count++
		}
	}

	return count, nil
}

// GetStatementLinesInRange returns statement lines within a specific line range
func (m *StatementLineMapper) GetStatementLinesInRange(filePath string, startLine, endLine int) (map[int]bool, error) {
	allStatements, err := m.GetStatementLines(filePath)
	if err != nil {
		return nil, err
	}

	statementsInRange := make(map[int]bool)
	for line := startLine; line <= endLine; line++ {
		if allStatements[line] {
			statementsInRange[line] = true
		}
	}

	return statementsInRange, nil
}
