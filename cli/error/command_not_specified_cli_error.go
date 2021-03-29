package error

type CommandNotSpecifiedError struct {
	basicCliError
}

//--------------------

func NewCommandNotSpecifiedError() *CommandNotSpecifiedError {
	return &CommandNotSpecifiedError{
		basicCliError{
			status:  1,
			message: "command not found",
		},
	}
}
