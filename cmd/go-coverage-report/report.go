package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Report struct {
	Old, New        *Coverage
	ChangedFiles    []string
	ChangedPackages []string
	MinCoverage     float64   // Minimum coverage threshold for new code (0 to disable)
	DiffInfo        *DiffInfo // Optional: git diff information for line-level coverage
	astMapper       *StatementLineMapper
	astCache        map[string]map[int]bool // Cache of file -> statement lines
}

func NewReport(oldCov, newCov *Coverage, changedFiles []string) *Report {
	sort.Strings(changedFiles)
	return &Report{
		Old:             oldCov,
		astMapper:       NewStatementLineMapper(),
		astCache:        make(map[string]map[int]bool),
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

// NewCodeBlock represents a block of new code with coverage information
type NewCodeBlock struct {
	FileName  string
	StartLine int
	EndLine   int
	NumStmt   int
	Covered   bool
	Lines     []string // Actual source code lines
}

// calculateNewCodeCoverage calculates coverage for statements that are new in this PR
func (r *Report) calculateNewCodeCoverage() (totalNew, coveredNew int64) {
	// If we have diff information, use it for accurate line-level coverage
	if r.DiffInfo != nil {
		return r.calculateNewCodeCoverageFromDiff()
	}

	// Fallback to block-based comparison (old behavior)
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

// readSourceLines reads lines from a source file
// Returns a map of line numbers to their content
func readSourceLines(fileName string) (map[int]string, error) {
	// Try multiple paths to find the source file
	pathsToTry := []string{
		fileName, // Original path (e.g., "github.com/user/repo/pkg/file.go")
	}

	// Try stripping common package path prefixes to get relative path
	// Coverage files often have full package paths like "github.com/user/repo/pkg/file.go"
	// but the actual file is at "./pkg/file.go"
	parts := strings.Split(fileName, "/")
	for i := range parts {
		if i > 0 {
			// Try progressively shorter paths
			// e.g., "user/repo/pkg/file.go", "repo/pkg/file.go", "pkg/file.go"
			relativePath := filepath.Join(parts[i:]...)
			pathsToTry = append(pathsToTry, relativePath)
		}
	}

	// Also try testdata directory (for test files)
	pathsToTry = append(pathsToTry, filepath.Join("testdata", fileName))

	var file *os.File
	var err error

	for _, path := range pathsToTry {
		file, err = os.Open(path)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, err
	}
	defer file.Close()

	lines := make(map[int]string)
	scanner := bufio.NewScanner(file)
	lineNum := 1
	for scanner.Scan() {
		lines[lineNum] = scanner.Text()
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// getNewCodeBlocks returns detailed information about all new code blocks
func (r *Report) getNewCodeBlocks() []NewCodeBlock {
	var blocks []NewCodeBlock

	// If we have diff information, use it for accurate line-level coverage
	if r.DiffInfo != nil {
		blocks = r.getNewCodeBlocksFromDiff()
	} else {
		blocks = r.getNewCodeBlocksFromComparison()
	}

	// Try to populate actual source code lines for each block
	// Only include lines that were actually added/modified according to the diff
	fileCache := make(map[string]map[int]string)
	for i := range blocks {
		block := &blocks[i]

		// Check if we've already read this file
		sourceLines, ok := fileCache[block.FileName]
		if !ok {
			// Try to read the file
			var err error
			sourceLines, err = readSourceLines(block.FileName)
			if err != nil {
				// If we can't read the file, just skip adding source lines
				// This can happen if the file path doesn't exist locally
				fileCache[block.FileName] = nil
				continue
			}
			fileCache[block.FileName] = sourceLines
		}

		if sourceLines != nil {
			// Extract only the lines that were actually added/modified
			// This prevents showing unchanged lines that happen to be in the same coverage block
			for lineNum := block.StartLine; lineNum <= block.EndLine; lineNum++ {
				// Only include lines that are in the diff (added or modified)
				if r.DiffInfo != nil {
					fileDiff := r.DiffInfo.findFileDiff(block.FileName)
					if fileDiff != nil {
						// Only add lines that were actually changed
						if !fileDiff.AddedLines[lineNum] && !fileDiff.ModifiedLines[lineNum] {
							continue
						}
					}
				}
				
				if line, exists := sourceLines[lineNum]; exists {
					block.Lines = append(block.Lines, line)
				}
			}
		}
	}

	return blocks
}

// getNewCodeBlocksFromComparison gets new code blocks by comparing old and new profiles
func (r *Report) getNewCodeBlocksFromComparison() []NewCodeBlock {
	var blocks []NewCodeBlock

	for _, fileName := range r.ChangedFiles {
		oldProfile := r.Old.Files[fileName]
		newProfile := r.New.Files[fileName]

		if newProfile == nil {
			continue // File was deleted or no coverage data
		}

		if oldProfile == nil {
			// Entire file is new
			for _, block := range newProfile.Blocks {
				blocks = append(blocks, NewCodeBlock{
					FileName:  fileName,
					StartLine: block.StartLine,
					EndLine:   block.EndLine,
					NumStmt:   block.NumStmt,
					Covered:   block.Count > 0,
				})
			}
			continue
		}

		// Compare blocks to find new code
		oldBlocks := makeBlockMap(oldProfile.Blocks)

		for _, newBlock := range newProfile.Blocks {
			blockKey := fmt.Sprintf("%d:%d-%d:%d", newBlock.StartLine, newBlock.StartCol, newBlock.EndLine, newBlock.EndCol)

			if _, exists := oldBlocks[blockKey]; !exists {
				// This block is new in this PR
				blocks = append(blocks, NewCodeBlock{
					FileName:  fileName,
					StartLine: newBlock.StartLine,
					EndLine:   newBlock.EndLine,
					NumStmt:   newBlock.NumStmt,
					Covered:   newBlock.Count > 0,
				})
			}
		}
	}

	return blocks
}

// getNewCodeBlocksFromDiff gets new code blocks using git diff information
func (r *Report) getNewCodeBlocksFromDiff() []NewCodeBlock {
	var blocks []NewCodeBlock

	for _, fileName := range r.ChangedFiles {
		oldProfile := r.Old.Files[fileName]
		newProfile := r.New.Files[fileName]

		if newProfile == nil {
			continue // File was deleted or no coverage data
		}

		// If file is entirely new (not in old coverage), count all blocks
		if oldProfile == nil {
			for _, block := range newProfile.Blocks {
				blocks = append(blocks, NewCodeBlock{
					FileName:  fileName,
					StartLine: block.StartLine,
					EndLine:   block.EndLine,
					NumStmt:   block.NumStmt,
					Covered:   block.Count > 0,
				})
			}
			continue
		}

		// Check if we have diff info for this file
		fileDiff := r.DiffInfo.findFileDiff(fileName)
		if fileDiff == nil || len(fileDiff.AddedLines) == 0 {
			// No diff info for this file, fall back to counting all blocks as new
			for _, block := range newProfile.Blocks {
				blocks = append(blocks, NewCodeBlock{
					FileName:  fileName,
					StartLine: block.StartLine,
					EndLine:   block.EndLine,
					NumStmt:   block.NumStmt,
					Covered:   block.Count > 0,
				})
			}
			continue
		}

		// Check each block in the new coverage
		for _, block := range newProfile.Blocks {
			// Check if this block contains any lines that were added/modified
			if r.DiffInfo.IsLineInRange(fileName, block.StartLine, block.EndLine) {
				blocks = append(blocks, NewCodeBlock{
					FileName:  fileName,
					StartLine: block.StartLine,
					EndLine:   block.EndLine,
					NumStmt:   block.NumStmt,
					Covered:   block.Count > 0,
				})
			}
		}
	}

	return blocks
}

// calculateNewCodeCoverageFromDiff calculates coverage using git diff information
// This is more accurate as it only considers lines that were actually added/modified
//
// Note: Go coverage works at the block/statement level, not line level. A coverage block
// may span multiple lines, and we can't know which specific lines contain which statements.
// When a block contains both changed and unchanged lines, we estimate the number of changed
// statements based on the proportion of changed lines in that block.
func (r *Report) calculateNewCodeCoverageFromDiff() (totalNew, coveredNew int64) {
	for _, fileName := range r.ChangedFiles {
		oldProfile := r.Old.Files[fileName]
		newProfile := r.New.Files[fileName]

		if newProfile == nil {
			continue // File was deleted or no coverage data
		}

		// If file is entirely new (not in old coverage), count all statements
		if oldProfile == nil {
			totalNew += newProfile.TotalStmt
			coveredNew += newProfile.CoveredStmt
			continue
		}

		// Check if we have diff info for this file
		fileDiff := r.DiffInfo.findFileDiff(fileName)
		if fileDiff == nil || len(fileDiff.AddedLines) == 0 {
			// No diff info for this file, fall back to counting all blocks as new
			// This handles the case where diff wasn't generated for this file
			totalNew += newProfile.TotalStmt
			coveredNew += newProfile.CoveredStmt
			continue
		}

		// Check each block in the new coverage
		for _, block := range newProfile.Blocks {
			// Try AST-based counting first (more accurate)
			stmtCount, covered := r.countStatementsInBlockUsingAST(fileName, block, fileDiff)
			
			if stmtCount >= 0 {
				// AST-based counting succeeded
				totalNew += int64(stmtCount)
				if covered {
					coveredNew += int64(stmtCount)
				}
				continue
			}
			
			// Fallback to proportional estimation if AST parsing fails
			changedLinesInBlock := 0
			totalLinesInBlock := block.EndLine - block.StartLine + 1
			
			for line := block.StartLine; line <= block.EndLine; line++ {
				if fileDiff.AddedLines[line] || fileDiff.ModifiedLines[line] {
					changedLinesInBlock++
				}
			}
			
			// Only count this block if at least one line was changed
			// Estimate the number of statements that were changed based on the proportion of changed lines
			if changedLinesInBlock > 0 {
				// Calculate the proportion of lines that were changed
				proportion := float64(changedLinesInBlock) / float64(totalLinesInBlock)
				
				// Estimate the number of statements that were actually new/changed
				// Round up to ensure we count at least 1 statement if any line changed
				estimatedStmts := int64(float64(block.NumStmt) * proportion)
				if estimatedStmts == 0 && changedLinesInBlock > 0 {
					estimatedStmts = 1
				}
				
				totalNew += estimatedStmts
				if block.Count > 0 {
					coveredNew += estimatedStmts
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
	r.addNewCodeDetailsSection(report)

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

	// Add threshold warning if enabled and not met this will make the CI Step fail
	if r.MinCoverage > 0 && totalNew > 0 {
		newCodeCoverage := float64(coveredNew) / float64(totalNew) * 100
		if newCodeCoverage < r.MinCoverage {
			fmt.Fprintln(report, "> [!WARNING]")
			fmt.Fprintf(report, "> **Coverage threshold not met:** New code coverage is **%.2f%%**, which is below the required threshold of **%.2f%%**.\n", newCodeCoverage, r.MinCoverage)
			fmt.Fprintln(report)
		}
	}

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

// addNewCodeDetailsSection adds the new code coverage details section at the end of the report
func (r *Report) addNewCodeDetailsSection(report *strings.Builder) {
	// Check if there's new code to report
	totalNew, _ := r.calculateNewCodeCoverage()
	if totalNew == 0 {
		return
	}

	r.addNewCodeDetails(report)
}

// addNewCodeDetails adds a detailed breakdown of new code coverage
func (r *Report) addNewCodeDetails(report *strings.Builder) {
	blocks := r.getNewCodeBlocks()
	if len(blocks) == 0 {
		return
	}

	// Group blocks by file
	fileBlocks := make(map[string][]NewCodeBlock)
	for _, block := range blocks {
		fileBlocks[block.FileName] = append(fileBlocks[block.FileName], block)
	}

	// Sort files for consistent output
	var sortedFiles []string
	for fileName := range fileBlocks {
		sortedFiles = append(sortedFiles, fileName)
	}
	sort.Strings(sortedFiles)

	fmt.Fprintln(report, "<details>")
	fmt.Fprintln(report)
	fmt.Fprintln(report, "<summary>New Code Coverage Details</summary>")
	fmt.Fprintln(report)
	fmt.Fprintln(report, "This section shows the coverage status of each new code block added in this PR.")
	fmt.Fprintln(report)

	for _, fileName := range sortedFiles {
		blocks := fileBlocks[fileName]

		fmt.Fprintf(report, "#### %s\n", fileName)
		fmt.Fprintln(report)
		fmt.Fprintln(report, "```diff")

		for _, block := range blocks {
			// If we have actual source lines, display them
			if len(block.Lines) > 0 {
				prefix := "+"
				if !block.Covered {
					prefix = "-"
				}

				for _, line := range block.Lines {
					fmt.Fprintf(report, "%s %s\n", prefix, line)
				}
			} else {
				// Fallback to line number display if source is not available
				lineRange := fmt.Sprintf("Lines %d-%d", block.StartLine, block.EndLine)
				if block.StartLine == block.EndLine {
					lineRange = fmt.Sprintf("Line %d", block.StartLine)
				}

				stmtText := "statement"
				if block.NumStmt != 1 {
					stmtText = "statements"
				}

				if block.Covered {
					// Green for covered code
					fmt.Fprintf(report, "+ %s (%d %s) - COVERED ✓\n", lineRange, block.NumStmt, stmtText)
				} else {
					// Red for uncovered code
					fmt.Fprintf(report, "- %s (%d %s) - NOT COVERED ✗\n", lineRange, block.NumStmt, stmtText)
				}
			}
		}

		fmt.Fprintln(report, "```")
		fmt.Fprintln(report)
	}

	fmt.Fprintln(report, "</details>")
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

// countStatementsInBlockUsingAST uses AST parsing to accurately count statements
// in changed lines within a coverage block. Returns -1 if AST parsing fails.
func (r *Report) countStatementsInBlockUsingAST(fileName string, block ProfileBlock, fileDiff *FileDiff) (count int, covered bool) {
	// Check if AST mapper is available
	if r.astMapper == nil {
		return -1, false
	}
	
	// Get or compute statement lines for this file
	statementLines, ok := r.astCache[fileName]
	if !ok {
		// Try to resolve the file path
		paths := r.resolveFilePath(fileName)
		var err error
		
		for _, path := range paths {
			statementLines, err = r.astMapper.GetStatementLines(path)
			if err == nil {
				r.astCache[fileName] = statementLines
				break
			}
		}
		
		if err != nil {
			// AST parsing failed, return -1 to indicate fallback needed
			return -1, false
		}
	}
	
	// Count statements on changed lines within this block
	count = 0
	for line := block.StartLine; line <= block.EndLine; line++ {
		// Check if this line was changed and contains a statement
		if (fileDiff.AddedLines[line] || fileDiff.ModifiedLines[line]) && statementLines[line] {
			count++
		}
	}
	
	// If no statements found on changed lines, return -1 to use fallback
	if count == 0 {
		return -1, false
	}
	
	covered = block.Count > 0
	return count, covered
}

// resolveFilePath tries multiple paths to locate the source file
func (r *Report) resolveFilePath(fileName string) []string {
	paths := []string{fileName}
	
	// Try stripping package path prefixes
	parts := strings.Split(fileName, "/")
	for i := range parts {
		if i > 0 {
			relativePath := filepath.Join(parts[i:]...)
			paths = append(paths, relativePath)
		}
	}
	
	// Try testdata directory
	paths = append(paths, filepath.Join("testdata", fileName))
	
	return paths
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
