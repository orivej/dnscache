package main

import (
	"fmt"
	"os"
	"path"
	"runtime"
)

func context() string {
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		return ""
	}
	fn := runtime.FuncForPC(pc)
	return fmt.Sprintf("%s (%s:%d)", fn.Name(), path.Base(file), line)
}

func eexit(e error) {
	if e != nil {
		fmt.Printf("%v: %v\n", context(), e)
		os.Exit(1)
	}
}

func eprint(e error) bool {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
	}
	return e != nil
}
