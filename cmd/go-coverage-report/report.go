package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type Report struct {
	Old, New        *Coverage
	ChangedFiles    []string
	ChangedPackages []string
}

func NewReport(oldCov, newCov *Coverage, changedFiles []string) *Report {
	sort.Strings(changedFiles)
	return &Report{
		Old:             oldCov,
		New:             newCov,
		ChangedFiles:    changedFiles,
		ChangedPackages: changedPackages(changedFiles),
	}
}

func changedPackages(changedFiles []string) []string {
	packages := map[string]bool{}
	for _, file := range changedFiles {
		pkg := filepath.Dir(file)
		packages[pkg] = true
	}

	result := make([]string, 0, len(packages))
	for pkg := range packages {
		result = append(result, pkg)
	}

	sort.Strings(result)

	return result
}

// OverallCoverageDelta returns the difference between new and old overall coverage
func (r *Report) OverallCoverageDelta() float64 {
	return r.New.Percent() - r.Old.Percent()
}

// OverallCoverageInfo returns formatted strings for old, new coverage percentages and delta
func (r *Report) OverallCoverageInfo() (oldCov, newCov, deltaStr string, emoji string) {
	oldPercent := r.Old.Percent()
	newPercent := r.New.Percent()

	oldCov = fmt.Sprintf("%.2f%%", oldPercent)
	newCov = fmt.Sprintf("%.2f%%", newPercent)

	emoji, deltaStr = emojiScore(newPercent, oldPercent)

	return oldCov, newCov, deltaStr, emoji
}

// PRCoverageInfo returns coverage information for newly added code in this PR
func (r *Report) PRCoverageInfo() (prCov string, emoji string, totalNew, coveredNew int64) {
	totalNew, coveredNew = r.calculateNewCodeCoverage()

	var prPercent float64
	if totalNew > 0 {
		prPercent = float64(coveredNew) / float64(totalNew) * 100
	}

	prCov = fmt.Sprintf("%.2f%%", prPercent)

	// Use a simplified emoji scoring for PR coverage
	switch {
	case prPercent >= 90:
		emoji = ":star2:"
	case prPercent >= 80:
		emoji = ":tada:"
	case prPercent >= 70:
		emoji = ":thumbsup:"
	case prPercent >= 50:
		emoji = ":neutral_face:"
	case prPercent >= 30:
		emoji = ":thumbsdown:"
	default:
		emoji = ":skull:"
	}

	return prCov, emoji, totalNew, coveredNew
}

// calculateNewCodeCoverage calculates coverage for statements that are new in this PR
func (r *Report) calculateNewCodeCoverage() (totalNew, coveredNew int64) {
	for _, fileName := range r.ChangedFiles {
		oldProfile := r.Old.Files[fileName]
		newProfile := r.New.Files[fileName]

		if newProfile == nil {
			continue // File was deleted or no coverage data
		}

		if oldProfile == nil {
			// Entire file is new
			totalNew += newProfile.TotalStmt
			coveredNew += newProfile.CoveredStmt
			continue
		}

		// Compare blocks to find new code
		oldBlocks := makeBlockMap(oldProfile.Blocks)

		for _, newBlock := range newProfile.Blocks {
			blockKey := fmt.Sprintf("%d:%d-%d:%d", newBlock.StartLine, newBlock.StartCol, newBlock.EndLine, newBlock.EndCol)

			if _, exists := oldBlocks[blockKey]; !exists {
				// This block is new in this PR
				totalNew += int64(newBlock.NumStmt)
				if newBlock.Count > 0 {
					coveredNew += int64(newBlock.NumStmt)
				}
			}
		}
	}

	return totalNew, coveredNew
}

// makeBlockMap creates a map of blocks for quick lookup
func makeBlockMap(blocks []ProfileBlock) map[string]ProfileBlock {
	blockMap := make(map[string]ProfileBlock)
	for _, block := range blocks {
		key := fmt.Sprintf("%d:%d-%d:%d", block.StartLine, block.StartCol, block.EndLine, block.EndCol)
		blockMap[key] = block
	}
	return blockMap
}

