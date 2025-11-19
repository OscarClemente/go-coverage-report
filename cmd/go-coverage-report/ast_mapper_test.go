package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatementLineMapper_GetStatementLines(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

import "fmt"

func example() {
	x := 5        // Line 6 - statement
	y := 10       // Line 7 - statement
	              // Line 8 - empty
	if x > 0 {    // Line 9 - statement (if condition)
		fmt.Println(x)  // Line 10 - statement
	}
	
	for i := 0; i < 10; i++ {  // Line 13 - statement (for loop)
		y++                     // Line 14 - statement
	}
	
	return  // Line 17 - statement
}
`
	err := os.WriteFile(testFile, []byte(code), 0644)
	require.NoError(t, err)

	mapper := NewStatementLineMapper()
	statementLines, err := mapper.GetStatementLines(testFile)
	require.NoError(t, err)

	// Verify that statement lines are detected
	assert.True(t, statementLines[6], "Line 6 should be a statement (x := 5)")
	assert.True(t, statementLines[7], "Line 7 should be a statement (y := 10)")
	assert.False(t, statementLines[8], "Line 8 should not be a statement (empty line)")
	assert.True(t, statementLines[9], "Line 9 should be a statement (if condition)")
	assert.True(t, statementLines[10], "Line 10 should be a statement (fmt.Println)")
	assert.True(t, statementLines[13], "Line 13 should be a statement (for loop)")
	assert.True(t, statementLines[14], "Line 14 should be a statement (y++)")
	assert.True(t, statementLines[17], "Line 17 should be a statement (return)")
}

func TestStatementLineMapper_CountStatementsInLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

func example() {
	x := 5        // Line 4 - statement
	y := 10       // Line 5 - statement
	z := 15       // Line 6 - statement
	              // Line 7 - empty
	if x > 0 {    // Line 8 - statement
		return y  // Line 9 - statement
	}
}
`
	err := os.WriteFile(testFile, []byte(code), 0644)
	require.NoError(t, err)

	mapper := NewStatementLineMapper()

	// Count statements in lines 4, 5, 7 (should be 2: lines 4 and 5)
	changedLines := map[int]bool{
		4: true, // statement
		5: true, // statement
		7: true, // empty line
	}

	count, err := mapper.CountStatementsInLines(testFile, changedLines)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "Should count 2 statements (lines 4 and 5, not line 7)")
}

func TestStatementLineMapper_GetStatementLinesInRange(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

func example() {
	x := 5        // Line 4 - statement
	y := 10       // Line 5 - statement
	              // Line 6 - empty
	z := 15       // Line 7 - statement
}
`
	err := os.WriteFile(testFile, []byte(code), 0644)
	require.NoError(t, err)

	mapper := NewStatementLineMapper()
	statements, err := mapper.GetStatementLinesInRange(testFile, 4, 6)
	require.NoError(t, err)

	assert.True(t, statements[4], "Line 4 should be a statement")
	assert.True(t, statements[5], "Line 5 should be a statement")
	assert.False(t, statements[6], "Line 6 should not be a statement")
	assert.False(t, statements[7], "Line 7 should not be included (outside range)")
}

func TestStatementLineMapper_RealWorldExample(t *testing.T) {
	// Test with the actual math.go file from our test data
	testFile := "testdata/example.com/calculator/math.go"
	
	// Check if file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file doesn't exist, skipping")
		return
	}

	mapper := NewStatementLineMapper()
	statementLines, err := mapper.GetStatementLines(testFile)
	require.NoError(t, err)

	// The Divide function should have statements
	// Based on the file content, we know certain lines have statements
	t.Logf("Found %d statement lines in %s", len(statementLines), testFile)
	
	// Just verify we found some statements
	assert.Greater(t, len(statementLines), 0, "Should find at least some statements")
}

