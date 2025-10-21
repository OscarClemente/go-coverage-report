package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseUnifiedDiff(t *testing.T) {
	// Create a temporary diff file
	diffContent := `diff --git a/test.go b/test.go
index 1234567..abcdefg 100644
--- a/test.go
+++ b/test.go
@@ -10,6 +10,9 @@ func main() {
 	fmt.Println("Hello")
 }
 
+func newFunction() {
+	fmt.Println("New")
+}
+
 func oldFunction() {
 	fmt.Println("Old")
 }
`

	tmpFile, err := os.CreateTemp("", "test-diff-*.patch")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(diffContent)
	require.NoError(t, err)
	tmpFile.Close()

	// Parse the diff
	diffInfo, err := ParseUnifiedDiff(tmpFile.Name())
	require.NoError(t, err)
	require.NotNil(t, diffInfo)

	// Check that we detected the added lines
	assert.True(t, diffInfo.IsLineAdded("test.go", 13), "Line 13 should be marked as added")
	assert.True(t, diffInfo.IsLineAdded("test.go", 14), "Line 14 should be marked as added")
	assert.True(t, diffInfo.IsLineAdded("test.go", 15), "Line 15 should be marked as added")

	// Lines that weren't added should return false
	assert.False(t, diffInfo.IsLineAdded("test.go", 10), "Line 10 should not be marked as added")
	assert.False(t, diffInfo.IsLineAdded("test.go", 11), "Line 11 should not be marked as added")
}

func TestIsLineInRange(t *testing.T) {
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"test.go": {
				FileName: "test.go",
				AddedLines: map[int]bool{
					5:  true,
					10: true,
					15: true,
				},
				ModifiedLines: map[int]bool{},
			},
		},
	}

	// Test ranges that contain added lines
	assert.True(t, diffInfo.IsLineInRange("test.go", 1, 10), "Range 1-10 contains line 5 and 10")
	assert.True(t, diffInfo.IsLineInRange("test.go", 10, 20), "Range 10-20 contains lines 10 and 15")
	assert.True(t, diffInfo.IsLineInRange("test.go", 5, 5), "Range 5-5 contains line 5")

	// Test ranges that don't contain added lines
	assert.False(t, diffInfo.IsLineInRange("test.go", 1, 4), "Range 1-4 doesn't contain any added lines")
	assert.False(t, diffInfo.IsLineInRange("test.go", 20, 30), "Range 20-30 doesn't contain any added lines")

	// Test non-existent file
	assert.False(t, diffInfo.IsLineInRange("nonexistent.go", 1, 100), "Non-existent file should return false")
}

func TestCalculateNewCodeCoverageFromDiff(t *testing.T) {
	// Create a simple coverage profile
	oldCov := &Coverage{
		Files: map[string]*Profile{
			"github.com/test/file.go": {
				FileName:    "github.com/test/file.go",
				TotalStmt:   10,
				CoveredStmt: 8,
				Blocks: []ProfileBlock{
					{StartLine: 1, EndLine: 5, NumStmt: 5, Count: 1},
					{StartLine: 6, EndLine: 10, NumStmt: 5, Count: 1},
				},
			},
		},
		TotalStmt:   10,
		CoveredStmt: 8,
	}

	newCov := &Coverage{
		Files: map[string]*Profile{
			"github.com/test/file.go": {
				FileName:    "github.com/test/file.go",
				TotalStmt:   15,
				CoveredStmt: 12,
				Blocks: []ProfileBlock{
					{StartLine: 1, EndLine: 5, NumStmt: 5, Count: 1},
					{StartLine: 6, EndLine: 10, NumStmt: 5, Count: 1},
					{StartLine: 11, EndLine: 15, NumStmt: 5, Count: 1}, // New block
				},
			},
		},
		TotalStmt:   15,
		CoveredStmt: 12,
	}

	// Create diff info indicating lines 11-15 were added
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"github.com/test/file.go": {
				FileName: "github.com/test/file.go",
				AddedLines: map[int]bool{
					11: true,
					12: true,
					13: true,
					14: true,
					15: true,
				},
				ModifiedLines: map[int]bool{},
			},
		},
	}

	report := &Report{
		Old:          oldCov,
		New:          newCov,
		ChangedFiles: []string{"github.com/test/file.go"},
		DiffInfo:     diffInfo,
	}

	totalNew, coveredNew := report.calculateNewCodeCoverageFromDiff()

	// Should only count the new block (lines 11-15)
	assert.Equal(t, int64(5), totalNew, "Should count 5 new statements")
	assert.Equal(t, int64(5), coveredNew, "Should count 5 covered new statements")
}