func (r *Report) Title() string {
	// Use overall coverage delta to determine increase/decrease
	overallDelta := r.OverallCoverageDelta()
	_, newCov, deltaStr, _ := r.OverallCoverageInfo()

	switch {
	case overallDelta == 0:
		return fmt.Sprintf("### Coverage Report - %s (no change)", newCov)
	case overallDelta > 0:
		return fmt.Sprintf("### Coverage Report - %s (%s) - **increase**", newCov, deltaStr)
	case overallDelta < 0:
		return fmt.Sprintf("### Coverage Report - %s (%s) - **decrease**", newCov, deltaStr)
	default:
		// This should never happen, but just in case
		return fmt.Sprintf("### Coverage Report - %s (%s)", newCov, deltaStr)
	}
}

func (r *Report) Markdown() string {
	report := new(strings.Builder)

	fmt.Fprintln(report, r.Title())
	r.addOverallCoverageSummary(report)
	r.addPackageDetails(report)
	r.addFileDetails(report)

	return report.String()
}

func (r *Report) addOverallCoverageSummary(report *strings.Builder) {
	oldCov, newCov, deltaStr, emoji := r.OverallCoverageInfo()
	prCov, prEmoji, totalNew, coveredNew := r.PRCoverageInfo()

	fmt.Fprintln(report)
	fmt.Fprintln(report, "#### Overall Coverage Summary")
	fmt.Fprintln(report)
	fmt.Fprintln(report, "| Metric | Old Coverage | New Coverage | Change | :robot: |")
	fmt.Fprintln(report, "|--------|-------------|-------------|--------|---------|")
	fmt.Fprintf(report, "| **Total** | %s | %s | %s | %s |\n", oldCov, newCov, deltaStr, emoji)

	// Add PR-specific coverage if there's new code
	if totalNew > 0 {
		fmt.Fprintf(report, "| **New Code** | N/A | %s | %d/%d statements | %s |\n", prCov, coveredNew, totalNew, prEmoji)
	}

	fmt.Fprintln(report)

	// Add statements summary
	oldStmt := r.Old.TotalStmt
	newStmt := r.New.TotalStmt
	oldCovered := r.Old.CoveredStmt
	newCovered := r.New.CoveredStmt

	stmtChange := newStmt - oldStmt
	coveredChange := newCovered - oldCovered

	stmtChangeStr := ""
	if stmtChange > 0 {
		stmtChangeStr = fmt.Sprintf(" (+%d)", stmtChange)
	} else if stmtChange < 0 {
		stmtChangeStr = fmt.Sprintf(" (%d)", stmtChange)
	}

	coveredChangeStr := ""
	if coveredChange > 0 {
		coveredChangeStr = fmt.Sprintf(" (+%d)", coveredChange)
	} else if coveredChange < 0 {
		coveredChangeStr = fmt.Sprintf(" (%d)", coveredChange)
	}

	fmt.Fprintln(report, "| **Statements** | Total | Covered | Missed |")
	fmt.Fprintln(report, "|---|---|---|---|")
	fmt.Fprintf(report, "| **Old** | %d | %d | %d |\n", oldStmt, oldCovered, r.Old.MissedStmt)
	fmt.Fprintf(report, "| **New** | %d%s | %d%s | %d |\n", newStmt, stmtChangeStr, newCovered, coveredChangeStr, r.New.MissedStmt)
	fmt.Fprintln(report)
}

func (r *Report) addPackageDetails(report *strings.Builder) {
	fmt.Fprintln(report, "---")
	fmt.Fprintln(report)
	fmt.Fprintln(report, "<details>")
	fmt.Fprintln(report)
	fmt.Fprintln(report, "<summary>Impacted Packages</summary>")
	fmt.Fprintln(report)

	fmt.Fprintln(report, "| Impacted Packages | Coverage Δ | :robot: |")
	fmt.Fprintln(report, "|-------------------|------------|---------|")

	oldCovPkgs := r.Old.ByPackage()
	newCovPkgs := r.New.ByPackage()
	for _, pkg := range r.ChangedPackages {
		var oldPercent, newPercent float64

		if cov, ok := oldCovPkgs[pkg]; ok {
			oldPercent = cov.Percent()
		}

		if cov, ok := newCovPkgs[pkg]; ok {
			newPercent = cov.Percent()
		}

		emoji, diffStr := emojiScore(newPercent, oldPercent)
		fmt.Fprintf(report, "| %s | %.2f%% (%s) | %s |\n",
			pkg,
			newPercent,
			diffStr,
			emoji,
		)
	}

	fmt.Fprintln(report)
	fmt.Fprintln(report, "</details>")
	fmt.Fprintln(report)
}

