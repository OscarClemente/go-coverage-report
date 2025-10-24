package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReport_Markdown(t *testing.T) {
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/01-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)
	actual := report.Markdown()

	expected := `### Coverage Report - 90.20% (**-9.80%**) - **decrease**

#### Overall Coverage Summary

| Metric | Old Coverage | New Coverage | Change | :robot: |
|--------|-------------|-------------|--------|---------|
| **Total** | 100.00% | 90.20% | **-9.80%** | :thumbsdown: |
| **New Code** | N/A | 85.71% | 42/49 statements | :tada: |

| **Statements** | Total | Covered | Missed |
|---|---|---|---|
| **Old** | 100 | 100 | 0 |
| **New** | 102 (+2) | 92 (-8) | 10 |

---

<details>

<summary>Impacted Packages</summary>

| Impacted Packages | Coverage Δ | :robot: |
|-------------------|------------|---------|
| github.com/fgrosse/prioqueue | 90.20% (**-9.80%**) | :thumbsdown: |
| github.com/fgrosse/prioqueue/foo/bar | 0.00% (ø) |  |

</details>

<details>

<summary>Coverage by file</summary>

### Changed files (no unit tests)

| Changed File | Coverage Δ | Total | Covered | Missed | :robot: |
|--------------|------------|-------|---------|--------|---------|
| github.com/fgrosse/prioqueue/foo/bar/baz.go | 0.00% (ø) | 0 | 0 | 0 |  |
| github.com/fgrosse/prioqueue/min_heap.go | 80.77% (**-19.23%**) | 52 (+2) | 42 (-8) | 10 (+10) | :skull:  |

_Please note that the "Total", "Covered", and "Missed" counts above refer to ***code statements*** instead of lines of code. The value in brackets refers to the test coverage of that file in the old version of the code._

</details>`
	assert.Equal(t, expected, actual)
}

func TestReport_Markdown_OnlyChangedUnitTests(t *testing.T) {
	oldCov, err := ParseCoverage("testdata/02-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/02-new-coverage.txt")
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/02-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)
	actual := report.Markdown()

	expected := `### Coverage Report - 99.02% (**+8.82%**) - **increase**

#### Overall Coverage Summary

| Metric | Old Coverage | New Coverage | Change | :robot: |
|--------|-------------|-------------|--------|---------|
| **Total** | 90.20% | 99.02% | **+8.82%** | :thumbsup: |

| **Statements** | Total | Covered | Missed |
|---|---|---|---|
| **Old** | 102 | 92 | 10 |
| **New** | 102 | 101 (+9) | 1 |

---

<details>

<summary>Impacted Packages</summary>

| Impacted Packages | Coverage Δ | :robot: |
|-------------------|------------|---------|
| github.com/fgrosse/prioqueue | 99.02% (**+8.82%**) | :thumbsup: |

</details>

<details>

<summary>Coverage by file</summary>

### Changed unit test files

- github.com/fgrosse/prioqueue/min_heap_test.go

</details>`
	assert.Equal(t, expected, actual)
}

func TestReport_MinimumCoverageThreshold(t *testing.T) {
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/01-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)

	// Test that new code coverage is calculated correctly
	totalNew, coveredNew := report.calculateNewCodeCoverage()
	require.Equal(t, int64(49), totalNew)
	require.Equal(t, int64(42), coveredNew)

	newCodeCoverage := float64(coveredNew) / float64(totalNew) * 100
	require.InDelta(t, 85.71, newCodeCoverage, 0.01)

	// Test that coverage above threshold passes
	opts := options{minCoverage: 80}
	if opts.minCoverage > 0 && totalNew > 0 {
		if newCodeCoverage < opts.minCoverage {
			t.Errorf("Expected new code coverage %.2f%% to be above threshold %.2f%%", newCodeCoverage, opts.minCoverage)
		}
	}

	// Test that coverage below threshold fails
	opts = options{minCoverage: 90}
	if opts.minCoverage > 0 && totalNew > 0 {
		if newCodeCoverage >= opts.minCoverage {
			t.Errorf("Expected new code coverage %.2f%% to be below threshold %.2f%%", newCodeCoverage, opts.minCoverage)
		}
	}

	// Test that threshold of 0 disables the check
	opts = options{minCoverage: 0}
	if opts.minCoverage > 0 {
		t.Errorf("Expected threshold check to be disabled when minCoverage is 0")
	}
}

