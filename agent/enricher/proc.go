package enricher

func GetProcessName(pid int) (string, error) {
	return "dummy_proc", nil
}

func GetRemoteAddr(fd int) (string, error) {
	return "127.0.0.1:8080", nil
}