func (r *Report) addFileDetails(report *strings.Builder) {
	fmt.Fprintln(report, "<details>")
	fmt.Fprintln(report)

	fmt.Fprintln(report, "<summary>Coverage by file</summary>")
	fmt.Fprintln(report)

	var codeFiles, unitTestFiles []string
	for _, f := range r.ChangedFiles {
		if strings.HasSuffix(f, "_test.go") {
			unitTestFiles = append(unitTestFiles, f)
		} else {
			codeFiles = append(codeFiles, f)
		}
	}

	if len(codeFiles) > 0 {
		r.addCodeFileDetails(report, codeFiles)
	}
	if len(unitTestFiles) > 0 {
		r.addTestFileDetails(report, unitTestFiles)
	}

	fmt.Fprint(report, "</details>")
}

func (r *Report) addCodeFileDetails(report *strings.Builder, files []string) {
	fmt.Fprintln(report, "### Changed files (no unit tests)")
	fmt.Fprintln(report)
	fmt.Fprintln(report, "| Changed File | Coverage Δ | Total | Covered | Missed | :robot: |")
	fmt.Fprintln(report, "|--------------|------------|-------|---------|--------|---------|")

	for _, name := range files {
		var oldPercent, newPercent float64

		oldProfile := r.Old.Files[name]
		newProfile := r.New.Files[name]

		if oldProfile != nil {
			oldPercent = oldProfile.CoveragePercent()
		}

		if newProfile != nil {
			newPercent = newProfile.CoveragePercent()
		}

		valueWithDelta := func(oldVal, newVal int64) string {
			diff := oldVal - newVal
			switch {
			case diff < 0:
				return fmt.Sprintf("%d (+%d)", newVal, -diff)
			case diff > 0:
				return fmt.Sprintf("%d (-%d)", newVal, diff)
			default:
				return fmt.Sprintf("%d", newVal)
			}
		}

		emoji, diffStr := emojiScore(newPercent, oldPercent)
		fmt.Fprintf(report, "| %s | %.2f%% (%s) | %s | %s | %s | %s |\n",
			name,
			newPercent, diffStr,
			valueWithDelta(oldProfile.GetTotal(), newProfile.GetTotal()),
			valueWithDelta(oldProfile.GetCovered(), newProfile.GetCovered()),
			valueWithDelta(oldProfile.GetMissed(), newProfile.GetMissed()),
			emoji,
		)
	}

	fmt.Fprintln(report)
	fmt.Fprintln(report, `_Please note that the "Total", "Covered", and "Missed" counts `+
		"above refer to ***code statements*** instead of lines of code. The value in brackets "+
		"refers to the test coverage of that file in the old version of the code._")
	fmt.Fprintln(report)
}

func (r *Report) addTestFileDetails(report *strings.Builder, files []string) {
	fmt.Fprintln(report, "### Changed unit test files")
	fmt.Fprintln(report)

	for _, name := range files {
		fmt.Fprintf(report, "- %s\n", name)
	}

	fmt.Fprintln(report)
}

func (r *Report) JSON() string {
	data, err := json.MarshalIndent(r, "", "    ")
	if err != nil {
		panic(err) // should never happen
	}

	return string(data)
}

func (r *Report) TrimPrefix(prefix string) {
	for i, name := range r.ChangedPackages {
		r.ChangedPackages[i] = trimPrefix(name, prefix)
	}
	for i, name := range r.ChangedFiles {
		r.ChangedFiles[i] = trimPrefix(name, prefix)
	}

	r.Old.TrimPrefix(prefix)
	r.New.TrimPrefix(prefix)
}

func trimPrefix(name, prefix string) string {
	trimmed := strings.TrimPrefix(name, prefix)
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		trimmed = "."
	}

	return trimmed
}

func emojiScore(newPercent, oldPercent float64) (emoji, diffStr string) {
	diff := newPercent - oldPercent
	switch {
	case diff < -50:
		emoji = strings.Repeat(":skull: ", 5)
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	case diff < -10:
		emoji = strings.Repeat(":skull: ", int(-diff/10))
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	case diff < 0:
		emoji = ":thumbsdown:"
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	case diff == 0:
		emoji = ""
		diffStr = "ø"
	case diff > 20:
		emoji = ":star2:"
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	case diff > 10:
		emoji = ":tada:"
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	case diff > 0:
		emoji = ":thumbsup:"
		diffStr = fmt.Sprintf("**%+.2f%%**", diff)
	}

	return emoji, diffStr
}
