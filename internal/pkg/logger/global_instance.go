package logger

import "sync"

var gl Logger = &NopLogger{}
var LMutex sync.RWMutex

func SetGlobalLogger(l Logger) {
	LMutex.Lock()
	defer LMutex.Unlock()
	gl = l
}

func L() Logger {
	LMutex.RLock()
	defer LMutex.RUnlock()
	return gl
}
