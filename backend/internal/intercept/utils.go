package intercept

import "github.com/gtopng/backend/internal/analyzer"

var _ analyzer.Logger = (*analyzerLogger)(nil)

// analyzerLogger adapts engineLogger to the analyzer.Logger interface.
type analyzerLogger struct {
	StreamID int64
	Name     string
	Logger   engineLogger
}

func (l *analyzerLogger) Debugf(format string, args ...interface{}) {
	l.Logger.AnalyzerDebugf(l.StreamID, l.Name, format, args...)
}

func (l *analyzerLogger) Infof(format string, args ...interface{}) {
	l.Logger.AnalyzerInfof(l.StreamID, l.Name, format, args...)
}

func (l *analyzerLogger) Errorf(format string, args ...interface{}) {
	l.Logger.AnalyzerErrorf(l.StreamID, l.Name, format, args...)
}
