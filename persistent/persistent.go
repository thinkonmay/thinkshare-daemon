package persistent





type Persistent interface {
	Initialize(username string, password string)

	Log(level string, log string)
	// Log(level string, log string)
}