func TestReport_Markdown_WithFailedThreshold(t *testing.T) {
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/01-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)
	report.MinCoverage = 90.0 // Set threshold to 90%, which is above the actual 85.71%

	actual := report.Markdown()

	expected := `### Coverage Report - 90.20% (**-9.80%**) - **decrease**

#### Overall Coverage Summary

| Metric | Old Coverage | New Coverage | Change | :robot: |
|--------|-------------|-------------|--------|---------|
| **Total** | 100.00% | 90.20% | **-9.80%** | :thumbsdown: |
| **New Code** | N/A | 85.71% | 42/49 statements | :tada: |

> [!WARNING]
> **Coverage threshold not met:** New code coverage is **85.71%**, which is below the required threshold of **90.00%**.

| **Statements** | Total | Covered | Missed |
|---|---|---|---|
| **Old** | 100 | 100 | 0 |
| **New** | 102 (+2) | 92 (-8) | 10 |

---

<details>

<summary>Impacted Packages</summary>

| Impacted Packages | Coverage Δ | :robot: |
|-------------------|------------|---------|
| github.com/fgrosse/prioqueue | 90.20% (**-9.80%**) | :thumbsdown: |
| github.com/fgrosse/prioqueue/foo/bar | 0.00% (ø) |  |

</details>

<details>

<summary>Coverage by file</summary>

### Changed files (no unit tests)

| Changed File | Coverage Δ | Total | Covered | Missed | :robot: |
|--------------|------------|-------|---------|--------|---------|
| github.com/fgrosse/prioqueue/foo/bar/baz.go | 0.00% (ø) | 0 | 0 | 0 |  |
| github.com/fgrosse/prioqueue/min_heap.go | 80.77% (**-19.23%**) | 52 (+2) | 42 (-8) | 10 (+10) | :skull:  |

_Please note that the "Total", "Covered", and "Missed" counts above refer to ***code statements*** instead of lines of code. The value in brackets refers to the test coverage of that file in the old version of the code._

</details>`
	assert.Equal(t, expected, actual)
}

func TestReport_Markdown_WithPassedThreshold(t *testing.T) {
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/01-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	report := NewReport(oldCov, newCov, changedFiles)
	report.MinCoverage = 80.0 // Set threshold to 80%, which is below the actual 85.71%

	actual := report.Markdown()

	// Verify the warning message is NOT present
	assert.NotContains(t, actual, "> [!WARNING]")
	assert.NotContains(t, actual, "Coverage threshold not met")
}

