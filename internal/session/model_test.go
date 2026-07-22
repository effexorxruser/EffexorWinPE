package session

import (
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
)

func TestSessionRecordsContextWithoutDuplicatingSensitiveTextInEvents(t *testing.T) {
	now := time.Unix(100, 0)
	value, err := New("report-1", now)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := value.AddSymptom("Automatic repair loop after an update", now.Add(time.Second)); err != nil {
		t.Fatalf("AddSymptom() error = %v", err)
	}
	if err := value.RecordAnswer("client-data-backed-up", "no", diagnosis.AnswerYesNo, now.Add(2*time.Second)); err != nil {
		t.Fatalf("RecordAnswer() error = %v", err)
	}
	if len(value.Symptoms) != 1 || len(value.Answers) != 1 {
		t.Fatalf("unexpected context: %+v", value)
	}
	for _, event := range value.Events {
		if event.Reference == value.Symptoms[0].Text {
			t.Fatal("event log duplicated symptom free text")
		}
	}
}

func TestSessionRejectsMismatchedReportAndInvalidYesNoAnswer(t *testing.T) {
	value, err := New("report-1", time.Now())
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := value.Validate("report-2"); err == nil {
		t.Fatal("Validate() error = nil, want report mismatch")
	}
	if err := value.RecordAnswer("question", "maybe", diagnosis.AnswerYesNo, time.Now()); err == nil {
		t.Fatal("RecordAnswer() error = nil, want yes/no validation error")
	}
}