func TestDiffInfo_PathNormalization(t *testing.T) {
	// Test that path normalization works correctly
	// Coverage files have full package paths, but git diff has relative paths
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"cmd/go-coverage-report/report.go": {
				FileName: "cmd/go-coverage-report/report.go",
				AddedLines: map[int]bool{
					10: true,
					20: true,
				},
				ModifiedLines: map[int]bool{},
			},
			"internal/utils/helper.go": {
				FileName: "internal/utils/helper.go",
				AddedLines: map[int]bool{
					5: true,
				},
				ModifiedLines: map[int]bool{},
			},
		},
	}

	// Test exact match
	assert.True(t, diffInfo.IsLineAdded("cmd/go-coverage-report/report.go", 10),
		"Exact path match should work")

	// Test with package prefix (coverage file format)
	assert.True(t, diffInfo.IsLineAdded("github.com/user/repo/cmd/go-coverage-report/report.go", 10),
		"Should match when coverage path has package prefix")
	assert.True(t, diffInfo.IsLineAdded("github.com/user/repo/cmd/go-coverage-report/report.go", 20),
		"Should match when coverage path has package prefix")

	// Test with different package prefix
	assert.True(t, diffInfo.IsLineAdded("github.com/another/project/internal/utils/helper.go", 5),
		"Should match with different package prefix")

	// Test non-matching paths
	assert.False(t, diffInfo.IsLineAdded("github.com/user/repo/cmd/go-coverage-report/report.go", 99),
		"Non-added line should return false")
	assert.False(t, diffInfo.IsLineAdded("github.com/user/repo/nonexistent.go", 10),
		"Non-existent file should return false")

	// Test IsLineInRange with normalized paths
	assert.True(t, diffInfo.IsLineInRange("github.com/user/repo/cmd/go-coverage-report/report.go", 5, 15),
		"Range should match with normalized path")
	assert.False(t, diffInfo.IsLineInRange("github.com/user/repo/cmd/go-coverage-report/report.go", 30, 40),
		"Range without added lines should return false")
}

func TestCalculateNewCodeCoverageFromDiff_PathMismatch(t *testing.T) {
	// Test the real-world scenario where coverage has full paths but diff has relative paths
	oldCov := &Coverage{
		Files: map[string]*Profile{
			"github.com/test/cmd/app/main.go": {
				FileName:    "github.com/test/cmd/app/main.go",
				TotalStmt:   10,
				CoveredStmt: 8,
				Blocks: []ProfileBlock{
					{StartLine: 1, EndLine: 10, NumStmt: 10, Count: 1},
				},
			},
		},
		TotalStmt:   10,
		CoveredStmt: 8,
	}

	newCov := &Coverage{
		Files: map[string]*Profile{
			"github.com/test/cmd/app/main.go": {
				FileName:    "github.com/test/cmd/app/main.go",
				TotalStmt:   15,
				CoveredStmt: 12,
				Blocks: []ProfileBlock{
					{StartLine: 1, EndLine: 10, NumStmt: 10, Count: 1},
					{StartLine: 11, EndLine: 15, NumStmt: 5, Count: 1}, // New block
				},
			},
		},
		TotalStmt:   15,
		CoveredStmt: 12,
	}

	// Diff has relative paths (as git diff produces)
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"cmd/app/main.go": {
				FileName: "cmd/app/main.go",
				AddedLines: map[int]bool{
					11: true,
					12: true,
					13: true,
					14: true,
					15: true,
				},
				ModifiedLines: map[int]bool{},
			},
		},
	}

	report := &Report{
		Old:          oldCov,
		New:          newCov,
		ChangedFiles: []string{"github.com/test/cmd/app/main.go"},
		DiffInfo:     diffInfo,
	}

	totalNew, coveredNew := report.calculateNewCodeCoverageFromDiff()

	// Should correctly match paths and count only the new block
	assert.Equal(t, int64(5), totalNew, "Should count 5 new statements despite path mismatch")
	assert.Equal(t, int64(5), coveredNew, "Should count 5 covered new statements despite path mismatch")
}
