// Package errclass builds sentinel errors that belong to a broader class:
// the message stays clean, errors.Is matches both the sentinel and its class
// through Unwrap.
package errclass

func New(msg string, class error) error {
	return tagged{msg: msg, class: class}
}

type tagged struct {
	msg   string
	class error
}

func (e tagged) Error() string { return e.msg }

func (e tagged) Unwrap() error { return e.class }
