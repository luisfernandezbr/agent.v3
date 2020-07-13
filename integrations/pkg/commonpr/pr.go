package commonpr

import (
	"strings"

	"github.com/russross/blackfriday"
)

const extensions = blackfriday.NoIntraEmphasis |
	blackfriday.Tables |
	blackfriday.FencedCode |
	blackfriday.Autolink |
	blackfriday.Strikethrough |
	blackfriday.SpaceHeadings |
	blackfriday.NoEmptyLineBeforeBlock

func ConvertMarkdownToHTML(text string) string {
	input := strings.ReplaceAll(text, "\r", "")
	output := blackfriday.Run([]byte(input), blackfriday.WithExtensions(extensions))
	return string(output)
}
