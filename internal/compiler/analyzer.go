package compiler

import (
	"fmt"
	"os"
	"strings"
)

// FuncSize represents a named function and its size in the WASM binary
type FuncSize struct {
	Name   string
	Size   int64
	Offset int64
}

// SectionSize represents a WASM section
type SectionSize struct {
	Name string
	Size int64
}

// SizeReport holds the analyzed size data from a .wasm file
type SizeReport struct {
	Path      string
	TotalSize int64
	Functions []FuncSize
	Sections  []SectionSize
}

// AnalyzeWASM reads a .wasm file and produces a basic size report.
// Parses the WASM binary format directly (no external parser dependency).
func AnalyzeWASM(wasmPath string) (*SizeReport, error) {
	data, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("analyzer: cannot read %s: %w", wasmPath, err)
	}

	report := &SizeReport{
		Path:      wasmPath,
		TotalSize: int64(len(data)),
	}

	if len(data) < 8 {
		return report, fmt.Errorf("analyzer: not a valid WASM file")
	}

	// WASM binary format: magic (4 bytes) + version (4 bytes) + sections
	// Section header: id (1 byte) + size (LEB128 u32) + content
	pos := 8 // skip magic + version

	sectionNames := map[byte]string{
		0:  "custom",
		1:  "type",
		2:  "import",
		3:  "function",
		4:  "table",
		5:  "memory",
		6:  "global",
		7:  "export",
		8:  "start",
		9:  "element",
		10: "code",
		11: "data",
		12: "data_count",
	}

	var nameMap map[uint32]string // function index → name (from name section)

	for pos < len(data) {
		sectionID := data[pos]
		pos++
		sectionSize, bytesRead := readLEB128(data[pos:])
		pos += bytesRead

		sectionEnd := pos + int(sectionSize)

		name, ok := sectionNames[sectionID]
		if !ok {
			name = fmt.Sprintf("section_%d", sectionID)
		}

		section := SectionSize{
			Name: name,
			Size: sectionSize,
		}
		report.Sections = append(report.Sections, section)

		// Extract function sizes from code section (id 10)
		if sectionID == 10 {
			funcCount, fr := readLEB128(data[pos:])
			funcPos := pos + fr

			for i := uint32(0); i < uint32(funcCount) && funcPos < sectionEnd; i++ {
				bodySize, br := readLEB128(data[funcPos:])
				funcPos += br

				funcName := fmt.Sprintf("func_%d", i)
				if nameMap != nil {
					if n, exists := nameMap[i]; exists {
						funcName = n
					}
				}

				report.Functions = append(report.Functions, FuncSize{
					Name:   funcName,
					Size:   bodySize,
					Offset: int64(funcPos),
				})
				funcPos += int(bodySize)
			}
		}

		// Extract function names from name custom section (id 0)
		if sectionID == 0 && pos < sectionEnd {
			sectionStr := string(data[pos:min(sectionEnd, pos+200)]) // check first 200 bytes
			if strings.HasPrefix(sectionStr, "name") {
				nameMap = parseNameSection(data, pos, sectionEnd)
			}
		}

		pos = sectionEnd
	}

	return report, nil
}

// AnalyzeMulti analyzes all .wasm chunks and produces a combined report
func AnalyzeMulti(chunksDir string) (*SizeReport, error) {
	entries, err := os.ReadDir(chunksDir)
	if err != nil {
		return nil, fmt.Errorf("analyzer: cannot read chunks dir: %w", err)
	}

	combined := &SizeReport{
		Path: chunksDir,
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}

		wasmPath := chunksDir + "/" + entry.Name()
		rep, err := AnalyzeWASM(wasmPath)
		if err != nil {
			continue
		}

		combined.TotalSize += rep.TotalSize
		combined.Functions = append(combined.Functions, FuncSize{
			Name: entry.Name(),
			Size: rep.TotalSize,
		})
	}

	return combined, nil
}

