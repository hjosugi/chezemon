package state

import "time"

type Snapshot struct {
	GeneratedAt    time.Time `json:"generatedAt"`
	DurationMS     int64     `json:"durationMs"`
	ChezmoiVersion string    `json:"chezmoiVersion"`
	SourceDir      string    `json:"sourceDir"`
	DestDir        string    `json:"destDir"`
	ReadOnly       bool      `json:"readOnly"`
	Entries        []Entry   `json:"entries"`
	Counts         Counts    `json:"counts"`
	Git            GitState  `json:"git"`
	Workflow       Workflow  `json:"workflow"`
	Notices        []Notice  `json:"notices"`
}

type Entry struct {
	Code              string `json:"code"`
	Path              string `json:"path"`
	DisplayPath       string `json:"displayPath"`
	SourcePath        string `json:"sourcePath,omitempty"`
	Kind              string `json:"kind"`
	Label             string `json:"label"`
	Risk              string `json:"risk"`
	Explanation       string `json:"explanation"`
	RecommendedAction string `json:"recommendedAction"`
	ScriptTiming      string `json:"scriptTiming,omitempty"`
	Sensitive         bool   `json:"sensitive"`
	SensitiveReason   string `json:"sensitiveReason,omitempty"`
}

type Counts struct {
	Total         int `json:"total"`
	Critical      int `json:"critical"`
	High          int `json:"high"`
	Pending       int `json:"pending"`
	Scripts       int `json:"scripts"`
	LocalChanges  int `json:"localChanges"`
	PendingCreate int `json:"pendingCreate"`
	PendingModify int `json:"pendingModify"`
	PendingDelete int `json:"pendingDelete"`
}

type GitState struct {
	Available   bool        `json:"available"`
	Clean       bool        `json:"clean"`
	Branch      string      `json:"branch"`
	Ahead       int         `json:"ahead"`
	Behind      int         `json:"behind"`
	HasUpstream bool        `json:"hasUpstream"`
	Changes     []GitChange `json:"changes"`
	Commits     []GitCommit `json:"commits"`
	Error       string      `json:"error,omitempty"`
}

type GitChange struct {
	Code string `json:"code"`
	Path string `json:"path"`
}

type GitCommit struct {
	Hash    string `json:"hash"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

type Notice struct {
	Level   string `json:"level"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

type Workflow struct {
	Phase      string `json:"phase"`
	PhaseLabel string `json:"phaseLabel"`
	Summary    string `json:"summary"`
	// Clear counts stages with nothing outstanding. It is not a measure of
	// work the reader has done — see stepClear.
	Clear int `json:"clear"`
	Total int `json:"total"`
	// Outstanding is the number of items still queued across all stages.
	Outstanding int            `json:"outstanding"`
	CurrentStep int            `json:"currentStep"`
	Steps       []WorkflowStep `json:"steps"`
}

type WorkflowStep struct {
	Number      int    `json:"number"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	ShortTitle  string `json:"shortTitle"`
	Description string `json:"description"`
	State       string `json:"state"`
	Count       int    `json:"count"`
	QueueFilter string `json:"queueFilter,omitempty"`
}

type Diff struct {
	Path       string `json:"path"`
	SourcePath string `json:"sourcePath,omitempty"`
	Content    string `json:"content,omitempty"`
	Sensitive  bool   `json:"sensitive"`
	Revealed   bool   `json:"revealed"`
	Message    string `json:"message,omitempty"`
}

type DoctorResult struct {
	Output string `json:"output"`
	OK     bool   `json:"ok"`
}
