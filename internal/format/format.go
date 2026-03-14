package format

import (
	"fmt"
	"regexp"
	"strings"
)

func EscapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

func MarkdownToHTML(md string) string {
	placeholders := make(map[string]string)
	idx := 0

	// Protect fenced code blocks
	fencedRe := regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")
	md = fencedRe.ReplaceAllStringFunc(md, func(match string) string {
		m := fencedRe.FindStringSubmatch(match)
		lang := m[1]
		code := EscapeHTML(m[2])
		var tag string
		if lang != "" {
			tag = fmt.Sprintf(`<pre><code class="language-%s">%s</code></pre>`, lang, code)
		} else {
			tag = fmt.Sprintf("<pre><code>%s</code></pre>", code)
		}
		key := fmt.Sprintf("PLACEHOLDER_%d_FENCED", idx)
		idx++
		placeholders[key] = tag
		return key
	})

	// Protect inline code
	inlineRe := regexp.MustCompile("`([^`]+)`")
	md = inlineRe.ReplaceAllStringFunc(md, func(match string) string {
		m := inlineRe.FindStringSubmatch(match)
		code := EscapeHTML(m[1])
		tag := fmt.Sprintf("<code>%s</code>", code)
		key := fmt.Sprintf("PLACEHOLDER_%d_INLINE", idx)
		idx++
		placeholders[key] = tag
		return key
	})

	// Bold: **text** and __text__
	boldRe1 := regexp.MustCompile(`\*\*(.+?)\*\*`)
	md = boldRe1.ReplaceAllString(md, "<b>$1</b>")
	boldRe2 := regexp.MustCompile(`__(.+?)__`)
	md = boldRe2.ReplaceAllString(md, "<b>$1</b>")

	// Italic: *text* but not ** (already replaced)
	italicRe := regexp.MustCompile(`(?:^|[^*])\*([^*]+)\*(?:[^*]|$)`)
	md = italicRe.ReplaceAllStringFunc(md, func(match string) string {
		inner := italicRe.FindStringSubmatch(match)
		if len(inner) < 2 {
			return match
		}
		// Preserve surrounding non-* chars
		prefix := ""
		suffix := ""
		if len(match) > 0 && match[0] != '*' {
			prefix = string(match[0])
		}
		if len(match) > 0 && match[len(match)-1] != '*' {
			suffix = string(match[len(match)-1])
		}
		return prefix + "<i>" + inner[1] + "</i>" + suffix
	})

	// Links [text](url)
	linkRe := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	md = linkRe.ReplaceAllString(md, `<a href="$2">$1</a>`)

	// Restore placeholders
	for key, val := range placeholders {
		md = strings.ReplaceAll(md, key, val)
	}

	return md
}

func SplitMessage(text string, maxLen int) []string {
	if maxLen <= 0 {
		maxLen = 4096
	}
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		chunk := text[:maxLen]
		splitAt := -1

		// Prefer paragraph break
		if i := strings.LastIndex(chunk, "\n\n"); i > 0 {
			splitAt = i + 2
		} else if i := strings.LastIndex(chunk, "\n"); i > 0 {
			splitAt = i + 1
		}

		if splitAt > 0 {
			chunks = append(chunks, strings.TrimRight(text[:splitAt], "\n"))
			text = text[splitAt:]
		} else {
			// Hard split
			chunks = append(chunks, chunk)
			text = text[maxLen:]
		}
	}

	// Remove empty chunks
	result := chunks[:0]
	for _, c := range chunks {
		if strings.TrimSpace(c) != "" {
			result = append(result, c)
		}
	}
	return result
}
