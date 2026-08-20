package tools

import (
	"strings"
)

// deletionAnalysis 是删除命令检测的结果。
type deletionAnalysis struct {
	Blocked bool
	Tool    string
	Paths   []string
}

var deletionCommandNames = map[string]bool{
	// Unix / POSIX shell
	"rm": true, "rmdir": true,
	// Windows cmd
	"del": true, "erase": true, "rd": true, "deltree": true,
	// PowerShell 及其别名
	"remove-item": true, "ri": true,
	// 安全删除工具
	"shred": true, "srm": true,
}

var prefixCommandNames = map[string]bool{
	"sudo": true, "time": true, "env": true, "nice": true, "busybox": true,
}

var prefixValueOptions = map[string]map[string]bool{
	"sudo": {
		"-u": true, "-U": true, "-g": true, "-C": true, "-T": true,
		"--user": true, "--group": true, "--host": true, "--chdir": true,
		"--close-from": true, "--prompt": true,
	},
	"env":     {"-u": true, "--unset": true},
	"nice":    {"-n": true, "--adjustment": true},
	"time":    {},
	"busybox": {},
}

var shellLauncherNames = map[string]bool{
	"sh": true, "bash": true, "cmd": true, "powershell": true, "pwsh": true,
}

// commandKeywords 是 shell 关键字：其后一个 token 仍处于命令位置。
var commandKeywords = map[string]bool{
	"if": true, "then": true, "else": true, "elif": true, "fi": true,
	"while": true, "until": true, "for": true, "do": true, "done": true,
	"case": true, "esac": true, "function": true, "select": true,
	"{": true, "}": true, "!": true, "coproc": true,
}

const maxAnalyzeDepth = 16

type tokenKind int

const (
	tokenWord tokenKind = iota
	tokenOp
)

// token 是 shell 词法分析的最小单元。
type token struct {
	kind       tokenKind
	text       string
	content    string
	quoted     bool
	commandPos bool
}

// isDeletionCommand 判断命令名是否为删除命令（大小写不敏感）。
func isDeletionCommand(name string) bool {
	return deletionCommandNames[strings.ToLower(name)]
}

// isPrefixCommand 判断命令名是否为前缀命令。
func isPrefixCommand(name string) bool {
	return prefixCommandNames[strings.ToLower(name)]
}

// isShellLauncher 判断命令名是否为解释器命令。
func isShellLauncher(name string) bool {
	return shellLauncherNames[strings.ToLower(name)]
}

func isAssignment(word string) bool {
	eq := strings.IndexByte(word, '=')
	if eq <= 0 {
		return false
	}
	ident := word[:eq]
	for i, r := range ident {
		if i == 0 {
			if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
				return false
			}
			continue
		}
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// isLauncherOption 判断解释器命令后是否跟了执行参数选项（sh -c / cmd /c / powershell -Command 等）。
func isLauncherOption(launcher, opt string) bool {
	o := strings.ToLower(opt)
	switch strings.ToLower(launcher) {
	case "sh", "bash":
		return o == "-c"
	case "cmd":
		return o == "/c"
	case "powershell", "pwsh":
		return o == "-c" || o == "-command"
	}
	return false
}

// AnalyzeDeletion 检测命令字符串中的删除操作。命中时返回 Blocked=true，并尽量回显识别到的路径。
func AnalyzeDeletion(command string) deletionAnalysis {
	return analyzeDeletionInternal(command, 0)
}

func analyzeDeletionInternal(command string, depth int) deletionAnalysis {
	if depth > maxAnalyzeDepth || command == "" {
		return deletionAnalysis{}
	}

	lines := strings.Split(command, "\n")

	type heredoc struct {
		delim     string
		stripTabs bool
	}
	var active []heredoc
	var tokens []token

	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")

		if len(active) > 0 {
			trimmed := line
			if active[len(active)-1].stripTabs {
				trimmed = strings.TrimLeft(trimmed, "\t")
			}
			if strings.TrimSpace(trimmed) == active[len(active)-1].delim {
				active = active[:len(active)-1]
			}
			continue
		}

		lineTokens := tokenizeLine(line)

		for idx, tk := range lineTokens {
			if tk.kind == tokenOp && (tk.text == "<<" || tk.text == "<<-") {
				if idx+1 < len(lineTokens) {
					dt := lineTokens[idx+1]
					active = append(active, heredoc{delim: dt.content, stripTabs: tk.text == "<<-"})
				}
			}
		}

		tokens = append(tokens, lineTokens...)
		tokens = append(tokens, token{kind: tokenOp, text: "\n"})
	}

	assignCommandPositions(tokens)
	return analyzeTokens(tokens, depth)
}