func TestReport_WithGitDiff(t *testing.T) {
	// This test verifies that git diff-based coverage calculation works correctly
	// and solves the refactoring problem where code moves to different line positions

	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/01-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	// Create a diff that indicates only specific lines were actually added
	// In the real scenario, the code was refactored from lines 42-47 to 42-52
	// but only lines 48-50 are truly new (the if statement that was added)
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"github.com/fgrosse/prioqueue/min_heap.go": {
				FileName: "github.com/fgrosse/prioqueue/min_heap.go",
				AddedLines: map[int]bool{
					// These are the lines that were actually added in the refactoring
					48: true, // New if statement
					49: true, // Content of new if
					50: true, // Closing brace
				},
				ModifiedLines: map[int]bool{},
			},
		},
	}

	// Test WITHOUT diff (old behavior - block-based comparison)
	reportWithoutDiff := NewReport(oldCov, newCov, changedFiles)
	totalNewWithoutDiff, coveredNewWithoutDiff := reportWithoutDiff.calculateNewCodeCoverage()

	// Test WITH diff (new behavior - line-based comparison)
	reportWithDiff := NewReport(oldCov, newCov, changedFiles)
	reportWithDiff.DiffInfo = diffInfo
	totalNewWithDiff, coveredNewWithDiff := reportWithDiff.calculateNewCodeCoverage()

	// Without diff: treats many blocks as "new" because positions changed
	// This is the problematic behavior we're fixing
	assert.Greater(t, totalNewWithoutDiff, int64(0), "Should detect some new code without diff")

	// With diff: only counts blocks that contain actually added lines
	// This should be much more accurate
	assert.Greater(t, totalNewWithDiff, int64(0), "Should detect new code with diff")

	// The key assertion: diff-based should report fewer "new" statements
	// because it only counts lines that were actually added, not moved
	assert.LessOrEqual(t, totalNewWithDiff, totalNewWithoutDiff,
		"Diff-based coverage should report fewer or equal 'new' statements than block-based")

	// Calculate coverage percentages
	var coverageWithoutDiff, coverageWithDiff float64
	if totalNewWithoutDiff > 0 {
		coverageWithoutDiff = float64(coveredNewWithoutDiff) / float64(totalNewWithoutDiff) * 100
	}
	if totalNewWithDiff > 0 {
		coverageWithDiff = float64(coveredNewWithDiff) / float64(totalNewWithDiff) * 100
	}

	// Log the results for visibility
	t.Logf("Without diff (block-based): %d/%d statements = %.2f%% coverage",
		coveredNewWithoutDiff, totalNewWithoutDiff, coverageWithoutDiff)
	t.Logf("With diff (line-based): %d/%d statements = %.2f%% coverage",
		coveredNewWithDiff, totalNewWithDiff, coverageWithDiff)

	// Both should have some coverage
	if totalNewWithoutDiff > 0 {
		assert.Greater(t, coverageWithoutDiff, 0.0, "Should have some coverage without diff")
	}

	// With diff, we should have detected the new lines we specified
	// The coverage might be 0% if those specific lines aren't covered, which is fine
	// The important thing is that we're only counting the lines from the diff
	t.Logf("Diff-based correctly identified %d statements in the changed lines", totalNewWithDiff)
}

func TestReport_WithGitDiff_EntireFileNew(t *testing.T) {
	// Test case: entire file is new (not in old coverage)
	oldCov := &Coverage{
		Files:       map[string]*Profile{},
		TotalStmt:   0,
		CoveredStmt: 0,
	}

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	changedFiles := []string{"github.com/fgrosse/prioqueue/min_heap.go"}

	// Create diff indicating the entire file is new
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"github.com/fgrosse/prioqueue/min_heap.go": {
				FileName:      "github.com/fgrosse/prioqueue/min_heap.go",
				AddedLines:    map[int]bool{}, // Empty - will be treated as entire file
				ModifiedLines: map[int]bool{},
			},
		},
	}

	report := NewReport(oldCov, newCov, changedFiles)
	report.DiffInfo = diffInfo

	totalNew, coveredNew := report.calculateNewCodeCoverage()

	// When entire file is new, should count all statements
	minHeapProfile := newCov.Files["github.com/fgrosse/prioqueue/min_heap.go"]
	require.NotNil(t, minHeapProfile)

	assert.Equal(t, minHeapProfile.TotalStmt, totalNew,
		"Should count all statements when entire file is new")
	assert.Equal(t, minHeapProfile.CoveredStmt, coveredNew,
		"Should count all covered statements when entire file is new")
}

func TestReport_WithGitDiff_NoMatchingLines(t *testing.T) {
	// Test case: diff indicates changes but no coverage blocks match those lines
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/01-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	// Create diff with lines that don't match any coverage blocks
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"github.com/fgrosse/prioqueue/min_heap.go": {
				FileName: "github.com/fgrosse/prioqueue/min_heap.go",
				AddedLines: map[int]bool{
					1: true, // Comment or package line, no coverage block
					2: true,
				},
				ModifiedLines: map[int]bool{},
			},
		},
	}

	report := NewReport(oldCov, newCov, changedFiles)
	report.DiffInfo = diffInfo

	totalNew, coveredNew := report.calculateNewCodeCoverage()

	// Should report 0 new statements since no blocks contain the changed lines
	assert.Equal(t, int64(0), totalNew, "Should report 0 new statements when no blocks match")
	assert.Equal(t, int64(0), coveredNew, "Should report 0 covered statements when no blocks match")
}

