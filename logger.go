package tinyhiera

type Logger interface {
	Debug(msg string, args ...any)
}
