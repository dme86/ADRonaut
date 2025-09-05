package app

/* ------------------------------ Messages --------------------------------- */

type badge struct {
	Label string
	Count int
}

type saveDoneMsg struct {
	path string
	err  error
}

type autosaveTickMsg struct{}
type autosaveDoneMsg struct {
	path string
	err  error
}

type gitInfoLoadedMsg struct {
	name       string
	email      string
	signingKey string
}