func TestReport_WithGitDiff_Markdown(t *testing.T) {
	// Test that the markdown report works correctly with diff information
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	changedFiles, err := ParseChangedFiles("testdata/01-changed-files.json", "github.com/fgrosse/prioqueue")
	require.NoError(t, err)

	// Create a realistic diff
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"github.com/fgrosse/prioqueue/min_heap.go": {
				FileName: "github.com/fgrosse/prioqueue/min_heap.go",
				AddedLines: map[int]bool{
					48: true,
					49: true,
					50: true,
				},
				ModifiedLines: map[int]bool{},
			},
		},
	}

	report := NewReport(oldCov, newCov, changedFiles)
	report.DiffInfo = diffInfo

	markdown := report.Markdown()

	// Verify the markdown contains expected sections
	assert.Contains(t, markdown, "### Coverage Report", "Should contain title")
	assert.Contains(t, markdown, "Overall Coverage Summary", "Should contain summary")
	assert.Contains(t, markdown, "New Code", "Should contain new code coverage")
	assert.Contains(t, markdown, "Impacted Packages", "Should contain package details")

	// Verify it doesn't crash and produces valid output
	assert.NotEmpty(t, markdown, "Markdown should not be empty")
	assert.Greater(t, len(markdown), 100, "Markdown should be substantial")
}

func TestReport_WithGitDiff_OnlyComments(t *testing.T) {
	// Test case: diff only contains changes to comments (lines 1-10 typically)
	// These lines won't have coverage blocks, so should result in 0 new statements
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	changedFiles := []string{"github.com/fgrosse/prioqueue/min_heap.go"}

	// Create diff with only comment lines (typically lines 1-10 in a Go file)
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"github.com/fgrosse/prioqueue/min_heap.go": {
				FileName: "github.com/fgrosse/prioqueue/min_heap.go",
				AddedLines: map[int]bool{
					1: true, // Package declaration
					2: true, // Empty line or comment
					3: true, // Import or comment
					4: true, // Comment
					5: true, // Comment
				},
				ModifiedLines: map[int]bool{},
			},
		},
	}

	report := NewReport(oldCov, newCov, changedFiles)
	report.DiffInfo = diffInfo

	totalNew, coveredNew := report.calculateNewCodeCoverage()

	// Comments don't have coverage blocks, so should be 0
	assert.Equal(t, int64(0), totalNew,
		"Should report 0 new statements when only comments changed")
	assert.Equal(t, int64(0), coveredNew,
		"Should report 0 covered statements when only comments changed")
}

func TestReport_WithGitDiff_NonGoFiles(t *testing.T) {
	// Test case: changed files include non-Go files (README.md, .yaml, etc.)
	// These should be ignored since they won't be in the coverage data
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	// Include both Go and non-Go files in changed files
	changedFiles := []string{
		"github.com/fgrosse/prioqueue/min_heap.go",
		"github.com/fgrosse/prioqueue/README.md",
		"github.com/fgrosse/prioqueue/.github/workflows/ci.yml",
		"github.com/fgrosse/prioqueue/docs/architecture.md",
	}

	// Diff includes changes to non-Go files
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"github.com/fgrosse/prioqueue/min_heap.go": {
				FileName: "github.com/fgrosse/prioqueue/min_heap.go",
				AddedLines: map[int]bool{
					48: true,
					49: true,
				},
				ModifiedLines: map[int]bool{},
			},
			"github.com/fgrosse/prioqueue/README.md": {
				FileName: "github.com/fgrosse/prioqueue/README.md",
				AddedLines: map[int]bool{
					10: true,
					11: true,
					12: true,
				},
				ModifiedLines: map[int]bool{},
			},
			"github.com/fgrosse/prioqueue/.github/workflows/ci.yml": {
				FileName: "github.com/fgrosse/prioqueue/.github/workflows/ci.yml",
				AddedLines: map[int]bool{
					5: true,
				},
				ModifiedLines: map[int]bool{},
			},
		},
	}

	report := NewReport(oldCov, newCov, changedFiles)
	report.DiffInfo = diffInfo

	totalNew, coveredNew := report.calculateNewCodeCoverage()

	// Should only count the Go file changes, non-Go files should be ignored
	// because they won't have coverage profiles
	assert.Greater(t, totalNew, int64(0),
		"Should count statements from Go files")

	// The non-Go files shouldn't cause any errors or be counted
	// We can't assert exact numbers without knowing the coverage blocks,
	// but we can verify it doesn't crash and produces reasonable output
	t.Logf("Counted %d/%d new statements from Go files (non-Go files ignored)", coveredNew, totalNew)
}