// BuildReportHTML generates an HTML report from a size report
func BuildReportHTML(report *SizeReport) string {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Nguyen.go Bundle Analyzer</title>
<style>
  body { font-family: system-ui; max-width: 800px; margin: 2rem auto; padding: 0 1rem; }
  .bar { height: 20px; background: #6366f1; border-radius: 4px; margin: 4px 0; }
  .bar-wrap { background: #e5e7eb; border-radius: 4px; margin: 8px 0; }
  .label { display: flex; justify-content: space-between; font-size: 14px; }
  h1 { color: #6366f1; }
  table { width: 100%; border-collapse: collapse; margin: 1rem 0; }
  th, td { padding: 8px 12px; text-align: left; border-bottom: 1px solid #e5e7eb; }
  th { font-weight: 600; color: #6b7280; }
  .total { font-size: 24px; font-weight: 700; color: #6366f1; }
</style>
</head>
<body>
<h1>⬡ Nguyen.go Bundle Analyzer</h1>
`)

	sb.WriteString(fmt.Sprintf(`<p class="total">Total: %s</p>`, formatSizeHuman(report.TotalSize)))

	if len(report.Functions) > 0 && report.Functions[0].Offset == 0 {
		// Per-chunk report (from AnalyzeMulti)
		sb.WriteString("<h2>Chunks</h2>\n")
		maxSize := int64(0)
		for _, f := range report.Functions {
			if f.Size > maxSize {
				maxSize = f.Size
			}
		}
		for _, f := range report.Functions {
			pct := float64(f.Size) / float64(maxSize) * 100
			sb.WriteString(fmt.Sprintf(`<div class="label"><span>%s</span><span>%s</span></div>`,
				f.Name, formatSizeHuman(f.Size)))
			sb.WriteString(fmt.Sprintf(`<div class="bar-wrap"><div class="bar" style="width:%.1f%%"></div></div>`, pct))
		}
	}

	sb.WriteString("<h2>Sections</h2>\n")
	sb.WriteString("<table><tr><th>Section</th><th>Size</th></tr>\n")
	maxSection := int64(0)
	for _, s := range report.Sections {
		if s.Size > maxSection {
			maxSection = s.Size
		}
	}
	for _, s := range report.Sections {
		sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>\n", s.Name, formatSizeHuman(s.Size)))
	}
	sb.WriteString("</table>\n")

	if len(report.Functions) > 0 && report.Functions[0].Offset > 0 {
		sb.WriteString("<h2>Top Functions</h2>\n<table><tr><th>Function</th><th>Size</th></tr>\n")
		for i, f := range report.Functions {
			if i >= 20 {
				break
			}
			sb.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td></tr>\n", f.Name, formatSizeHuman(f.Size)))
		}
		sb.WriteString("</table>\n")
	}

	sb.WriteString("</body></html>")
	return sb.String()
}

// --- Helpers ---

// readLEB128 reads an unsigned LEB128 integer and returns (value, bytesRead)
func readLEB128(data []byte) (int64, int) {
	var result int64
	var shift uint
	for i, b := range data {
		result |= int64(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, i + 1
		}
		shift += 7
	}
	return 0, len(data)
}

// parseNameSection extracts function names from the name custom section
func parseNameSection(data []byte, start, end int) map[uint32]string {
	// name section: "name" + subsection_id(1) + subsection_size + count + name_map
	names := make(map[uint32]string)
	pos := start

	for pos < end-4 {
		// Read subsection
		if pos+1 > end {
			break
		}
		subsectionID := data[pos]
		pos++
		subSize, br := readLEB128(data[pos:])
		pos += br

		if subsectionID == 1 { // function names
			count, cr := readLEB128(data[pos:])
			pos += cr

			for i := uint32(0); i < uint32(count) && pos < end; i++ {
				idx, ir := readLEB128(data[pos:])
				pos += ir
				nameLen, nr := readLEB128(data[pos:])
				pos += nr
				if pos+int(nameLen) <= end {
					names[uint32(idx)] = string(data[pos : pos+int(nameLen)])
					pos += int(nameLen)
				}
			}
		} else {
			pos += int(subSize)
		}
	}

	return names
}

func formatSizeHuman(size int64) string {
	switch {
	case size >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	case size >= 1024:
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	default:
		return fmt.Sprintf("%d B", size)
	}
}
