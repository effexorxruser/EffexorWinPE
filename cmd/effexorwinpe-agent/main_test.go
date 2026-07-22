package main

import (
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

func TestUnansweredQuestionsFiltersRecordedAnswers(t *testing.T) {
	value, err := session.New("report-1", time.Now())
	if err != nil {
		t.Fatalf("session.New() error = %v", err)
	}
	if err := value.RecordAnswer("backup", "no", diagnosis.AnswerYesNo, time.Now()); err != nil {
		t.Fatalf("RecordAnswer() error = %v", err)
	}
	questions := []diagnosis.Question{
		{ID: "backup", AnswerType: diagnosis.AnswerYesNo},
		{ID: "symptom", AnswerType: diagnosis.AnswerFreeText},
	}
	pending := unansweredQuestions(value, questions)
	if len(pending) != 1 || pending[0].ID != "symptom" {
		t.Fatalf("unansweredQuestions() = %+v", pending)
	}
}

func TestNormalizeAnswerSupportsRussianYesNoInput(t *testing.T) {
	if normalizeAnswer(" да ") != "yes" || normalizeAnswer("НЕТ") != "no" || normalizeAnswer("не знаю") != "unknown" {
		t.Fatal("normalizeAnswer() did not normalize Russian yes/no input")
	}
}
