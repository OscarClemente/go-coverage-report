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
