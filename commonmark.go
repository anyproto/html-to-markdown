package md

import (
	"fmt"

	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode"

	"github.com/JohannesKaufmann/html-to-markdown/escape"
	"github.com/PuerkitoBio/goquery"
)

var multipleSpacesR = regexp.MustCompile(`  +`)

var commonmark = []Rule{
	{
		Filter: []string{"ul", "ol"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			parent := selec.Parent()
			lastContentTextNode := strings.TrimRight(parent.Nodes[0].FirstChild.Data, " \t")
			parent.Nodes[0].FirstChild.Data = lastContentTextNode

			if parent.Is("li") && parent.Children().Last().IsSelection(selec) {
				lastContentTextNode := strings.TrimRight(parent.Nodes[0].FirstChild.Data, " \t")
				if !strings.HasSuffix(lastContentTextNode, "\n") {
					// add a line break prefix if the parent's text node doesn't have it
					content = "\n" + content
				}

				// remove
				trimmedSpaceContent := strings.TrimRight(content, " \t")
				if strings.HasSuffix(trimmedSpaceContent, "\n") {
					content = strings.TrimRightFunc(content, unicode.IsSpace)
				}

				// panic("ul&li -> parent is li & something")
			} else {
				content = "\n\n" + content + "\n\n"
			}
			return &content
		},
	},
	{
		Filter: []string{"li"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			if strings.TrimSpace(content) == "" {
				return nil
			}

			parent := selec.Parent()
			index := selec.Index()

			var prefix string
			if parent.Is("ol") {
				prefix = strconv.Itoa(index+1) + ". "
			} else {
				prefix = opt.BulletListMarker + " "
			}
			// remove leading newlines
			content = leadingNewlinesR.ReplaceAllString(content, "")
			// replace trailing newlines with just a single one
			content = trailingNewlinesR.ReplaceAllString(content, "\n")
			// indent
			content = indentR.ReplaceAllString(content, "\n    ")

			return String(prefix + content + "\n")
		},
	},
	{
		Filter: []string{"#text"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			text := selec.Text()
			if trimmed := strings.TrimSpace(text); trimmed == "" {
				return String("")
			}
			text = tabR.ReplaceAllString(text, " ")

			// replace multiple spaces by one space: dont accidentally make
			// normal text be indented and thus be a code block.
			text = multipleSpacesR.ReplaceAllString(text, " ")

			if !opt.DisableEscaping {
				text = escape.MarkdownCharacters(text)
			}

			return &text
		},
	},
	{
		Filter: []string{"p", "div"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			parent := goquery.NodeName(selec.Parent())
			if IsInlineElement(parent) || parent == "li" {
				content = "\n" + content + "\n"
				return &content
			}

			// remove unnecessary spaces to have clean markdown
			content = TrimpLeadingSpaces(content)

			content = "\n\n" + content + "\n\n"
			return &content
		},
	},
	{
		Filter: []string{"h1", "h2", "h3", "h4", "h5", "h6"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			if strings.TrimSpace(content) == "" {
				return nil
			}

			content = strings.Replace(content, "\n", " ", -1)
			content = strings.Replace(content, "\r", " ", -1)
			content = strings.Replace(content, `#`, `\#`, -1)

			insideLink := selec.ParentsFiltered("a").Length() > 0
			if insideLink {
				text := opt.StrongDelimiter + content + opt.StrongDelimiter
				return &text
			}

			node := goquery.NodeName(selec)
			level, err := strconv.Atoi(node[1:])
			if err != nil {
				return nil
			}

			if opt.HeadingStyle == "setext" && level < 3 {
				line := "-"
				if level == 1 {
					line = "="
				}

				underline := strings.Repeat(line, len(content))
				return String("\n\n" + content + "\n" + underline + "\n\n")
			}

			prefix := strings.Repeat("#", level)
			content = strings.TrimSpace(content)
			text := "\n\n" + prefix + " " + content + "\n\n"
			return &text
		},
	},
	{
		Filter: []string{"strong", "b"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			rightTrimmed := strings.TrimRightFunc(content, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
			rightExtra := content[len(rightTrimmed):]

			trimmed := strings.TrimLeftFunc(rightTrimmed, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
			leftExtra := content[0 : len(rightTrimmed)-len(trimmed)]

			if strings.TrimSpace(trimmed) == "" {
				trimmed = leftExtra + rightExtra
				return &trimmed
			}

			trimmed = leftExtra + opt.StrongDelimiter + trimmed + opt.StrongDelimiter + rightExtra

			return &trimmed
		},
	},
	{
		Filter: []string{"i", "em"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			rightTrimmed := strings.TrimRightFunc(content, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
			rightExtra := content[len(rightTrimmed):]

			trimmed := strings.TrimLeftFunc(rightTrimmed, func(r rune) bool { return unicode.IsPunct(r) || unicode.IsSpace(r) })
			leftExtra := content[0 : len(rightTrimmed)-len(trimmed)]

			if strings.TrimSpace(trimmed) == "" {
				trimmed = leftExtra + rightExtra
				return &trimmed
			}

			trimmed = leftExtra + opt.EmDelimiter + trimmed + opt.EmDelimiter + rightExtra

			return &trimmed
		},
	},
	{
		Filter: []string{"img"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			src := selec.AttrOr("src", "")
			src = strings.TrimSpace(src)
			if src == "" {
				return String("")
			}

			src = opt.GetAbsoluteURL(selec, src, opt.domain)

			alt := selec.AttrOr("alt", "")
			alt = strings.Replace(alt, "\n", " ", -1)

			text := fmt.Sprintf("![%s](%s)", alt, src)
			return &text
		},
	},
	{
		Filter: []string{"a"},
		AdvancedReplacement: func(content string, selec *goquery.Selection, opt *Options) (AdvancedResult, bool) {
			// if there is no href, no link is used. So just return the content inside the link
			href, ok := selec.Attr("href")
			if !ok || strings.TrimSpace(href) == "" || strings.TrimSpace(href) == "#" {
				return AdvancedResult{
					Markdown: content,
				}, false
			}

			href = opt.GetAbsoluteURL(selec, href, opt.domain)

			// having multiline content inside a link is a bit tricky
			content = EscapeMultiLine(content)

			var title string
			if t, ok := selec.Attr("title"); ok {
				t = strings.Replace(t, "\n", " ", -1)
				title = fmt.Sprintf(` "%s"`, t)
			}

			// if there is no link content (for example because it contains an svg)
			// the 'title' or 'aria-label' attribute is used instead.
			if strings.TrimSpace(content) == "" {
				content = selec.AttrOr("title", selec.AttrOr("aria-label", ""))
			}

			// a link without text won't de displayed anyway
			if content == "" {
				return AdvancedResult{}, true
			}

			if opt.LinkStyle == "inlined" {
				md := fmt.Sprintf("[%s](%s%s)", content, href, title)
				md = AddSpaceIfNessesary(selec, md)

				return AdvancedResult{
					Markdown: md,
				}, false
			}

			var replacement string
			var reference string

			switch opt.LinkReferenceStyle {
			case "collapsed":

				replacement = "[" + content + "][]"
				reference = "[" + content + "]: " + href + title
			case "shortcut":
				replacement = "[" + content + "]"
				reference = "[" + content + "]: " + href + title

			default:
				id := selec.AttrOr("data-index", "")
				replacement = "[" + content + "][" + id + "]"
				reference = "[" + id + "]: " + href + title
			}

			replacement = AddSpaceIfNessesary(selec, replacement)
			return AdvancedResult{Markdown: replacement, Footer: reference}, false
		},
	},
	{
		Filter: []string{"code"},
		Replacement: func(_ string, selec *goquery.Selection, opt *Options) *string {
			code := getHTML(selec)

			// Newlines in the text aren't great, since this is inline code and not a code block.
			// Newlines will be stripped anyway in the browser, but it won't be recognized as code
			// from the markdown parser when there is more than one newline.
			// So limit to
			code = multipleNewLinesRegex.ReplaceAllString(code, "\n")

			fenceChar := '`'
			maxCount := calculateCodeFenceOccurrences(fenceChar, code)
			maxCount++

			fence := strings.Repeat(string(fenceChar), maxCount)

			// code block contains a backtick as first character
			if strings.HasPrefix(code, "`") {
				code = " " + code
			}
			// code block contains a backtick as last character
			if strings.HasSuffix(code, "`") {
				code = code + " "
			}

			// TODO: configure delimeter in options?
			text := fence + code + fence
			text = AddSpaceIfNessesary(selec, text)
			return &text
		},
	},
	{
		Filter: []string{"pre"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			codeElement := selec.Find("code")
			language := codeElement.AttrOr("class", "")
			language = strings.Replace(language, "language-", "", 1)

			code := getHTML(codeElement)
			if codeElement.Length() == 0 {
				code = getHTML(selec)
			}

			fenceChar, _ := utf8.DecodeRuneInString(opt.Fence)
			fence := CalculateCodeFence(fenceChar, code)

			text := "\n\n" + fence + language + "\n" +
				code +
				"\n" + fence + "\n\n"
			return &text
		},
	},
	{
		Filter: []string{"hr"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			text := "\n\n" + opt.HorizontalRule + "\n\n"
			return &text
		},
	},
	{
		Filter: []string{"br"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			return String("\n\n")
		},
	},
	{
		Filter: []string{"blockquote"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			content = strings.TrimSpace(content)
			if content == "" {
				return nil
			}

			content = multipleNewLinesRegex.ReplaceAllString(content, "\n\n")

			text := "\n\n" + content + "\n\n"
			return &text
		},
	},
	{
		Filter: []string{"noscript"},
		Replacement: func(content string, selec *goquery.Selection, opt *Options) *string {
			// for now remove the contents of noscript. But in the future we could
			// tell goquery to parse the contents of the tag.
			// -> https://github.com/PuerkitoBio/goquery/issues/139#issuecomment-517526070
			return nil
		},
	},
}
