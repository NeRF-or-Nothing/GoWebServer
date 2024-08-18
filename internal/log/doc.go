// Package log contains the Logger used by the entire application. The Logger is a wrapper around zap.SugaredLogger
// zap was chosen as the logging library because it *very* fast and has a simple API. 
// There should be a single instance of the Logger in the application, and it should be injected into any structs that need to log.
package log