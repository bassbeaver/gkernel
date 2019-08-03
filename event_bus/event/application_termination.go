package event

type ApplicationTermination struct {
	Propagator
	containerAccessor
	Errors *[]error
}

//--------------------

func NewApplicationTermination(containerAccessorObj containerAccessor, terminationErrors *[]error) *ApplicationTermination {
	return &ApplicationTermination{
		containerAccessor: containerAccessorObj,
		Errors:            terminationErrors,
	}
}
