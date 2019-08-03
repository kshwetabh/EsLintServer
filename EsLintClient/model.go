package main

// Config object for various configuration information
type Config struct {
	WorkspacePath   string // location of workspace directory
	EslintServerURL string // Address of the NodeJS server component
}

// ESLintError struct
type ESLintError struct {
	FilePath            string    `json:"filePath"`
	Messages            []Message `json:"messages"`
	ErrorCount          int       `json:"errorCount"`
	WarningCount        int       `json:"warningCount"`
	FixableErrorCount   int       `json:"fixableErrorCount"`
	FixableWarningCount int       `json:"fixableWarningCount"`
}

// Message struct
type Message struct {
	RuleID    string `json:"ruleId"`
	Severity  int    `json:"severity"`
	Message   string `json:"message"`
	Line      int    `json:"line"`
	Column    int    `json:"column"`
	NodeType  string `json:"nodeType"`
	EndLine   int    `json:"endLine,omitempty"`
	EndColumn int    `json:"endColumn,omitempty"`
	MessageID string `json:"messageId,omitempty"`
	Fix       struct {
		Range []int  `json:"range"`
		Text  string `json:"text"`
	} `json:"fix,omitempty"`
	ESLintErrorID uint
}
