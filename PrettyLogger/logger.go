package main

type ErrorType int

const (
	ErrorNetwork ErrorType = iota
	ErrorDatabase
	ErrorProtocol
	ErrorInternal
	ErrorNeedsRetry
)

type PrettyLogger struct{}
