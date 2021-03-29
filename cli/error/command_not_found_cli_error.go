package error

type CommandNotFoundError struct {
	basicCliError
}

//--------------------

func NewCommandNotFoundError() *CommandNotFoundError {
	return &CommandNotFoundError{
		basicCliError{
			status:  2,
			message: "command not found",
		},
	}
}
