package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
)

const SchemaVersion = "0.2.0"

const (
	EventSessionStarted           = "session_started"
	EventSymptomRecorded          = "symptom_recorded"
	EventQuestionAnswered         = "question_answered"
	EventOfflineAssessmentCreated = "offline_assessment_created"
	EventGatewaySubmitted         = "gateway_submitted"
	EventOnlineAssessmentReceived = "online_assessment_received"
)

// Session keeps technician-provided context beside the immutable collector
// report. Free text remains local unless the technician explicitly approves an
// online gateway request.
type Session struct {
	SchemaVersion    string                `json:"schema_version"`
	SessionID        string                `json:"session_id"`
	ReportID         string                `json:"report_id"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
	Symptoms         []Symptom             `json:"symptoms"`
	Answers          []Answer              `json:"answers"`
	Events           []Event               `json:"events"`
	LatestAssessment *diagnosis.Assessment `json:"latest_assessment,omitempty"`
}

type Symptom struct {
	ID         string    `json:"id"`
	Text       string    `json:"text"`
	RecordedAt time.Time `json:"recorded_at"`
}

type Answer struct {
	QuestionID string    `json:"question_id"`
	Value      string    `json:"value"`
	RecordedAt time.Time `json:"recorded_at"`
}

type Event struct {
	At        time.Time `json:"at"`
	Kind      string    `json:"kind"`
	Reference string    `json:"reference,omitempty"`
}

func New(reportID string, now time.Time) (Session, error) {
	if strings.TrimSpace(reportID) == "" {
		return Session{}, fmt.Errorf("report_id is required")
	}
	id, err := randomID("session")
	if err != nil {
		return Session{}, fmt.Errorf("create session id: %w", err)
	}
	now = now.UTC()
	return Session{
		SchemaVersion: SchemaVersion,
		SessionID:     id,
		ReportID:      reportID,
		CreatedAt:     now,
		UpdatedAt:     now,
		Symptoms:      []Symptom{},
		Answers:       []Answer{},
		Events:        []Event{{At: now, Kind: EventSessionStarted}},
	}, nil
}

func (s *Session) Validate(reportID string) error {
	if s.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported session schema %q; expected %q", s.SchemaVersion, SchemaVersion)
	}
	if s.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}
	if s.ReportID != reportID {
		return fmt.Errorf("session belongs to report %q, not %q", s.ReportID, reportID)
	}
	return nil
}

func (s *Session) AddSymptom(text string, now time.Time) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("symptom text is empty")
	}
	if len([]rune(text)) > 2000 {
		return fmt.Errorf("symptom text exceeds 2000 characters")
	}
	id, err := randomID("symptom")
	if err != nil {
		return fmt.Errorf("create symptom id: %w", err)
	}
	now = now.UTC()
	s.Symptoms = append(s.Symptoms, Symptom{ID: id, Text: text, RecordedAt: now})
	s.touch(now, EventSymptomRecorded, id)
	return nil
}

func (s *Session) RecordAnswer(questionID, value string, answerType string, now time.Time) error {
	questionID = strings.TrimSpace(questionID)
	value = strings.TrimSpace(value)
	if questionID == "" || value == "" {
		return fmt.Errorf("question id and answer value are required")
	}
	if len([]rune(value)) > 2000 {
		return fmt.Errorf("answer exceeds 2000 characters")
	}
	if answerType == diagnosis.AnswerYesNo {
		value = strings.ToLower(value)
		if value != "yes" && value != "no" && value != "unknown" {
			return fmt.Errorf("answer to %q must be yes, no, or unknown", questionID)
		}
	}
	now = now.UTC()
	for index := range s.Answers {
		if s.Answers[index].QuestionID == questionID {
			s.Answers[index].Value = value
			s.Answers[index].RecordedAt = now
			s.touch(now, EventQuestionAnswered, questionID)
			return nil
		}
	}
	s.Answers = append(s.Answers, Answer{QuestionID: questionID, Value: value, RecordedAt: now})
	s.touch(now, EventQuestionAnswered, questionID)
	return nil
}

func (s *Session) HasAnswer(questionID string) bool {
	for _, answer := range s.Answers {
		if answer.QuestionID == questionID {
			return true
		}
	}
	return false
}

func (s *Session) SetAssessment(assessment diagnosis.Assessment, eventKind string, now time.Time) {
	s.LatestAssessment = &assessment
	s.touch(now.UTC(), eventKind, assessment.Mode)
}

func (s *Session) AddEvent(kind, reference string, now time.Time) {
	s.touch(now.UTC(), kind, reference)
}

func (s *Session) touch(now time.Time, kind, reference string) {
	s.UpdatedAt = now
	s.Events = append(s.Events, Event{At: now, Kind: kind, Reference: reference})
}

func randomID(prefix string) (string, error) {
	var value [12]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return prefix + "-" + hex.EncodeToString(value[:]), nil
}
