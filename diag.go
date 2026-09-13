package vcx

import (
	"fmt"
	"strings"

	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/token"
)

// A Diagnostic is one report, sited in the file the user wrote.
type Diagnostic struct {
	Severity token.Severity
	Site     preprocessor.Site
	Message  string
	Name     string
	Notes    []Note
}

// A Note is a secondary position attached below a diagnostic.
type Note struct {
	Site preprocessor.Site
	Msg  string
}

// String renders the diagnostic as file:line:col: severity: message.
func (d Diagnostic) String() string {
	name := ""
	if d.Name != "" {
		name = " [" + d.Name + "]"
	}
	return fmt.Sprintf("%s: %s: %s%s", SiteString(d.Site), d.Severity, d.Message, name)
}

// SiteString renders one site as file:line:col.
func SiteString(s preprocessor.Site) string {
	if s.Origin == nil {
		return "<unknown>"
	}
	if s.Origin.File == nil {
		return s.Origin.Name()
	}
	p := s.Origin.File.Position(s.Pos)
	return fmt.Sprintf("%s:%d:%d", s.Origin.Name(), p.Line, p.Column)
}

// HasErrors reports whether any diagnostic is an error.
func HasErrors(diags []Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == token.Error {
			return true
		}
	}
	return false
}

// A DiagnosticError is the error a compile returns when the program is invalid.
type DiagnosticError struct {
	Diagnostics []Diagnostic
}

func (e *DiagnosticError) Error() string {
	var b strings.Builder
	n := 0
	for _, d := range e.Diagnostics {
		if d.Severity != token.Error {
			continue
		}
		if n > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(d.String())
		n++
	}
	if n == 0 {
		return "compilation failed"
	}
	return b.String()
}
