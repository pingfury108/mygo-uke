package asr

import "testing"

func TestLinesFrom(t *testing.T) {
	got := linesFrom([]Utterance{
		{Text: " 第一句 ", StartMS: 1000, EndMS: 2500},
		{Text: "", StartMS: 2500, EndMS: 3000},
		{Text: "第二句", StartMS: 3000, EndMS: 3000},
		{Text: "第三句", StartMS: 59000, EndMS: 61000},
	}, 60)
	if len(got) != 2 {
		t.Fatalf("lines = %#v", got)
	}
	if got[0].Start != 1 || got[0].End != 2.5 || got[0].Text != "第一句" {
		t.Fatalf("first = %#v", got[0])
	}
	if got[1].End != 60 || got[1].Text != "第三句" {
		t.Fatalf("clamped = %#v", got[1])
	}
}

func TestLinesFromKeepsLatestRevision(t *testing.T) {
	got := linesFrom([]Utterance{
		{Text: "今天。", StartMS: 0, EndMS: 500, Definite: false},
		{Text: "今天天气。", StartMS: 0, EndMS: 900, Definite: false},
		{Text: "今天天气不错。", StartMS: 0, EndMS: 1500, Definite: false},
		{Text: "今天天气不错，我们一起弹尤克里里。", StartMS: 0, EndMS: 6000, Definite: true},
	}, 10)
	if len(got) != 1 || got[0].End != 6 || got[0].Text != "今天天气不错，我们一起弹尤克里里。" {
		t.Fatalf("lines = %#v", got)
	}
}

func TestSplitLongLines(t *testing.T) {
	got := splitLongLines([]Line{{
		Start: 0,
		End:   10,
		Text:  "第一句。第二句！第三句？",
	}})
	if len(got) != 3 {
		t.Fatalf("lines = %#v", got)
	}
	if got[0].Start != 0 || got[0].End <= got[0].Start || got[2].End != 10 {
		t.Fatalf("times = %#v", got)
	}
	if got[1].Text != "第二句！" || got[2].Text != "第三句？" {
		t.Fatalf("texts = %#v", got)
	}

	long := splitLongLines([]Line{{
		Start: 0,
		End:   9,
		Text:  "这一句没有任何标点但是非常非常长需要硬切分成多行字幕",
	}})
	if len(long) < 2 {
		t.Fatalf("hard split = %#v", long)
	}
	if long[len(long)-1].End != 9 {
		t.Fatalf("last end = %v", long[len(long)-1].End)
	}
}