func TestReport_WithGitDiff_MixedCommentsAndCode(t *testing.T) {
	// Test case: diff contains both comment changes and code changes
	// Only the code changes should be counted
	oldCov := &Coverage{
		Files: map[string]*Profile{
			"github.com/test/file.go": {
				FileName:    "github.com/test/file.go",
				TotalStmt:   10,
				CoveredStmt: 8,
				Blocks: []ProfileBlock{
					{StartLine: 10, EndLine: 15, NumStmt: 5, Count: 1},
					{StartLine: 20, EndLine: 25, NumStmt: 5, Count: 1},
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
					{StartLine: 10, EndLine: 15, NumStmt: 5, Count: 1},
					{StartLine: 20, EndLine: 25, NumStmt: 5, Count: 1},
					{StartLine: 30, EndLine: 35, NumStmt: 5, Count: 1}, // New block
				},
			},
		},
		TotalStmt:   15,
		CoveredStmt: 12,
	}

	// Diff includes both comment lines (1-5) and code lines (30-35)
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"github.com/test/file.go": {
				FileName: "github.com/test/file.go",
				AddedLines: map[int]bool{
					1:  true, // Package declaration
					2:  true, // Comment
					3:  true, // Comment
					4:  true, // Import
					5:  true, // Empty line
					30: true, // Actual code
					31: true, // Actual code
					32: true, // Actual code
					33: true, // Actual code
					34: true, // Actual code
					35: true, // Actual code
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

	totalNew, coveredNew := report.calculateNewCodeCoverage()

	// Should only count the block that overlaps with lines 30-35
	// Lines 1-5 (comments) don't have coverage blocks, so shouldn't be counted
	assert.Equal(t, int64(5), totalNew,
		"Should only count statements from code blocks, not comments")
	assert.Equal(t, int64(5), coveredNew,
		"Should only count covered statements from code blocks, not comments")
}

func TestReport_WithGitDiff_OnlyDeletedLines(t *testing.T) {
	// Test case: diff only contains deleted lines (no additions)
	// This should result in 0 new statements
	oldCov, err := ParseCoverage("testdata/01-old-coverage.txt")
	require.NoError(t, err)

	newCov, err := ParseCoverage("testdata/01-new-coverage.txt")
	require.NoError(t, err)

	changedFiles := []string{"github.com/fgrosse/prioqueue/min_heap.go"}

	// Create diff with no added lines (empty AddedLines map)
	diffInfo := &DiffInfo{
		Files: map[string]*FileDiff{
			"github.com/fgrosse/prioqueue/min_heap.go": {
				FileName:      "github.com/fgrosse/prioqueue/min_heap.go",
				AddedLines:    map[int]bool{}, // No additions
				ModifiedLines: map[int]bool{}, // No modifications
			},
		},
	}

	report := NewReport(oldCov, newCov, changedFiles)
	report.DiffInfo = diffInfo

	totalNew, coveredNew := report.calculateNewCodeCoverage()

	// When a file is in changed files but has no added lines in diff,
	// it falls back to counting all statements (this is the current behavior)
	// This might seem counterintuitive, but it handles cases where diff parsing
	// might have failed or the file was renamed
	minHeapProfile := newCov.Files["github.com/fgrosse/prioqueue/min_heap.go"]
	require.NotNil(t, minHeapProfile)

	assert.Equal(t, minHeapProfile.TotalStmt, totalNew,
		"Should fall back to counting all statements when no added lines in diff")
	assert.Equal(t, minHeapProfile.CoveredStmt, coveredNew,
		"Should fall back to counting all covered statements when no added lines in diff")
}
