package api

import (
	"testing"

	"github.com/pinpt/agent/integrations/pkg/commonpr"
)

func TestConvertMarkdownToHTML(t *testing.T) {

	text := `This is the first merge request test

- elemento 1 ✍️
- elemento 2
- ` + "`#F00`" + `
- ` + "`#F00A`" + `
- ` + "`#FF0000`" + `
- ` + "`#FF0000AA`" + `
`

	expected := `<p>This is the first merge request test</p>

<ul>
<li>elemento 1 ✍️</li>
<li>elemento 2</li>
<li><code>#F00</code></li>
<li><code>#F00A</code></li>
<li><code>#FF0000</code></li>
<li><code>#FF0000AA</code></li>
</ul>
`

	actual := commonpr.ConvertMarkdownToHTML(text)

	if expected != actual {
		t.Errorf("wanted %+q, got %+q", expected, actual)
	}

	text = `simple text`

	expected = `<p>simple text</p>
`
	actual = commonpr.ConvertMarkdownToHTML(text)

	if expected != actual {
		t.Errorf("wanted %+q, got %+q", expected, actual)
	}

	text = "\r\n- item 1\r\n\r\nparagraph\r\n\r\n- item 2"
	expected = `<ul>
<li>item 1</li>
</ul>

<p>paragraph</p>

<ul>
<li>item 2</li>
</ul>
`

	actual = commonpr.ConvertMarkdownToHTML(text)

	if expected != actual {
		t.Errorf("wanted %+q, got %+q", expected, actual)
	}

}