// tokenizeLine 将单行命令切分为 token，识别引号、注释与操作符。
func tokenizeLine(line string) []token {
	var tokens []token
	i := 0
	n := len(line)
	atWordStart := true

	for i < n {
		ch := line[i]
		if ch == ' ' || ch == '\t' || ch == '\r' {
			atWordStart = true
			i++
			continue
		}

		if ch == '#' && atWordStart {
			break
		}

		if ch == '\'' || ch == '"' || ch == '`' {
			content, end := readQuoted(line, i)
			tokens = append(tokens, token{
				kind:    tokenWord,
				text:    line[i:end],
				content: content,
				quoted:  true,
			})
			i = end
			atWordStart = false
			continue
		}

		// 操作符
		if op, ok := readOperator(line, i); ok {
			tokens = append(tokens, token{kind: tokenOp, text: op})
			i += len(op)
			atWordStart = true
			continue
		}

		// 普通词
		start := i
		for i < n {
			c := line[i]
			if c == ' ' || c == '\t' || c == '\r' {
				break
			}
			if c == '\'' || c == '"' || c == '`' {
				break
			}
			if _, isOp := readOperator(line, i); isOp {
				break
			}
			i++
		}
		tokens = append(tokens, token{
			kind:    tokenWord,
			text:    line[start:i],
			content: line[start:i],
		})
		atWordStart = false
	}
	return tokens
}

// readQuoted 读取从 start 处引号开始的引号字符串，返回引号内内容与结束位置。
func readQuoted(line string, start int) (content string, end int) {
	quote := line[start]
	i := start + 1
	var sb strings.Builder
	for i < len(line) {
		c := line[i]
		if c == '\\' && i+1 < len(line) {
			sb.WriteByte(c)
			sb.WriteByte(line[i+1])
			i += 2
			continue
		}
		if c == quote {
			return sb.String(), i + 1
		}
		sb.WriteByte(c)
		i++
	}
	return sb.String(), len(line)
}

// readOperator 读取从 i 处开始的操作符，未命中时返回 false。
func readOperator(line string, i int) (string, bool) {
	rest := line[i:]
	for _, op := range []string{">>", "<<-", "<<", "||", "&&", ";;", ">", "<", "|", "&", ";", "(", ")"} {
		if strings.HasPrefix(rest, op) {
			return op, true
		}
	}
	return "", false
}

// assignCommandPositions 标记每个 token 是否处于「命令位置」。
func assignCommandPositions(tokens []token) {
	atCommandStart := true
	for i := range tokens {
		t := &tokens[i]
		if t.kind == tokenOp {
			switch t.text {
			case ";", ";;", "&", "|", "&&", "||", "\n", "(", "{":
				atCommandStart = true
			default:
				atCommandStart = false
			}
			continue
		}
		t.commandPos = atCommandStart
		if isAssignment(t.text) {
			// 赋值语句后下一个 token 仍处于命令位置（如 FOO=bar rm x）
			continue
		}
		if commandKeywords[strings.ToLower(t.text)] {
			atCommandStart = true
		} else {
			atCommandStart = false
		}
	}
}

// analyzeTokens 遍历 token 序列，识别删除命令并收集命中信息。
func analyzeTokens(tokens []token, depth int) deletionAnalysis {
	var res deletionAnalysis
	i := 0
	for i < len(tokens) {
		t := tokens[i]
		if t.kind == tokenOp || !t.commandPos || t.quoted || isAssignment(t.content) {
			i++
			continue
		}

		lower := strings.ToLower(t.content)

		switch {
		case isPrefixCommand(lower):
			j := i + 1
			for j < len(tokens) && tokens[j].kind == tokenWord && !tokens[j].quoted && strings.HasPrefix(tokens[j].text, "-") {
				opt := tokens[j].text
				j++
				if opt == "--" {
					break
				}
				if prefixValueOptions[lower][opt] {
					j++
				}
			}
			if j < len(tokens) {
				tokens[j].commandPos = true
			}
			i = j

		case isShellLauncher(lower):
			// sh -c "..." / cmd /c "..." / powershell -Command "..."：递归检测参数内容
			optIdx := i + 1
			if optIdx < len(tokens) && !tokens[optIdx].quoted && isLauncherOption(lower, tokens[optIdx].text) {
				argIdx := optIdx + 1
				if argIdx < len(tokens) {
					res.merge(analyzeDeletionInternal(tokens[argIdx].content, depth+1))
					i = argIdx + 1
				} else {
					i = len(tokens)
				}
			} else {
				i++
			}

		case lower == "find":
			for k := i + 1; k < len(tokens); k++ {
				tk := tokens[k]
				if tk.kind == tokenOp {
					break
				}
				if tk.quoted {
					continue
				}
				if tk.text == "-delete" {
					res.Blocked = true
					if res.Tool == "" {
						res.Tool = "find -delete"
					}
					break
				}
			}
			i++

		default:
			if isDeletionCommand(lower) {
				res.Blocked = true
				if res.Tool == "" {
					res.Tool = t.content
				}
				for k := i + 1; k < len(tokens); k++ {
					tk := tokens[k]
					if tk.kind == tokenOp {
						break
					}
					if tk.quoted || strings.HasPrefix(tk.text, "-") {
						continue
					}
					res.Paths = append(res.Paths, tk.text)
					if len(res.Paths) >= 5 {
						break
					}
				}
			}
			i++
		}
	}
	return res
}

// merge 合并递归分析结果。
func (r *deletionAnalysis) merge(o deletionAnalysis) {
	if !o.Blocked {
		return
	}
	r.Blocked = true
	if r.Tool == "" {
		r.Tool = o.Tool
	}
	r.Paths = append(r.Paths, o.Paths...)
	if len(r.Paths) > 5 {
		r.Paths = r.Paths[:5]
	}
}
