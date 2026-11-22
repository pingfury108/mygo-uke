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

func TestMergeUtterances(t *testing.T) {
	a := Utterance{Text: "第一句", StartMS: 0, EndMS: 1000}
	b := Utterance{Text: "第二句", StartMS: 1000, EndMS: 2000}
	got := mergeUtterances([]Utterance{a}, []Utterance{a, b})
	if len(got) != 2 || got[1].Text != "第二句" {
		t.Fatalf("merged = %#v", got)
	}
}
